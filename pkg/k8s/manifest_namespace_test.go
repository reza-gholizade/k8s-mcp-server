package k8s

import "testing"

func TestResolveManifestNamespace(t *testing.T) {
	tests := []struct {
		name             string
		override         string
		manifestNamespace string
		namespaced       bool
		want             string
	}{
		{name: "explicit override wins", override: "override-ns", manifestNamespace: "manifest-ns", namespaced: true, want: "override-ns"},
		{name: "manifest namespace used", manifestNamespace: "manifest-ns", namespaced: true, want: "manifest-ns"},
		{name: "default namespace used", namespaced: true, want: "default"},
		{name: "cluster scoped ignores namespace", override: "override-ns", manifestNamespace: "manifest-ns", namespaced: false, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveManifestNamespace(tt.override, tt.manifestNamespace, tt.namespaced); got != tt.want {
				t.Fatalf("resolveManifestNamespace() = %q, want %q", got, tt.want)
			}
		})
	}
}
