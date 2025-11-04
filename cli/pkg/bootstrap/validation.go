package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/clusterkit/clusterkit/pkg/k8s"
	"github.com/clusterkit/clusterkit/pkg/log"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validator performs end-to-end validation of the bootstrap
type Validator struct {
	config     *Config
	k8sClient  *k8s.Client
	logger     *logrus.Logger
	ctx        context.Context
}

// ValidationResult contains validation results
type ValidationResult struct {
	Checks      []ValidationCheck
	AllPassed   bool
	FailedCount int
	Duration    time.Duration
}

// ValidationCheck represents a single validation check
type ValidationCheck struct {
	Name     string
	Category string
	Passed   bool
	Message  string
	Error    error
}

// NewValidator creates a new validator
func NewValidator(config *Config) (*Validator, error) {
	k8sClient, err := k8s.NewClient(config.Kubeconfig, config.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &Validator{
		config:    config,
		k8sClient: k8sClient,
		logger:    log.GetLogger(),
		ctx:       context.Background(),
	}, nil
}

// Run executes all validation checks
func (v *Validator) Run() (*ValidationResult, error) {
	startTime := time.Now()
	result := &ValidationResult{
		Checks: make([]ValidationCheck, 0),
	}

	v.logger.Info("Running validation checks...")

	// Cluster connectivity
	result.Checks = append(result.Checks, v.checkClusterConnectivity())

	// Component health checks (skip if component not installed)
	if !v.config.SkipKnative {
		result.Checks = append(result.Checks, v.checkKnativeInstallation()...)
	}
	if !v.config.SkipIngress {
		result.Checks = append(result.Checks, v.checkIngressInstallation()...)
	}
	if !v.config.SkipCertManager {
		result.Checks = append(result.Checks, v.checkCertManagerInstallation()...)
	}
	if !v.config.SkipExternalDNS {
		result.Checks = append(result.Checks, v.checkExternalDNSInstallation()...)
	}

	// Functional tests (skip if dependencies not installed)
	if !v.config.SkipExternalDNS {
		result.Checks = append(result.Checks, v.checkDNSConfiguration())
	}
	if !v.config.SkipCertManager {
		result.Checks = append(result.Checks, v.checkTLSConfiguration())
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

// checkClusterConnectivity verifies we can connect to the cluster
func (v *Validator) checkClusterConnectivity() ValidationCheck {
	err := v.k8sClient.TestConnection()
	if err != nil {
		return ValidationCheck{
			Name:     "Cluster Connectivity",
			Category: "Infrastructure",
			Passed:   false,
			Message:  "Cannot connect to cluster",
			Error:    err,
		}
	}

	version, err := v.k8sClient.GetServerVersion()
	if err != nil {
		return ValidationCheck{
			Name:     "Cluster Connectivity",
			Category: "Infrastructure",
			Passed:   false,
			Message:  "Cannot get cluster version",
			Error:    err,
		}
	}

	return ValidationCheck{
		Name:     "Cluster Connectivity",
		Category: "Infrastructure",
		Passed:   true,
		Message:  fmt.Sprintf("Connected to cluster (version: %s)", version),
	}
}

// checkKnativeInstallation verifies Knative is installed and healthy
func (v *Validator) checkKnativeInstallation() []ValidationCheck {
	checks := make([]ValidationCheck, 0)

	// Check knative-serving namespace exists
	namespace, err := v.k8sClient.Clientset.CoreV1().Namespaces().Get(v.ctx, "knative-serving", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "Knative Namespace",
			Category: "Knative",
			Passed:   false,
			Message:  "knative-serving namespace not found",
			Error:    err,
		})
		return checks
	}

	checks = append(checks, ValidationCheck{
		Name:     "Knative Namespace",
		Category: "Knative",
		Passed:   true,
		Message:  fmt.Sprintf("Namespace exists (status: %s)", namespace.Status.Phase),
	})

	// Check Knative pods are running
	pods, err := v.k8sClient.Clientset.CoreV1().Pods("knative-serving").List(v.ctx, metav1.ListOptions{})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "Knative Pods",
			Category: "Knative",
			Passed:   false,
			Message:  "Cannot list Knative pods",
			Error:    err,
		})
		return checks
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		checks = append(checks, ValidationCheck{
			Name:     "Knative Pods",
			Category: "Knative",
			Passed:   false,
			Message:  "No Knative pods are running",
		})
	} else {
		checks = append(checks, ValidationCheck{
			Name:     "Knative Pods",
			Category: "Knative",
			Passed:   true,
			Message:  fmt.Sprintf("%d pods running", runningPods),
		})
	}

	return checks
}

