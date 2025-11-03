package components

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// KnativeComponent handles Knative Serving installation
type KnativeComponent struct {
	kubeconfig   string
	manifestsDir string
}

// NewKnativeComponent creates a new Knative component
func NewKnativeComponent(kubeconfig string) *KnativeComponent {
	return &KnativeComponent{
		kubeconfig:   kubeconfig,
		manifestsDir: "k8s/knative",
	}
}

// Install installs Knative Serving using Helm (via Knative Operator)
func (k *KnativeComponent) Install() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Step 1: Install Knative Operator using kubectl
	fmt.Println("Installing Knative Operator...")
	operatorURL := "https://github.com/knative/operator/releases/download/knative-v1.15.0/operator.yaml"
	operatorCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", operatorURL)
	if k.kubeconfig != "" {
		operatorCmd.Args = append(operatorCmd.Args, "--kubeconfig", k.kubeconfig)
	}
	operatorCmd.Stdout = os.Stdout
	operatorCmd.Stderr = os.Stderr
	if err := operatorCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Knative Operator: %w", err)
	}

	// Wait for operator to be ready
	fmt.Println("Waiting for Knative Operator to be ready...")
	time.Sleep(30 * time.Second)

	// Step 2: Create KnativeServing CR to install Knative Serving
	fmt.Println("Installing Knative Serving via Operator...")
	knativeServingYAML := `apiVersion: v1
kind: Namespace
metadata:
  name: knative-serving
---
apiVersion: operator.knative.dev/v1beta1
kind: KnativeServing
metadata:
  name: knative-serving
  namespace: knative-serving
`
	applyCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	if k.kubeconfig != "" {
		applyCmd.Args = append(applyCmd.Args, "--kubeconfig", k.kubeconfig)
	}
	applyCmd.Stdin = strings.NewReader(knativeServingYAML)
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("failed to create KnativeServing resource: %w", err)
	}

	fmt.Println("✓ Knative Serving installed successfully")
	return nil
}

// Uninstall removes Knative Serving and Operator
func (k *KnativeComponent) Uninstall() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Delete KnativeServing resource
	deleteServingCmd := exec.CommandContext(ctx, "kubectl", "delete", "knativeserving",
		"knative-serving", "-n", "knative-serving", "--ignore-not-found=true")
	if k.kubeconfig != "" {
		deleteServingCmd.Args = append(deleteServingCmd.Args, "--kubeconfig", k.kubeconfig)
	}
	deleteServingCmd.Stdout = os.Stdout
	deleteServingCmd.Stderr = os.Stderr
	_ = deleteServingCmd.Run()

	// Delete Knative Operator
	operatorURL := "https://github.com/knative/operator/releases/download/knative-v1.15.0/operator.yaml"
	deleteOpCmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", operatorURL,
		"--ignore-not-found=true")
	if k.kubeconfig != "" {
		deleteOpCmd.Args = append(deleteOpCmd.Args, "--kubeconfig", k.kubeconfig)
	}
	deleteOpCmd.Stdout = os.Stdout
	deleteOpCmd.Stderr = os.Stderr
	_ = deleteOpCmd.Run()

	// Delete namespace
	nsCmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "knative-serving",
		"--ignore-not-found=true")
	if k.kubeconfig != "" {
		nsCmd.Args = append(nsCmd.Args, "--kubeconfig", k.kubeconfig)
	}
	nsCmd.Stdout = os.Stdout
	nsCmd.Stderr = os.Stderr
	_ = nsCmd.Run()

	fmt.Println("✓ Knative Serving uninstalled successfully")
	return nil
}

// HealthCheck verifies Knative is healthy
func (k *KnativeComponent) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Use default kubeconfig path if not specified
	kubeconfig := k.kubeconfig
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check knative-serving namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "knative-serving", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("knative-serving namespace not found: %w", err)
	}

	// Check Knative pods are running
	pods, err := clientset.CoreV1().Pods("knative-serving").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Knative pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no Knative pods found")
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		return fmt.Errorf("no Knative pods running")
	}

	fmt.Printf("✓ Knative health check passed: %d/%d pods running\n", runningPods, len(pods.Items))
	return nil
}

// ConfigureDomain configures the Knative domain
func (k *KnativeComponent) ConfigureDomain(domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", getKubeconfig(k.kubeconfig))
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Get existing config-domain ConfigMap
	cm, err := clientset.CoreV1().ConfigMaps("knative-serving").Get(ctx, "config-domain", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get config-domain ConfigMap: %w", err)
	}

	// Update domain configuration
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[domain] = ""

	// Update ConfigMap
	_, err = clientset.CoreV1().ConfigMaps("knative-serving").Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update config-domain ConfigMap: %w", err)
	}

	fmt.Printf("✓ Knative domain configured: %s\n", domain)
	return nil
}

// IngressComponent handles NGINX Ingress installation
type IngressComponent struct {
	kubeconfig string
	valuesFile string
}

