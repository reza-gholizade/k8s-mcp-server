package k8s

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

func TestDecodeManifestJSONPreservesLargeIntegers(t *testing.T) {
	const largeInteger = "9007199254740993" // 2^53 + 1, not exactly representable as float64.
	obj := &unstructured.Unstructured{}
	if err := decodeManifestJSON([]byte(`{"apiVersion":"example.com/v1","kind":"Example","metadata":{"name":"sample"},"spec":{"sequence":9007199254740993}}`), obj); err != nil {
		t.Fatalf("decodeManifestJSON() error = %v", err)
	}
	encoded, err := json.Marshal(obj.Object)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"sequence":`+largeInteger) {
		t.Fatalf("encoded manifest = %s, want sequence %s preserved exactly", encoded, largeInteger)
	}
}
