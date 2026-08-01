package ozon

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	defaultBaseURL  = "https://api-seller.ozon.ru"
	defaultTimeout  = 30 * time.Second
	errorNoClientID = "ozon: client id required"
	errorNoAPIKey   = "ozon: api key required"
)

// RuntimeConfig holds the per-shop Ozon Seller API credentials (never logged).
type RuntimeConfig struct {
	ClientID string
	APIKey   string
	BaseURL  string
	Timeout  time.Duration
}

// ResolveRuntime builds a RuntimeConfig from a decrypted shop auth request.
// Client-ID and Api-Key are stored as AppKey / AccessToken by the shop module.
func ResolveRuntime(req platformp.TestConnectionRequest) (RuntimeConfig, error) {
	clientID := strings.TrimSpace(req.AppKey)
	apiKey := strings.TrimSpace(req.AccessToken)
	if req.Extra != nil {
		if v := strings.TrimSpace(req.Extra["client_id"]); v != "" {
			clientID = v
		}
		if v := strings.TrimSpace(req.Extra["api_key"]); v != "" {
			apiKey = v
		}
	}
	if clientID == "" {
		return RuntimeConfig{}, fmt.Errorf("%s", errorNoClientID)
	}
	if apiKey == "" {
		return RuntimeConfig{}, fmt.Errorf("%s", errorNoAPIKey)
	}
	cfg := RuntimeConfig{
		ClientID: clientID,
		APIKey:   apiKey,
		BaseURL:  defaultBaseURL,
		Timeout:  defaultTimeout,
	}
	if req.Extra != nil {
		if v := strings.TrimSpace(req.Extra["api_base_url"]); v != "" {
			cfg.BaseURL = strings.TrimSuffix(v, "/")
		}
		if v := strings.TrimSpace(req.Extra["timeout_sec"]); v != "" {
			if sec, err := strconv.Atoi(v); err == nil && sec >= 5 && sec <= 600 {
				cfg.Timeout = time.Duration(sec) * time.Second
			}
		}
	}
	return cfg, nil
}
