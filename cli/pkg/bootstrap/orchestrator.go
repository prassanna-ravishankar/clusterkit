package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/clusterkit/clusterkit/pkg/bootstrap/components"
	"github.com/clusterkit/clusterkit/pkg/log"
	"github.com/sirupsen/logrus"
)

// Orchestrator manages the bootstrap process
type Orchestrator struct {
	config *Config
	ctx    context.Context
	dryRun bool
	logger *logrus.Logger
}

// Config contains bootstrap configuration
type Config struct {
	// GCP Configuration
	ProjectID   string
	Region      string
	ClusterName string

	// Domain Configuration
	Domain          string
	CloudflareToken string

	// Component Flags
	SkipTerraform   bool
	SkipExternalDNS bool
	SkipKnative     bool
	SkipIngress     bool
	SkipCertManager bool

	// Kubernetes Configuration
	Kubeconfig string
	Context    string
}

// BootstrapResult contains the results of the bootstrap operation
type BootstrapResult struct {
	Success    bool
	Steps      []StepResult
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Error      error
}

// StepResult contains the result of a single bootstrap step
type StepResult struct {
	Name       string
	Component  string
	Status     StepStatus
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Error      error
	Message    string
	Retries    int
}

// StepStatus represents the status of a bootstrap step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusSuccess   StepStatus = "success"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusRetrying  StepStatus = "retrying"
)

// NewOrchestrator creates a new bootstrap orchestrator
func NewOrchestrator(config *Config, dryRun bool) *Orchestrator {
	return &Orchestrator{
		config: config,
		ctx:    context.Background(),
		dryRun: dryRun,
		logger: log.GetLogger(),
	}
}

// Run executes the complete bootstrap process
func (o *Orchestrator) Run(progressCallback func(StepResult)) (*BootstrapResult, error) {
	result := &BootstrapResult{
		StartTime: time.Now(),
		Steps:     make([]StepResult, 0),
	}

	o.logger.Info("Starting ClusterKit bootstrap")
	if o.dryRun {
		o.logger.Info("Running in DRY-RUN mode - no changes will be made")
	}

	// Define bootstrap steps in dependency order
	steps := []struct {
		name      string
		component string
		skip      bool
		execute   func() error
		healthCheck func() error
	}{
		{
			name:      "Deploy GKE Cluster",
			component: "terraform",
			skip:      o.config.SkipTerraform,
			execute:   o.deployTerraform,
			healthCheck: o.checkClusterHealth,
		},
		{
			name:      "Install ExternalDNS",
			component: "external-dns",
			skip:      o.config.SkipExternalDNS,
			execute:   o.installExternalDNS,
			healthCheck: o.checkExternalDNSHealth,
		},
		{
			name:      "Verify End-to-End Functionality",
			component: "validation",
			skip:      false,
			execute:   o.runValidation,
			healthCheck: nil,
		},
	}

	// Execute each step
	for _, step := range steps {
		stepResult := o.executeStep(step.name, step.component, step.skip, step.execute, step.healthCheck)
		result.Steps = append(result.Steps, stepResult)

		if progressCallback != nil {
			progressCallback(stepResult)
		}

		// Stop on failure unless it's a skipped step
		if stepResult.Status == StepStatusFailed {
			result.Success = false
			result.Error = stepResult.Error
			o.logger.Errorf("Bootstrap failed at step '%s': %v", step.name, stepResult.Error)
			break
		}
	}

	// Calculate final result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = result.Error == nil

	if result.Success {
		o.logger.Infof("Bootstrap completed successfully in %s", result.Duration)
	} else {
		o.logger.Errorf("Bootstrap failed after %s", result.Duration)
	}

	return result, nil
}

