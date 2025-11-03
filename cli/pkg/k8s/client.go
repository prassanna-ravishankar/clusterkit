package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset with additional functionality
type Client struct {
	Clientset *kubernetes.Clientset
	Config    *rest.Config
	Context   string
}

// ClientInterface defines methods for Kubernetes operations (enables mocking)
type ClientInterface interface {
	GetClientset() *kubernetes.Clientset
	TestConnection() error
	GetServerVersion() (string, error)
}

// NewClient creates a new Kubernetes client from kubeconfig
func NewClient(kubeconfigPath string, contextName string) (*Client, error) {
	config, err := buildConfig(kubeconfigPath, contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Config:    config,
		Context:   contextName,
	}, nil
}

// NewClientFromConfig creates a client from an existing rest.Config
func NewClientFromConfig(config *rest.Config) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Config:    config,
	}, nil
}

// buildConfig builds a Kubernetes client config from kubeconfig
func buildConfig(kubeconfigPath string, contextName string) (*rest.Config, error) {
	// Build config from kubeconfig file
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		configOverrides.CurrentContext = contextName
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return config, nil
}

// GetClientset returns the underlying Kubernetes clientset
func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.Clientset
}

// TestConnection tests the connection to the Kubernetes cluster
func (c *Client) TestConnection() error {
	ctx := context.TODO()

	// Try to get server version as a connectivity test
	_, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	// Try to list namespaces to verify RBAC permissions
	_, err = c.Clientset.CoreV1().Namespaces().List(ctx, v1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to list namespaces (check permissions): %w", err)
	}

	return nil
}

// GetServerVersion returns the Kubernetes server version
func (c *Client) GetServerVersion() (string, error) {
	version, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}

	return version.GitVersion, nil
}

// GetClusterInfo returns information about the connected cluster
func (c *Client) GetClusterInfo() (*ClusterInfo, error) {
	version, err := c.GetServerVersion()
	if err != nil {
		return nil, err
	}

	// Get cluster endpoint
	host := c.Config.Host

	return &ClusterInfo{
		Version:  version,
		Endpoint: host,
		Context:  c.Context,
	}, nil
}

// ClusterInfo contains information about a Kubernetes cluster
type ClusterInfo struct {
	Version  string
	Endpoint string
	Context  string
}
