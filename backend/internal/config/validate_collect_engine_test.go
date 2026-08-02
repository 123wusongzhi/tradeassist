package config

import (
	"strings"
	"testing"
)

func TestValidateCollectEngines(t *testing.T) {
	cfg := &Config{
		CollectorBaseURL:                "http://127.0.0.1:3001",
		CollectDefaultEngineTaobaoTmall: "opencli",
		OpenCLIBridgeEnabled:            true,
		OpenCLIBridgeBaseURL:            "http://127.0.0.1:3100",
	}
	if err := cfg.validateCollectEngines(); err != nil {
		t.Fatalf("expected valid collect engine config: %v", err)
	}

	cfg.CollectDefaultEngineTaobaoTmall = "unknown"
	if err := cfg.validateCollectEngines(); err == nil ||
		!strings.Contains(err.Error(), "COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL") {
		t.Fatalf("expected invalid default engine error, got %v", err)
	}
}

func TestValidateCollectEnginesRequiresBridgeURLWhenEnabled(t *testing.T) {
	cfg := &Config{
		CollectorBaseURL:                "http://127.0.0.1:3001",
		CollectDefaultEngineTaobaoTmall: "playwright",
		OpenCLIBridgeEnabled:            true,
	}
	if err := cfg.validateCollectEngines(); err == nil ||
		!strings.Contains(err.Error(), "OPENCLI_BRIDGE_BASE_URL") {
		t.Fatalf("expected bridge URL error, got %v", err)
	}
}

func TestValidateCollectEnginesRejectsInvalidURLs(t *testing.T) {
	cfg := &Config{
		CollectorBaseURL:                "collector:3001",
		CollectDefaultEngineTaobaoTmall: "playwright",
	}
	if err := cfg.validateCollectEngines(); err == nil ||
		!strings.Contains(err.Error(), "COLLECTOR_PLAYWRIGHT_BASE_URL") {
		t.Fatalf("expected invalid Playwright URL error, got %v", err)
	}

	cfg.CollectorBaseURL = "http://127.0.0.1:3001"
	cfg.OpenCLIBridgeEnabled = true
	cfg.OpenCLIBridgeBaseURL = "http://user:secret@127.0.0.1:3100"
	if err := cfg.validateCollectEngines(); err == nil ||
		!strings.Contains(err.Error(), "OPENCLI_BRIDGE_BASE_URL") {
		t.Fatalf("expected invalid Bridge URL error, got %v", err)
	}
}

func TestValidateCollectEnginesRequiresTokenForRemoteCollector(t *testing.T) {
	cfg := &Config{
		CollectorBaseURL:                "http://collector:3001",
		CollectDefaultEngineTaobaoTmall: "playwright",
	}
	if err := cfg.validateCollectEngines(); err == nil ||
		!strings.Contains(err.Error(), "COLLECTOR_INTERNAL_TOKEN") {
		t.Fatalf("expected remote collector token error, got %v", err)
	}

	cfg.CollectorToken = "collector-test-token"
	if err := cfg.validateCollectEngines(); err != nil {
		t.Fatalf("expected authenticated remote collector config: %v", err)
	}
}
