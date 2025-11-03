package database

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PostgresConfig contains configuration for PostgreSQL database
type PostgresConfig struct {
	Name         string
	Namespace    string
	StorageSize  string
	CPURequest   string
	MemoryRequest string
	CPULimit     string
	MemoryLimit  string

	// Generated credentials
	Database string
	Username string
	Password string
}

// NewPostgresConfig creates a new PostgreSQL configuration with defaults
func NewPostgresConfig(name, namespace string) *PostgresConfig {
	return &PostgresConfig{
		Name:          name,
		Namespace:     namespace,
		StorageSize:   "10Gi",
		CPURequest:    "100m",
		MemoryRequest: "256Mi",
		CPULimit:      "1000m",
		MemoryLimit:   "512Mi",
		Database:      name,
		Username:      name,
		Password:      generateSecurePassword(),
	}
}

// GenerateStatefulSet creates a StatefulSet for PostgreSQL
func (c *PostgresConfig) GenerateStatefulSet() *appsv1.StatefulSet {
	replicas := int32(1)

	labels := map[string]string{
		"app":                          c.Name,
		"app.kubernetes.io/name":       c.Name,
		"app.kubernetes.io/component":  "database",
		"app.kubernetes.io/managed-by": "clusterkit",
	}

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: c.Name,
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
							Name:  "postgres",
							Image: "postgres:16-alpine",
							Ports: []corev1.ContainerPort{
								{
									Name:          "postgres",
									ContainerPort: 5432,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POSTGRES_DB",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: c.Name + "-credentials",
											},
											Key: "database",
										},
									},
								},
								{
									Name: "POSTGRES_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: c.Name + "-credentials",
											},
											Key: "username",
										},
									},
								},
								{
									Name: "POSTGRES_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: c.Name + "-credentials",
											},
											Key: "password",
										},
									},
								},
								{
									Name:  "PGDATA",
									Value: "/var/lib/postgresql/data/pgdata",
								},
							},
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/postgresql/data",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"pg_isready", "-U", c.Username},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"pg_isready", "-U", c.Username},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: int64Ptr(999), // postgres user
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(c.StorageSize),
							},
						},
						StorageClassName: stringPtr("standard-rwo"),
					},
				},
			},
		},
	}
}

// GenerateService creates a Service for PostgreSQL
func (c *PostgresConfig) GenerateService() *corev1.Service {
	labels := map[string]string{
		"app":                         c.Name,
		"app.kubernetes.io/name":      c.Name,
		"app.kubernetes.io/component": "database",
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
					Name:     "postgres",
					Port:     5432,
					Protocol: corev1.ProtocolTCP,
				},
			},
			ClusterIP: "None", // Headless service for StatefulSet
		},
	}
}

// GenerateSecret creates a Secret with database credentials
func (c *PostgresConfig) GenerateSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name + "-credentials",
			Namespace: c.Namespace,
			Labels: map[string]string{
				"app":                          c.Name,
				"app.kubernetes.io/name":       c.Name,
				"app.kubernetes.io/component":  "database",
				"app.kubernetes.io/managed-by": "clusterkit",
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"database": c.Database,
			"username": c.Username,
			"password": c.Password,
			"host":     fmt.Sprintf("%s.%s.svc.cluster.local", c.Name, c.Namespace),
			"port":     "5432",
			"url":      c.GetConnectionURL(),
		},
	}
}

// GetConnectionURL returns the PostgreSQL connection URL
func (c *PostgresConfig) GetConnectionURL() string {
	return fmt.Sprintf("postgresql://%s:%s@%s.%s.svc.cluster.local:5432/%s",
		c.Username, c.Password, c.Name, c.Namespace, c.Database)
}

// generateSecurePassword generates a secure random password
func generateSecurePassword() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:32]
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