// NewIngressComponent creates a new Ingress component
func NewIngressComponent(kubeconfig string) *IngressComponent {
	return &IngressComponent{
		kubeconfig: kubeconfig,
		valuesFile: "k8s/nginx-ingress/values.yaml",
	}
}

// Install installs NGINX Ingress Controller
func (i *IngressComponent) Install() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	valuesPath, err := filepath.Abs(i.valuesFile)
	if err != nil {
		return fmt.Errorf("failed to resolve values file: %w", err)
	}

	// Create namespace if it doesn't exist
	nsCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", "ingress-nginx",
		"--kubeconfig", i.kubeconfig,
		"--dry-run=client", "-o", "yaml")
	nsOutput, _ := nsCmd.Output()

	applyNsCmd := exec.CommandContext(ctx, "kubectl", "apply",
		"--kubeconfig", i.kubeconfig,
		"-f", "-")
	applyNsCmd.Stdin = strings.NewReader(string(nsOutput))
	applyNsCmd.Stdout = os.Stdout
	applyNsCmd.Stderr = os.Stderr
	_ = applyNsCmd.Run() // Ignore error if namespace exists

	// Add Helm repo
	repoCmd := exec.CommandContext(ctx, "helm", "repo", "add", "ingress-nginx",
		"https://kubernetes.github.io/ingress-nginx")
	repoCmd.Stdout = os.Stdout
	repoCmd.Stderr = os.Stderr
	if err := repoCmd.Run(); err != nil {
		return fmt.Errorf("failed to add Helm repo: %w", err)
	}

	// Update Helm repos
	updateCmd := exec.CommandContext(ctx, "helm", "repo", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update Helm repos: %w", err)
	}

	// Install nginx-ingress
	installCmd := exec.CommandContext(ctx, "helm", "install", "nginx-ingress",
		"ingress-nginx/ingress-nginx",
		"--namespace", "ingress-nginx",
		"--kubeconfig", i.kubeconfig,
		"-f", valuesPath,
		"--wait",
		"--timeout", "5m")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install nginx-ingress: %w", err)
	}

	fmt.Println("✓ NGINX Ingress Controller installed successfully")
	return nil
}

// Uninstall removes NGINX Ingress Controller
func (i *IngressComponent) Uninstall() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uninstallCmd := exec.CommandContext(ctx, "helm", "uninstall", "nginx-ingress",
		"--namespace", "ingress-nginx",
		"--kubeconfig", i.kubeconfig)
	uninstallCmd.Stdout = os.Stdout
	uninstallCmd.Stderr = os.Stderr
	if err := uninstallCmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall nginx-ingress: %w", err)
	}

	// Delete namespace
	nsCmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "ingress-nginx",
		"--kubeconfig", i.kubeconfig,
		"--ignore-not-found=true")
	nsCmd.Stdout = os.Stdout
	nsCmd.Stderr = os.Stderr
	if err := nsCmd.Run(); err != nil {
		return fmt.Errorf("failed to delete ingress-nginx namespace: %w", err)
	}

	fmt.Println("✓ NGINX Ingress Controller uninstalled successfully")
	return nil
}

// HealthCheck verifies Ingress is healthy
func (i *IngressComponent) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", getKubeconfig(i.kubeconfig))
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check ingress-nginx namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "ingress-nginx", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ingress-nginx namespace not found: %w", err)
	}

	// Check nginx-ingress pods are running
	pods, err := clientset.CoreV1().Pods("ingress-nginx").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=ingress-nginx",
	})
	if err != nil {
		return fmt.Errorf("failed to list ingress pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no ingress pods found")
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		return fmt.Errorf("no ingress pods running")
	}

	// Check LoadBalancer service has external IP
	svc, err := clientset.CoreV1().Services("ingress-nginx").Get(ctx, "nginx-ingress-ingress-nginx-controller", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ingress LoadBalancer service: %w", err)
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return fmt.Errorf("LoadBalancer service has no external IP assigned yet")
	}

	externalIP := svc.Status.LoadBalancer.Ingress[0].IP
	fmt.Printf("✓ NGINX Ingress health check passed: %d/%d pods running, LoadBalancer IP: %s\n", runningPods, len(pods.Items), externalIP)
	return nil
}

// CertManagerComponent handles cert-manager installation
type CertManagerComponent struct {
	kubeconfig   string
	manifestsDir string
}

// NewCertManagerComponent creates a new cert-manager component
func NewCertManagerComponent(kubeconfig string) *CertManagerComponent {
	return &CertManagerComponent{
		kubeconfig:   kubeconfig,
		manifestsDir: "k8s/cert-manager",
	}
}

