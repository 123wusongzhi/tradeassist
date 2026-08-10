package shop

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPutOzonCategoryMappingPersistsOperatorEvidence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&PlatformCategory{}, &PlatformCategoryAttribute{}, &OzonCategoryMapping{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX ux_test_ozon_mapping_scope_source ON ozon_category_mappings (tenant_id, scope_key, source_category_key)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rows := []PlatformCategory{
		{Platform: ozonPlatform, CategoryID: "home", Name: "住宅和花园", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "storage", ParentID: "home", Name: "收纳", Level: 2, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "17027937:95482", ParentID: "storage", Name: "储物箱", Level: 3, IsLeaf: true, Status: "active", SyncedAt: &now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PlatformCategoryAttribute{
		Platform: ozonPlatform, CategoryID: "17027937:95482", AttrID: "85", Name: "品牌", SyncedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	adminID := uuid.New()
	svc := &Service{DB: db}
	got, err := svc.PutOzonCategoryMapping(context.Background(), 7, PutOzonCategoryMappingBody{
		SourceCategoryKey:  "storage-box",
		SourceCategoryName: "储物箱",
		CategoryID:         "17027937:95482",
		Status:             OzonMappingActive,
		SelectionMethod:    "recommended_then_manual",
		ConfirmationReason: "商品用途、材质和规格与储物箱叶子类目一致",
	}, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CategoryPath != "住宅和花园 / 收纳 / 储物箱" || got.DescriptionCategoryID != "17027937" || got.TypeID != "95482" {
		t.Fatalf("category evidence = %+v", got)
	}
	if got.SelectionMethod != "recommended_then_manual" || got.ConfirmationReason == "" || got.Scope != "tenant" {
		t.Fatalf("operator evidence = %+v", got)
	}
	if got.TemplateSyncedAt == nil || got.ConfirmedAt == nil || got.ConfirmedBy == nil || *got.ConfirmedBy != adminID {
		t.Fatalf("confirmation timestamps = %+v", got)
	}
}
