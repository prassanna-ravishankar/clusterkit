package knative

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceConfig contains configuration for creating a Knative Service
type ServiceConfig struct {
	Name      string
	Namespace string
	Image     string
	Domains   []string
	Env       []corev1.EnvVar

	// Resource specifications
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string

	// Autoscaling
	MinScale    int
	MaxScale    int
	Concurrency int

	// Labels and annotations
	Labels      map[string]string
	Annotations map[string]string
}

// Service represents a Knative Service (using unstructured since we don't import Knative SDK)
type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceSpec   `json:"spec,omitempty"`
	Status            ServiceStatus `json:"status,omitempty"`
}

// ServiceSpec contains the specification for a Knative Service
type ServiceSpec struct {
	Template RevisionTemplateSpec `json:"template,omitempty"`
}

// RevisionTemplateSpec describes the data a revision should have when created from a template
type RevisionTemplateSpec struct {
	ObjectMeta metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec       RevisionSpec      `json:"spec,omitempty"`
}

// RevisionSpec holds the desired state of the Revision
type RevisionSpec struct {
	PodSpec            corev1.PodSpec `json:",inline"`
	ContainerConcurrency *int64       `json:"containerConcurrency,omitempty"`
}

// ServiceStatus represents the status of a Knative Service
type ServiceStatus struct {
	// We won't populate this during creation
}

// NewServiceConfig creates a new ServiceConfig with defaults
func NewServiceConfig(name, namespace, image string) *ServiceConfig {
	return &ServiceConfig{
		Name:          name,
		Namespace:     namespace,
		Image:         image,
		Domains:       []string{},
		Env:           []corev1.EnvVar{},
		CPURequest:    "100m",
		MemoryRequest: "128Mi",
		CPULimit:      "1000m",
		MemoryLimit:   "256Mi",
		MinScale:      0,
		MaxScale:      5,
		Concurrency:   10,
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
	}
}

// GenerateService creates a Knative Service from the configuration
func (c *ServiceConfig) GenerateService() (*Service, error) {
	if c.Name == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if c.Image == "" {
		return nil, fmt.Errorf("container image is required")
	}

	// Parse resource quantities
	cpuRequest, err := resource.ParseQuantity(c.CPURequest)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU request: %w", err)
	}

	memoryRequest, err := resource.ParseQuantity(c.MemoryRequest)
	if err != nil {
		return nil, fmt.Errorf("invalid memory request: %w", err)
	}

	cpuLimit, err := resource.ParseQuantity(c.CPULimit)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU limit: %w", err)
	}

	memoryLimit, err := resource.ParseQuantity(c.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid memory limit: %w", err)
	}

	// Initialize labels
	labels := map[string]string{
		"app":                          c.Name,
		"app.kubernetes.io/name":       c.Name,
		"app.kubernetes.io/managed-by": "clusterkit",
	}
	for k, v := range c.Labels {
		labels[k] = v
	}

	// Initialize annotations with autoscaling configuration
	annotations := map[string]string{
		"autoscaling.knative.dev/min-scale":   fmt.Sprintf("%d", c.MinScale),
		"autoscaling.knative.dev/max-scale":   fmt.Sprintf("%d", c.MaxScale),
		"autoscaling.knative.dev/target":      fmt.Sprintf("%d", c.Concurrency),
	}
	for k, v := range c.Annotations {
		annotations[k] = v
	}

	// Container concurrency
	concurrency := int64(c.Concurrency)

	// Create the service
	service := &Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "serving.knative.dev/v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.Name,
			Namespace:   c.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: ServiceSpec{
			Template: RevisionTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: annotations,
				},
				Spec: RevisionSpec{
					ContainerConcurrency: &concurrency,
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  c.Name,
								Image: c.Image,
								Env:   c.Env,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    cpuRequest,
										corev1.ResourceMemory: memoryRequest,
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    cpuLimit,
										corev1.ResourceMemory: memoryLimit,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return service, nil
}

// Validate validates the service configuration
func (c *ServiceConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("service name is required")
	}

	if c.Image == "" {
		return fmt.Errorf("container image is required")
	}

	if c.MinScale < 0 {
		return fmt.Errorf("min-scale must be >= 0")
	}

	if c.MaxScale <= 0 {
		return fmt.Errorf("max-scale must be > 0")
	}

	if c.MinScale > c.MaxScale {
		return fmt.Errorf("min-scale must be <= max-scale")
	}

	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be > 0")
	}

	// Validate resource specifications
	if _, err := resource.ParseQuantity(c.CPURequest); err != nil {
		return fmt.Errorf("invalid CPU request: %w", err)
	}

	if _, err := resource.ParseQuantity(c.MemoryRequest); err != nil {
		return fmt.Errorf("invalid memory request: %w", err)
	}

	if _, err := resource.ParseQuantity(c.CPULimit); err != nil {
		return fmt.Errorf("invalid CPU limit: %w", err)
	}

	if _, err := resource.ParseQuantity(c.MemoryLimit); err != nil {
		return fmt.Errorf("invalid memory limit: %w", err)
	}

	return nil
}
