package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubeconfigManager handles kubeconfig operations
type KubeconfigManager struct {
	kubeconfigPath string
}

// NewKubeconfigManager creates a new kubeconfig manager
func NewKubeconfigManager(kubeconfigPath string) *KubeconfigManager {
	if kubeconfigPath == "" {
		// Use default kubeconfig path
		home, err := os.UserHomeDir()
		if err == nil {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		} else {
			kubeconfigPath = clientcmd.RecommendedHomeFile
		}
	}

	return &KubeconfigManager{
		kubeconfigPath: kubeconfigPath,
	}
}

// GetKubeconfigPath returns the kubeconfig file path
func (km *KubeconfigManager) GetKubeconfigPath() string {
	return km.kubeconfigPath
}

// LoadConfig loads the kubeconfig file
func (km *KubeconfigManager) LoadConfig() (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(km.kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", km.kubeconfigPath, err)
	}
	return config, nil
}

// GetCurrentContext returns the current context name
func (km *KubeconfigManager) GetCurrentContext() (string, error) {
	config, err := km.LoadConfig()
	if err != nil {
		return "", err
	}

	if config.CurrentContext == "" {
		return "", fmt.Errorf("no current context set in kubeconfig")
	}

	return config.CurrentContext, nil
}

// ListContexts returns all available contexts
func (km *KubeconfigManager) ListContexts() ([]string, error) {
	config, err := km.LoadConfig()
	if err != nil {
		return nil, err
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}

	return contexts, nil
}

// GetContextInfo returns information about a specific context
func (km *KubeconfigManager) GetContextInfo(contextName string) (*ContextInfo, error) {
	config, err := km.LoadConfig()
	if err != nil {
		return nil, err
	}

	context, ok := config.Contexts[contextName]
	if !ok {
		return nil, fmt.Errorf("context %s not found in kubeconfig", contextName)
	}

	cluster, ok := config.Clusters[context.Cluster]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found for context %s", context.Cluster, contextName)
	}

	return &ContextInfo{
		Name:      contextName,
		Cluster:   context.Cluster,
		Namespace: context.Namespace,
		Server:    cluster.Server,
	}, nil
}

// SetCurrentContext sets the current context in kubeconfig
func (km *KubeconfigManager) SetCurrentContext(contextName string) error {
	config, err := km.LoadConfig()
	if err != nil {
		return err
	}

	// Verify context exists
	if _, ok := config.Contexts[contextName]; !ok {
		return fmt.Errorf("context %s not found in kubeconfig", contextName)
	}

	// Set current context
	config.CurrentContext = contextName

	// Save config
	if err := clientcmd.WriteToFile(*config, km.kubeconfigPath); err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	return nil
}

// ValidateContext validates that a context exists and is accessible
func (km *KubeconfigManager) ValidateContext(contextName string) error {
	// Check context exists
	_, err := km.GetContextInfo(contextName)
	if err != nil {
		return err
	}

	// Try to create a client and test connection
	client, err := NewClient(km.kubeconfigPath, contextName)
	if err != nil {
		return fmt.Errorf("failed to create client for context %s: %w", contextName, err)
	}

	// Test connection
	if err := client.TestConnection(); err != nil {
		return fmt.Errorf("failed to connect to cluster with context %s: %w", contextName, err)
	}

	return nil
}

// ContextInfo contains information about a Kubernetes context
type ContextInfo struct {
	Name      string
	Cluster   string
	Namespace string
	Server    string
}

// GetClusterContexts returns contexts that match a specific cluster pattern (e.g., GKE contexts)
func (km *KubeconfigManager) GetClusterContexts(clusterName string) ([]string, error) {
	config, err := km.LoadConfig()
	if err != nil {
		return nil, err
	}

	matches := []string{}
	for name, context := range config.Contexts {
		if context.Cluster == clusterName {
			matches = append(matches, name)
		}
	}

	return matches, nil
}

// EnsureKubeconfigExists checks if kubeconfig file exists
func (km *KubeconfigManager) EnsureKubeconfigExists() error {
	if _, err := os.Stat(km.kubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig file not found at %s", km.kubeconfigPath)
	}
	return nil
}