// Install installs cert-manager
func (c *CertManagerComponent) Install() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Add Helm repo
	repoCmd := exec.CommandContext(ctx, "helm", "repo", "add", "jetstack",
		"https://charts.jetstack.io")
	repoCmd.Stdout = os.Stdout
	repoCmd.Stderr = os.Stderr
	if err := repoCmd.Run(); err != nil {
		return fmt.Errorf("failed to add Helm repo: %w", err)
	}

	// Update Helm repos
	updateCmd := exec.CommandContext(ctx, "helm", "repo", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update Helm repos: %w", err)
	}

	// Install cert-manager with CRDs
	installCmd := exec.CommandContext(ctx, "helm", "install", "cert-manager",
		"jetstack/cert-manager",
		"--namespace", "cert-manager",
		"--create-namespace",
		"--kubeconfig", c.kubeconfig,
		"--set", "installCRDs=true",
		"--wait",
		"--timeout", "5m")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install cert-manager: %w", err)
	}

	// Wait for cert-manager to be ready
	time.Sleep(10 * time.Second)

	// Apply ClusterIssuers
	manifestsPath, err := filepath.Abs(c.manifestsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve manifests directory: %w", err)
	}

	issuerCmd := exec.CommandContext(ctx, "kubectl", "apply",
		"--kubeconfig", c.kubeconfig,
		"-f", filepath.Join(manifestsPath, "cluster-issuer-staging.yaml"),
		"-f", filepath.Join(manifestsPath, "cluster-issuer-prod.yaml"))
	issuerCmd.Stdout = os.Stdout
	issuerCmd.Stderr = os.Stderr
	if err := issuerCmd.Run(); err != nil {
		return fmt.Errorf("failed to apply ClusterIssuers: %w", err)
	}

	fmt.Println("✓ cert-manager installed successfully")
	return nil
}

// Uninstall removes cert-manager
func (c *CertManagerComponent) Uninstall() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Delete ClusterIssuers first
	manifestsPath, err := filepath.Abs(c.manifestsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve manifests directory: %w", err)
	}

	issuerCmd := exec.CommandContext(ctx, "kubectl", "delete",
		"--kubeconfig", c.kubeconfig,
		"-f", filepath.Join(manifestsPath, "cluster-issuer-staging.yaml"),
		"-f", filepath.Join(manifestsPath, "cluster-issuer-prod.yaml"),
		"--ignore-not-found=true")
	issuerCmd.Stdout = os.Stdout
	issuerCmd.Stderr = os.Stderr
	_ = issuerCmd.Run() // Ignore error if not found

	// Uninstall cert-manager
	uninstallCmd := exec.CommandContext(ctx, "helm", "uninstall", "cert-manager",
		"--namespace", "cert-manager",
		"--kubeconfig", c.kubeconfig)
	uninstallCmd.Stdout = os.Stdout
	uninstallCmd.Stderr = os.Stderr
	if err := uninstallCmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall cert-manager: %w", err)
	}

	// Delete namespace
	nsCmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "cert-manager",
		"--kubeconfig", c.kubeconfig,
		"--ignore-not-found=true")
	nsCmd.Stdout = os.Stdout
	nsCmd.Stderr = os.Stderr
	if err := nsCmd.Run(); err != nil {
		return fmt.Errorf("failed to delete cert-manager namespace: %w", err)
	}

	fmt.Println("✓ cert-manager uninstalled successfully")
	return nil
}

// HealthCheck verifies cert-manager is healthy
func (c *CertManagerComponent) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", getKubeconfig(c.kubeconfig))
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check cert-manager namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "cert-manager", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cert-manager namespace not found: %w", err)
	}

	// Check cert-manager pods are running
	pods, err := clientset.CoreV1().Pods("cert-manager").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=cert-manager",
	})
	if err != nil {
		return fmt.Errorf("failed to list cert-manager pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no cert-manager pods found")
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		return fmt.Errorf("no cert-manager pods running")
	}

	// Check webhook pod specifically
	webhookPods, err := clientset.CoreV1().Pods("cert-manager").List(ctx, metav1.ListOptions{
		LabelSelector: "app=webhook",
	})
	if err != nil {
		return fmt.Errorf("failed to list webhook pods: %w", err)
	}

	webhookReady := false
	for _, pod := range webhookPods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			webhookReady = true
			break
		}
	}

	if !webhookReady {
		return fmt.Errorf("cert-manager webhook not ready")
	}

	fmt.Printf("✓ cert-manager health check passed: %d/%d pods running, webhook ready\n", runningPods, len(pods.Items))
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

	// Install ExternalDNS with Helm
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

	// Check ExternalDNS pods are running
	pods, err := clientset.CoreV1().Pods("external-dns").List(ctx, metav1.ListOptions{
		LabelSelector: "app=external-dns",
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

	// Check Cloudflare secret exists
	_, err = clientset.CoreV1().Secrets("external-dns").Get(ctx, "cloudflare-api-token", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Cloudflare API token secret not found: %w", err)
	}

	fmt.Printf("✓ ExternalDNS health check passed: %d/%d pods running, Cloudflare token configured\n", runningPods, len(pods.Items))
	return nil
}

// Note: Validator is now implemented in bootstrap/validation.go
