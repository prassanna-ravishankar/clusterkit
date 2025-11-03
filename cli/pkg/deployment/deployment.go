package deployment

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeploymentConfig contains configuration for a traditional Kubernetes Deployment
type DeploymentConfig struct {
	Name      string
	Namespace string
	Image     string
	Domains   []string
	Env       []corev1.EnvVar

	Replicas    int32
	MinReplicas int32
	MaxReplicas int32

	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string

	Port int32
}

// NewDeploymentConfig creates a new DeploymentConfig with defaults
func NewDeploymentConfig(name, namespace, image string) *DeploymentConfig {
	return &DeploymentConfig{
		Name:          name,
		Namespace:     namespace,
		Image:         image,
		Replicas:      2,
		MinReplicas:   2,
		MaxReplicas:   10,
		CPURequest:    "100m",
		MemoryRequest: "128Mi",
		CPULimit:      "1000m",
		MemoryLimit:   "256Mi",
		Port:          8080,
	}
}

// GenerateDeployment creates a Kubernetes Deployment
func (c *DeploymentConfig) GenerateDeployment() *appsv1.Deployment {
	labels := map[string]string{
		"app":                          c.Name,
		"app.kubernetes.io/name":       c.Name,
		"app.kubernetes.io/managed-by": "clusterkit",
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &c.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  c.Name,
							Image: c.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: c.Port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: c.Env,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(c.CPURequest),
									corev1.ResourceMemory: resource.MustParse(c.MemoryRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(c.CPULimit),
									corev1.ResourceMemory: resource.MustParse(c.MemoryLimit),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(int(c.Port)),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(int(c.Port)),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
}

// GenerateService creates a Service for the Deployment
func (c *DeploymentConfig) GenerateService() *corev1.Service {
	labels := map[string]string{
		"app":                    c.Name,
		"app.kubernetes.io/name": c.Name,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(int(c.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// GenerateIngress creates an Ingress with TLS
func (c *DeploymentConfig) GenerateIngress() *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingressClass := "nginx"

	annotations := map[string]string{
		"cert-manager.io/cluster-issuer":                 "letsencrypt-prod",
		"external-dns.alpha.kubernetes.io/hostname":      c.Domains[0],
		"nginx.ingress.kubernetes.io/ssl-redirect":       "true",
		"nginx.ingress.kubernetes.io/force-ssl-redirect": "true",
	}

	// Add all domains to ExternalDNS annotation
	if len(c.Domains) > 1 {
		domainList := ""
		for i, d := range c.Domains {
			if i > 0 {
				domainList += ","
			}
			domainList += d
		}
		annotations["external-dns.alpha.kubernetes.io/hostname"] = domainList
	}

	tls := []networkingv1.IngressTLS{
		{
			Hosts:      c.Domains,
			SecretName: fmt.Sprintf("%s-tls", c.Name),
		},
	}

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
									Name: c.Name,
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

	return &networkingv1.Ingress{
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
			IngressClassName: &ingressClass,
			TLS:              tls,
			Rules:            rules,
		},
	}
}

// GenerateHPA creates a HorizontalPodAutoscaler
func (c *DeploymentConfig) GenerateHPA() *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2",
			Kind:       "HorizontalPodAutoscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       c.Name,
			},
			MinReplicas: &c.MinReplicas,
			MaxReplicas: c.MaxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: int32Ptr(70),
						},
					},
				},
			},
		},
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
