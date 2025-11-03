package apply

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// ApplyClient handles applying Kubernetes manifests
type ApplyClient struct {
	dynamicClient  dynamic.Interface
	discoveryClient discovery.CachedDiscoveryInterface
	mapper         meta.RESTMapper
	config         *rest.Config
}

// ApplyOptions contains options for applying manifests
type ApplyOptions struct {
	Namespace string
	Timeout   time.Duration
	Wait      bool
	DryRun    bool
}

// ApplyResult contains the result of an apply operation
type ApplyResult struct {
	Applied        []AppliedResource
	Failed         []FailedResource
	TotalApplied   int
	TotalFailed    int
	Duration       time.Duration
}

// AppliedResource represents a successfully applied resource
type AppliedResource struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Created    bool // true if created, false if updated
}

// FailedResource represents a failed resource application
type FailedResource struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Error      error
}

// NewApplyClient creates a new ApplyClient from a kubeconfig path
func NewApplyClient(kubeconfig, context string) (*ApplyClient, error) {
	// Build config from kubeconfig
	config, err := buildConfig(kubeconfig, context)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Create cached discovery client
	cachedDiscovery := memory.NewMemCacheClient(discoveryClient)

	// Create REST mapper
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)

	return &ApplyClient{
		dynamicClient:   dynamicClient,
		discoveryClient: cachedDiscovery,
		mapper:          mapper,
		config:          config,
	}, nil
}

// buildConfig builds a Kubernetes client config
func buildConfig(kubeconfig, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		// Use in-cluster config if no kubeconfig specified
		config, err := rest.InClusterConfig()
		if err == nil {
			return config, nil
		}
		// Fall back to default kubeconfig location
		kubeconfig = clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	}

	// Build config from kubeconfig file
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		configOverrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules,
		configOverrides,
	)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// ApplyManifest applies a YAML manifest to the cluster using server-side apply
func (c *ApplyClient) ApplyManifest(ctx context.Context, manifestYAML string, opts ApplyOptions) (*ApplyResult, error) {
	startTime := time.Now()

	result := &ApplyResult{
		Applied: make([]AppliedResource, 0),
		Failed:  make([]FailedResource, 0),
	}

	// Set timeout context
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Decode YAML into unstructured object
	obj := &unstructured.Unstructured{}
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := decoder.Decode([]byte(manifestYAML), nil, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	// Set namespace if specified in options
	if opts.Namespace != "" && obj.GetNamespace() == "" {
		obj.SetNamespace(opts.Namespace)
	}

	// Get REST mapping for the resource
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST mapping for %s: %w", gvk.String(), err)
	}

	// Get dynamic resource interface
	var resourceInterface dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}
		resourceInterface = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		resourceInterface = c.dynamicClient.Resource(mapping.Resource)
	}

	// Check if resource exists
	existingResource, err := resourceInterface.Get(ctx, obj.GetName(), metav1.GetOptions{})
	resourceExists := err == nil && existingResource != nil
	created := !resourceExists

	// Apply resource using server-side apply
	applyOpts := metav1.ApplyOptions{
		FieldManager: "clusterkit-cli",
		Force:        true,
	}

	if opts.DryRun {
		applyOpts.DryRun = []string{metav1.DryRunAll}
	}

	appliedObj, err := resourceInterface.Apply(ctx, obj.GetName(), obj, applyOpts)
	if err != nil {
		failedResource := FailedResource{
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
			Error:      err,
		}
		result.Failed = append(result.Failed, failedResource)
		result.TotalFailed++
		return result, fmt.Errorf("failed to apply resource: %w", err)
	}

	// Record successful application
	appliedResource := AppliedResource{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Namespace:  appliedObj.GetNamespace(),
		Name:       appliedObj.GetName(),
		Created:    created,
	}
	result.Applied = append(result.Applied, appliedResource)
	result.TotalApplied++

	result.Duration = time.Since(startTime)
	return result, nil
}

// WaitForDeployment waits for a resource to be ready
func (c *ApplyClient) WaitForDeployment(ctx context.Context, apiVersion, kind, namespace, name string, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Parse API version
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return fmt.Errorf("failed to parse API version %s: %w", apiVersion, err)
	}

	// Get REST mapping
	gvk := gv.WithKind(kind)
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to get REST mapping for %s: %w", gvk.String(), err)
	}

	// Get dynamic resource interface
	var resourceInterface dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if namespace == "" {
			namespace = "default"
		}
		resourceInterface = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		resourceInterface = c.dynamicClient.Resource(mapping.Resource)
	}

	// Poll for ready status
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s/%s to be ready: %w", kind, name, ctx.Err())
		case <-ticker.C:
			obj, err := resourceInterface.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					continue // Resource not yet created
				}
				return fmt.Errorf("failed to get %s/%s: %w", kind, name, err)
			}

			// Check if resource is ready based on kind
			ready, err := c.isResourceReady(obj)
			if err != nil {
				return fmt.Errorf("failed to check readiness: %w", err)
			}

			if ready {
				return nil
			}
		}
	}
}

