// Package k8s provides a client for interacting with the Kubernetes API.
package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Client encapsulates Kubernetes client functionality including dynamic,
// discovery, and metrics clients.
// It also caches API resource information for performance.
type Client struct {
	clientset        *kubernetes.Clientset
	dynamicClient    dynamic.Interface
	discoveryClient  *discovery.DiscoveryClient
	metricsClientset *metricsclientset.Clientset // Add metrics client
	restConfig       *rest.Config
	apiResourceCache map[string]*schema.GroupVersionResource
	cacheLock        sync.RWMutex
}

// BuildKubernetesConfig builds a Kubernetes REST config using multiple authentication methods.
// It supports the following methods in order of priority:
// 1. Kubeconfig content from KUBECONFIG_DATA environment variable
// 2. API server URL and token from KUBERNETES_SERVER and KUBERNETES_TOKEN environment variables
// 3. In-cluster authentication (service account token from /var/run/secrets/kubernetes.io/serviceaccount/token)
// 4. Kubeconfig file path (provided or default ~/.kube/config)
func BuildKubernetesConfig(kubeconfigPath string) (*rest.Config, error) {
	// Method 1: Kubeconfig content from environment variable
	if kubeconfigData := os.Getenv("KUBECONFIG_DATA"); kubeconfigData != "" {
		// Load kubeconfig from bytes
		configObj, err := clientcmd.Load([]byte(kubeconfigData))
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from KUBECONFIG_DATA: %w", err)
		}
		// Build REST config from the loaded config
		clientConfig := clientcmd.NewDefaultClientConfig(*configObj, &clientcmd.ConfigOverrides{})
		config, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build REST config from KUBECONFIG_DATA: %w", err)
		}
		return config, nil
	}

	// Method 2: API server URL and token from environment variables
	if serverURL := os.Getenv("KUBERNETES_SERVER"); serverURL != "" {
		token := os.Getenv("KUBERNETES_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("KUBERNETES_TOKEN environment variable is required when KUBERNETES_SERVER is set")
		}

		config := &rest.Config{
			Host:        serverURL,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: os.Getenv("KUBERNETES_INSECURE") == "true",
			},
		}

		// Set CA certificate if provided
		if caCert := os.Getenv("KUBERNETES_CA_CERT"); caCert != "" {
			config.TLSClientConfig.CAData = []byte(caCert)
		} else if caCertPath := os.Getenv("KUBERNETES_CA_CERT_PATH"); caCertPath != "" {
			caCertData, err := os.ReadFile(caCertPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA certificate from %s: %w", caCertPath, err)
			}
			config.TLSClientConfig.CAData = caCertData
		}

		return config, nil
	}

	// Method 3: In-cluster authentication (service account token)
	// Check if we're running inside a Kubernetes cluster
	serviceAccountTokenPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	if _, err := os.Stat(serviceAccountTokenPath); err == nil {
		// We're in a cluster, use in-cluster config
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
		return config, nil
	}

	// Method 4: Kubeconfig file path (provided or default)
	var kubeconfig string
	if kubeconfigPath != "" {
		kubeconfig = kubeconfigPath
	} else if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		kubeconfig = kubeconfigEnv
	} else if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes configuration: %w", err)
	}

	return config, nil
}

// NewClient creates a new Kubernetes client.
// It initializes the standard clientset, dynamic client, discovery client,
// and metrics client using multiple authentication methods:
// 1. Kubeconfig content from KUBECONFIG_DATA environment variable
// 2. API server URL and token from KUBERNETES_SERVER and KUBERNETES_TOKEN environment variables
// 3. In-cluster authentication (service account token)
// 4. Kubeconfig file path (provided or default ~/.kube/config)
// If kubeconfigPath is empty, it will try to auto-detect the authentication method.
func NewClient(kubeconfigPath string) (*Client, error) {
	config, err := BuildKubernetesConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Initialize metrics client
	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	return &Client{
		clientset:        clientset,
		dynamicClient:    dynamicClient,
		discoveryClient:  discoveryClient,
		metricsClientset: metricsClient, // Assign metrics client
		restConfig:       config,
		apiResourceCache: make(map[string]*schema.GroupVersionResource),
	}, nil
}

