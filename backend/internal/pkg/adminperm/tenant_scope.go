package adminperm

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// TenantIDFromGin returns trusted tenant id from request context.
func TenantIDFromGin(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, errTenantContextMissing
	}
	if v, ok := c.Get(ctxkey.TenantID); ok {
		switch tid := v.(type) {
		case int64:
			if tid > 0 {
				return tid, nil
			}
		case int:
			if tid > 0 {
				return int64(tid), nil
			}
		}
	}
	return 0, errTenantContextMissing
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
