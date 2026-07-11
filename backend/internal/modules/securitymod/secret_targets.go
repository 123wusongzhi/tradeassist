package securitymod

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
	"gorm.io/gorm"
)

// ReencryptScope identifies tenant or system scope for secret targets.
type ReencryptScope struct {
	TenantID int64
}

// ReencryptCursor is pagination for target listing.
type ReencryptCursor struct {
	LastID string
}

// ReencryptItem is one record to re-encrypt.
type ReencryptItem struct {
	RecordID   string
	TenantID   int64
	Ciphertext string
	KeyID      string
}

// ReencryptTargetAdapter counts and re-encrypts secrets for one storage location.
type ReencryptTargetAdapter interface {
	Name() string
	CountByKeyID(ctx context.Context, scope ReencryptScope, keyID string) (int64, error)
	ListByKeyID(ctx context.Context, scope ReencryptScope, keyID string, cursor ReencryptCursor, limit int) ([]ReencryptItem, ReencryptCursor, error)
	Reencrypt(ctx context.Context, item ReencryptItem, targetKeyID string) error
}

// SettingsSecretTarget covers encrypted settings.item_value rows.
type SettingsSecretTarget struct {
	DB *gorm.DB
	KR *crypto.KeyRing
}

func (t *SettingsSecretTarget) Name() string { return "settings_encrypted" }

