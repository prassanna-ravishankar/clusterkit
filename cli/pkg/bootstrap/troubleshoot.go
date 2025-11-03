package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clusterkit/clusterkit/pkg/k8s"
	"github.com/clusterkit/clusterkit/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Troubleshooter performs diagnostic checks and collects troubleshooting information
type Troubleshooter struct {
	k8sClient *k8s.Client
	logger    *log.Logger
	ctx       context.Context
}

// DiagnosticResult contains diagnostic check results
type DiagnosticResult struct {
	Checks      []DiagnosticCheck
	AllPassed   bool
	FailedCount int
	Duration    time.Duration
}

// DiagnosticCheck represents a single diagnostic check
type DiagnosticCheck struct {
	Name        string
	Component   string
	Passed      bool
	Message     string
	Error       error
	Remediation string
}

// NewTroubleshooter creates a new troubleshooter
func NewTroubleshooter() *Troubleshooter {
	// Try to create k8s client
	k8sClient, err := k8s.NewClient("", "")
	if err != nil {
		log.GetLogger().Warnf("Could not connect to cluster: %v", err)
	}

	return &Troubleshooter{
		k8sClient: k8sClient,
		logger:    log.GetLogger(),
		ctx:       context.Background(),
	}
}

// RunDiagnostics runs all diagnostic checks
func (t *Troubleshooter) RunDiagnostics(component string) (*DiagnosticResult, error) {
	startTime := time.Now()
	result := &DiagnosticResult{
		Checks: make([]DiagnosticCheck, 0),
	}

	t.logger.Info("Running diagnostic checks...")

	// Check cluster connectivity first
	if t.k8sClient == nil {
		result.Checks = append(result.Checks, DiagnosticCheck{
			Name:      "Cluster Connectivity",
			Component: "Infrastructure",
			Passed:    false,
			Message:   "Cannot connect to Kubernetes cluster",
			Remediation: `Ensure kubeconfig is properly configured:
  - Check: kubectl cluster-info
  - Verify: gcloud container clusters get-credentials <cluster> --region=<region>
  - Check context: kubectl config current-context`,
		})
		result.FailedCount++
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Run connectivity check
	result.Checks = append(result.Checks, t.checkClusterConnectivity())

	// Run component-specific or all checks
	if component == "" || component == "knative" {
		result.Checks = append(result.Checks, t.diagnoseKnative()...)
	}
	if component == "" || component == "ingress" {
		result.Checks = append(result.Checks, t.diagnoseIngress()...)
	}
	if component == "" || component == "cert-manager" {
		result.Checks = append(result.Checks, t.diagnoseCertManager()...)
	}
	if component == "" || component == "external-dns" {
		result.Checks = append(result.Checks, t.diagnoseExternalDNS()...)
	}

	// Count failures
	for _, check := range result.Checks {
		if !check.Passed {
			result.FailedCount++
		}
	}

	result.AllPassed = result.FailedCount == 0
	result.Duration = time.Since(startTime)

	return result, nil
}

// checkClusterConnectivity checks basic cluster connectivity
func (t *Troubleshooter) checkClusterConnectivity() DiagnosticCheck {
	err := t.k8sClient.TestConnection()
	if err != nil {
		return DiagnosticCheck{
			Name:      "Cluster Connectivity",
			Component: "Infrastructure",
			Passed:    false,
			Message:   fmt.Sprintf("Cannot connect to cluster: %v", err),
			Error:     err,
			Remediation: `Check cluster connectivity:
  - Verify kubeconfig: kubectl config view
  - Test connection: kubectl cluster-info
  - Check credentials: gcloud auth list
  - Verify cluster exists: gcloud container clusters list`,
		}
	}

	version, err := t.k8sClient.GetServerVersion()
	if err != nil {
		return DiagnosticCheck{
			Name:      "Cluster Connectivity",
			Component: "Infrastructure",
			Passed:    false,
			Message:   "Connected but cannot get version",
			Error:     err,
		}
	}

	return DiagnosticCheck{
		Name:      "Cluster Connectivity",
		Component: "Infrastructure",
		Passed:    true,
		Message:   fmt.Sprintf("Connected successfully (Kubernetes %s)", version),
	}
}

// diagnoseKnative diagnoses Knative Serving issues
func (t *Troubleshooter) diagnoseKnative() []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0)

	// Check namespace
	ns, err := t.k8sClient.Clientset.CoreV1().Namespaces().Get(t.ctx, "knative-serving", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "Knative Namespace",
			Component: "Knative",
			Passed:    false,
			Message:   "knative-serving namespace not found",
			Error:     err,
			Remediation: `Install Knative Serving:
  kubectl apply -f https://github.com/knative/serving/releases/latest/download/serving-crds.yaml
  kubectl apply -f https://github.com/knative/serving/releases/latest/download/serving-core.yaml`,
		})
		return checks
	}

	checks = append(checks, DiagnosticCheck{
		Name:      "Knative Namespace",
		Component: "Knative",
		Passed:    true,
		Message:   fmt.Sprintf("Namespace exists (phase: %s)", ns.Status.Phase),
	})

	// Check pods
	pods, err := t.k8sClient.Clientset.CoreV1().Pods("knative-serving").List(t.ctx, metav1.ListOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "Knative Pods",
			Component: "Knative",
			Passed:    false,
			Message:   "Cannot list pods",
			Error:     err,
		})
		return checks
	}

	// Analyze pod status
	podStatus := analyzePodStatus(pods.Items)
	if podStatus.Failed > 0 || podStatus.Running == 0 {
		checks = append(checks, DiagnosticCheck{
			Name:      "Knative Pods",
			Component: "Knative",
			Passed:    false,
			Message:   fmt.Sprintf("Issues detected: %d running, %d pending, %d failed", podStatus.Running, podStatus.Pending, podStatus.Failed),
			Remediation: `Check pod issues:
  kubectl get pods -n knative-serving
  kubectl describe pods -n knative-serving
  kubectl logs -n knative-serving -l app=controller`,
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name:      "Knative Pods",
			Component: "Knative",
			Passed:    true,
			Message:   fmt.Sprintf("%d pods running", podStatus.Running),
		})
	}

	// Check webhook
	checks = append(checks, t.checkWebhook("knative-serving", "Knative"))

	return checks
}

