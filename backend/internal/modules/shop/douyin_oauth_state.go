package shop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// DouyinOAuthState persists OAuth state hash for one-time-use verification.
// In addition to the Redis TTL (fast path), the DB record provides:
//   - Durability across Redis restarts
//   - One-time-consume enforcement (ConsumedAt)
//   - Audit trail
type DouyinOAuthState struct {
	model.Base
	StateHash          string     `gorm:"size:128;uniqueIndex;not null" json:"stateHash"`
	UserID             *uuid.UUID `gorm:"type:char(36);index" json:"userId,omitempty"`
	ShopBindingContext string     `gorm:"size:255" json:"shopBindingContext,omitempty"`
	RedirectURL        string     `gorm:"type:text" json:"redirectUrl,omitempty"`
	ExpiresAt          time.Time  `gorm:"index;not null" json:"expiresAt"`
	ConsumedAt         *time.Time `gorm:"index" json:"consumedAt,omitempty"`
}

func (DouyinOAuthState) TableName() string { return "douyin_oauth_states" }

// IsExpired returns true if the state is past its expiry time.
func (s *DouyinOAuthState) IsExpired() bool {
	if s == nil {
		return true
	}
	return time.Now().UTC().After(s.ExpiresAt)
}

// IsConsumed returns true if the state has already been used.
func (s *DouyinOAuthState) IsConsumed() bool {
	return s != nil && s.ConsumedAt != nil
}

func hashDouyinOAuthState(rawState string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawState)))
	return hex.EncodeToString(sum[:])
}

func (s *Service) persistDouyinOAuthState(ctx context.Context, rawState string, userID, shopID *uuid.UUID, redirectURL string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: db unavailable")
	}
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		return douyinErr(platformdouyin.CodeDouyinOAuthRedirectNotAllowed, "抖店回调地址未配置，禁止发起授权。", nil)
	}
	row := DouyinOAuthState{
		StateHash:          hashDouyinOAuthState(rawState),
		UserID:             userID,
		ShopBindingContext: "",
		RedirectURL:        redirectURL,
		ExpiresAt:          time.Now().UTC().Add(10 * time.Minute),
	}
	if shopID != nil && *shopID != uuid.Nil {
		row.ShopBindingContext = shopID.String()
	}
	return s.DB.WithContext(ctx).Create(&row).Error
}

// consumeDouyinOAuthState marks state as used exactly once. Returns typed DouyinAuthError on failure.
func (s *Service) consumeDouyinOAuthState(ctx context.Context, rawState string) error {
	if s == nil || s.DB == nil {
		return douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), nil)
	}
	hash := hashDouyinOAuthState(rawState)
	var row DouyinOAuthState
	if err := s.DB.WithContext(ctx).Where("state_hash = ?", hash).First(&row).Error; err != nil {
		return douyinErr(platformdouyin.CodeDouyinOAuthStateMissing, "抖店授权状态无效或不存在，请重新发起授权。", err)
	}
	if row.IsConsumed() {
		return douyinErr(platformdouyin.CodeDouyinOAuthStateAlreadyUsed, "抖店授权状态已使用，请重新发起授权。", nil)
	}
	if row.IsExpired() {
		return douyinErr(platformdouyin.CodeDouyinOAuthStateExpired, "抖店授权状态已过期，请重新发起授权。", nil)
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&DouyinOAuthState{}).
		Where("id = ? AND consumed_at IS NULL", row.ID).
		Updates(map[string]any{"consumed_at": now, "updated_at": now})
	if res.Error != nil {
		return douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), res.Error)
	}
	if res.RowsAffected != 1 {
		return douyinErr(platformdouyin.CodeDouyinOAuthStateAlreadyUsed, "抖店授权状态已使用，请重新发起授权。", nil)
	}
	return nil
}
