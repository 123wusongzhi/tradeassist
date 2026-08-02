package shop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
)

const tiktokOAuthRedisPrefix = "oauth:tiktok:state:"

type platformOAuthStatePayload struct {
	Platform string `json:"platform"`
	TenantID int64  `json:"tenantId"`
	ShopID   string `json:"shopId"`
}

func oauthTenantContext(c *gin.Context) (context.Context, int64, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID < 0 {
		return nil, 0, fmt.Errorf("tenant context required")
	}
	return security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: tenantID}), tenantID, nil
}

func encodePlatformOAuthState(platform string, tenantID int64, shopID uuid.UUID) (string, error) {
	b, err := json.Marshal(platformOAuthStatePayload{Platform: platform, TenantID: tenantID, ShopID: shopID.String()})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodePlatformOAuthState(raw, platform string, tenantID int64, shopID uuid.UUID) error {
	var p platformOAuthStatePayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &p); err != nil {
		return fmt.Errorf("invalid oauth state: %w", err)
	}
	if p.Platform != platform || p.TenantID != tenantID || p.ShopID != shopID.String() {
		return fmt.Errorf("state does not match tenant or shop")
	}
	return nil
}

func randomOAuthState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// TikTokAuthorizeURLResult is GET /shops/:id/oauth/tiktok/authorize-url payload.
type TikTokAuthorizeURLResult struct {
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

// TikTokOAuthCallbackBody POST /shops/:id/oauth/tiktok/callback input.
type TikTokOAuthCallbackBody struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func (s *Service) TikTokOAuthAuthorizeURL(c *gin.Context, shopID uuid.UUID, redirectOverride string, adminID *uuid.UUID) (*TikTokAuthorizeURLResult, error) {
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
	if strings.TrimSpace(row.Platform) != "tiktok" {
		return nil, fmt.Errorf("shop platform must be tiktok")
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}

	_, _, auth, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}

	cfg, err := platformtiktok.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	u, err := platformtiktok.BuildAuthorizeURL(cfg, st, redirectOverride)
	if err != nil {
		return nil, err
	}

	key := tiktokOAuthRedisPrefix + st
	saved, err := encodePlatformOAuthState("tiktok", tenantID, shopID)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.Set(ctx, key, saved, 10*time.Minute).Err(); err != nil {
		return nil, err
	}

	return &TikTokAuthorizeURLResult{AuthorizeURL: u, State: st}, nil
}

func (s *Service) TikTokOAuthCallback(c *gin.Context, shopID uuid.UUID, body TikTokOAuthCallbackBody, adminID *uuid.UUID) (*ShopDetailDTO, error) {
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
	key := tiktokOAuthRedisPrefix + st
	saved, err := s.Redis.Get(ctxBase, key).Result()
	if err != nil || strings.TrimSpace(saved) == "" {
		return nil, fmt.Errorf("invalid or expired oauth state")
	}
	if err := decodePlatformOAuthState(saved, "tiktok", tenantID, shopID); err != nil {
		return nil, err
	}
	_ = s.Redis.Del(ctxBase, key)

	ctx, cancel := context.WithTimeout(ctxBase, 45*time.Second)
	defer cancel()

	row, err := s.tenantShopCtx(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "tiktok" {
		return nil, fmt.Errorf("shop platform must be tiktok")
	}

	c2 := c.Copy()
	c2.Request = c.Request.WithContext(ctx)

	_, _, auth, err := s.decryptedAuth(c2, shopID)
	if err != nil {
		return nil, err
	}

	tbundle, _, _, err := platformtiktok.ExchangeAuthCode(ctx, auth, code)
	if err != nil {
		s.oauthTikTokLog(c, adminID, shopID, "failed", err.Error())
		_ = s.setAuthStatusCtx(ctx, tenantID, shopID, AuthError)
		return nil, err
	}

	ub := UpdateAuthBody{
		AuthType:         "oauth2",
		AccessToken:      tbundle.AccessToken,
		RefreshToken:     tbundle.RefreshToken,
		ExpiresAt:        tbundle.AccessExpiresAt,
		RefreshExpiresAt: tbundle.RefreshExpiresAt,
	}
	if _, err := s.UpdateAuth(c, shopID, ub, adminID); err != nil {
		s.oauthTikTokLog(c, adminID, shopID, "failed", err.Error())
		return nil, err
	}

	_, _, auth2, err := s.decryptedAuth(c2, shopID)
	if err == nil {
		if cipher, nm, ext, region, currency, perr := platformtiktok.PrimaryAuthorizedShop(ctx, auth2, auth2.AccessToken); perr == nil && cipher != "" {
			_ = s.DB.WithContext(ctx).Model(&ShopAuthToken{}).Where("shop_id = ? AND EXISTS (SELECT 1 FROM shops sh WHERE sh.id = shop_auth_tokens.shop_id AND sh.tenant_id = ?)", shopID, tenantID).Update("merchant_id", cipher).Error
			updates := map[string]any{}
			if nm != "" {
				updates["shop_name"] = nm
			}
			if ext != "" {
				updates["external_shop_id"] = ext
			}
			if region != "" {
				updates["region"] = region
			}
			if currency != "" {
				updates["currency"] = strings.ToUpper(currency)
			}
			updates["auth_status"] = AuthAuthorized
			if len(updates) > 0 {
				_ = s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenantID).Updates(updates).Error
			}
		}
	}

	s.oauthTikTokLog(c, adminID, shopID, "success", "tiktok oauth tokens saved")
	return s.GetDetail(c, shopID)
}

func (s *Service) oauthTikTokLog(c *gin.Context, adminID *uuid.UUID, shopID uuid.UUID, st, msg string) {
	if s.OpLog == nil {
		return
	}
	action := "shop.oauth.tiktok.success"
	if st != "success" {
		action = "shop.oauth.tiktok.failed"
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
