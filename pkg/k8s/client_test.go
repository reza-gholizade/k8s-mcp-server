package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: first
clusters:
- name: first
  cluster:
    server: https://first.example.com
- name: second
  cluster:
    server: https://second.example.com
users:
- name: user
  user:
    token: test-token
contexts:
- name: first
  context:
    cluster: first
    user: user
- name: second
  context:
    cluster: second
    user: user
`

func TestBuildKubernetesConfigFromDataHonorsContext(t *testing.T) {
	t.Setenv("KUBECONFIG_DATA", testKubeconfig)

	t.Setenv("KUBERNETES_CONTEXT", "")
	config, err := BuildKubernetesConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "https://first.example.com" {
		t.Errorf("without KUBERNETES_CONTEXT got host %q, want the current context", config.Host)
	}

	t.Setenv("KUBERNETES_CONTEXT", "second")
	config, err = BuildKubernetesConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "https://second.example.com" {
		t.Errorf("with KUBERNETES_CONTEXT=second got host %q", config.Host)
	}
}

func TestBuildKubernetesConfigFromFileHonorsContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG_DATA", "")
	t.Setenv("KUBERNETES_SERVER", "")
	t.Setenv("KUBERNETES_CONTEXT", "second")

	config, err := BuildKubernetesConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "https://second.example.com" {
		t.Errorf("with KUBERNETES_CONTEXT=second got host %q", config.Host)
	}
}
