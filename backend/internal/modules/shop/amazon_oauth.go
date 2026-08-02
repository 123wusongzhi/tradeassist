package shop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformamazon "github.com/trademind-ai/trademind/backend/internal/providers/platform/amazon"
)

const amazonOAuthRedisPrefix = "oauth:amazon:state:"

// AmazonAuthorizeURLResult is GET /shops/:id/oauth/amazon/authorize-url payload.
type AmazonAuthorizeURLResult struct {
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

// AmazonOAuthCallbackBody POST /shops/:id/oauth/amazon/callback input.
type AmazonOAuthCallbackBody struct {
	Code             string `json:"code"`
	State            string `json:"state"`
	SellingPartnerID string `json:"sellingPartnerId"`
	MarketplaceID    string `json:"marketplaceId"`
}

func (s *Service) AmazonOAuthAuthorizeURL(c *gin.Context, shopID uuid.UUID, redirectOverride string, adminID *uuid.UUID) (*AmazonAuthorizeURLResult, error) {
	_ = adminID
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	if s.Redis == nil || s.Redis.Client == nil {
		return nil, fmt.Errorf("redis required for OAuth state")
	}
	ctx, tenantID, err := oauthTenantContext(c)
	if err != nil {
		return nil, err
	}
	row, err := s.tenantShop(c, shopID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "amazon" {
		return nil, fmt.Errorf("shop platform must be amazon")
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}
	_, _, auth, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}
	cfg, err := platformamazon.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	u, err := platformamazon.BuildAuthorizeURL(cfg, st, redirectOverride)
	if err != nil {
		return nil, err
	}
	key := amazonOAuthRedisPrefix + st
	saved, err := encodePlatformOAuthState("amazon", tenantID, shopID)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.Set(ctx, key, saved, 10*time.Minute).Err(); err != nil {
		return nil, err
	}
	return &AmazonAuthorizeURLResult{AuthorizeURL: u, State: st}, nil
}

func (s *Service) AmazonOAuthCallback(c *gin.Context, shopID uuid.UUID, body AmazonOAuthCallbackBody, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	code := strings.TrimSpace(body.Code)
	st := strings.TrimSpace(body.State)
	if code == "" || st == "" {
		return nil, fmt.Errorf("code and state required")
	}
	if s.Redis == nil || s.Redis.Client == nil {
		return nil, fmt.Errorf("redis required")
	}
	ctxBase, tenantID, err := oauthTenantContext(c)
	if err != nil {
		return nil, err
	}
	if _, err := s.tenantShop(c, shopID); err != nil {
		return nil, err
	}
	key := amazonOAuthRedisPrefix + st
	saved, err := s.Redis.Get(ctxBase, key).Result()
	if err != nil || strings.TrimSpace(saved) == "" {
		return nil, fmt.Errorf("invalid or expired oauth state")
	}
	if err := decodePlatformOAuthState(saved, "amazon", tenantID, shopID); err != nil {
		return nil, err
	}
	_ = s.Redis.Del(ctxBase, key)

	ctx, cancel := context.WithTimeout(ctxBase, 45*time.Second)
	defer cancel()

	row, err := s.tenantShopCtx(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "amazon" {
		return nil, fmt.Errorf("shop platform must be amazon")
	}

	c2 := c.Copy()
	c2.Request = c.Request.WithContext(ctx)
	_, _, auth, err := s.decryptedAuth(c2, shopID)
	if err != nil {
		return nil, err
	}
	cfg, err := platformamazon.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	rd := strings.TrimSpace(cfg.RedirectURI)

	tok, err := platformamazon.ExchangeAuthCode(ctx, cfg, code, rd)
	if err != nil {
		s.oauthAmazonLog(c, adminID, shopID, "failed", err.Error())
		_ = s.setAuthStatusCtx(ctx, tenantID, shopID, AuthError)
		return nil, err
	}

	marketplaceID := strings.TrimSpace(body.MarketplaceID)
	if marketplaceID == "" {
		marketplaceID = strings.TrimSpace(cfg.MarketplaceID)
	}
	spid := strings.TrimSpace(body.SellingPartnerID)

	acfg := map[string]any{
		"marketplaceId": marketplaceID,
	}
	if r := strings.TrimSpace(cfg.Region); r != "" {
		acfg["region"] = r
	}
	if spid != "" {
		acfg["sellingPartnerId"] = spid
	}

	ub := UpdateAuthBody{
		AuthType:      "oauth2",
		AccessToken:   tok.AccessToken,
		RefreshToken:  tok.RefreshToken,
		ExpiresAt:     tok.AccessExpiresAt,
		SellerID:      spid,
		MarketplaceID: marketplaceID,
		AuthConfig:    acfg,
	}
	if tok.RefreshExpiresAt != nil {
		ub.RefreshExpiresAt = tok.RefreshExpiresAt
	}

	if _, err := s.UpdateAuth(c, shopID, ub, adminID); err != nil {
		s.oauthAmazonLog(c, adminID, shopID, "failed", err.Error())
		return nil, err
	}

	if spid != "" {
		_ = s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenantID).Updates(map[string]any{
			"external_shop_id": spid,
			"shop_code":        spid,
		}).Error
	}

	req2 := platformp.TestConnectionRequest{
		AuthType:             "oauth2",
		AccessToken:          tok.AccessToken,
		RefreshToken:         tok.RefreshToken,
		SellerID:             spid,
		MarketplaceID:        marketplaceID,
		AccessTokenExpiresAt: tok.AccessExpiresAt,
	}
	cfg2, errC := platformamazon.ResolveRuntime(req2)
	if errC == nil {
		if pr, perr := platformamazon.MarketplaceParticipationsProbe(ctx, cfg2, tok.AccessToken); perr == nil && pr != nil && pr.OK {
			updates := map[string]any{"auth_status": AuthAuthorized}
			if pr.Currency != "" {
				updates["currency"] = pr.Currency
			}
			if pr.Region != "" {
				updates["region"] = pr.Region
			}
			_ = s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenantID).Updates(updates).Error
		}
	}

	raw := map[string]any{
		"provider":        "amazon",
		"saved_at":        time.Now().UTC().Format(time.RFC3339),
		"marketplace_ref": marketplaceID,
		"seller_ref":      spid,
	}
	rawB, _ := json.Marshal(raw)
	_ = s.DB.WithContext(ctx).Model(&ShopAuthToken{}).Where("shop_id = ? AND EXISTS (SELECT 1 FROM shops sh WHERE sh.id = shop_auth_tokens.shop_id AND sh.tenant_id = ?)", shopID, tenantID).
		Updates(map[string]any{"raw_data": datatypes.JSON(rawB)}).Error

	s.oauthAmazonLog(c, adminID, shopID, "success", "amazon oauth tokens saved")
	return s.GetDetail(c, shopID)
}

func (s *Service) oauthAmazonLog(c *gin.Context, adminID *uuid.UUID, shopID uuid.UUID, st, msg string) {
	if s.OpLog == nil {
		return
	}
	action := "shop.oauth.amazon.success"
	if st != "success" {
		action = "shop.oauth.amazon.failed"
	}
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "shop",
		ResourceID:  shopID.String(),
		Status:      st,
		Message:     fmt.Sprintf("shopId=%s %s", shopID.String(), msg),
	})
}