func (t *SettingsSecretTarget) CountByKeyID(ctx context.Context, scope ReencryptScope, keyID string) (int64, error) {
	if t == nil || t.DB == nil {
		return 0, fmt.Errorf("settings target: unavailable")
	}
	var rows []struct {
		ItemValue string
	}
	q := t.DB.WithContext(ctx).Table("settings").Select("item_value").Where("is_encrypted = ?", true)
	if scope.TenantID > 0 {
		q = q.Where("tenant_id = ?", scope.TenantID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return 0, err
	}
	var n int64
	for _, r := range rows {
		kid, ok := crypto.ParseKeyID(strings.TrimSpace(r.ItemValue))
		if ok && kid == keyID {
			n++
		}
	}
	return n, nil
}

func (t *SettingsSecretTarget) ListByKeyID(ctx context.Context, scope ReencryptScope, keyID string, cursor ReencryptCursor, limit int) ([]ReencryptItem, ReencryptCursor, error) {
	if t == nil || t.DB == nil {
		return nil, cursor, fmt.Errorf("settings target: unavailable")
	}
	if limit <= 0 {
		limit = 50
	}
	var rows []struct {
		ID        int64
		TenantID  int64
		ItemValue string
	}
	q := t.DB.WithContext(ctx).Table("settings").Select("id, tenant_id, item_value").Where("is_encrypted = ?", true)
	if scope.TenantID > 0 {
		q = q.Where("tenant_id = ?", scope.TenantID)
	}
	if cursor.LastID != "" {
		q = q.Where("id > ?", cursor.LastID)
	}
	if err := q.Order("id ASC").Limit(limit * 3).Find(&rows).Error; err != nil {
		return nil, cursor, err
	}
	out := make([]ReencryptItem, 0, limit)
	next := cursor
	for _, r := range rows {
		next.LastID = fmt.Sprintf("%d", r.ID)
		v := strings.TrimSpace(r.ItemValue)
		kid, ok := crypto.ParseKeyID(v)
		if !ok || kid != keyID {
			continue
		}
		out = append(out, ReencryptItem{
			RecordID:   next.LastID,
			TenantID:   r.TenantID,
			Ciphertext: v,
			KeyID:      kid,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, next, nil
}

func (t *SettingsSecretTarget) Reencrypt(ctx context.Context, item ReencryptItem, targetKeyID string) error {
	if t == nil || t.DB == nil || t.KR == nil {
		return fmt.Errorf("settings target: unavailable")
	}
	plain, err := t.KR.Decrypt(item.Ciphertext)
	if err != nil {
		return err
	}
	cipher, err := t.KR.Encrypt(plain)
	if err != nil {
		return err
	}
	res := t.DB.WithContext(ctx).Table("settings").
		Where("id = ? AND item_value = ?", item.RecordID, item.Ciphertext).
		Update("item_value", cipher)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	_ = targetKeyID
	return nil
}

// ShopAuthTokenTarget covers shop_auth_tokens encrypted columns.
type ShopAuthTokenTarget struct {
	DB *gorm.DB
	KR *crypto.KeyRing
}

func (t *ShopAuthTokenTarget) Name() string { return "shop_auth_tokens" }

func (t *ShopAuthTokenTarget) encryptedFields() []string {
	return []string{"app_secret_enc", "access_token_enc", "refresh_token_enc"}
}

func (t *ShopAuthTokenTarget) CountByKeyID(ctx context.Context, scope ReencryptScope, keyID string) (int64, error) {
	items, _, err := t.ListByKeyID(ctx, scope, keyID, ReencryptCursor{}, 10000)
	if err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

func (t *ShopAuthTokenTarget) ListByKeyID(ctx context.Context, scope ReencryptScope, keyID string, cursor ReencryptCursor, limit int) ([]ReencryptItem, ReencryptCursor, error) {
	if t == nil || t.DB == nil {
		return nil, cursor, fmt.Errorf("shop token target: unavailable")
	}
	if limit <= 0 {
		limit = 50
	}
	var rows []struct {
		ID              uuid.UUID
		AppSecretEnc    string
		AccessTokenEnc  string
		RefreshTokenEnc string
	}
	q := t.DB.WithContext(ctx).Table("shop_auth_tokens").Select("id, app_secret_enc, access_token_enc, refresh_token_enc")
	if cursor.LastID != "" {
		q = q.Where("id > ?", cursor.LastID)
	}
	if scope.TenantID > 0 {
		q = q.Where("shop_id IN (SELECT id FROM shops WHERE tenant_id = ?)", scope.TenantID)
	}
	if err := q.Order("id ASC").Limit(limit * 2).Find(&rows).Error; err != nil {
		return nil, cursor, err
	}
	out := make([]ReencryptItem, 0)
	next := cursor
	for _, r := range rows {
		next.LastID = r.ID.String()
		for _, pair := range []struct {
			field string
			val   string
		}{
			{"app_secret_enc", r.AppSecretEnc},
			{"access_token_enc", r.AccessTokenEnc},
			{"refresh_token_enc", r.RefreshTokenEnc},
		} {
			v := strings.TrimSpace(pair.val)
			if v == "" {
				continue
			}
			kid, ok := crypto.ParseKeyID(v)
			if !ok || kid != keyID {
				continue
			}
			out = append(out, ReencryptItem{
				RecordID:   r.ID.String() + ":" + pair.field,
				Ciphertext: v,
				KeyID:      kid,
			})
			if len(out) >= limit {
				return out, next, nil
			}
		}
	}
	return out, next, nil
}

func (t *ShopAuthTokenTarget) Reencrypt(ctx context.Context, item ReencryptItem, targetKeyID string) error {
	if t == nil || t.DB == nil || t.KR == nil {
		return fmt.Errorf("shop token target: unavailable")
	}
	parts := strings.SplitN(item.RecordID, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid shop token record id")
	}
	shopTokenID, field := parts[0], parts[1]
	plain, err := t.KR.Decrypt(item.Ciphertext)
	if err != nil {
		return err
	}
	cipher, err := t.KR.Encrypt(plain)
	if err != nil {
		return err
	}
	res := t.DB.WithContext(ctx).Table("shop_auth_tokens").
		Where("id = ? AND "+field+" = ?", shopTokenID, item.Ciphertext).
		Update(field, cipher)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	_ = targetKeyID
	return nil
}

// AllReencryptTargets returns registered secret targets for rotation.
func AllReencryptTargets(db *gorm.DB, kr *crypto.KeyRing) []ReencryptTargetAdapter {
	return []ReencryptTargetAdapter{
		&SettingsSecretTarget{DB: db, KR: kr},
		&ShopAuthTokenTarget{DB: db, KR: kr},
	}
}
