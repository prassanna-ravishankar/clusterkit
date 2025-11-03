package knative

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// IngressConfig contains configuration for creating an Ingress
type IngressConfig struct {
	Name            string
	Namespace       string
	ServiceName     string
	Domains         []string
	ClusterIssuer   string // cert-manager ClusterIssuer (e.g., "letsencrypt-prod")
	IngressClass    string // Ingress class (e.g., "nginx")
	Annotations     map[string]string
}

// NewIngressConfig creates a new IngressConfig with defaults
func NewIngressConfig(name, namespace, serviceName string, domains []string) *IngressConfig {
	return &IngressConfig{
		Name:          name,
		Namespace:     namespace,
		ServiceName:   serviceName,
		Domains:       domains,
		ClusterIssuer: "letsencrypt-prod",
		IngressClass:  "nginx",
		Annotations:   make(map[string]string),
	}
}

// GenerateIngress creates an Ingress with TLS for the service
func (c *IngressConfig) GenerateIngress() (*networkingv1.Ingress, error) {
	if len(c.Domains) == 0 {
		return nil, fmt.Errorf("at least one domain is required")
	}

	pathType := networkingv1.PathTypePrefix

	// Initialize annotations
	annotations := map[string]string{
		"cert-manager.io/cluster-issuer": c.ClusterIssuer,
	}
	for k, v := range c.Annotations {
		annotations[k] = v
	}

	// Create TLS configuration
	tls := []networkingv1.IngressTLS{
		{
			Hosts:      c.Domains,
			SecretName: fmt.Sprintf("%s-tls", c.Name),
		},
	}

	// Create rules for each domain
	rules := make([]networkingv1.IngressRule, 0, len(c.Domains))
	for _, domain := range c.Domains {
		rules = append(rules, networkingv1.IngressRule{
			Host: domain,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: c.ServiceName,
									Port: networkingv1.ServiceBackendPort{
										Number: 80,
									},
								},
							},
						},
					},
				},
			},
		})
	}

	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.Name,
			Namespace:   c.Namespace,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &c.IngressClass,
			TLS:              tls,
			Rules:            rules,
		},
	}

	return ingress, nil
}

// ToYAML converts an Ingress to YAML format
func IngressToYAML(ingress *networkingv1.Ingress) ([]byte, error) {
	return yaml.Marshal(ingress)
}

// ToYAMLString converts an Ingress to YAML string
func IngressToYAMLString(ingress *networkingv1.Ingress) (string, error) {
	data, err := IngressToYAML(ingress)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
