package inventory

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProcessQueuedTaskRejectsMissingTenantContextWithoutClaim(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&InventorySyncTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	task := InventorySyncTask{TenantID: 2, Status: StatusPending}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := (&Service{DB: db}).ProcessQueuedTask(context.Background(), task.ID, "test-worker"); err == nil {
		t.Fatal("missing tenant context was accepted")
	}
	var got InventorySyncTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if got.Status != StatusPending || got.LockedBy != nil {
		t.Fatalf("untrusted execution mutated task: %+v", got)
	}
}
