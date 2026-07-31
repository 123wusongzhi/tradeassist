package collectextension

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"gorm.io/gorm"
)

const (
	pairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	pairingCodeLength   = 10
	pairingTTL          = 10 * time.Minute
	deviceTTL           = 90 * 24 * time.Hour
)

var (
	ErrPairingInvalid       = errors.New("pairing code is invalid or expired")
	ErrDeviceUnauthorized   = errors.New("browser extension device token is invalid or expired")
	ErrDeviceAlreadyRevoked = errors.New("browser extension device is already revoked")
)

type Service struct {
	DB      *gorm.DB
	Collect *collect.Service
	OpLog   *operationlog.Service
}

func normalizePairingCode(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, " ", "")
	return raw
}

func displayPairingCode(raw string) string {
	raw = normalizePairingCode(raw)
	if len(raw) == pairingCodeLength {
		return raw[:5] + "-" + raw[5:]
	}
	return raw
}

func newPairingCode() (string, error) {
	var out strings.Builder
	out.Grow(pairingCodeLength)
	max := big.NewInt(int64(len(pairingCodeAlphabet)))
	for i := 0; i < pairingCodeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("pairing code: %w", err)
		}
		out.WriteByte(pairingCodeAlphabet[n.Int64()])
	}
	return out.String(), nil
}

func cleanDeviceName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = "Chrome 侧边栏"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	return name
}

func (s *Service) CreatePairing(ctx context.Context, tenantID int64, adminID uuid.UUID) (CreatePairingResult, error) {
	if s == nil || s.DB == nil || adminID == uuid.Nil {
		return CreatePairingResult{}, fmt.Errorf("browser extension pairing unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	_ = s.DB.WithContext(ctx).
		Where("expires_at < ? OR used_at IS NOT NULL", now.Add(-24*time.Hour)).
		Delete(&BrowserExtensionPairing{}).Error

	for attempt := 0; attempt < 5; attempt++ {
		code, err := newPairingCode()
		if err != nil {
			return CreatePairingResult{}, err
		}
		expiresAt := now.Add(pairingTTL)
		row := &BrowserExtensionPairing{
			TenantID:    tenantID,
			AdminUserID: adminID,
			CodeHash:    authutil.HashToken(code, ""),
			ExpiresAt:   expiresAt,
		}
		if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
			continue
		}
		return CreatePairingResult{Code: displayPairingCode(code), ExpiresAt: expiresAt}, nil
	}
	return CreatePairingResult{}, fmt.Errorf("could not allocate a unique pairing code")
}

func (s *Service) ExchangePairing(
	ctx context.Context,
	code, deviceName string,
) (ExchangePairingResult, error) {
	if s == nil || s.DB == nil {
		return ExchangePairingResult{}, fmt.Errorf("browser extension pairing unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := normalizePairingCode(code)
	if len(normalized) != pairingCodeLength {
		return ExchangePairingResult{}, ErrPairingInvalid
	}
	now := time.Now().UTC()
	var result ExchangePairingResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pairing BrowserExtensionPairing
		if err := tx.
			Where("code_hash = ? AND used_at IS NULL AND expires_at > ?", authutil.HashToken(normalized, ""), now).
			First(&pairing).Error; err != nil {
			return ErrPairingInvalid
		}
		used := tx.Model(&BrowserExtensionPairing{}).
			Where("id = ? AND used_at IS NULL", pairing.ID).
			Update("used_at", now)
		if used.Error != nil {
			return used.Error
		}
		if used.RowsAffected != 1 {
			return ErrPairingInvalid
		}

		rawToken, err := authutil.NewOpaqueToken(32)
		if err != nil {
			return err
		}
		rawToken = "tmx_" + rawToken
		device := &BrowserExtensionDevice{
			TenantID:    pairing.TenantID,
			AdminUserID: pairing.AdminUserID,
			Name:        cleanDeviceName(deviceName),
			TokenHash:   authutil.HashToken(rawToken, ""),
			Status:      DeviceStatusActive,
			ExpiresAt:   now.Add(deviceTTL),
		}
		if err := tx.Create(device).Error; err != nil {
			return err
		}
		result = ExchangePairingResult{
			Device:      deviceToDTO(device),
			DeviceToken: rawToken,
		}
		return nil
	})
	if err != nil {
		return ExchangePairingResult{}, err
	}
	if s.OpLog != nil {
		var device BrowserExtensionDevice
		if err := s.DB.WithContext(ctx).First(&device, "id = ?", result.Device.ID).Error; err == nil {
			adminID := device.AdminUserID
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				TenantID:    device.TenantID,
				AdminUserID: &adminID,
				Action:      "collect.browser_extension.device.paired",
				Resource:    "browser_extension_device",
				ResourceID:  device.ID.String(),
				Status:      "success",
				Message:     "browser extension device paired",
			})
		}
	}
	return result, nil
}

func (s *Service) AuthenticateDevice(ctx context.Context, rawToken string) (*BrowserExtensionDevice, error) {
	if s == nil || s.DB == nil {
		return nil, ErrDeviceUnauthorized
	}
	rawToken = strings.TrimSpace(rawToken)
	if !strings.HasPrefix(rawToken, "tmx_") || len(rawToken) < 32 {
		return nil, ErrDeviceUnauthorized
	}
	now := time.Now().UTC()
	var device BrowserExtensionDevice
	if err := s.DB.WithContext(ctx).
		Where("token_hash = ? AND status = ? AND expires_at > ?",
			authutil.HashToken(rawToken, ""), DeviceStatusActive, now).
		First(&device).Error; err != nil {
		return nil, ErrDeviceUnauthorized
	}
	if device.LastUsedAt == nil || now.Sub(*device.LastUsedAt) >= 5*time.Minute {
		_ = s.DB.WithContext(ctx).Model(&BrowserExtensionDevice{}).
			Where("id = ?", device.ID).
			Update("last_used_at", now).Error
		device.LastUsedAt = &now
	}
	return &device, nil
}

func (s *Service) ListDevices(ctx context.Context, tenantID int64) ([]DeviceDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("browser extension pairing unavailable")
	}
	var rows []BrowserExtensionDevice
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(100).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]DeviceDTO, 0, len(rows))
	for i := range rows {
		out = append(out, deviceToDTO(&rows[i]))
	}
	return out, nil
}

func (s *Service) RevokeDevice(ctx context.Context, tenantID int64, deviceID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("browser extension pairing unavailable")
	}
	now := time.Now().UTC()
	result := s.DB.WithContext(ctx).Model(&BrowserExtensionDevice{}).
		Where("id = ? AND tenant_id = ? AND status = ?", deviceID, tenantID, DeviceStatusActive).
		Updates(map[string]any{
			"status":     DeviceStatusRevoked,
			"revoked_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := s.DB.WithContext(ctx).Model(&BrowserExtensionDevice{}).
			Where("id = ? AND tenant_id = ?", deviceID, tenantID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		return ErrDeviceAlreadyRevoked
	}
	return nil
}
