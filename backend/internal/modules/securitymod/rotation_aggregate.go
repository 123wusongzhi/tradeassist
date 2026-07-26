package securitymod

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
)

func (s *Service) reencryptTargets() ([]ReencryptTargetAdapter, *crypto.KeyRing, error) {
	if s == nil || s.DB == nil {
		return nil, nil, fmt.Errorf("security: unavailable")
	}
	kr, err := s.keyRing()
	if err != nil {
		return nil, nil, err
	}
	return AllReencryptTargets(s.DB, kr), kr, nil
}

func countKey(targetName, table, field string, tenantID int64, keyID string, counts map[string]*SecretReferenceCount) *SecretReferenceCount {
	k := fmt.Sprintf("%s|%s|%s|%s|%d", targetName, keyID, table, field, tenantID)
	if counts[k] == nil {
		counts[k] = &SecretReferenceCount{
			TableName: table,
			FieldName: field,
			TenantID:  tenantID,
			KeyID:     keyID,
		}
	}
	return counts[k]
}

func (s *Service) aggregateSecretReferences(ctx context.Context, previousKeyIDs []string) ([]SecretReferenceCount, error) {
	targets, kr, err := s.reencryptTargets()
	if err != nil {
		return nil, err
	}
	counts := map[string]*SecretReferenceCount{}
	scope := ReencryptScope{}

	for _, target := range targets {
		switch t := target.(type) {
		case *SettingsSecretTarget:
			if err := s.scanSettingsReferences(ctx, kr, previousKeyIDs, t.Name(), counts); err != nil {
				return nil, err
			}
		case *ShopAuthTokenTarget:
			if err := s.scanShopTokenReferences(ctx, kr, previousKeyIDs, t.Name(), scope, counts); err != nil {
				return nil, err
			}
		default:
			for _, kid := range previousKeyIDs {
				n, err := target.CountByKeyID(ctx, scope, kid)
				if err != nil {
					return nil, fmt.Errorf("count %s: %w", target.Name(), err)
				}
				if n > 0 {
					c := countKey(target.Name(), target.Name(), "encrypted", 0, kid, counts)
					c.ReferenceCount += n
				}
			}
		}
	}
	out := make([]SecretReferenceCount, 0, len(counts))
	for _, c := range counts {
		out = append(out, *c)
	}
	return out, nil
}

func (s *Service) scanSettingsReferences(ctx context.Context, kr *crypto.KeyRing, previousKeyIDs []string, targetName string, counts map[string]*SecretReferenceCount) error {
	var rows []struct {
		ID        int64
		ItemValue string
		TenantID  int64
	}
	if err := s.DB.WithContext(ctx).Table("settings").
		Select("id, item_value, tenant_id").
		Where("is_encrypted = ?", true).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		st, kid := classifyCiphertext(kr, r.ItemValue, previousKeyIDs)
		switch st {
		case ciphertextNeedsReencrypt:
			c := countKey(targetName, "settings", "item_value", r.TenantID, kid, counts)
			c.ReferenceCount++
			if kr != nil {
				if _, err := kr.Decrypt(strings.TrimSpace(r.ItemValue)); err != nil {
					c.DecryptFailures++
				}
			}
		case ciphertextUnknown:
			c := countKey(targetName, "settings", "item_value", r.TenantID, "unknown", counts)
			c.UnknownFormat++
		}
	}
	return nil
}

