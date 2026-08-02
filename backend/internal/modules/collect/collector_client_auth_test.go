package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCollectorClientAuthenticatesEveryV1Request(t *testing.T) {
	t.Parallel()

	const token = "collector-test-token"
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("health request unexpectedly carried authorization: %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("%s authorization = %q", r.URL.Path, got)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		seen[r.URL.Path]++
		mu.Unlock()

		data := any(map[string]any{})
		switch r.URL.Path {
		case "/v1/providers":
			data = []any{}
		case "/v1/collect":
			data = map[string]any{"product": map[string]any{"title": "test"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
	}))
	t.Cleanup(server.Close)

	client := NewCollectorClient(server.URL, time.Second, token)
	ctx := context.Background()
	if _, err := client.AnalyzePage(ctx, "https://example.com", nil); err != nil {
		t.Fatalf("AnalyzePage: %v", err)
	}
	if _, err := client.CustomRuleTest(ctx, "https://example.com", map[string]any{}); err != nil {
		t.Fatalf("CustomRuleTest: %v", err)
	}
	if _, err := client.OpenBrowserProfileLogin(ctx, "profile", "https://example.com"); err != nil {
		t.Fatalf("OpenBrowserProfileLogin: %v", err)
	}
	if _, err := client.CheckBrowserProfileAccess(ctx, "profile", "https://example.com"); err != nil {
		t.Fatalf("CheckBrowserProfileAccess: %v", err)
	}
	if _, err := client.CollectWithTimeout(ctx, "custom", "https://example.com", nil, time.Second); err != nil {
		t.Fatalf("CollectWithTimeout: %v", err)
	}
	if _, err := client.FetchProviders(ctx); err != nil {
		t.Fatalf("FetchProviders: %v", err)
	}
	if _, err := client.Get1688AuthStatus(ctx, "tenant_7_1688"); err != nil {
		t.Fatalf("Get1688AuthStatus: %v", err)
	}
	if _, err := client.CheckPinduoduoLogin(ctx, "tenant_7_pinduoduo", "https://example.com", ""); err != nil {
		t.Fatalf("CheckPinduoduoLogin: %v", err)
	}
	if ok, message := client.ProbeHealth(ctx); !ok || message != "ok" {
		t.Fatalf("ProbeHealth = %v, %q", ok, message)
	}

	for _, path := range []string{
		"/v1/custom/analyze-page",
		"/v1/collect/custom-rule-test",
		"/v1/browser-profiles/profile/open-login",
		"/v1/browser-profiles/profile/check",
		"/v1/collect",
		"/v1/providers",
		"/v1/providers/1688/auth-status",
		"/v1/providers/pinduoduo/check-login",
	} {
		if seen[path] != 1 {
			t.Errorf("expected one authenticated request to %s, saw %d", path, seen[path])
		}
	}
}