// checkIngressInstallation verifies NGINX Ingress is installed and healthy
func (v *Validator) checkIngressInstallation() []ValidationCheck {
	checks := make([]ValidationCheck, 0)

	// Check ingress-nginx namespace exists
	namespace, err := v.k8sClient.Clientset.CoreV1().Namespaces().Get(v.ctx, "ingress-nginx", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "Ingress Namespace",
			Category: "Ingress",
			Passed:   false,
			Message:  "ingress-nginx namespace not found",
			Error:    err,
		})
		return checks
	}

	checks = append(checks, ValidationCheck{
		Name:     "Ingress Namespace",
		Category: "Ingress",
		Passed:   true,
		Message:  fmt.Sprintf("Namespace exists (status: %s)", namespace.Status.Phase),
	})

	// Check Ingress controller pods
	pods, err := v.k8sClient.Clientset.CoreV1().Pods("ingress-nginx").List(v.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=controller",
	})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "Ingress Controller",
			Category: "Ingress",
			Passed:   false,
			Message:  "Cannot list Ingress controller pods",
			Error:    err,
		})
		return checks
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		checks = append(checks, ValidationCheck{
			Name:     "Ingress Controller",
			Category: "Ingress",
			Passed:   false,
			Message:  "No Ingress controller pods are running",
		})
	} else {
		checks = append(checks, ValidationCheck{
			Name:     "Ingress Controller",
			Category: "Ingress",
			Passed:   true,
			Message:  fmt.Sprintf("%d controller pods running", runningPods),
		})
	}

	// Check LoadBalancer service
	svc, err := v.k8sClient.Clientset.CoreV1().Services("ingress-nginx").Get(v.ctx, "ingress-nginx-controller", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "Ingress LoadBalancer",
			Category: "Ingress",
			Passed:   false,
			Message:  "LoadBalancer service not found",
			Error:    err,
		})
		return checks
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		checks = append(checks, ValidationCheck{
			Name:     "Ingress LoadBalancer",
			Category: "Ingress",
			Passed:   false,
			Message:  "LoadBalancer IP not assigned yet",
		})
	} else {
		ip := svc.Status.LoadBalancer.Ingress[0].IP
		checks = append(checks, ValidationCheck{
			Name:     "Ingress LoadBalancer",
			Category: "Ingress",
			Passed:   true,
			Message:  fmt.Sprintf("LoadBalancer IP: %s", ip),
		})
	}

	return checks
}

// checkCertManagerInstallation verifies cert-manager is installed and healthy
func (v *Validator) checkCertManagerInstallation() []ValidationCheck {
	checks := make([]ValidationCheck, 0)

	// Check cert-manager namespace
	namespace, err := v.k8sClient.Clientset.CoreV1().Namespaces().Get(v.ctx, "cert-manager", metav1.GetOptions{})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "cert-manager Namespace",
			Category: "cert-manager",
			Passed:   false,
			Message:  "cert-manager namespace not found",
			Error:    err,
		})
		return checks
	}

	checks = append(checks, ValidationCheck{
		Name:     "cert-manager Namespace",
		Category: "cert-manager",
		Passed:   true,
		Message:  fmt.Sprintf("Namespace exists (status: %s)", namespace.Status.Phase),
	})

	// Check cert-manager pods
	pods, err := v.k8sClient.Clientset.CoreV1().Pods("cert-manager").List(v.ctx, metav1.ListOptions{})
	if err != nil {
		checks = append(checks, ValidationCheck{
			Name:     "cert-manager Pods",
			Category: "cert-manager",
			Passed:   false,
			Message:  "Cannot list cert-manager pods",
			Error:    err,
		})
		return checks
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods < 3 { // cert-manager, cainjector, webhook
		checks = append(checks, ValidationCheck{
			Name:     "cert-manager Pods",
			Category: "cert-manager",
			Passed:   false,
			Message:  fmt.Sprintf("Expected at least 3 pods, found %d running", runningPods),
		})
	} else {
		checks = append(checks, ValidationCheck{
			Name:     "cert-manager Pods",
			Category: "cert-manager",
			Passed:   true,
			Message:  fmt.Sprintf("%d pods running", runningPods),
		})
	}

	return checks
}

// checkExternalDNSInstallation verifies ExternalDNS is installed and healthy
func (v *Validator) checkExternalDNSInstallation() []ValidationCheck {
	checks := make([]ValidationCheck, 0)

	// Check ExternalDNS pods in kube-system or external-dns namespace
	namespaces := []string{"external-dns", "kube-system"}
	var pods *corev1.PodList
	var err error
	var foundNamespace string

	for _, ns := range namespaces {
		pods, err = v.k8sClient.Clientset.CoreV1().Pods(ns).List(v.ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=external-dns",
		})
		if err == nil && len(pods.Items) > 0 {
			foundNamespace = ns
			break
		}
	}

	if foundNamespace == "" {
		checks = append(checks, ValidationCheck{
			Name:     "ExternalDNS Pods",
			Category: "ExternalDNS",
			Passed:   false,
			Message:  "ExternalDNS pods not found in expected namespaces",
		})
		return checks
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods == 0 {
		checks = append(checks, ValidationCheck{
			Name:     "ExternalDNS Pods",
			Category: "ExternalDNS",
			Passed:   false,
			Message:  "No ExternalDNS pods are running",
		})
	} else {
		checks = append(checks, ValidationCheck{
			Name:     "ExternalDNS Pods",
			Category: "ExternalDNS",
			Passed:   true,
			Message:  fmt.Sprintf("%d pods running in %s namespace", runningPods, foundNamespace),
		})
	}

	return checks
}

