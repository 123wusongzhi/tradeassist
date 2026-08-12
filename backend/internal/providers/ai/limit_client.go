package ai

import (
	"net/http"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httpclient"
	"github.com/trademind-ai/trademind/backend/internal/pkg/providerlimit"
)

func limitedAIHTTPClient(timeout time.Duration) *http.Client {
	// Chat completions may legitimately take longer than the shared provider
	// client's 30-second response-header default. Keep both HTTP bounds aligned
	// with the finite timeout computed by AIGateway for this completion size.
	return httpclient.LimitedStdHTTPClientWithHeaderTimeout(timeout, timeout, providerlimit.Global(), providerlimit.ProviderAI, providerlimit.OperationText)
}
