package ozon

import (
	"testing"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestResolveRuntime(t *testing.T) {
	t.Run("standard auth fields", func(t *testing.T) {
		cfg, err := ResolveRuntime(platformp.TestConnectionRequest{
			AppKey:      "123456",
			AccessToken: "api-key-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientID != "123456" || cfg.APIKey != "api-key-1" {
			t.Fatalf("unexpected credentials: %+v", cfg)
		}
		if cfg.BaseURL != defaultBaseURL {
			t.Fatalf("unexpected base url: %s", cfg.BaseURL)
		}
		if cfg.Timeout != defaultTimeout {
			t.Fatalf("unexpected timeout: %s", cfg.Timeout)
		}
	})

	t.Run("extra fallback", func(t *testing.T) {
		cfg, err := ResolveRuntime(platformp.TestConnectionRequest{
			Extra: map[string]string{
				"client_id":    "111",
				"api_key":      "222",
				"api_base_url": "https://example.com/",
				"timeout_sec":  "60",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientID != "111" || cfg.APIKey != "222" {
			t.Fatalf("unexpected credentials: %+v", cfg)
		}
		if cfg.BaseURL != "https://example.com" {
			t.Fatalf("unexpected base url: %s", cfg.BaseURL)
		}
		if cfg.Timeout != 60*time.Second {
			t.Fatalf("unexpected timeout: %s", cfg.Timeout)
		}
	})

	t.Run("missing credentials", func(t *testing.T) {
		if _, err := ResolveRuntime(platformp.TestConnectionRequest{}); err == nil {
			t.Fatal("expected error for missing credentials")
		}
		if _, err := ResolveRuntime(platformp.TestConnectionRequest{AppKey: "1"}); err == nil {
			t.Fatal("expected error for missing api key")
		}
	})
}
