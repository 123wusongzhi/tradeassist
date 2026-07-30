package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	CollectEnginePlaywright = "playwright"
	CollectEngineOpenCLI    = "opencli"
)

type CollectEngineRoutingError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *CollectEngineRoutingError) Error() string {
	if e == nil {
		return "collect engine routing error"
	}
	return e.Message
}

type CollectEngineStatusItem struct {
	Engine           string   `json:"engine"`
	Enabled          bool     `json:"enabled"`
	Configured       bool     `json:"configured"`
	Reachable        bool     `json:"reachable"`
	Ready            bool     `json:"ready"`
	Status           string   `json:"status"`
	Message          string   `json:"message"`
	SupportedSources []string `json:"supportedSources"`
}

type CollectEnginesStatus struct {
	DefaultEngine string                    `json:"defaultEngine"`
	Engines       []CollectEngineStatusItem `json:"engines"`
}

// CollectorEngineRouter isolates Playwright and OpenCLI transport failures.
// It never falls back to another engine after a task has started.
type CollectorEngineRouter struct {
	Playwright         *CollectorClient
	OpenCLI            *OpenCLIBridgeClient
	OpenCLIEnabled     bool
	DefaultTaobaoTmall string
	OpenCLITimeout     time.Duration
}

func NewCollectorEngineRouter(
	playwright *CollectorClient,
	opencli *OpenCLIBridgeClient,
	opencliEnabled bool,
	defaultTaobaoTmall string,
	opencliTimeout time.Duration,
) *CollectorEngineRouter {
	defaultEngine := strings.ToLower(strings.TrimSpace(defaultTaobaoTmall))
	if defaultEngine != CollectEngineOpenCLI {
		defaultEngine = CollectEnginePlaywright
	}
	if opencliTimeout <= 0 {
		opencliTimeout = 120 * time.Second
	}
	return &CollectorEngineRouter{
		Playwright:         playwright,
		OpenCLI:            opencli,
		OpenCLIEnabled:     opencliEnabled,
		DefaultTaobaoTmall: defaultEngine,
		OpenCLITimeout:     opencliTimeout,
	}
}

func normalizeCollectEngine(raw string) (string, error) {
	switch v := strings.ToLower(strings.TrimSpace(raw)); v {
	case "", CollectEnginePlaywright:
		return v, nil
	case CollectEngineOpenCLI:
		return CollectEngineOpenCLI, nil
	default:
		return "", &CollectEngineRoutingError{
			Code:       "COLLECT_ENGINE_INVALID",
			Message:    fmt.Sprintf("invalid engine %q (allowed: playwright | opencli)", raw),
			HTTPStatus: http.StatusBadRequest,
		}
	}
}

