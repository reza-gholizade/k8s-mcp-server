package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureMCPHandlerRequiresBearerToken(t *testing.T) {
	called := false
	handler := secureMCPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}), "secret", nil)

	for _, auth := range []string{"", "Bearer wrong"} {
		req := httptest.NewRequest(http.MethodPost, "/message?sessionId=test", nil)
		req.Header.Set("Authorization", auth)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q: got status %d, want %d", auth, res.Code, http.StatusUnauthorized)
		}
	}
	if called {
		t.Fatal("protected handler was called without valid authentication")
	}

	req := httptest.NewRequest(http.MethodPost, "/message?sessionId=test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted || !called {
		t.Fatalf("valid token: got status %d, handler called=%v", res.Code, called)
	}
}

func TestSecureMCPHandlerRejectsUntrustedOrigin(t *testing.T) {
	handler := secureMCPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "secret", []string{"https://trusted.example"})

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Origin", "https://attacker.example")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestSecureMCPHandlerAllowsConfiguredOriginPreflight(t *testing.T) {
	handler := secureMCPHandler(http.NotFoundHandler(), "secret", []string{"https://trusted.example"})
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://trusted.example")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", res.Code, http.StatusNoContent)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://trusted.example" {
		t.Fatalf("got Access-Control-Allow-Origin %q", got)
	}
}

func TestHTTPSecurityConfigFailsClosed(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKEN", "")
	if _, _, err := httpSecurityConfig(); err == nil {
		t.Fatal("expected missing MCP_AUTH_TOKEN to fail")
	}

	t.Setenv("MCP_AUTH_TOKEN", "too-short")
	if _, _, err := httpSecurityConfig(); err == nil {
		t.Fatal("expected short MCP_AUTH_TOKEN to fail")
	}
}
