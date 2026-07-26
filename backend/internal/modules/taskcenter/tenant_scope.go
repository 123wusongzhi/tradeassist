package taskcenter

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tenantquery"
	"gorm.io/gorm"
)

// applyListTenantScope adds tenant filter from gin context to task list queries.
func (s *Service) applyListTenantScope(c *gin.Context, tx *gorm.DB, tenantColumn string) (*gorm.DB, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, gorm.ErrInvalidDB
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, 0, err
	}
	if tenantColumn == "" {
		return tenantquery.ScopeTenant(tx, tid), tid, nil
	}
	return tenantquery.ScopeTenant(tx, tid), tid, nil
}

// tenantIDFromGin extracts trusted tenant for task commands.
func tenantIDFromGin(c *gin.Context) (int64, error) {
	return adminperm.TenantIDFromGin(c)
}