// executeStep executes a single bootstrap step with retry logic
func (o *Orchestrator) executeStep(name, component string, skip bool, execute func() error, healthCheck func() error) StepResult {
	result := StepResult{
		Name:      name,
		Component: component,
		Status:    StepStatusPending,
		StartTime: time.Now(),
	}

	if skip {
		result.Status = StepStatusSkipped
		result.Message = "Skipped by configuration"
		o.logger.Infof("[SKIPPED] %s", name)
		return result
	}

	o.logger.Infof("[RUNNING] %s", name)
	result.Status = StepStatusRunning

	// Execute with retry logic
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			result.Status = StepStatusRetrying
			result.Retries = attempt
			o.logger.Warnf("[RETRY %d/%d] %s", attempt, maxRetries-1, name)
			time.Sleep(time.Duration(attempt*10) * time.Second)
		}

		if o.dryRun {
			// In dry-run mode, simulate success
			o.logger.Infof("[DRY-RUN] Would execute: %s", name)
			result.Status = StepStatusSuccess
			result.Message = "Dry-run simulation"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}

		// Execute the step
		err := execute()
		if err == nil {
			// Run health check if provided
			if healthCheck != nil {
				o.logger.Debugf("Running health check for %s", name)
				if err := healthCheck(); err != nil {
					lastErr = fmt.Errorf("health check failed: %w", err)
					continue
				}
			}

			result.Status = StepStatusSuccess
			result.Message = "Completed successfully"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			o.logger.Infof("[SUCCESS] %s (took %s)", name, result.Duration)
			return result
		}

		lastErr = err
		o.logger.Warnf("Step failed: %v", err)
	}

	// All retries failed
	result.Status = StepStatusFailed
	result.Error = lastErr
	result.Message = fmt.Sprintf("Failed after %d attempts: %v", maxRetries, lastErr)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	o.logger.Errorf("[FAILED] %s: %v", name, lastErr)

	return result
}

// deployTerraform deploys infrastructure using Terraform
func (o *Orchestrator) deployTerraform() error {
	o.logger.Info("Deploying GKE cluster and infrastructure with Terraform")

	terraform := components.NewTerraformComponent(o.config.ProjectID, o.config.Region, o.config.ClusterName)
	if err := terraform.Apply(); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	return nil
}

// checkClusterHealth verifies the GKE cluster is healthy
func (o *Orchestrator) checkClusterHealth() error {
	o.logger.Debug("Checking GKE cluster health")

	checker := components.NewClusterHealthChecker(o.config.ProjectID, o.config.Region, o.config.ClusterName, o.config.Kubeconfig)
	return checker.Check()
}

// installExternalDNS installs ExternalDNS
func (o *Orchestrator) installExternalDNS() error {
	o.logger.Info("Installing ExternalDNS")

	externalDNS := components.NewExternalDNSComponent(o.config.Kubeconfig, o.config.CloudflareToken)
	if err := externalDNS.Install(); err != nil {
		return fmt.Errorf("external-dns install failed: %w", err)
	}

	return nil
}

// checkExternalDNSHealth verifies ExternalDNS is healthy
func (o *Orchestrator) checkExternalDNSHealth() error {
	o.logger.Debug("Checking ExternalDNS health")

	externalDNS := components.NewExternalDNSComponent(o.config.Kubeconfig, o.config.CloudflareToken)
	return externalDNS.HealthCheck()
}

// runValidation runs end-to-end validation
func (o *Orchestrator) runValidation() error {
	o.logger.Info("Running end-to-end validation")

	validator, err := NewValidator(o.config)
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	result, err := validator.Run()
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	PrintValidationResults(result)

	if !result.AllPassed {
		return fmt.Errorf("validation failed: %d checks failed", result.FailedCount)
	}

	return nil
}

// Rollback attempts to rollback failed bootstrap
func (o *Orchestrator) Rollback(result *BootstrapResult) error {
	o.logger.Warn("Starting bootstrap rollback")

	// Rollback in reverse order
	for i := len(result.Steps) - 1; i >= 0; i-- {
		step := result.Steps[i]
		if step.Status == StepStatusSuccess {
			o.logger.Infof("Rolling back: %s", step.Name)
			// Implement specific rollback logic per component
			if err := o.rollbackStep(step); err != nil {
				o.logger.Errorf("Rollback failed for %s: %v", step.Name, err)
			}
		}
	}

	o.logger.Info("Rollback completed")
	return nil
}

// rollbackStep rolls back a specific step
func (o *Orchestrator) rollbackStep(step StepResult) error {
	switch step.Component {
	case "terraform":
		terraform := components.NewTerraformComponent(o.config.ProjectID, o.config.Region, o.config.ClusterName)
		return terraform.Destroy()
	case "external-dns":
		externalDNS := components.NewExternalDNSComponent(o.config.Kubeconfig, o.config.CloudflareToken)
		return externalDNS.Uninstall()
	default:
		return nil
	}
}
