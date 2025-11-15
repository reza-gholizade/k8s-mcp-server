package helm

import (
	"context"
	"fmt"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"log"
	"os"
	"path/filepath"

	"github.com/reza-gholizade/k8s-mcp-server/pkg/k8s"
)

// Client wraps Helm operations
type Client struct {
	settings         *cli.EnvSettings
	restConfig       *rest.Config
	k8sClient        kubernetes.Interface
	restClientGetter genericclioptions.RESTClientGetter
}

// customRESTClientGetter is a custom RESTClientGetter that uses a pre-built rest.Config
// instead of reading from kubeconfig files. This ensures Helm uses the same authentication
// method that was used to build the restConfig (KUBECONFIG_DATA, KUBERNETES_SERVER/TOKEN, etc.)
type customRESTClientGetter struct {
	restConfig *rest.Config
}

// ToRESTConfig returns the pre-built REST config
func (g *customRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.restConfig, nil
}

// ToRawKubeConfigLoader returns a clientcmd.ClientConfig that uses the pre-built config
func (g *customRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return &customClientConfig{restConfig: g.restConfig}
}

// ToDiscoveryClient returns a discovery client using the pre-built REST config
func (g *customRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(g.restConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(discoveryClient), nil
}

// ToRESTMapper returns a REST mapper using the discovery client
func (g *customRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient, nil)
	return expander, nil
}

// customClientConfig implements clientcmd.ClientConfig interface
type customClientConfig struct {
	restConfig *rest.Config
}

// RawConfig returns an empty api.Config since we're using a direct rest.Config
func (c *customClientConfig) RawConfig() (api.Config, error) {
	return api.Config{}, fmt.Errorf("raw config not available when using direct REST config")
}

// ClientConfig returns the pre-built REST config
func (c *customClientConfig) ClientConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

// Namespace returns the default namespace from the config
func (c *customClientConfig) Namespace() (string, bool, error) {
	return "default", false, nil
}

// ConfigAccess returns nil as we don't use file-based config access
func (c *customClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

// NewClient creates a new Helm client.
// It uses the same authentication methods as the Kubernetes client:
// 1. Kubeconfig content from KUBECONFIG_DATA environment variable
// 2. API server URL and token from KUBERNETES_SERVER and KUBERNETES_TOKEN environment variables
// 3. In-cluster authentication (service account token)
// 4. Kubeconfig file path (provided or default ~/.kube/config)
func NewClient(kubeconfig string) (*Client, error) {
	settings := cli.New()

	// Get Kubernetes REST config using the shared config builder
	restConfig, err := k8s.BuildKubernetesConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Create a custom RESTClientGetter that uses our pre-built restConfig
	// This ensures Helm uses the same authentication method (KUBECONFIG_DATA, 
	// KUBERNETES_SERVER/TOKEN, in-cluster, etc.) instead of trying to read from
	// settings.KubeConfig which may not be set or may point to a different config.
	restClientGetter := &customRESTClientGetter{restConfig: restConfig}

	// Set kubeconfig path in settings if provided (for Helm's internal use in other contexts)
	// Note: This is mainly for compatibility, but Helm operations will use restClientGetter
	if kubeconfig != "" {
		settings.KubeConfig = kubeconfig
	} else if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		settings.KubeConfig = kubeconfigEnv
	}

	// Create Kubernetes client
	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Client{
		settings:         settings,
		restConfig:       restConfig,
		k8sClient:        k8sClient,
		restClientGetter: restClientGetter,
	}, nil
}

func (c *Client) InstallChart(ctx context.Context, namespace, releaseName, chartName, repoURL string, values map[string]interface{}) (*release.Release, error) {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = namespace
	client.ReleaseName = releaseName
	client.CreateNamespace = true
	cln, err := registry.NewClient(
		registry.ClientOptDebug(true),
		registry.ClientOptCredentialsFile(""),
		registry.ClientOptEnableCache(false))

	if err != nil {
		return nil, fmt.Errorf("failed to initialize registry: %w", err)
	}
	fmt.Println("Registry client created successfully:", cln)

	if values == nil {
		values = make(map[string]interface{})
	}

	// If repoURL is provided, add it to settings or append to chartName accordingly
	if repoURL != "" {
		client.RepoURL = repoURL
	}

	// Locate the chart (resolves repo/chart or OCI)
	chartPath, err := client.LocateChart(chartName, c.settings)
	if err != nil {
		return nil, fmt.Errorf("failed to locate chart: %w", err)
	}

	// Load the chart from the resolved path (can be a URL or OCI reference)
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	// Run the install action
	release, err := client.Run(chart, values)
	if err != nil {
		return nil, fmt.Errorf("failed to install chart: %w", err)
	}

	return release, nil
}

