package ozon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func newTestClient(t *testing.T, handler http.Handler) *ozonClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := RuntimeConfig{ClientID: "1", APIKey: "2", BaseURL: srv.URL, Timeout: defaultTimeout}
	return newClient(cfg)
}

func TestPostJSONErrorClassification(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantSubstr string
		wantRetry  bool
	}{
		{name: "permission denied", status: 403, body: `{"code":20,"message":"access denied"}`, wantSubstr: "permission", wantRetry: false},
		{name: "unauthorized", status: 401, body: `{"code":1,"message":"invalid api key"}`, wantSubstr: "permission", wantRetry: false},
		{name: "rate limited", status: 429, body: `{"code":9,"message":"too many requests"}`, wantSubstr: "retryable", wantRetry: true},
		{name: "upstream", status: 500, body: `{"code":100,"message":"boom"}`, wantSubstr: "retryable", wantRetry: true},
		{name: "bad request", status: 400, body: `{"code":3,"message":"invalid param"}`, wantSubstr: "invalid param", wantRetry: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			var out any
			err := client.postJSON(context.Background(), "/v1/test", map[string]any{}, &out)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantSubstr)) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSubstr)
			}
			if gotRetry := strings.Contains(err.Error(), "retryable"); gotRetry != tt.wantRetry {
				t.Fatalf("retryable=%v, want %v (err=%v)", gotRetry, tt.wantRetry, err)
			}
		})
	}
}

func TestPostJSONInvalidBody(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":`))
	}))
	var out any
	err := client.postJSON(context.Background(), "/v1/test", map[string]any{}, &out)
	if err == nil || !strings.Contains(err.Error(), "invalid json") {
		t.Fatalf("expected invalid json error, got %v", err)
	}
}

func TestClassifyPermissionDeniedIsPlatformError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":20,"message":"forbidden"}`))
	}))
	var out any
	err := client.postJSON(context.Background(), "/v1/test", map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), platformp.ErrPlatformProductPublishPermissionDenied.Error()) {
		t.Fatalf("expected permission denied wrap, got %v", err)
	}
}

func TestPostJSONRetryRebuildsRequestBody(t *testing.T) {
	requests := 0
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"value":"kept-on-retry"}` {
			t.Fatalf("request %d body = %q", requests, body)
		}
		if requests == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"retry"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.postJSON(context.Background(), "/v1/test", map[string]any{"value": "kept-on-retry"}, &out); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !out.OK {
		t.Fatalf("requests=%d out=%+v", requests, out)
	}
}