func collectEngineFromRequestOptions(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var snapshot struct {
		Engine string `json:"engine"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(snapshot.Engine))
}

func injectEngineIntoRequestOptions(reqOpts []byte, engine string) []byte {
	engine = strings.TrimSpace(engine)
	if engine == "" {
		return reqOpts
	}
	merged := mergeJSONIntoCollectorOpts(nil, reqOpts)
	if merged == nil {
		merged = map[string]any{}
	}
	merged["engine"] = engine
	blob, err := json.Marshal(merged)
	if err != nil {
		return nil
	}
	return blob
}

func (r *CollectorEngineRouter) ResolveEngine(source, requested string) (string, error) {
	engine, err := normalizeCollectEngine(requested)
	if err != nil {
		return "", err
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if engine == "" {
		engine = CollectEnginePlaywright
		if isTaobaoTmallCollectSource(source) && r != nil &&
			r.OpenCLIEnabled && r.DefaultTaobaoTmall == CollectEngineOpenCLI {
			engine = CollectEngineOpenCLI
		}
	}
	if engine != CollectEngineOpenCLI {
		return CollectEnginePlaywright, nil
	}
	if !isTaobaoTmallCollectSource(source) {
		return "", &CollectEngineRoutingError{
			Code:       "COLLECT_ENGINE_SOURCE_UNSUPPORTED",
			Message:    fmt.Sprintf("opencli engine does not support source %q", source),
			HTTPStatus: http.StatusBadRequest,
		}
	}
	if r == nil || !r.OpenCLIEnabled || r.OpenCLI == nil || strings.TrimSpace(r.OpenCLI.BaseURL) == "" {
		return "", &CollectEngineRoutingError{
			Code:       "OPENCLI_BRIDGE_DISABLED",
			Message:    "opencli bridge is not enabled; use playwright or enable OPENCLI_BRIDGE_ENABLED",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	return CollectEngineOpenCLI, nil
}

func (r *CollectorEngineRouter) Collect(
	ctx context.Context,
	engine, source, rawURL string,
	options map[string]any,
	playwrightTimeout time.Duration,
) (*CollectOutcome, error) {
	resolved, err := r.ResolveEngine(source, engine)
	if err != nil {
		return nil, err
	}
	if resolved == CollectEngineOpenCLI {
		return r.OpenCLI.CollectWithTimeout(ctx, source, rawURL, options, r.OpenCLITimeout)
	}
	if r == nil || r.Playwright == nil {
		return nil, fmt.Errorf("playwright collector client unavailable")
	}
	return r.Playwright.CollectWithTimeout(ctx, source, rawURL, options, playwrightTimeout)
}

func (r *CollectorEngineRouter) Status(ctx context.Context, defaultEngine string) CollectEnginesStatus {
	effectiveDefault := CollectEnginePlaywright
	if resolved, err := r.ResolveEngine("taobao_tmall", defaultEngine); err == nil {
		effectiveDefault = resolved
	}

	playwright := CollectEngineStatusItem{
		Engine:           CollectEnginePlaywright,
		Enabled:          true,
		Configured:       r != nil && r.Playwright != nil && strings.TrimSpace(r.Playwright.BaseURL) != "",
		Status:           "unavailable",
		Message:          "playwright collector is not configured",
		SupportedSources: []string{"1688", "pinduoduo", "taobao_tmall", "aliexpress", "shein_temu", "custom"},
	}
	if playwright.Configured {
		playwright.Message = "playwright collector is checking"
	}

	opencli := CollectEngineStatusItem{
		Engine:           CollectEngineOpenCLI,
		Enabled:          r != nil && r.OpenCLIEnabled,
		Configured:       r != nil && r.OpenCLI != nil && strings.TrimSpace(r.OpenCLI.BaseURL) != "",
		Status:           "disabled",
		Message:          "opencli bridge is disabled",
		SupportedSources: []string{"taobao_tmall"},
	}
	if opencli.Enabled {
		opencli.Status = "unavailable"
		opencli.Message = "opencli bridge is not configured"
		if opencli.Configured {
			opencli.Message = "opencli bridge is checking"
		}
	}

	var probes sync.WaitGroup
	if playwright.Configured {
		probes.Add(1)
		go func() {
			defer probes.Done()
			playwright.Reachable, _ = r.Playwright.ProbeHealth(ctx)
			playwright.Ready = playwright.Reachable
			if playwright.Ready {
				playwright.Status = "ready"
				playwright.Message = "playwright collector is ready"
			} else {
				playwright.Message = "playwright collector is unreachable"
			}
		}()
	}
	if opencli.Enabled && opencli.Configured {
		probes.Add(1)
		go func() {
			defer probes.Done()
			status, err := r.OpenCLI.ProbeStatus(ctx)
			if err != nil {
				opencli.Message = "opencli bridge is unreachable"
			} else {
				opencli.Reachable = true
				opencli.Ready = status.Ready
				opencli.Message = status.Message
				if opencli.Ready {
					opencli.Status = "ready"
				} else {
					opencli.Status = "degraded"
				}
			}
		}()
	}
	probes.Wait()

	return CollectEnginesStatus{
		DefaultEngine: effectiveDefault,
		Engines:       []CollectEngineStatusItem{opencli, playwright},
	}
}

func (s *Service) GetCollectEnginesStatus(ctx context.Context) CollectEnginesStatus {
	if ctx == nil {
		ctx = context.Background()
	}
	router := (*CollectorEngineRouter)(nil)
	if s != nil {
		router = s.EngineRouter
		if router == nil {
			router = NewCollectorEngineRouter(s.Client, nil, false, CollectEnginePlaywright, 0)
		}
	}
	if router == nil {
		router = NewCollectorEngineRouter(nil, nil, false, CollectEnginePlaywright, 0)
	}
	return router.Status(ctx, "")
}