func (c *Client) UpgradeChart(ctx context.Context, namespace, releaseName, chartName string, values map[string]interface{}) (*release.Release, error) {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, fmt.Errorf("failed to initialize action config: %w", err)
	}

	// Create and assign registry client
	regClient, err := registry.NewClient(
		registry.ClientOptDebug(true),
		registry.ClientOptEnableCache(false),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize registry client: %w", err)
	}
	fmt.Println("Registry client created successfully:", regClient)

	client := action.NewUpgrade(actionConfig)
	client.Namespace = namespace

	if values == nil {
		values = make(map[string]interface{})
	}

	// Locate the chart (for both OCI and regular charts)
	chartPath, err := client.LocateChart(chartName, c.settings)
	if err != nil {
		return nil, fmt.Errorf("failed to locate chart: %w", err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	release, err := client.Run(releaseName, chart, values)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade chart: %w", err)
	}

	return release, nil
}

// UninstallChart uninstalls a Helm release
func (c *Client) UninstallChart(ctx context.Context, namespace, releaseName string) error {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewUninstall(actionConfig)
	_, err := client.Run(releaseName)
	if err != nil {
		return fmt.Errorf("failed to uninstall release: %w", err)
	}

	return nil
}

func (c *Client) ListReleases(ctx context.Context, namespace string) ([]*release.Release, error) {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewList(actionConfig)
	client.AllNamespaces = namespace == ""

	releases, err := client.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}
	// remove useless fields from releases
	for _, release := range releases {
		release.Chart.Templates = nil
		release.Chart.Files = nil
		release.Chart.Values = nil
		release.Chart.Schema = nil
		release.Config = nil
		release.Manifest = ""
		release.Chart.Lock = nil
		release.Hooks = nil
	}

	return releases, nil
}

func (c *Client) GetRelease(ctx context.Context, namespace, releaseName string) (*release.Release, error) {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewGet(actionConfig)
	release, err := client.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	return release, nil
}

func (c *Client) GetReleaseHistory(ctx context.Context, namespace, releaseName string) ([]*release.Release, error) {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewHistory(actionConfig)
	releases, err := client.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release history: %w", err)
	}

	return releases, nil
}

// RollbackRelease rolls back a Helm release
func (c *Client) RollbackRelease(ctx context.Context, namespace, releaseName string, revision int) error {
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(c.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewRollback(actionConfig)
	client.Version = revision

	if err := client.Run(releaseName); err != nil {
		return fmt.Errorf("failed to rollback release: %w", err)
	}

	return nil
}

// addRepo adds a Helm repository
func (c *Client) HelmRepoAdd(ctx context.Context, name, url string) error {
	repoFile := c.settings.RepositoryConfig

	// Ensure the file directory exists
	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		return err
	}

	// Load existing repositories
	f, err := repo.LoadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if f == nil {
		f = repo.NewFile()
	}

	// Check if repo already exists
	if f.Has(name) {
		return nil // Already exists
	}

	// Add the repository
	entry := &repo.Entry{
		Name: name,
		URL:  url,
	}

	r, err := repo.NewChartRepository(entry, getter.All(c.settings))
	if err != nil {
		return err
	}

	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download repository index: %w", err)
	}

	f.Update(entry)
	return f.WriteFile(repoFile, 0644)
}

func (c *Client) HelmRepoList(ctx context.Context) ([]*repo.Entry, error) {
	repoFile := c.settings.RepositoryConfig
	f, err := repo.LoadFile(repoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository file: %w", err)
	}
	return f.Repositories, nil
}