// GetAPIResources retrieves all API resource types in the cluster.
// It uses the discovery client to fetch server-preferred resources.
// Filters resources based on includeNamespaceScoped and includeClusterScoped flags.
// Returns a slice of maps, each representing an API resource, or an error.
func (c *Client) GetAPIResources(ctx context.Context, includeNamespaceScoped, includeClusterScoped bool) ([]map[string]interface{}, error) {
	resourceLists, err := c.discoveryClient.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, fmt.Errorf("failed to retrieve API resources: %w", err)
	}

	var resources []map[string]interface{}
	for _, resourceList := range resourceLists {
		for _, resource := range resourceList.APIResources {
			if (resource.Namespaced && !includeNamespaceScoped) || (!resource.Namespaced && !includeClusterScoped) {
				continue
			}
			resources = append(resources, map[string]interface{}{
				"name":         resource.Name,
				"singularName": resource.SingularName,
				"namespaced":   resource.Namespaced,
				"kind":         resource.Kind,
				"group":        resource.Group,
				"version":      resource.Version,
				"verbs":        resource.Verbs,
			})
		}
	}
	return resources, nil
}

// GetResource retrieves detailed information about a specific resource.
// It uses the dynamic client to fetch the resource by kind, name, and namespace.
// It utilizes a cached GroupVersionResource (GVR) for efficiency.
// Returns the unstructured content of the resource as a map, or an error.
func (c *Client) GetResource(ctx context.Context, kind, name, namespace string) (map[string]interface{}, error) {
	gvr, err := c.getCachedGVR(kind)
	if err != nil {
		return nil, err
	}

	var obj *unstructured.Unstructured
	if namespace != "" {
		obj, err = c.dynamicClient.Resource(*gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = c.dynamicClient.Resource(*gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve resource: %w", err)
	}

	return obj.UnstructuredContent(), nil
}

// ListResources lists all instances of a specific resource type.
// It uses the dynamic client and supports filtering by namespace, labelSelector,
// and fieldSelector.
// It utilizes a cached GroupVersionResource (GVR) for efficiency.
// Returns a slice of maps, each representing a resource instance, or an error.
func (c *Client) ListResources(ctx context.Context, kind, namespace, labelSelector, fieldSelector string) ([]map[string]interface{}, error) {
	gvr, err := c.getCachedGVR(kind)
	if err != nil {
		return nil, err
	}

	options := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}

	var list *unstructured.UnstructuredList
	if namespace != "" {
		list, err = c.dynamicClient.Resource(*gvr).Namespace(namespace).List(ctx, options)
	} else {
		list, err = c.dynamicClient.Resource(*gvr).List(ctx, options)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	var resources []map[string]interface{}
	for _, item := range list.Items {
		metadata := item.GetLabels()
		resources = append(resources, map[string]interface{}{
			"name":      item.GetName(),
			"kind":      item.GetKind(),
			"namespace": item.GetNamespace(),
			"labels":    metadata,
		})
	}

	return resources, nil
}

// CreateOrUpdateResource creates a new resource or updates an existing one.
// It parses the provided manifest string into an unstructured object.
// It uses the dynamic client to first attempt an update, and if that fails
// (e.g., resource not found), it attempts to create the resource.
// Requires the resource manifest to include a name.
// Returns the unstructured content of the created/updated resource, or an error.
func (c *Client) CreateOrUpdateResourceJSON(ctx context.Context, namespace, manifestJSON, kind string) (map[string]interface{}, error) {
	// Decode JSON into unstructured object directly (no YAML conversion)

	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(manifestJSON), &obj.Object); err != nil {
		return nil, fmt.Errorf("failed to parse resource manifest JSON: %w", err)
	}

	// Determine the resource GVR
	gvr, err := c.getCachedGVR(kind)
	if err != nil {
		return nil, err
	}

	// Check if ns exists
	_, err = c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("Namespace %s exists\n", namespace)
	}
	if errors.IsNotFound(err) {
		fmt.Printf("Namespace %s does not exist, creating one\n", namespace)
		_, err = c.clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"kubernetes.io/metadata.name": namespace,
				},
				Name: namespace,
			},
			Spec: corev1.NamespaceSpec{
				Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes},
			},
			Status: corev1.NamespaceStatus{
				Phase:      corev1.NamespaceActive,
				Conditions: nil,
			},
		}, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve namespace resource: %w", err)
	}

	obj.SetNamespace(namespace)

	if obj.GetName() == "" {
		return nil, fmt.Errorf("resource name is required")
	}

	resource := c.dynamicClient.Resource(*gvr).Namespace(obj.GetNamespace())

	// Try to patch; if not found, create
	rawJSON := []byte(manifestJSON) // manifestJSON is already JSON
	result, err := resource.Patch(
		ctx,
		obj.GetName(),
		types.MergePatchType,
		rawJSON,
		metav1.PatchOptions{},
	)
	if errors.IsNotFound(err) {
		result, err = resource.Create(ctx, obj, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create or patch resource: %w", err)
	}

	return result.UnstructuredContent(), nil
}