// diagnoseIngress diagnoses NGINX Ingress issues
func (t *Troubleshooter) diagnoseIngress() []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0)

	// Check namespace
	ns, err := t.k8sClient.Clientset.CoreV1().Namespaces().Get(t.ctx, "ingress-nginx", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress Namespace",
			Component: "Ingress",
			Passed:    false,
			Message:   "ingress-nginx namespace not found",
			Error:     err,
			Remediation: `Install NGINX Ingress Controller:
  kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-latest/deploy/static/provider/cloud/deploy.yaml`,
		})
		return checks
	}

	checks = append(checks, DiagnosticCheck{
		Name:      "Ingress Namespace",
		Component: "Ingress",
		Passed:    true,
		Message:   fmt.Sprintf("Namespace exists (phase: %s)", ns.Status.Phase),
	})

	// Check controller pods
	pods, err := t.k8sClient.Clientset.CoreV1().Pods("ingress-nginx").List(t.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=controller",
	})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress Controller Pods",
			Component: "Ingress",
			Passed:    false,
			Message:   "Cannot list controller pods",
			Error:     err,
		})
		return checks
	}

	podStatus := analyzePodStatus(pods.Items)
	if podStatus.Running == 0 {
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress Controller Pods",
			Component: "Ingress",
			Passed:    false,
			Message:   "No controller pods running",
			Remediation: `Check ingress controller:
  kubectl get pods -n ingress-nginx
  kubectl describe pod -n ingress-nginx -l app.kubernetes.io/component=controller
  kubectl logs -n ingress-nginx -l app.kubernetes.io/component=controller`,
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress Controller Pods",
			Component: "Ingress",
			Passed:    true,
			Message:   fmt.Sprintf("%d controller pods running", podStatus.Running),
		})
	}

	// Check LoadBalancer service
	svc, err := t.k8sClient.Clientset.CoreV1().Services("ingress-nginx").Get(t.ctx, "ingress-nginx-controller", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress LoadBalancer",
			Component: "Ingress",
			Passed:    false,
			Message:   "LoadBalancer service not found",
			Error:     err,
		})
	} else if len(svc.Status.LoadBalancer.Ingress) == 0 {
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress LoadBalancer",
			Component: "Ingress",
			Passed:    false,
			Message:   "LoadBalancer IP not assigned",
			Remediation: `Wait for LoadBalancer IP assignment or check:
  kubectl get svc -n ingress-nginx ingress-nginx-controller
  kubectl describe svc -n ingress-nginx ingress-nginx-controller`,
		})
	} else {
		ip := svc.Status.LoadBalancer.Ingress[0].IP
		checks = append(checks, DiagnosticCheck{
			Name:      "Ingress LoadBalancer",
			Component: "Ingress",
			Passed:    true,
			Message:   fmt.Sprintf("LoadBalancer IP: %s", ip),
		})
	}

	return checks
}

