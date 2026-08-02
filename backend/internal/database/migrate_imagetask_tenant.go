package database

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// migrateImageTaskTenantScope backfills tenant ownership for pre-tenant image
// tasks. New rows persist tenant_id directly; this keeps historical task/item
// API access fail-closed after the tenant-scoped handlers are enabled.
func migrateImageTaskTenantScope(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate image task tenant scope: nil db")
	}
	if !db.Migrator().HasTable("image_tasks") {
		return nil
	}
	var tasks []struct{ ID uuid.UUID }
	if err := db.Table("image_tasks").Select("id").Where("tenant_id = 0").Find(&tasks).Error; err != nil {
		return fmt.Errorf("list legacy image tasks: %w", err)
	}
	for _, task := range tasks {
		candidates, err := imageTaskMigrationTenants(db, task.ID)
		if err != nil {
			return err
		}
		if len(candidates) != 1 {
			continue
		}
		for tenantID := range candidates {
			if err := db.Table("image_tasks").Where("id = ? AND tenant_id = 0", task.ID).Update("tenant_id", tenantID).Error; err != nil {
				return fmt.Errorf("backfill image task %s: %w", task.ID, err)
			}
		}
	}
	if !db.Migrator().HasTable("ai_image_task_items") {
		return nil
	}
	var items []struct{ ID uuid.UUID }
	if err := db.Table("ai_image_task_items").Select("id").Where("tenant_id = 0").Find(&items).Error; err != nil {
		return fmt.Errorf("list legacy image task items: %w", err)
	}
	for _, item := range items {
		candidates, err := imageTaskItemMigrationTenants(db, item.ID)
		if err != nil {
			return err
		}
		if len(candidates) != 1 {
			continue
		}
		for tenantID := range candidates {
			if err := db.Table("ai_image_task_items").Where("id = ? AND tenant_id = 0", item.ID).Update("tenant_id", tenantID).Error; err != nil {
				return fmt.Errorf("backfill image task item %s: %w", item.ID, err)
			}
		}
	}
	return nil
}

func imageTaskMigrationTenants(db *gorm.DB, taskID uuid.UUID) (map[int64]struct{}, error) {
	var refs struct {
		ProductID     *uuid.UUID
		SourceImageID *uuid.UUID
		ResultFileID  *uuid.UUID
		CreatedBy     *uuid.UUID
		BatchID       *uuid.UUID
	}
	if err := db.Table("image_tasks").Select("product_id", "source_image_id", "result_file_id", "created_by", "batch_id").Where("id = ?", taskID).Take(&refs).Error; err != nil {
		return nil, fmt.Errorf("load image task %s references: %w", taskID, err)
	}
	candidates := make(map[int64]struct{})
	for _, ref := range []struct {
		table string
		id    *uuid.UUID
	}{{"products", refs.ProductID}, {"files", refs.SourceImageID}, {"files", refs.ResultFileID}, {"admin_users", refs.CreatedBy}, {"ai_operation_batches", refs.BatchID}} {
		if err := addDirectTenantCandidate(db, candidates, ref.table, ref.id); err != nil {
			return nil, fmt.Errorf("resolve image task %s tenant from %s: %w", taskID, ref.table, err)
		}
	}
	if err := addProductImageTenantCandidate(db, candidates, refs.SourceImageID); err != nil {
		return nil, fmt.Errorf("resolve image task %s tenant from product image: %w", taskID, err)
	}
	return candidates, nil
}

func imageTaskItemMigrationTenants(db *gorm.DB, itemID uuid.UUID) (map[int64]struct{}, error) {
	var refs struct {
		TaskID        uuid.UUID
		ProductID     *uuid.UUID
		SourceImageID *uuid.UUID
		OutputFileID  *uuid.UUID
	}
	if err := db.Table("ai_image_task_items").Select("task_id", "product_id", "source_image_id", "output_file_id").Where("id = ?", itemID).Take(&refs).Error; err != nil {
		return nil, fmt.Errorf("load image task item %s references: %w", itemID, err)
	}
	candidates := make(map[int64]struct{})
	for _, ref := range []struct {
		table string
		id    *uuid.UUID
	}{{"image_tasks", &refs.TaskID}, {"products", refs.ProductID}, {"files", refs.SourceImageID}, {"files", refs.OutputFileID}} {
		if err := addDirectTenantCandidate(db, candidates, ref.table, ref.id); err != nil {
			return nil, fmt.Errorf("resolve image task item %s tenant from %s: %w", itemID, ref.table, err)
		}
	}
	if err := addProductImageTenantCandidate(db, candidates, refs.SourceImageID); err != nil {
		return nil, fmt.Errorf("resolve image task item %s tenant from product image: %w", itemID, err)
	}
	return candidates, nil
}

func addDirectTenantCandidate(db *gorm.DB, candidates map[int64]struct{}, table string, id *uuid.UUID) error {
	if id == nil || *id == uuid.Nil || !db.Migrator().HasTable(table) {
		return nil
	}
	var tenantID int64
	err := db.Table(table).Select("tenant_id").Where("id = ?", *id).Take(&tenantID).Error
	if err == nil {
		candidates[tenantID] = struct{}{}
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return err
}

func addProductImageTenantCandidate(db *gorm.DB, candidates map[int64]struct{}, imageID *uuid.UUID) error {
	if imageID == nil || *imageID == uuid.Nil || !db.Migrator().HasTable("product_images") || !db.Migrator().HasTable("products") {
		return nil
	}
	var tenantID int64
	err := db.Table("product_images AS pi").Select("p.tenant_id").Joins("JOIN products p ON p.id = pi.product_id").Where("pi.id = ?", *imageID).Take(&tenantID).Error
	if err == nil {
		candidates[tenantID] = struct{}{}
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return err
}