// CreateOrUpdateResourceYAML creates a new resource or updates an existing one from a YAML manifest.
// This function is specifically designed for YAML input and provides optimized YAML parsing.
// It converts the YAML manifest to JSON internally and then uses the dynamic client
// to first attempt an update, and if that fails (e.g., resource not found), it attempts to create the resource.
// Requires the resource manifest to include a name.
// Returns the unstructured content of the created/updated resource, or an error.
//
// Parameters:
//   - ctx: Context for the operation
//   - namespace: Target namespace for the resource (overrides manifest namespace if provided)
//   - yamlManifest: YAML manifest string of the Kubernetes resource
//   - kind: Resource kind (optional, will be inferred from manifest if empty)
//
// Example YAML manifest:
//
//	apiVersion: v1
//	kind: Pod
//	metadata:
//	  name: my-pod
//	  namespace: default
//	spec:
//	  containers:
//	  - name: nginx
//	    image: nginx:latest
func (c *Client) CreateOrUpdateResourceYAML(ctx context.Context, namespace, yamlManifest, kind string) (map[string]interface{}, error) {
	// Convert YAML to JSON
	jsonData, err := yaml.YAMLToJSON([]byte(yamlManifest))
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML manifest: %w", err)
	}

	// Parse the converted JSON into unstructured object
	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal(jsonData, &obj.Object); err != nil {
		return nil, fmt.Errorf("failed to parse converted JSON from YAML manifest: %w", err)
	}

	// Infer kind from manifest if not provided
	resourceKind := kind
	if resourceKind == "" {
		resourceKind = obj.GetKind()
		if resourceKind == "" {
			return nil, fmt.Errorf("resource kind is required: either provide it as a parameter or include it in the YAML manifest")
		}
	}

	// Determine the resource GVR
	gvr, err := c.getCachedGVR(resourceKind)
	if err != nil {
		return nil, err
	}

	// Set namespace if provided (overrides manifest namespace)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}

	if obj.GetName() == "" {
		return nil, fmt.Errorf("resource name is required in YAML manifest")
	}

	resource := c.dynamicClient.Resource(*gvr).Namespace(obj.GetNamespace())

	// Try to patch; if not found, create
	result, err := resource.Patch(
		ctx,
		obj.GetName(),
		types.MergePatchType,
		jsonData,
		metav1.PatchOptions{},
	)
	if errors.IsNotFound(err) {
		result, err = resource.Create(ctx, obj, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create or patch resource from YAML manifest: %w", err)
	}

	return result.UnstructuredContent(), nil
}

// DeleteResource deletes a specific resource.
// It uses the dynamic client to delete the resource by kind, name, and namespace.
// It utilizes a cached GroupVersionResource (GVR) for efficiency.
// Returns an error if the deletion fails.
func (c *Client) DeleteResource(ctx context.Context, kind, name, namespace string) error {
	gvr, err := c.getCachedGVR(kind)
	if err != nil {
		return err
	}

	var deleteErr error
	if namespace != "" {
		deleteErr = c.dynamicClient.Resource(*gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		deleteErr = c.dynamicClient.Resource(*gvr).Delete(ctx, name, metav1.DeleteOptions{})
	}
	if deleteErr != nil {
		return fmt.Errorf("failed to delete resource: %w", deleteErr)
	}
	return nil
}

// getCachedGVR retrieves the GroupVersionResource for a given kind, using a cache for performance
func (c *Client) getCachedGVR(kind string) (*schema.GroupVersionResource, error) {
	c.cacheLock.RLock()
	if gvr, exists := c.apiResourceCache[kind]; exists {
		c.cacheLock.RUnlock()
		return gvr, nil
	}
	c.cacheLock.RUnlock()

	// Cache miss; fetch from discovery client
	resourceLists, err := c.discoveryClient.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, fmt.Errorf("failed to retrieve API resources: %w", err)
	}

	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range resourceList.APIResources {
			if resource.Kind == kind {
				gvr := &schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: resource.Name,
				}
				c.cacheLock.Lock()
				c.apiResourceCache[kind] = gvr
				c.cacheLock.Unlock()
				return gvr, nil
			}
		}
	}

	return nil, fmt.Errorf("resource type %s not found", kind)
}

