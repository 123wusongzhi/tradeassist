package repository

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// TenantResolver resolves trusted tenant id for a request.
type TenantResolver interface {
	ResolveRequestTenantID(rawTenantID int64) (int64, string, error)
}

// RequireTenantID extracts tenant from gin context with optional dev/demo fallback.
func RequireTenantID(c *gin.Context, cfg TenantResolver) (int64, error) {
	if c == nil {
		return 0, security.ErrTenantContextMissing
	}
	tc := security.FromGin(c)
	raw := int64(0)
	if tc != nil {
		raw = tc.TenantID
	}
	if cfg != nil {
		tid, _, err := cfg.ResolveRequestTenantID(raw)
		if err != nil {
			return 0, err
		}
		if tid > 0 {
			return tid, nil
		}
	}
	if raw > 0 {
		return raw, nil
	}
	return 0, security.ErrTenantContextMissing
}

// ScopeTenant adds tenant_id filter to query.
func ScopeTenant(tx *gorm.DB, tenantID int64) *gorm.DB {
	return security.TenantScopedQuery(tx, tenantID)
}

// ApplyTenantScope scopes query by trusted tenant from gin context.
func ApplyTenantScope(c *gin.Context, cfg TenantResolver, tx *gorm.DB) (*gorm.DB, int64, error) {
	tid, err := RequireTenantID(c, cfg)
	if err != nil {
		return nil, 0, err
	}
	return ScopeTenant(tx, tid), tid, nil
}

// FindByID loads one row with tenant scope.
func FindByID(ctx context.Context, tx *gorm.DB, dest any, tenantID int64, id any) error {
	if tx == nil {
		return fmt.Errorf("repository: nil db")
	}
	q := ScopeTenant(tx.WithContext(ctx), tenantID)
	if err := q.First(dest, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}

// DeleteByID deletes with tenant scope.
func DeleteByID(ctx context.Context, tx *gorm.DB, model any, tenantID int64, id any) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("repository: nil db")
	}
	res := ScopeTenant(tx.WithContext(ctx), tenantID).Delete(model, "id = ?", id)
	return res.RowsAffected, res.Error
}

// UpdateByID saves model after tenant-scoped load.
func UpdateByID(ctx context.Context, tx *gorm.DB, model any, tenantID int64, id any, updates any) error {
	if tx == nil {
		return fmt.Errorf("repository: nil db")
	}
	res := ScopeTenant(tx.WithContext(ctx), tenantID).Model(model).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SystemFindByID is for internal system queries without tenant filter (must be audited).
func SystemFindByID(ctx context.Context, tx *gorm.DB, dest any, id any) error {
	if SystemFromContext(ctx) == nil {
		return security.ErrSystemContextRequired
	}
	return tx.WithContext(ctx).First(dest, "id = ?", id).Error
}

// SystemFromContext re-exports security system context check.
func SystemFromContext(ctx context.Context) *security.SystemContext {
	return security.SystemFromContext(ctx)
}

// ConfigAdapter wraps *config.Config as TenantResolver.
type ConfigAdapter struct {
	Cfg *config.Config
}

func (a ConfigAdapter) ResolveRequestTenantID(rawTenantID int64) (int64, string, error) {
	if a.Cfg == nil {
		return 0, "", security.ErrTenantContextMissing
	}
	return a.Cfg.ResolveRequestTenantID(rawTenantID)
}
