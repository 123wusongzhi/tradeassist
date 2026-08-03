package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestStartOzonCategorySyncReusesActiveRun(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	if err := db.AutoMigrate(&OzonCategorySyncRun{}, &OzonCategoryChange{}); err != nil {
		t.Fatal(err)
	}
	encrypter, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, encrypter)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	shopID := seedOzonAuthorizedShop(t, db, encrypter, api.URL)
	active := OzonCategorySyncRun{TenantID: 0, ShopID: shopID, Status: OzonCategorySyncPending}
	if err := db.Create(&active).Error; err != nil {
		t.Fatal(err)
	}

	started, err := svc.StartOzonCategorySync(context.Background(), 0, shopID)
	if err != nil {
		t.Fatal(err)
	}
	if started.RunID != active.ID || started.Run == nil || started.Run.Status != OzonCategorySyncPending {
		t.Fatalf("expected existing pending run, got %+v", started)
	}
	var count int64
	if err := db.Model(&OzonCategorySyncRun{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("active run must be de-duplicated; count=%d err=%v", count, err)
	}
}

func TestStartOzonCategorySyncProcessesLegacyTenantZeroInline(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	if err := db.AutoMigrate(&OzonCategorySyncRun{}, &OzonCategoryChange{}); err != nil {
		t.Fatal(err)
	}
	encrypter, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, encrypter)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	shopID := seedOzonAuthorizedShop(t, db, encrypter, api.URL)

	started, err := svc.StartOzonCategorySync(context.Background(), 0, shopID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Run == nil || started.RunID == uuid.Nil {
		t.Fatalf("expected persisted sync run, got %+v", started)
	}
	if started.Run.Status != OzonCategorySyncSucceeded {
		t.Fatalf("legacy tenant sync status = %q, want %q", started.Run.Status, OzonCategorySyncSucceeded)
	}
}

func TestProcessOzonCategorySyncRunRequiresMatchingTenant(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	if err := db.AutoMigrate(&OzonCategorySyncRun{}, &OzonCategoryChange{}); err != nil {
		t.Fatal(err)
	}
	run := OzonCategorySyncRun{TenantID: 7, ShopID: seedOzonUnauthorizedShop(t, db), Status: OzonCategorySyncPending}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 8})
	if err := svc.ProcessOzonCategorySyncRun(ctx, run.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected tenant-scoped not found, got %v", err)
	}
}
