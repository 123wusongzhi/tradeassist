package ai

import (
	"net/http"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httpclient"
	"github.com/trademind-ai/trademind/backend/internal/pkg/providerlimit"
)

func limitedAIHTTPClient(timeout time.Duration) *http.Client {
	return httpclient.LimitedStdHTTPClient(timeout, providerlimit.Global(), providerlimit.ProviderAI, providerlimit.OperationText)
}