// DescribeResource retrieves detailed information about a specific resource, similar to GetResource.
// It uses the dynamic client to fetch the resource by kind, name, and namespace.
// It utilizes a cached GroupVersionResource (GVR) for efficiency.
// Returns the unstructured content of the resource as a map, or an error.
// Note: This function currently has the same implementation as GetResource.
func (c *Client) DescribeResource(ctx context.Context, kind, name, namespace string) (map[string]interface{}, error) {
	gvr, err := c.getCachedGVR(kind)
	if err != nil {
		return nil, err
	}

	var obj *unstructured.Unstructured
	if namespace != "" {
		obj, err = c.dynamicClient.Resource(*gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = c.dynamicClient.Resource(*gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve resource: %w", err)
	}

	return obj.UnstructuredContent(), nil
}

// GetPodsLogs retrieves the logs for a specific pod.
// It uses the corev1 clientset to fetch logs, limiting to the last 100 lines by default.
// If containerName is provided, it gets logs for that specific container.
// If containerName is empty and the pod has multiple containers, it gets logs from all containers.
// Returns the logs as a string, or an error.
func (c *Client) GetPodsLogs(ctx context.Context, namespace, containerName, podName string) (string, error) {
	tailLines := int64(100)
	podLogOptions := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	// If container name is provided, use it
	if containerName != "" {
		podLogOptions.Container = containerName
		req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)
		logs, err := req.Stream(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get logs for container '%s': %w", containerName, err)
		}
		defer logs.Close()

		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, logs); err != nil {
			return "", fmt.Errorf("failed to read logs: %w", err)
		}
		return buf.String(), nil
	}

	// If no container name provided, first get the pod to check its containers
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod details: %w", err)
	}

	// If the pod has only one container, get logs from that container
	if len(pod.Spec.Containers) == 1 {
		req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)
		logs, err := req.Stream(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get logs: %w", err)
		}
		defer logs.Close()

		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, logs); err != nil {
			return "", fmt.Errorf("failed to read logs: %w", err)
		}
		return buf.String(), nil
	}

	// If the pod has multiple containers, get logs from each container
	var allLogs strings.Builder
	for _, container := range pod.Spec.Containers {
		containerLogOptions := podLogOptions.DeepCopy()
		containerLogOptions.Container = container.Name

		req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, containerLogOptions)
		logs, err := req.Stream(ctx)
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("\n--- Error getting logs for container %s: %v ---\n", container.Name, err))
			continue
		}

		allLogs.WriteString(fmt.Sprintf("\n--- Logs for container %s ---\n", container.Name))
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, logs)
		logs.Close()

		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Error reading logs: %v\n", err))
		} else {
			allLogs.WriteString(buf.String())
		}
	}

	return allLogs.String(), nil
}

// GetPodMetrics retrieves CPU and Memory metrics for a specific pod.
// It uses the metrics clientset to fetch pod metrics.
// Returns a map containing pod metadata and container metrics, or an error.
func (c *Client) GetPodMetrics(ctx context.Context, namespace, podName string) (map[string]interface{}, error) {
	podMetrics, err := c.metricsClientset.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics for pod '%s' in namespace '%s': %w", podName, namespace, err)
	}

	metricsResult := map[string]interface{}{
		"podName":    podName,
		"namespace":  namespace,
		"timestamp":  podMetrics.Timestamp.Time,
		"window":     podMetrics.Window.Duration.String(),
		"containers": []map[string]interface{}{},
	}

	containerMetricsList := []map[string]interface{}{}
	for _, container := range podMetrics.Containers {
		containerMetrics := map[string]interface{}{
			"name":   container.Name,
			"cpu":    container.Usage.Cpu().String(),    // Format Quantity
			"memory": container.Usage.Memory().String(), // Format Quantity
		}
		containerMetricsList = append(containerMetricsList, containerMetrics)
	}
	metricsResult["containers"] = containerMetricsList

	return metricsResult, nil
}

