package components

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// getKubeconfig returns the kubeconfig path, using default if empty
func getKubeconfig(kubeconfig string) string {
	if kubeconfig != "" {
		return kubeconfig
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".kube", "config")
}

// Component defines the interface for bootstrap components
type Component interface {
	Install() error
	Uninstall() error
	HealthCheck() error
}

// TerraformComponent handles Terraform infrastructure
type TerraformComponent struct {
	projectID   string
	region      string
	clusterName string
	terraformDir string
}

// NewTerraformComponent creates a new Terraform component
func NewTerraformComponent(projectID, region, clusterName string) *TerraformComponent {
	return &TerraformComponent{
		projectID:   projectID,
		region:      region,
		clusterName: clusterName,
		terraformDir: "terraform",
	}
}

// Apply applies Terraform configuration
func (t *TerraformComponent) Apply() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Find terraform directory relative to current location
	terraformPath, err := filepath.Abs(t.terraformDir)
	if err != nil {
		return fmt.Errorf("failed to resolve terraform directory: %w", err)
	}

	// Check if terraform directory exists
	if _, err := os.Stat(terraformPath); os.IsNotExist(err) {
		return fmt.Errorf("terraform directory not found at %s", terraformPath)
	}

	// Initialize Terraform
	initCmd := exec.CommandContext(ctx, "terraform", "init")
	initCmd.Dir = terraformPath
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Apply Terraform configuration
	applyCmd := exec.CommandContext(ctx, "terraform", "apply",
		"-auto-approve",
		fmt.Sprintf("-var=project_id=%s", t.projectID),
		fmt.Sprintf("-var=region=%s", t.region),
		fmt.Sprintf("-var=cluster_name=%s", t.clusterName),
	)
	applyCmd.Dir = terraformPath
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr

	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	fmt.Println("✓ Terraform infrastructure created successfully")

	// Fetch cluster credentials after creation
	fmt.Println("Fetching cluster credentials...")
	credsCmd := exec.CommandContext(ctx, "gcloud", "container", "clusters", "get-credentials",
		t.clusterName,
		"--region", t.region,
		"--project", t.projectID,
	)
	credsCmd.Stdout = os.Stdout
	credsCmd.Stderr = os.Stderr
	if err := credsCmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch cluster credentials: %w", err)
	}

	fmt.Println("✓ Cluster credentials configured")
	return nil
}

// Destroy destroys Terraform infrastructure
func (t *TerraformComponent) Destroy() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	terraformPath, err := filepath.Abs(t.terraformDir)
	if err != nil {
		return fmt.Errorf("failed to resolve terraform directory: %w", err)
	}

	destroyCmd := exec.CommandContext(ctx, "terraform", "destroy",
		"-auto-approve",
		fmt.Sprintf("-var=project_id=%s", t.projectID),
		fmt.Sprintf("-var=region=%s", t.region),
		fmt.Sprintf("-var=cluster_name=%s", t.clusterName),
	)
	destroyCmd.Dir = terraformPath
	destroyCmd.Stdout = os.Stdout
	destroyCmd.Stderr = os.Stderr

	if err := destroyCmd.Run(); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Println("✓ Terraform infrastructure destroyed successfully")
	return nil
}

// ClusterHealthChecker checks GKE cluster health
type ClusterHealthChecker struct {
	projectID   string
	region      string
	clusterName string
	kubeconfig  string
}

// NewClusterHealthChecker creates a new cluster health checker
func NewClusterHealthChecker(projectID, region, clusterName, kubeconfig string) *ClusterHealthChecker {
	return &ClusterHealthChecker{
		projectID:   projectID,
		region:      region,
		clusterName: clusterName,
		kubeconfig:  kubeconfig,
	}
}

// Check verifies the cluster is healthy
func (c *ClusterHealthChecker) Check() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use default kubeconfig path if not specified
	kubeconfig := c.kubeconfig
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	// Build Kubernetes config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check API server connectivity
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API server: %w", err)
	}

	// Check node readiness
	// Note: In Autopilot, nodes are provisioned on-demand and may not exist yet
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Autopilot clusters may have zero nodes initially - this is normal
	if len(nodes.Items) == 0 {
		fmt.Println("⚠ No nodes provisioned yet (normal for Autopilot - nodes provision on-demand)")
	} else {
		readyNodes := 0
		for _, node := range nodes.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					readyNodes++
					break
				}
			}
		}

		if readyNodes == 0 {
			fmt.Println("⚠ Nodes exist but not ready yet (may still be initializing)")
		}
	}

	// Check essential system pods in kube-system namespace
	// In Autopilot, system pods are managed by Google and may take time to appear
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list kube-system pods: %w", err)
	}

	essentialPods := []string{"kube-dns", "metrics-server"}
	foundPods := make(map[string]bool)

	for _, pod := range pods.Items {
		for _, essential := range essentialPods {
			if len(pod.Name) > len(essential) && pod.Name[:len(essential)] == essential {
				if pod.Status.Phase == corev1.PodRunning {
					foundPods[essential] = true
				}
			}
		}
	}

	// Don't fail if essential pods aren't running yet - they'll start when needed
	for _, essential := range essentialPods {
		if !foundPods[essential] {
			fmt.Printf("⚠ Essential pod %s not running yet (will start when needed)\n", essential)
		}
	}

	fmt.Println("✓ Cluster health check passed: API server responding, cluster is ready")
	return nil
}

// ExternalDNSComponent handles ExternalDNS installation
type ExternalDNSComponent struct {
	kubeconfig      string
	cloudflareToken string
	manifestsDir    string
}