// isResourceReady checks if a resource is ready based on its status conditions
func (c *ApplyClient) isResourceReady(obj *unstructured.Unstructured) (bool, error) {
	// Get status conditions
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return false, fmt.Errorf("failed to get status conditions: %w", err)
	}

	if !found || len(conditions) == 0 {
		return false, nil
	}

	// Check for Ready condition
	for _, condition := range conditions {
		condMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}

		condType, found, err := unstructured.NestedString(condMap, "type")
		if err != nil || !found {
			continue
		}

		if condType == "Ready" {
			condStatus, found, err := unstructured.NestedString(condMap, "status")
			if err != nil || !found {
				return false, nil
			}
			return condStatus == "True", nil
		}
	}

	return false, nil
}

// RollbackOnFailure deletes resources that were applied
func (c *ApplyClient) RollbackOnFailure(ctx context.Context, appliedResources []AppliedResource) error {
	var errors []error

	// Delete resources in reverse order
	for i := len(appliedResources) - 1; i >= 0; i-- {
		resource := appliedResources[i]

		// Parse API version
		gv, err := schema.ParseGroupVersion(resource.APIVersion)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to parse API version for %s/%s: %w", resource.Kind, resource.Name, err))
			continue
		}

		// Get REST mapping
		gvk := gv.WithKind(resource.Kind)
		mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get REST mapping for %s/%s: %w", resource.Kind, resource.Name, err))
			continue
		}

		// Get dynamic resource interface
		var resourceInterface dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			namespace := resource.Namespace
			if namespace == "" {
				namespace = "default"
			}
			resourceInterface = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
		} else {
			resourceInterface = c.dynamicClient.Resource(mapping.Resource)
		}

		// Delete resource
		err = resourceInterface.Delete(ctx, resource.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			errors = append(errors, fmt.Errorf("failed to delete %s/%s: %w", resource.Kind, resource.Name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with %d errors: %v", len(errors), errors)
	}

	return nil
}

// GetServiceURL retrieves the URL for a service (from Ingress or LoadBalancer)
func (c *ApplyClient) GetServiceURL(ctx context.Context, namespace, serviceName string) (string, error) {
	// Try to get Ingress first
	ingressInterface := c.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}).Namespace(namespace)

	ingresses, err := ingressInterface.List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ing := range ingresses.Items {
			// Check if ingress references this service
			rules, found, err := unstructured.NestedSlice(ing.Object, "spec", "rules")
			if err != nil || !found {
				continue
			}

			for _, rule := range rules {
				ruleMap, ok := rule.(map[string]interface{})
				if !ok {
					continue
				}

				host, found, err := unstructured.NestedString(ruleMap, "host")
				if err != nil || !found {
					continue
				}

				// Check TLS configuration
				tls, found, err := unstructured.NestedSlice(ing.Object, "spec", "tls")
				protocol := "http"
				if err == nil && found && len(tls) > 0 {
					protocol = "https"
				}

				return fmt.Sprintf("%s://%s", protocol, host), nil
			}
		}
	}

	// Fallback to LoadBalancer service
	serviceInterface := c.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}).Namespace(namespace)

	svc, err := serviceInterface.Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get service URL: %w", err)
	}

	// Check for LoadBalancer ingress
	ingress, found, err := unstructured.NestedSlice(svc.Object, "status", "loadBalancer", "ingress")
	if err != nil || !found || len(ingress) == 0 {
		return "", fmt.Errorf("service does not have an external IP yet")
	}

	ingressMap, ok := ingress[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid ingress format")
	}

	ip, found, err := unstructured.NestedString(ingressMap, "ip")
	if err != nil || !found {
		hostname, found, err := unstructured.NestedString(ingressMap, "hostname")
		if err != nil || !found {
			return "", fmt.Errorf("no IP or hostname found in service status")
		}
		return fmt.Sprintf("http://%s", hostname), nil
	}

	return fmt.Sprintf("http://%s", ip), nil
}