// GetNodeMetrics retrieves CPU and Memory metrics for a specific Node.
// It uses the metrics clientset to fetch node metrics.
// Returns a map containing node metadata and resource usage, or an error.
func (c *Client) GetNodeMetrics(ctx context.Context, nodeName string) (map[string]interface{}, error) {
	nodeMetrics, err := c.metricsClientset.MetricsV1beta1().NodeMetricses().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics for node '%s': %w", nodeName, err)
	}

	metricsResult := map[string]interface{}{
		"nodeName":  nodeName,
		"timestamp": nodeMetrics.Timestamp.Time,
		"window":    nodeMetrics.Window.Duration.String(),
		"usage": map[string]string{
			"cpu":    nodeMetrics.Usage.Cpu().String(),    // Format Quantity
			"memory": nodeMetrics.Usage.Memory().String(), // Format Quantity
		},
	}

	return metricsResult, nil
}

// GetEvents retrieves events for a specific namespace or all namespaces.
// It uses the corev1 clientset to fetch events.
// Returns a slice of maps, each representing an event, or an error.
func (c *Client) GetEvents(ctx context.Context, namespace string) ([]map[string]interface{}, error) {
	var eventList *corev1.EventList
	var err error

	if namespace != "" {
		eventList, err = c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	} else {
		eventList, err = c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve events: %w", err)
	}

	var events []map[string]interface{}
	for _, event := range eventList.Items {
		events = append(events, map[string]interface{}{
			"name":      event.Name,
			"namespace": event.Namespace,
			"reason":    event.Reason,
			"message":   event.Message,
			"source":    event.Source.Component,
			"type":      event.Type,
			"count":     event.Count,
			"firstTime": event.FirstTimestamp.Time,
			"lastTime":  event.LastTimestamp.Time,
		})
	}
	return events, nil
}

// GetIngresses retrieves ingresses and returns specific fields: name, namespace, hosts, paths, and backend services.
// It uses the networking.k8s.io/v1 clientset to fetch ingresses.
// Returns a slice of maps, each representing an ingress with the requested fields, or an error.
func (c *Client) GetIngresses(ctx context.Context, host string) ([]map[string]interface{}, error) {
	ingresses, err := c.clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ingresses: %w", err)
	}

	var ingressList []map[string]interface{}
	for _, ingress := range ingresses.Items {
		// Check if this ingress has any rules matching the given host
		hasMatchingHost := false
		var matchingPaths []string
		var matchingBackendServices []string

		for _, rule := range ingress.Spec.Rules {
			// If host filter is specified, only process rules matching the host
			if host != "" && rule.Host != host {
				continue
			}

			// If we reach here, either no host filter or host matches
			if host == "" || rule.Host == host {
				hasMatchingHost = true

				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						matchingPaths = append(matchingPaths, path.Path)

						// Extract backend service information
						if path.Backend.Service != nil {
							matchingBackendServices = append(matchingBackendServices, path.Backend.Service.Name)
						}
					}
				}
			}
		}

		// Only add this ingress if it has matching rules
		if hasMatchingHost {
			ingressList = append(ingressList, map[string]interface{}{
				"name":            ingress.Name,
				"namespace":       ingress.Namespace,
				"paths":           matchingPaths,
				"backendServices": matchingBackendServices,
			})
		}
	}

	return ingressList, nil
}

// RolloutRestart restarts any Kubernetes workload with a pod template (Deployment, DaemonSet, StatefulSet, etc.).
// It patches the spec.template.metadata.annotations with the current timestamp.
// Returns the patched resource content or an error if the resource doesn't support rollout restart.
func (c *Client) RolloutRestart(ctx context.Context, kind, name, namespace string) (map[string]interface{}, error) {
	gvr, err := c.getCachedGVR(kind)
	if err != nil {
		return nil, fmt.Errorf("failed to get GVR for kind %s: %w", kind, err)
	}

	resource := c.dynamicClient.Resource(*gvr).Namespace(namespace)

	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().Format(time.RFC3339),
	))

	result, err := resource.Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to rollout restart %s %s/%s: %w", kind, namespace, name, err)
	}

	content := result.UnstructuredContent()
	spec, found, _ := unstructured.NestedMap(content, "spec", "template")
	if !found || spec == nil {
		return nil, fmt.Errorf("resource kind %s does not support rollout restart (no spec.template)", kind)
	}

	return content, nil
}
