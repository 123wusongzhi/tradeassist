package douyinshop

import (
	"context"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/providerhealth"
)

// ConfigHealthChecker checks that the Douyin provider is minimally configured.
// Read-only — no writes to platform.
type ConfigHealthChecker struct {
	ShopID string
	Config RuntimeConfig
}

func (ch *ConfigHealthChecker) HealthCheck(_ context.Context) providerhealth.Result {
	now := time.Now().UTC()
	res := providerhealth.Result{
		Provider:    "douyin_shop",
		Capability:  "config",
		CheckedAt:   now,
		NextCheckAt: now.Add(5 * time.Minute),
	}
	if ch == nil {
		res.Status = providerhealth.StatusNotConfigured
		res.ErrorCode = CodeDouyinNotConfigured
		return res
	}
	if strings.TrimSpace(ch.Config.AppKey) == "" || strings.TrimSpace(ch.Config.AppSecret) == "" {
		res.Status = providerhealth.StatusNotConfigured
		res.ErrorCode = CodeDouyinNotConfigured
		return res
	}
	res.Status = providerhealth.StatusAvailable
	return res
}

// TokenMetaHealthChecker checks token expiry metadata without calling the platform.
// Read-only — inspects in-memory token state only.
type TokenMetaHealthChecker struct {
	ShopID string
	Client *Client
}

func (ch *TokenMetaHealthChecker) HealthCheck(_ context.Context) providerhealth.Result {
	now := time.Now().UTC()
	res := providerhealth.Result{
		Provider:    "douyin_shop",
		Capability:  "token_meta",
		CheckedAt:   now,
		NextCheckAt: now.Add(5 * time.Minute),
	}
	if ch == nil || ch.Client == nil {
		res.Status = providerhealth.StatusNotConfigured
		res.ErrorCode = CodeDouyinNotConfigured
		return res
	}
	c := ch.Client
	if _, ok := c.freshAccessToken(now); ok {
		res.Status = providerhealth.StatusAvailable
		return res
	}
	if c.refreshUsable(now) {
		res.Status = providerhealth.StatusDegraded
		res.ErrorCode = CodeDouyinTokenRefreshFailed
		return res
	}
	res.Status = providerhealth.StatusUnauthorized
	res.ErrorCode = CodeDouyinAuthExpired
	return res
}

// OAuthConfigHealthChecker checks OAuth config completeness (service_id, redirect_uri).
type OAuthConfigHealthChecker struct {
	Config RuntimeConfig
}

func (ch *OAuthConfigHealthChecker) HealthCheck(_ context.Context) providerhealth.Result {
	now := time.Now().UTC()
	res := providerhealth.Result{
		Provider:    "douyin_shop",
		Capability:  "oauth_config",
		CheckedAt:   now,
		NextCheckAt: now.Add(5 * time.Minute),
	}
	if ch == nil {
		res.Status = providerhealth.StatusNotConfigured
		res.ErrorCode = CodeDouyinNotConfigured
		return res
	}
	if strings.TrimSpace(ch.Config.ServiceID) == "" {
		res.Status = providerhealth.StatusNotConfigured
		res.ErrorCode = CodeDouyinNotConfigured
		return res
	}
	if strings.TrimSpace(ch.Config.RedirectURI) == "" {
		res.Status = providerhealth.StatusDegraded
		res.ErrorCode = CodeDouyinNotConfigured
		return res
	}
	res.Status = providerhealth.StatusAvailable
	return res
}
