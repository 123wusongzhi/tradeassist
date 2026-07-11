package adminperm

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// TenantIDFromGin returns trusted tenant id from request context.
// tenant_id=0 is valid for legacy single-tenant development data.
func TenantIDFromGin(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, errTenantContextMissing
	}
	v, ok := c.Get(ctxkey.TenantID)
	if !ok {
		return 0, errTenantContextMissing
	}
	switch tid := v.(type) {
	case int64:
		return tid, nil
	case int:
		return int64(tid), nil
	default:
		return 0, errTenantContextMissing
	}
}

// ApplyTenantScope restricts query to current tenant for all roles.
func ApplyTenantScope(c *gin.Context, tx *gorm.DB) (*gorm.DB, int64, error) {
	if tx == nil {
		return tx, 0, nil
	}
	tid, err := TenantIDFromGin(c)
	if err != nil {
		return nil, 0, err
	}
	return tx.Where("tenant_id = ?", tid), tid, nil
}
