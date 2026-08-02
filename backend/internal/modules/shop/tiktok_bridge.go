package shop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
	"gorm.io/gorm"
)

// TikTokShopsBridge satisfies platform/tiktok persistence hooks.
func (s *Service) TikTokShopsBridge() platformtiktok.ShopsBridge {
	return tikTokBridge{svc: s}
}

type tikTokBridge struct {
	svc *Service
}

func (b tikTokBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	tenantID, err := trustedBridgeTenant(ctx)
	if err != nil {
		return err
	}
	return b.svc.persistOAuthTokenRefresh(ctx, tenantID, shopID, access, refresh, accessExp, refreshExp)
}

func (b tikTokBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	tenantID, err := trustedBridgeTenant(ctx)
	if err != nil {
		return err
	}
	return b.svc.setAuthStatusCtx(ctx, tenantID, shopID, status)
}

func (b tikTokBridge) TikTokGlobalSettings(ctx context.Context) (map[string]string, error) {
	m, err := b.svc.tiktokGlobalSettingsPlain(ctx)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func (b tikTokBridge) TikTokPublishSettings(ctx context.Context) (map[string]string, error) {
	m, err := b.svc.tiktokPublishSettingsPlain(ctx)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func (s *Service) tiktokGlobalSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_tiktok")
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range m {
		out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	return out, nil
}

func (s *Service) tiktokPublishSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_publish_tiktok")
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range m {
		out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	return out, nil
}

func trustedBridgeTenant(ctx context.Context) (int64, error) {
	tc := security.FromContext(ctx)
	if tc == nil || tc.TenantID < 0 {
		return 0, security.ErrTenantContextMissing
	}
	return tc.TenantID, nil
}

func (s *Service) setAuthStatusCtx(ctx context.Context, tenantID int64, shopID uuid.UUID, status string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	st := strings.TrimSpace(status)
	if st == "" {
		return nil
	}
	res := s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenantID).Update("auth_status", st)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) persistOAuthTokenRefresh(ctx context.Context, tenantID int64, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	if s.Encrypter == nil {
		return fmt.Errorf("shop: encryption not configured")
	}
	var tok ShopAuthToken
	if err := s.DB.WithContext(ctx).
		Where("shop_id = ? AND EXISTS (SELECT 1 FROM shops sh WHERE sh.id = shop_auth_tokens.shop_id AND sh.tenant_id = ? AND sh.deleted_at IS NULL)", shopID, tenantID).
		First(&tok).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"expires_at":         accessExp,
		"refresh_expires_at": refreshExp,
		"token_version":      gorm.Expr("token_version + 1"),
	}
	if strings.TrimSpace(access) != "" {
		ct, err := s.Encrypter.Encrypt([]byte(strings.TrimSpace(access)))
		if err != nil {
			return err
		}
		updates["access_token_enc"] = ct
	}
	if strings.TrimSpace(refresh) != "" {
		ct, err := s.Encrypter.Encrypt([]byte(strings.TrimSpace(refresh)))
		if err != nil {
			return err
		}
		updates["refresh_token_enc"] = ct
	}
	res := s.DB.WithContext(ctx).Model(&ShopAuthToken{}).
		Where("id = ? AND shop_id = ? AND EXISTS (SELECT 1 FROM shops sh WHERE sh.id = shop_auth_tokens.shop_id AND sh.tenant_id = ? AND sh.deleted_at IS NULL)", tok.ID, shopID, tenantID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
