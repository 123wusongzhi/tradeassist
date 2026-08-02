package imagetask

import (
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func requestTenantID(c *gin.Context) (int64, error) {
	return adminperm.TenantIDFromGin(c)
}

func handlerTenantID(c *gin.Context) (int64, bool) {
	tenantID, err := requestTenantID(c)
	if err != nil {
		response.Fail(c, 401, response.CodeUnauthorized, "tenant context required")
		return 0, false
	}
	return tenantID, true
}

func handleTenantResourceError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, 404, response.CodeNotFound, "not found")
		return
	}
	response.HandleError(c, err)
}

func (s *Service) resolveCreateTenant(ctx context.Context, p CreatePayload) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("imagetask: no db")
	}
	var (
		resolved int64
		have     bool
	)
	if p.TenantID > 0 {
		resolved, have = p.TenantID, true
	}
	merge := func(candidate int64) error {
		if !have {
			resolved, have = candidate, true
			return nil
		}
		if resolved != candidate {
			return fmt.Errorf("imagetask: tenant mismatch")
		}
		return nil
	}

	if p.ProductID != nil && *p.ProductID != uuid.Nil {
		var row product.Product
		if err := s.DB.WithContext(ctx).Select("tenant_id").First(&row, "id = ?", *p.ProductID).Error; err != nil {
			return 0, err
		}
		if err := merge(row.TenantID); err != nil {
			return 0, err
		}
	}
	if p.SourceImageID != nil && *p.SourceImageID != uuid.Nil {
		var sourceFile files.FileRecord
		fileErr := s.DB.WithContext(ctx).Select("tenant_id").First(&sourceFile, "id = ?", *p.SourceImageID).Error
		if fileErr == nil {
			if err := merge(sourceFile.TenantID); err != nil {
				return 0, err
			}
		} else if fileErr != gorm.ErrRecordNotFound {
			return 0, fileErr
		}

		var sourceProduct struct{ TenantID int64 }
		imageErr := s.DB.WithContext(ctx).Table("product_images AS pi").
			Select("p.tenant_id").
			Joins("JOIN products AS p ON p.id = pi.product_id AND p.deleted_at IS NULL").
			Where("pi.id = ?", *p.SourceImageID).
			Take(&sourceProduct).Error
		if imageErr == nil {
			if err := merge(sourceProduct.TenantID); err != nil {
				return 0, err
			}
		} else if imageErr != gorm.ErrRecordNotFound {
			return 0, imageErr
		}
	}
	if p.CreatedBy != nil && *p.CreatedBy != uuid.Nil {
		var actor admin.AdminUser
		if err := s.DB.WithContext(ctx).Select("tenant_id").First(&actor, "id = ?", *p.CreatedBy).Error; err == nil {
			if err := merge(actor.TenantID); err != nil {
				return 0, err
			}
		} else if err != gorm.ErrRecordNotFound {
			return 0, err
		}
	}
	if have {
		return resolved, nil
	}
	return p.TenantID, nil
}

func (s *Service) requireTaskTenant(ctx context.Context, taskID uuid.UUID, tenantID int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var n int64
	if err := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id = ? AND tenant_id = ?", taskID, tenantID).
		Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) requireTaskItemTenant(ctx context.Context, itemID uuid.UUID, taskID *uuid.UUID, tenantID int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	tx := s.DB.WithContext(ctx).Model(&ImageTaskItem{}).
		Where("id = ? AND tenant_id = ?", itemID, tenantID)
	if taskID != nil {
		tx = tx.Where("task_id = ?", *taskID)
	}
	var n int64
	if err := tx.Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) requireProductTenant(ctx context.Context, productID uuid.UUID, tenantID int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var n int64
	if err := s.DB.WithContext(ctx).Model(&product.Product{}).
		Where("id = ? AND tenant_id = ?", productID, tenantID).
		Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) requireProductImageTenant(ctx context.Context, imageID uuid.UUID, tenantID int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var n int64
	if err := s.DB.WithContext(ctx).Table("product_images AS pi").
		Joins("JOIN products AS p ON p.id = pi.product_id AND p.deleted_at IS NULL").
		Where("pi.id = ? AND p.tenant_id = ?", imageID, tenantID).
		Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