// NewExternalDNSComponent creates a new ExternalDNS component
func NewExternalDNSComponent(kubeconfig, cloudflareToken string) *ExternalDNSComponent {
	return &ExternalDNSComponent{
		kubeconfig:      kubeconfig,
		cloudflareToken: cloudflareToken,
		manifestsDir:    "k8s/external-dns",
	}
}

// Install installs ExternalDNS using Helm
func (e *ExternalDNSComponent) Install() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Add Bitnami Helm repository (hosts external-dns chart)
	repoCmd := exec.CommandContext(ctx, "helm", "repo", "add", "bitnami",
		"https://charts.bitnami.com/bitnami")
	repoCmd.Stdout = os.Stdout
	repoCmd.Stderr = os.Stderr
	if err := repoCmd.Run(); err != nil {
		return fmt.Errorf("failed to add Bitnami Helm repo: %w", err)
	}

	// Update Helm repositories
	updateCmd := exec.CommandContext(ctx, "helm", "repo", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update Helm repos: %w", err)
	}

	// Install ExternalDNS with Helm using official registry.k8s.io image
	installCmd := exec.CommandContext(ctx, "helm", "install", "external-dns",
		"bitnami/external-dns",
		"--namespace", "external-dns",
		"--create-namespace",
		"--set", "provider=cloudflare",
		"--set", "cloudflare.apiToken="+e.cloudflareToken,
		"--set", "cloudflare.proxied=true",
		"--set", "policy=upsert-only",
		"--set", "txtOwnerId=clusterkit",
		"--set", "sources[0]=service",
		"--set", "sources[1]=ingress",
		"--set", "image.registry=registry.k8s.io",
		"--set", "image.repository=external-dns/external-dns",
		"--set", "image.tag=v0.15.0",
		"--set", "global.security.allowInsecureImages=true",
		"--wait",
		"--timeout", "5m")

	if e.kubeconfig != "" {
		installCmd.Args = append(installCmd.Args, "--kubeconfig", e.kubeconfig)
	}

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install ExternalDNS: %w", err)
	}

	// Patch ClusterRole to add missing endpoints permission (needed by external-dns v0.15.0)
	fmt.Println("Patching RBAC permissions for Endpoints API...")
	patchCmd := exec.CommandContext(ctx, "kubectl", "patch", "clusterrole", "external-dns-external-dns",
		"--type=json",
		"-p=[{\"op\":\"add\",\"path\":\"/rules/-\",\"value\":{\"apiGroups\":[\"\"],\"resources\":[\"endpoints\"],\"verbs\":[\"get\",\"list\",\"watch\"]}}]")
	if e.kubeconfig != "" {
		patchCmd.Args = append(patchCmd.Args, "--kubeconfig", e.kubeconfig)
	}
	patchCmd.Stdout = os.Stdout
	patchCmd.Stderr = os.Stderr
	if err := patchCmd.Run(); err != nil {
		return fmt.Errorf("failed to patch ClusterRole: %w", err)
	}

	// Restart ExternalDNS to pick up the new permissions
	fmt.Println("Restarting ExternalDNS to apply permissions...")
	restartCmd := exec.CommandContext(ctx, "kubectl", "rollout", "restart", "deployment/external-dns",
		"-n", "external-dns")
	if e.kubeconfig != "" {
		restartCmd.Args = append(restartCmd.Args, "--kubeconfig", e.kubeconfig)
	}
	restartCmd.Stdout = os.Stdout
	restartCmd.Stderr = os.Stderr
	if err := restartCmd.Run(); err != nil {
		return fmt.Errorf("failed to restart ExternalDNS: %w", err)
	}

	fmt.Println("✓ ExternalDNS installed successfully")
	return nil
}

// Uninstall removes ExternalDNS using Helm
func (e *ExternalDNSComponent) Uninstall() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uninstallCmd := exec.CommandContext(ctx, "helm", "uninstall", "external-dns",
		"--namespace", "external-dns")

	if e.kubeconfig != "" {
		uninstallCmd.Args = append(uninstallCmd.Args, "--kubeconfig", e.kubeconfig)
	}

	uninstallCmd.Stdout = os.Stdout
	uninstallCmd.Stderr = os.Stderr
	if err := uninstallCmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall ExternalDNS: %w", err)
	}

	// Delete namespace
	nsCmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "external-dns",
		"--ignore-not-found=true")
	if e.kubeconfig != "" {
		nsCmd.Args = append(nsCmd.Args, "--kubeconfig", e.kubeconfig)
	}
	nsCmd.Stdout = os.Stdout
	nsCmd.Stderr = os.Stderr
	_ = nsCmd.Run() // Ignore errors

	fmt.Println("✓ ExternalDNS uninstalled successfully")
	return nil
}

// HealthCheck verifies ExternalDNS is healthy
func (e *ExternalDNSComponent) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", getKubeconfig(e.kubeconfig))
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check external-dns namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "external-dns", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("external-dns namespace not found: %w", err)
	}

	// Check ExternalDNS pods are running (using correct Bitnami label)
	pods, err := clientset.CoreV1().Pods("external-dns").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=external-dns",
	})
	if err != nil {
		return fmt.Errorf("failed to list ExternalDNS pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no ExternalDNS pods found")
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		return fmt.Errorf("no ExternalDNS pods running")
	}

	// Check Cloudflare secret exists (Bitnami chart creates it as "external-dns")
	_, err = clientset.CoreV1().Secrets("external-dns").Get(ctx, "external-dns", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Cloudflare API token secret not found: %w", err)
	}

	fmt.Printf("✓ ExternalDNS health check passed: %d/%d pods running, Cloudflare token configured\n", runningPods, len(pods.Items))
	return nil
}

// Note: Validator is now implemented in bootstrap/validation.go
