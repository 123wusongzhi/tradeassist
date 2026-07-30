package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func collectSuccessPayload(title string) []byte {
	product := map[string]any{
		"source":            "taobao_tmall",
		"sourceUrl":         "https://item.taobao.com/item.htm?id=1",
		"title":             title,
		"currency":          "CNY",
		"mainImages":        []string{"https://img.example/1.jpg"},
		"descriptionImages": []string{},
		"attributes":        map[string]string{},
		"skus":              []any{},
		"raw":               map[string]any{},
	}
	payload, _ := json.Marshal(map[string]any{
		"ok": true,
		"data": map[string]any{
			"product": product,
		},
	})
	return payload
}

func TestEngineRouterPlaywrightUnaffectedByOpenCLIDown(t *testing.T) {
	playwrightServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(collectSuccessPayload("playwright"))
	}))
	defer playwrightServer.Close()

	router := NewCollectorEngineRouter(
		NewCollectorClient(playwrightServer.URL, time.Second),
		NewOpenCLIBridgeClient("http://127.0.0.1:1", "", time.Second),
		true,
		CollectEngineOpenCLI,
		time.Second,
	)
	outcome, err := router.Collect(
		context.Background(),
		CollectEnginePlaywright,
		"taobao_tmall",
		"https://item.taobao.com/item.htm?id=1",
		nil,
		time.Second,
	)
	if err != nil {
		t.Fatalf("playwright collect should remain available: %v", err)
	}
	if outcome == nil || len(outcome.ProductJSON) == 0 {
		t.Fatal("expected playwright product")
	}
}

func TestEngineRouterOpenCLIDoesNotFallBackToPlaywright(t *testing.T) {
	var playwrightCalls atomic.Int32
	playwrightServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		playwrightCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(collectSuccessPayload("unexpected fallback"))
	}))
	defer playwrightServer.Close()

	opencliServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"PROVIDER_NOT_AVAILABLE","message":"bridge down"}}`))
	}))
	defer opencliServer.Close()

	router := NewCollectorEngineRouter(
		NewCollectorClient(playwrightServer.URL, time.Second),
		NewOpenCLIBridgeClient(opencliServer.URL, "test-token", time.Second),
		true,
		CollectEngineOpenCLI,
		time.Second,
	)
	_, err := router.Collect(
		context.Background(),
		CollectEngineOpenCLI,
		"taobao_tmall",
		"https://item.taobao.com/item.htm?id=1",
		nil,
		time.Second,
	)
	if err == nil {
		t.Fatal("expected OpenCLI failure")
	}
	rejected, ok := err.(*CollectorRejectedError)
	if !ok || rejected.Code != "PROVIDER_NOT_AVAILABLE" {
		t.Fatalf("expected typed bridge error, got %#v", err)
	}
	if got := playwrightCalls.Load(); got != 0 {
		t.Fatalf("expected no fallback, playwright calls=%d", got)
	}
}

func TestEngineRouterResolution(t *testing.T) {
	bridge := NewOpenCLIBridgeClient("http://127.0.0.1:3100", "", time.Second)
	router := NewCollectorEngineRouter(nil, bridge, true, CollectEngineOpenCLI, time.Second)

	engine, err := router.ResolveEngine("taobao_tmall", "")
	if err != nil || engine != CollectEngineOpenCLI {
		t.Fatalf("expected OpenCLI default, engine=%q err=%v", engine, err)
	}
	if _, err := router.ResolveEngine("1688", CollectEngineOpenCLI); err == nil {
		t.Fatal("expected unsupported source error")
	}

	disabled := NewCollectorEngineRouter(nil, bridge, false, CollectEngineOpenCLI, time.Second)
	engine, err = disabled.ResolveEngine("taobao_tmall", "")
	if err != nil || engine != CollectEnginePlaywright {
		t.Fatalf("disabled bridge should make omitted engine use playwright, engine=%q err=%v", engine, err)
	}
	if _, err := disabled.ResolveEngine("taobao_tmall", CollectEngineOpenCLI); err == nil {
		t.Fatal("explicit OpenCLI should fail while bridge is disabled")
	}
}
