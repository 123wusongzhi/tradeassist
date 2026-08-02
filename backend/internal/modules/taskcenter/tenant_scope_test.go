package taskcenter

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/gorm"
)

func TestClassifyOneDoesNotAttachAnotherTenantsAlert(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taskcenter_classify_tenant?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&TaskAlert{}); err != nil {
		t.Fatal(err)
	}
	sourceID := uuid.NewString()
	other := TaskAlert{ID: uuid.New(), TenantID: 2, TaskType: TaskTypeCollect, SourceID: sourceID, FailureCategory: failureclassifier.CategoryUnknown, Status: TaskAlertStatusOpen, FirstSeenAt: time.Now(), LastSeenAt: time.Now()}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	dto := UnifiedTaskDTO{TaskType: TaskTypeCollect, SourceID: sourceID, NormalizedStatus: NormFailed}
	if err := svc.ClassifyOne(context.Background(), 1, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.RelatedAlertID != "" || dto.AlertStatus != AlertStatusNone {
		t.Fatalf("tenant 1 linked to tenant 2 alert: %+v", dto)
	}
}