// checkDNSConfiguration verifies DNS is configured correctly
func (v *Validator) checkDNSConfiguration() ValidationCheck {
	// Check if ExternalDNS is properly configured with Cloudflare
	// Verify ExternalDNS deployment exists and has Cloudflare provider configured
	deployments, err := v.k8sClient.Clientset.AppsV1().Deployments("external-dns").List(v.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=external-dns",
	})
	if err != nil {
		return ValidationCheck{
			Name:     "DNS Configuration",
			Category: "Configuration",
			Passed:   false,
			Message:  "Cannot find ExternalDNS deployment",
			Error:    err,
		}
	}

	if len(deployments.Items) == 0 {
		return ValidationCheck{
			Name:     "DNS Configuration",
			Category: "Configuration",
			Passed:   false,
			Message:  "ExternalDNS deployment not found",
		}
	}

	// Check that Cloudflare secret exists
	_, err = v.k8sClient.Clientset.CoreV1().Secrets("external-dns").Get(v.ctx, "external-dns", metav1.GetOptions{})
	if err != nil {
		return ValidationCheck{
			Name:     "DNS Configuration",
			Category: "Configuration",
			Passed:   false,
			Message:  "ExternalDNS Cloudflare secret not found",
			Error:    err,
		}
	}

	// Verify deployment has Cloudflare provider in args
	deployment := deployments.Items[0]
	hasCloudflare := false
	if deployment.Spec.Template.Spec.Containers != nil && len(deployment.Spec.Template.Spec.Containers) > 0 {
		container := deployment.Spec.Template.Spec.Containers[0]
		for _, arg := range container.Args {
			if strings.Contains(arg, "--provider=cloudflare") || strings.Contains(arg, "cloudflare") {
				hasCloudflare = true
				break
			}
		}
	}

	if !hasCloudflare {
		return ValidationCheck{
			Name:     "DNS Configuration",
			Category: "Configuration",
			Passed:   false,
			Message:  "ExternalDNS not configured with Cloudflare provider",
		}
	}

	return ValidationCheck{
		Name:     "DNS Configuration",
		Category: "Configuration",
		Passed:   true,
		Message:  fmt.Sprintf("ExternalDNS configured with Cloudflare provider for domain %s", v.config.Domain),
	}
}

// checkTLSConfiguration verifies TLS/cert-manager is configured
func (v *Validator) checkTLSConfiguration() ValidationCheck {
	// Check for ClusterIssuer
	// Note: This requires cert-manager CRDs which might not be accessible via standard client
	// For now, we'll just check if the cert-manager webhook is responsive

	svc, err := v.k8sClient.Clientset.CoreV1().Services("cert-manager").Get(v.ctx, "cert-manager-webhook", metav1.GetOptions{})
	if err != nil {
		return ValidationCheck{
			Name:     "TLS Configuration",
			Category: "Configuration",
			Passed:   false,
			Message:  "cert-manager webhook service not found",
			Error:    err,
		}
	}

	if svc.Spec.ClusterIP == "" {
		return ValidationCheck{
			Name:     "TLS Configuration",
			Category: "Configuration",
			Passed:   false,
			Message:  "cert-manager webhook has no ClusterIP",
		}
	}

	return ValidationCheck{
		Name:     "TLS Configuration",
		Category: "Configuration",
		Passed:   true,
		Message:  "cert-manager webhook is configured",
	}
}

// PrintValidationResults prints validation results in a readable format
func PrintValidationResults(result *ValidationResult) {
	logger := log.GetLogger()

	logger.Infof("\nValidation Results (completed in %s):\n", result.Duration)

	// Group by category
	categories := make(map[string][]ValidationCheck)
	for _, check := range result.Checks {
		categories[check.Category] = append(categories[check.Category], check)
	}

	// Print by category
	for category, checks := range categories {
		logger.Infof("\n%s:", category)
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
		logger.Infof("✓ All validation checks passed (%d/%d)", len(result.Checks)-result.FailedCount, len(result.Checks))
	} else {
		logger.Errorf("✗ Validation failed: %d/%d checks passed", len(result.Checks)-result.FailedCount, len(result.Checks))
	}
	logger.Infof(strings.Repeat("=", 50) + "\n")
}
