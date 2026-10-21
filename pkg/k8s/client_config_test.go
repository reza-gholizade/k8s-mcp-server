package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()

	config := api.NewConfig()
	config.Clusters["cluster-a"] = &api.Cluster{Server: "https://cluster-a.example.test"}
	config.Clusters["cluster-b"] = &api.Cluster{Server: "https://cluster-b.example.test"}
	config.AuthInfos["user-a"] = &api.AuthInfo{Token: "token-a"}
	config.AuthInfos["user-b"] = &api.AuthInfo{Token: "token-b"}
	config.Contexts["context-a"] = &api.Context{Cluster: "cluster-a", AuthInfo: "user-a"}
	config.Contexts["context-b"] = &api.Context{Cluster: "cluster-b", AuthInfo: "user-b"}
	config.CurrentContext = "context-a"

	path := filepath.Join(t.TempDir(), "config")
	if err := clientcmd.WriteToFile(*config, path); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func clearKubeconfigEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"KUBECONFIG_DATA", "KUBERNETES_SERVER", "KUBERNETES_TOKEN",
		"KUBERNETES_CONTEXT", "KUBECONFIG",
	} {
		t.Setenv(key, "")
	}
}

func TestBuildKubernetesConfigHonorsContextOverrideForFile(t *testing.T) {
	clearKubeconfigEnvironment(t)
	path := writeTestKubeconfig(t)
	t.Setenv("KUBERNETES_CONTEXT", "context-b")

	config, err := BuildKubernetesConfig(path)
	if err != nil {
		t.Fatalf("BuildKubernetesConfig() error = %v", err)
	}
	if config.Host != "https://cluster-b.example.test" {
		t.Fatalf("Host = %q, want context-b cluster URL", config.Host)
	}
	if config.BearerToken != "token-b" {
		t.Fatalf("BearerToken = %q, want token-b", config.BearerToken)
	}
}

func TestBuildKubernetesConfigHonorsContextOverrideForData(t *testing.T) {
	clearKubeconfigEnvironment(t)
	path := writeTestKubeconfig(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG_DATA", string(data))
	t.Setenv("KUBERNETES_CONTEXT", "context-b")

	config, err := BuildKubernetesConfig("")
	if err != nil {
		t.Fatalf("BuildKubernetesConfig() error = %v", err)
	}
	if config.Host != "https://cluster-b.example.test" {
		t.Fatalf("Host = %q, want context-b cluster URL", config.Host)
	}
	if config.BearerToken != "token-b" {
		t.Fatalf("BearerToken = %q, want token-b", config.BearerToken)
	}
}

func TestBuildKubernetesConfigUsesCurrentContextWithoutOverride(t *testing.T) {
	clearKubeconfigEnvironment(t)
	path := writeTestKubeconfig(t)

	config, err := BuildKubernetesConfig(path)
	if err != nil {
		t.Fatalf("BuildKubernetesConfig() error = %v", err)
	}
	if config.Host != "https://cluster-a.example.test" {
		t.Fatalf("Host = %q, want current-context cluster URL", config.Host)
	}
}