// diagnoseCertManager diagnoses cert-manager issues
func (t *Troubleshooter) diagnoseCertManager() []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0)

	// Check namespace
	ns, err := t.k8sClient.Clientset.CoreV1().Namespaces().Get(t.ctx, "cert-manager", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "cert-manager Namespace",
			Component: "cert-manager",
			Passed:    false,
			Message:   "cert-manager namespace not found",
			Error:     err,
			Remediation: `Install cert-manager:
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml`,
		})
		return checks
	}

	checks = append(checks, DiagnosticCheck{
		Name:      "cert-manager Namespace",
		Component: "cert-manager",
		Passed:    true,
		Message:   fmt.Sprintf("Namespace exists (phase: %s)", ns.Status.Phase),
	})

	// Check pods
	pods, err := t.k8sClient.Clientset.CoreV1().Pods("cert-manager").List(t.ctx, metav1.ListOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "cert-manager Pods",
			Component: "cert-manager",
			Passed:    false,
			Message:   "Cannot list pods",
			Error:     err,
		})
		return checks
	}

	podStatus := analyzePodStatus(pods.Items)
	if podStatus.Running < 3 {
		checks = append(checks, DiagnosticCheck{
			Name:      "cert-manager Pods",
			Component: "cert-manager",
			Passed:    false,
			Message:   fmt.Sprintf("Expected 3 pods, found %d running", podStatus.Running),
			Remediation: `Check cert-manager pods:
  kubectl get pods -n cert-manager
  kubectl describe pods -n cert-manager
  kubectl logs -n cert-manager -l app=cert-manager`,
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name:      "cert-manager Pods",
			Component: "cert-manager",
			Passed:    true,
			Message:   fmt.Sprintf("%d pods running (cert-manager, webhook, cainjector)", podStatus.Running),
		})
	}

	// Check webhook
	checks = append(checks, t.checkWebhook("cert-manager", "cert-manager"))

	return checks
}

// diagnoseExternalDNS diagnoses ExternalDNS issues
func (t *Troubleshooter) diagnoseExternalDNS() []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0)

	// Check in common namespaces
	namespaces := []string{"external-dns", "kube-system"}
	var pods *corev1.PodList
	var err error
	var foundNamespace string

	for _, ns := range namespaces {
		pods, err = t.k8sClient.Clientset.CoreV1().Pods(ns).List(t.ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=external-dns",
		})
		if err == nil && len(pods.Items) > 0 {
			foundNamespace = ns
			break
		}
	}

	if foundNamespace == "" {
		checks = append(checks, DiagnosticCheck{
			Name:      "ExternalDNS Pods",
			Component: "ExternalDNS",
			Passed:    false,
			Message:   "ExternalDNS pods not found",
			Remediation: `Install ExternalDNS:
  - Check installation in kube-system or external-dns namespace
  - Verify ExternalDNS is deployed with correct labels
  - See: https://github.com/kubernetes-sigs/external-dns`,
		})
		return checks
	}

	podStatus := analyzePodStatus(pods.Items)
	if podStatus.Running == 0 {
		checks = append(checks, DiagnosticCheck{
			Name:      "ExternalDNS Pods",
			Component: "ExternalDNS",
			Passed:    false,
			Message:   fmt.Sprintf("No pods running in %s", foundNamespace),
			Remediation: fmt.Sprintf(`Check ExternalDNS status:
  kubectl get pods -n %s -l app.kubernetes.io/name=external-dns
  kubectl describe pods -n %s -l app.kubernetes.io/name=external-dns
  kubectl logs -n %s -l app.kubernetes.io/name=external-dns`, foundNamespace, foundNamespace, foundNamespace),
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name:      "ExternalDNS Pods",
			Component: "ExternalDNS",
			Passed:    true,
			Message:   fmt.Sprintf("%d pods running in %s", podStatus.Running, foundNamespace),
		})
	}

	// Check for Cloudflare secret
	secret, err := t.k8sClient.Clientset.CoreV1().Secrets(foundNamespace).Get(t.ctx, "cloudflare-api-token", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name:      "Cloudflare API Token",
			Component: "ExternalDNS",
			Passed:    false,
			Message:   "Cloudflare API token secret not found",
			Remediation: `Create Cloudflare API token secret:
  kubectl create secret generic cloudflare-api-token \
    --from-literal=api-token=YOUR_TOKEN \
    -n ` + foundNamespace,
		})
	} else if len(secret.Data) == 0 {
		checks = append(checks, DiagnosticCheck{
			Name:      "Cloudflare API Token",
			Component: "ExternalDNS",
			Passed:    false,
			Message:   "Secret exists but is empty",
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name:      "Cloudflare API Token",
			Component: "ExternalDNS",
			Passed:    true,
			Message:   "API token secret configured",
		})
	}

	return checks
}

