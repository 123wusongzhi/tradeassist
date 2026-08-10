package shop

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestOzonCategorySearchIndexCoversFullLeafCacheAndRanksSemanticExpansion(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	now := time.Now().UTC()
	roots := []PlatformCategory{
		{Platform: ozonPlatform, CategoryID: "electronics", Name: "电子产品", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "instruments", Name: "乐器", Level: 1, Status: "active", SyncedAt: &now},
	}
	if err := db.Create(&roots).Error; err != nil {
		t.Fatal(err)
	}
	leaves := make([]PlatformCategory, 0, 7000)
	leaves = append(leaves,
		PlatformCategory{Platform: ozonPlatform, CategoryID: "relay:1", ParentID: "electronics", Name: "智能继电器", Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now},
		PlatformCategory{Platform: ozonPlatform, CategoryID: "voice:1", ParentID: "instruments", Name: "男高音", Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now},
	)
	for index := 2; index < 7000; index++ {
		leaves = append(leaves, PlatformCategory{
			Platform: ozonPlatform, CategoryID: fmt.Sprintf("other:%04d", index), ParentID: "electronics",
			Name: fmt.Sprintf("无关器材%04d", index), Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now,
		})
	}
	if err := db.CreateInBatches(leaves, 500).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	result, err := svc.SearchOzonLeafCategories(context.Background(), OzonCategorySearchQuery{
		ProductType: "固态继电器模组", SearchTerms: []string{"固态继电器", "智能继电器", "继电器"},
		ProductTitle: "薄款固态继电器模组导轨式直流控交流", Limit: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IndexedLeafCount != 7000 {
		t.Fatalf("expected all 7000 active leaves indexed, got %d", result.IndexedLeafCount)
	}
	if result.IndexVersion != OzonCategorySearchIndexVersion {
		t.Fatalf("unexpected index version %q", result.IndexVersion)
	}
	if len(result.Matches) == 0 || result.Matches[0].Node.CategoryID != "relay:1" {
		t.Fatalf("expected relay leaf first, got %+v", result.Matches)
	}
	for _, match := range result.Matches {
		if match.Node.CategoryID == "voice:1" {
			t.Fatalf("single-character overlap must not recall an unrelated leaf: %+v", match)
		}
	}
}

func TestOzonCategorySearchIndexUsesCachedTemplateKeywordsAndInvalidates(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	now := time.Now().UTC()
	rows := []PlatformCategory{
		{Platform: ozonPlatform, CategoryID: "control", Name: "工业控制", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "generic:1", ParentID: "control", Name: "工业电子控制元件", Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PlatformCategoryAttribute{
		Platform: ozonPlatform, CategoryID: "generic:1", AttrID: "relay-kind", Name: "固态继电器类型", SyncedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	query := OzonCategorySearchQuery{ProductType: "固态继电器", SearchTerms: []string{"固态继电器"}, Limit: 10}
	first, err := svc.SearchOzonLeafCategories(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Matches) == 0 || first.Matches[0].Node.CategoryID != "generic:1" {
		t.Fatalf("expected cached template keyword recall, got %+v", first.Matches)
	}
	newLeaf := PlatformCategory{
		Platform: ozonPlatform, CategoryID: "relay:exact", ParentID: "control", Name: "固态继电器",
		Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now,
	}
	if err := db.Create(&newLeaf).Error; err != nil {
		t.Fatal(err)
	}
	cached, err := svc.SearchOzonLeafCategories(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if cached.IndexedLeafCount != 1 {
		t.Fatalf("expected index to remain stable before invalidation, got %d", cached.IndexedLeafCount)
	}
	svc.invalidateOzonCategorySearchIndex()
	rebuilt, err := svc.SearchOzonLeafCategories(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.IndexedLeafCount != 2 || len(rebuilt.Matches) == 0 || rebuilt.Matches[0].Node.CategoryID != "relay:exact" {
		t.Fatalf("expected invalidated index to include exact leaf, got %+v", rebuilt)
	}
}

func TestOzonCategorySearchFiltersRootBeforeBoundedWeakFallback(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	now := time.Now().UTC()
	rows := []PlatformCategory{
		{Platform: ozonPlatform, CategoryID: "pet", Name: "宠物用品", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "home", Name: "家居用品", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "pet:mat", ParentID: "pet", Name: "宠物垫", Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "pet:bed", ParentID: "pet", Name: "宠物床垫", Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "pet:stake", ParentID: "pet", Name: "宠物站桩器", Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now},
	}
	for index := 0; index < 80; index++ {
		rows = append(rows, PlatformCategory{
			Platform: ozonPlatform, CategoryID: fmt.Sprintf("home:mat:%02d", index), ParentID: "home",
			Name: fmt.Sprintf("家居防滑垫%02d", index), Level: 2, IsLeaf: true, Status: "active", SyncedAt: &now,
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	result, err := svc.SearchOzonLeafCategories(context.Background(), OzonCategorySearchQuery{
		ProductType: "狗垫", ProductTitle: "老年犬站立辅助起身保暖狗垫",
		AllowedRootIDs: []string{"pet"}, AllowedRootNames: []string{"宠物用品"}, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) == 0 || result.Matches[0].Node.CategoryID != "pet:mat" {
		t.Fatalf("expected root-scoped weak fallback to recall pet mat first, got %+v", result.Matches)
	}
	for _, match := range result.Matches {
		if match.Node.CategoryID == "" || !strings.HasPrefix(match.Node.CategoryID, "pet:") {
			t.Fatalf("root filtering must happen before the bounded result set: %+v", match)
		}
	}
}