func (s *Service) scanShopTokenReferences(ctx context.Context, kr *crypto.KeyRing, previousKeyIDs []string, targetName string, scope ReencryptScope, counts map[string]*SecretReferenceCount) error {
	var rows []struct {
		ID              uuid.UUID
		AppSecretEnc    string
		AccessTokenEnc  string
		RefreshTokenEnc string
		TenantID        int64
	}
	q := s.DB.WithContext(ctx).Table("shop_auth_tokens").
		Select("shop_auth_tokens.id, shop_auth_tokens.app_secret_enc, shop_auth_tokens.access_token_enc, shop_auth_tokens.refresh_token_enc, shops.tenant_id").
		Joins("JOIN shops ON shops.id = shop_auth_tokens.shop_id")
	if scope.TenantID > 0 {
		q = q.Where("shops.tenant_id = ?", scope.TenantID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		for _, pair := range []struct {
			field string
			val   string
		}{
			{"app_secret_enc", r.AppSecretEnc},
			{"access_token_enc", r.AccessTokenEnc},
			{"refresh_token_enc", r.RefreshTokenEnc},
		} {
			st, kid := classifyCiphertext(kr, pair.val, previousKeyIDs)
			switch st {
			case ciphertextNeedsReencrypt:
				c := countKey(targetName, "shop_auth_tokens", pair.field, r.TenantID, kid, counts)
				c.ReferenceCount++
				if kr != nil {
					if _, err := kr.Decrypt(strings.TrimSpace(pair.val)); err != nil {
						c.DecryptFailures++
					}
				}
			case ciphertextUnknown:
				c := countKey(targetName, "shop_auth_tokens", pair.field, r.TenantID, "unknown", counts)
				c.UnknownFormat++
			}
		}
	}
	return nil
}

func (s *Service) countPendingReencrypt(ctx context.Context) (int64, error) {
	counts, err := s.aggregateSecretReferences(ctx, nil)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, c := range counts {
		total += c.ReferenceCount
	}
	return total, nil
}

func (s *Service) processTargetReencryptBatch(ctx context.Context, job *KeyRotationJob, target ReencryptTargetAdapter, kr *crypto.KeyRing, batchSize int) (int, bool, error) {
	if target == nil || kr == nil {
		return 0, false, nil
	}
	processed := 0
	switch t := target.(type) {
	case *SettingsSecretTarget:
		n, done, err := s.reencryptSettingsBatch(ctx, job, t, kr, batchSize)
		return n, done, err
	case *ShopAuthTokenTarget:
		n, done, err := s.reencryptShopTokensBatch(ctx, job, t, kr, batchSize)
		return n, done, err
	}
	return processed, true, nil
}

func (s *Service) reencryptSettingsBatch(ctx context.Context, job *KeyRotationJob, target *SettingsSecretTarget, kr *crypto.KeyRing, batchSize int) (int, bool, error) {
	var rows []struct {
		ID        int64
		ItemValue string
		TenantID  int64
	}
	q := s.DB.WithContext(ctx).Table("settings").
		Select("id, item_value, tenant_id").
		Where("is_encrypted = ?", true)
	if job.LastCursor != "" {
		q = q.Where("id > ?", job.LastCursor)
	}
	if err := q.Order("id ASC").Limit(batchSize * 2).Find(&rows).Error; err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, true, nil
	}
	previous := strings.Split(strings.TrimSpace(job.SourceKeyIDs), ",")
	processed := 0
	lastID := job.LastCursor
	for _, r := range rows {
		lastID = fmt.Sprintf("%d", r.ID)
		v := strings.TrimSpace(r.ItemValue)
		st, kid := classifyCiphertext(kr, v, previous)
		if st == ciphertextActive || st == ciphertextEmpty {
			job.SkippedRecords++
			continue
		}
		if st == ciphertextUnknown {
			job.FailedRecords++
			_ = s.recordFailure(ctx, job.ID, "settings", lastID, r.TenantID, "unknown", "secret_key_unknown", "unknown encryption format")
			continue
		}
		item := ReencryptItem{RecordID: lastID, TenantID: r.TenantID, Ciphertext: v, KeyID: kid}
		if err := target.Reencrypt(ctx, item, kr.ActiveID); err != nil {
			job.FailedRecords++
			_ = s.recordFailure(ctx, job.ID, "settings", lastID, r.TenantID, kid, "secret_reencrypt_failed", "reencrypt failed")
			continue
		}
		job.ReencryptedRecords++
		processed++
		if processed >= batchSize {
			job.LastCursor = lastID
			return processed, false, nil
		}
	}
	job.LastCursor = ""
	return processed, true, nil
}

func (s *Service) reencryptShopTokensBatch(ctx context.Context, job *KeyRotationJob, target *ShopAuthTokenTarget, kr *crypto.KeyRing, batchSize int) (int, bool, error) {
	var rows []struct {
		ID              uuid.UUID
		AppSecretEnc    string
		AccessTokenEnc  string
		RefreshTokenEnc string
		TenantID        int64
	}
	q := s.DB.WithContext(ctx).Table("shop_auth_tokens").
		Select("shop_auth_tokens.id, shop_auth_tokens.app_secret_enc, shop_auth_tokens.access_token_enc, shop_auth_tokens.refresh_token_enc, shops.tenant_id").
		Joins("JOIN shops ON shops.id = shop_auth_tokens.shop_id")
	if job.LastCursor != "" {
		q = q.Where("shop_auth_tokens.id > ?", job.LastCursor)
	}
	if err := q.Order("shop_auth_tokens.id ASC").Limit(batchSize * 2).Find(&rows).Error; err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, true, nil
	}
	previous := strings.Split(strings.TrimSpace(job.SourceKeyIDs), ",")
	processed := 0
	lastID := job.LastCursor
	for _, r := range rows {
		lastID = r.ID.String()
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
			st, kid := classifyCiphertext(kr, v, previous)
			if st == ciphertextActive {
				continue
			}
			if st == ciphertextUnknown {
				job.FailedRecords++
				_ = s.recordFailure(ctx, job.ID, "shop_auth_tokens", lastID+":"+pair.field, r.TenantID, "unknown", "secret_key_unknown", "unknown encryption format")
				continue
			}
			item := ReencryptItem{
				RecordID:   lastID + ":" + pair.field,
				TenantID:   r.TenantID,
				Ciphertext: v,
				KeyID:      kid,
			}
			if err := target.Reencrypt(ctx, item, kr.ActiveID); err != nil {
				job.FailedRecords++
				_ = s.recordFailure(ctx, job.ID, "shop_auth_tokens", item.RecordID, r.TenantID, kid, "secret_reencrypt_failed", "reencrypt failed")
				continue
			}
			job.ReencryptedRecords++
			processed++
			if processed >= batchSize {
				job.LastCursor = lastID
				return processed, false, nil
			}
		}
	}
	job.LastCursor = ""
	return processed, true, nil
}

func targetIndexByName(targets []ReencryptTargetAdapter, name string) int {
	for i, t := range targets {
		if t.Name() == name {
			return i
		}
	}
	return 0
}
