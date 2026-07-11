package douyinshop

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httpclient"
)

var (
	sharedHTTPMu   sync.Mutex
	sharedHTTPDoer HTTPDoer
	sharedBreaker  *httpclient.CircuitBreaker
	sharedMaxConc  = 16
)

// SetSharedHTTPConcurrency configures the shared Douyin outbound concurrency gate (0 = unlimited).
func SetSharedHTTPConcurrency(n int) {
	sharedHTTPMu.Lock()
	defer sharedHTTPMu.Unlock()
	sharedMaxConc = n
	sharedHTTPDoer = nil
}

// SharedHTTPDoer returns a process-wide HTTPDoer backed by unified httpclient + circuit breaker.
func SharedHTTPDoer() HTTPDoer {
	sharedHTTPMu.Lock()
	defer sharedHTTPMu.Unlock()
	if sharedHTTPDoer != nil {
		return sharedHTTPDoer
	}
	cfg := httpclient.DefaultConfig()
	cfg.RequestTimeout = 60 * time.Second
	cfg.UserAgent = "TradeMind-DouyinShop/1.0"
	cli := httpclient.New(cfg, slog.Default(), sharedMaxConc)
	br := httpclient.NewCircuitBreaker(5, 30*time.Second)
	cli.SetCircuitBreaker(br)
	sharedBreaker = br
	sharedHTTPDoer = &httpDoerAdapter{cli: cli}
	return sharedHTTPDoer
}

// SharedCircuitBreaker exposes the Douyin outbound breaker for health checks.
func SharedCircuitBreaker() *httpclient.CircuitBreaker {
	_ = SharedHTTPDoer()
	sharedHTTPMu.Lock()
	defer sharedHTTPMu.Unlock()
	return sharedBreaker
}

type httpDoerAdapter struct {
	cli *httpclient.Client
}

func (a *httpDoerAdapter) Do(req *http.Request) (*http.Response, error) {
	if a == nil || a.cli == nil {
		return nil, NewError(CodeDouyinNotConfigured, "douyin http client unavailable", "", "", "")
	}
	ctx := req.Context()
	return a.cli.Do(ctx, req)
}