// checkWebhook checks if a webhook is properly configured
func (t *Troubleshooter) checkWebhook(namespace, component string) DiagnosticCheck {
	svc, err := t.k8sClient.Clientset.CoreV1().Services(namespace).Get(t.ctx, fmt.Sprintf("%s-webhook", namespace), metav1.GetOptions{})
	if err != nil {
		return DiagnosticCheck{
			Name:      fmt.Sprintf("%s Webhook", component),
			Component: component,
			Passed:    false,
			Message:   "Webhook service not found",
			Error:     err,
		}
	}

	if svc.Spec.ClusterIP == "" {
		return DiagnosticCheck{
			Name:      fmt.Sprintf("%s Webhook", component),
			Component: component,
			Passed:    false,
			Message:   "Webhook service has no ClusterIP",
		}
	}

	return DiagnosticCheck{
		Name:      fmt.Sprintf("%s Webhook", component),
		Component: component,
		Passed:    true,
		Message:   "Webhook service configured",
	}
}

// PodStatus contains pod status summary
type PodStatus struct {
	Running int
	Pending int
	Failed  int
	Unknown int
}

// analyzePodStatus analyzes pod status
func analyzePodStatus(pods []corev1.Pod) PodStatus {
	status := PodStatus{}
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			status.Running++
		case corev1.PodPending:
			status.Pending++
		case corev1.PodFailed:
			status.Failed++
		default:
			status.Unknown++
		}
	}
	return status
}

// CollectLogs collects logs from all components
func (t *Troubleshooter) CollectLogs(outputDir string) error {
	t.logger.Info("Collecting logs from all components...")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	components := []struct {
		namespace string
		label     string
		name      string
	}{
		{"knative-serving", "app=controller", "knative-controller"},
		{"knative-serving", "app=webhook", "knative-webhook"},
		{"ingress-nginx", "app.kubernetes.io/component=controller", "ingress-controller"},
		{"cert-manager", "app=cert-manager", "cert-manager"},
		{"cert-manager", "app=webhook", "cert-manager-webhook"},
		{"external-dns", "app.kubernetes.io/name=external-dns", "external-dns"},
	}

	for _, comp := range components {
		logPath := filepath.Join(outputDir, fmt.Sprintf("%s.log", comp.name))
		if err := t.collectComponentLogs(comp.namespace, comp.label, logPath); err != nil {
			t.logger.Warnf("Failed to collect %s logs: %v", comp.name, err)
		} else {
			t.logger.Infof("Collected logs: %s", logPath)
		}
	}

	return nil
}

// collectComponentLogs collects logs from a specific component
func (t *Troubleshooter) collectComponentLogs(namespace, labelSelector, outputPath string) error {
	pods, err := t.k8sClient.Clientset.CoreV1().Pods(namespace).List(t.ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found with label %s in namespace %s", labelSelector, namespace)
	}

	// Collect logs from first pod
	podName := pods.Items[0].Name
	req := t.k8sClient.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: int64Ptr(1000),
	})

	logs, err := req.Stream(t.ctx)
	if err != nil {
		return fmt.Errorf("failed to stream logs: %w", err)
	}
	defer logs.Close()

	// Write logs to file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer file.Close()

	_, err = file.ReadFrom(logs)
	return err
}

func int64Ptr(i int64) *int64 {
	return &i
}

// PrintDiagnosticResults prints diagnostic results
func PrintDiagnosticResults(result *DiagnosticResult) {
	logger := log.GetLogger()

	logger.Infof("\nDiagnostic Results (completed in %s):\n", result.Duration)

	// Group by component
	components := make(map[string][]DiagnosticCheck)
	for _, check := range result.Checks {
		components[check.Component] = append(components[check.Component], check)
	}

	// Print by component
	for component, checks := range components {
		logger.Infof("\n%s:", component)
		for _, check := range checks {
			if check.Passed {
				logger.Infof("  ✓ %s: %s", check.Name, check.Message)
			} else {
				logger.Errorf("  ✗ %s: %s", check.Name, check.Message)
				if check.Error != nil {
					logger.Debugf("    Error: %v", check.Error)
				}
			}
		}
	}

	// Print summary
	logger.Infof("\n" + strings.Repeat("=", 50))
	if result.AllPassed {
		logger.Infof("✓ All diagnostic checks passed (%d/%d)", len(result.Checks)-result.FailedCount, len(result.Checks))
	} else {
		logger.Errorf("✗ Diagnostics found issues: %d/%d checks passed", len(result.Checks)-result.FailedCount, len(result.Checks))
	}
	logger.Infof(strings.Repeat("=", 50) + "\n")
}
