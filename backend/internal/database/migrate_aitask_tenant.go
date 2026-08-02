package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// migrateAITaskTenantScope backfills only tasks with one unambiguous trusted
// owner. Rows whose linked records disagree (or are absent) stay at tenant 0,
// which is isolated from every non-zero tenant by the task read service.
func migrateAITaskTenantScope(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate ai task tenant scope: nil db")
	}
	if !db.Migrator().HasTable("ai_tasks") {
		return nil
	}
	type taskRow struct{ ID uuid.UUID }
	var tasks []taskRow
	if err := db.Table("ai_tasks").Select("id").Where("tenant_id = 0").Find(&tasks).Error; err != nil {
		return fmt.Errorf("list legacy ai tasks: %w", err)
	}
	for _, task := range tasks {
		candidates, err := aiTaskMigrationTenants(db, task.ID)
		if err != nil {
			return err
		}
		if len(candidates) != 1 {
			continue
		}
		for tenantID := range candidates {
			if err := db.Table("ai_tasks").Where("id = ? AND tenant_id = 0", task.ID).Update("tenant_id", tenantID).Error; err != nil {
				return fmt.Errorf("backfill ai task %s: %w", task.ID, err)
			}
		}
	}
	return nil
}

func aiTaskMigrationTenants(db *gorm.DB, taskID uuid.UUID) (map[int64]struct{}, error) {
	var refs struct {
		ProductID      *uuid.UUID
		ConversationID *uuid.UUID
		CreatedBy      *uuid.UUID
		BatchID        *uuid.UUID
	}
	if err := db.Table("ai_tasks").Select("product_id", "conversation_id", "created_by", "batch_id").Where("id = ?", taskID).Take(&refs).Error; err != nil {
		return nil, fmt.Errorf("load ai task %s references: %w", taskID, err)
	}
	candidates := make(map[int64]struct{})
	for _, ref := range []struct {
		table string
		id    *uuid.UUID
	}{{"products", refs.ProductID}, {"customer_conversations", refs.ConversationID}, {"admin_users", refs.CreatedBy}, {"ai_operation_batches", refs.BatchID}} {
		if ref.id == nil || !db.Migrator().HasTable(ref.table) {
			continue
		}
		var tenantID int64
		err := db.WithContext(context.Background()).Table(ref.table).Select("tenant_id").Where("id = ?", *ref.id).Take(&tenantID).Error
		if err == nil {
			candidates[tenantID] = struct{}{}
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("resolve ai task tenant from %s: %w", ref.table, err)
		}
	}
	return candidates, nil
}
