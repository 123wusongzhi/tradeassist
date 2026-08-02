package aiopsworkbench

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newWorkbenchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&aiproducttext.AIProductTextBatch{},
		&aiproducttext.AIProductTextItem{},
		&aiproductimage.AIProductImageBatch{},
		&aiproductimage.AIProductImageItem{},
		&productpublish.ProductPublishBatch{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func workbenchTestContext(tenantID int64) context.Context {
	return security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: tenantID})
}

func TestTypeAndPriorityLabelsChinese(t *testing.T) {
	if TypeLabel(TodoTypeAITextReview) != "AI 文案待复核" {
		t.Fatal("ai text review label")
	}
	if PriorityLabel(PriorityP1) != "阻断" {
		t.Fatal("p1 label")
	}
}

func TestSortTodosByPriorityThenUpdatedAt(t *testing.T) {
	now := time.Now().UTC()
	items := []TodoItem{
		{ID: "a", Priority: PriorityP3, UpdatedAt: now},
		{ID: "b", Priority: PriorityP1, UpdatedAt: now.Add(-time.Hour)},
		{ID: "c", Priority: PriorityP2, UpdatedAt: now.Add(time.Hour)},
	}
	sortTodos(items)
	if items[0].Priority != PriorityP1 || items[1].Priority != PriorityP2 || items[2].Priority != PriorityP3 {
		t.Fatalf("unexpected order: %#v", items)
	}
}

func TestFilterTodosByTypeAndPriority(t *testing.T) {
	all := []TodoItem{
		{Type: TodoTypeAITextReview, Priority: PriorityP3, UpdatedAt: time.Now()},
		{Type: TodoTypePublishCheckFailed, Priority: PriorityP1, UpdatedAt: time.Now()},
	}
	out := filterTodos(all, Query{Type: TodoTypePublishCheckFailed})
	if len(out) != 1 || out[0].Type != TodoTypePublishCheckFailed {
		t.Fatalf("type filter failed: %#v", out)
	}
	out = filterTodos(all, Query{Priority: PriorityP1})
	if len(out) != 1 {
		t.Fatalf("priority filter failed: %#v", out)
	}
}

func TestDedupCollector(t *testing.T) {
	col := newCollector()
	col.add(TodoItem{ID: "todo:ai_text:i1:pending_review", Type: TodoTypeAITextReview})
	col.add(TodoItem{ID: "todo:ai_text:i1:pending_review", Type: TodoTypeAITextReview})
	if len(col.list()) != 1 {
		t.Fatalf("expected dedup, got %d", len(col.list()))
	}
}

func TestCollectAITextReviewAndConflict(t *testing.T) {
	db := newWorkbenchTestDB(t)
	p := product.Product{Title: "蓝牙耳机", Status: product.StatusDraft, Currency: "CNY"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	batch := aiproducttext.AIProductTextBatch{BatchNo: "B1", Status: aiproducttext.BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	warnJSON, _ := json.Marshal([]map[string]string{{"code": "title_too_long", "message": "标题偏长"}})
	review := aiproducttext.AIProductTextItem{
		BatchID:         batch.ID,
		ProductID:       p.ID,
		OperationType:   aiproducttext.OpTitle,
		Status:          aiproducttext.ItemPendingReview,
		QualityWarnings: datatypes.JSON(warnJSON),
	}
	conflict := aiproducttext.AIProductTextItem{
		BatchID:       batch.ID,
		ProductID:     p.ID,
		OperationType: aiproducttext.OpDescription,
		Status:        aiproducttext.ItemConflict,
	}
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&conflict).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db, ProductCheck: &productcheck.Service{DB: db}}
	items, err := svc.collectAllTodos(workbenchTestContext(0), Query{})
	if err != nil {
		t.Fatal(err)
	}
	var reviewN, conflictN int
	for _, it := range items {
		switch it.Type {
		case TodoTypeAITextReview:
			reviewN++
			if it.Priority != PriorityP2 {
				t.Fatalf("quality warning should be P2, got %s", it.Priority)
			}
		case TodoTypeAITextConflict:
			conflictN++
			if it.Priority != PriorityP1 {
				t.Fatalf("conflict should be P1")
			}
		}
	}
	if reviewN != 1 || conflictN != 1 {
		t.Fatalf("expected 1 review + 1 conflict, got review=%d conflict=%d all=%d", reviewN, conflictN, len(items))
	}
}

func TestCollectAIImageReview(t *testing.T) {
	db := newWorkbenchTestDB(t)
	p := product.Product{Title: "测试商品", Status: product.StatusDraft, Currency: "CNY"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	batch := aiproductimage.AIProductImageBatch{BatchNo: "IB1", Status: aiproductimage.BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	item := aiproductimage.AIProductImageItem{
		BatchID:       batch.ID,
		ProductID:     p.ID,
		OperationType: aiproductimage.OpQualityCheck,
		Status:        aiproductimage.ItemPendingReview,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	all, err := svc.collectAllTodos(workbenchTestContext(0), Query{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range all {
		if it.Type == TodoTypeAIImageReview {
			found = true
			if it.Priority != PriorityP3 {
				t.Fatalf("expected P3, got %s", it.Priority)
			}
		}
	}
	if !found {
		t.Fatal("expected ai image review todo")
	}
}

func TestCollectPublishBatchFailedAndPartial(t *testing.T) {
	db := newWorkbenchTestDB(t)
	pid := uuid.New()
	failed := productpublish.ProductPublishBatch{
		ProductID: &pid,
		Status:    productpublish.BatchFailed,
		BatchType: productpublish.BatchTypeSingleProduct,
	}
	partial := productpublish.ProductPublishBatch{
		Status:       productpublish.BatchPartialSuccess,
		BatchType:    productpublish.BatchTypeMultiProduct,
		ProductCount: 3,
	}
	if err := db.Create(&failed).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&partial).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	all, err := svc.collectAllTodos(workbenchTestContext(0), Query{})
	if err != nil {
		t.Fatal(err)
	}
	var failedN, partialN int
	for _, it := range all {
		switch it.Type {
		case TodoTypePublishBatchFailed:
			failedN++
		case TodoTypePublishBatchPartial:
			partialN++
		}
	}
	if failedN != 1 || partialN != 1 {
		t.Fatalf("batch todos failed=%d partial=%d total=%d", failedN, partialN, len(all))
	}
}

func TestCollectPublishCheckFailedAndWarning(t *testing.T) {
	db := newWorkbenchTestDB(t)
	p := product.Product{Status: product.StatusDraft, Currency: "CNY"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		DB:           db,
		ProductCheck: &productcheck.Service{DB: db},
	}
	all, err := svc.collectAllTodos(workbenchTestContext(0), Query{})
	if err != nil {
		t.Fatal(err)
	}
	var failedN, warnN int
	for _, it := range all {
		switch it.Type {
		case TodoTypePublishCheckFailed:
			failedN++
		case TodoTypePublishCheckWarning:
			warnN++
		}
	}
	if failedN == 0 {
		t.Fatal("expected publish check failed todo for missing title")
	}
	if warnN == 0 {
		t.Fatal("expected publish check warning todo")
	}
}

func TestListTodosPagination(t *testing.T) {
	db := newWorkbenchTestDB(t)
	batch := aiproducttext.AIProductTextBatch{BatchNo: "PX", Status: aiproducttext.BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		p := product.Product{Title: fmt.Sprintf("商品%d", i), Status: product.StatusDraft, Currency: "CNY"}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		item := aiproducttext.AIProductTextItem{
			BatchID:       batch.ID,
			ProductID:     p.ID,
			OperationType: aiproducttext.OpTitle,
			Status:        aiproducttext.ItemPendingReview,
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &Service{DB: db}
	res, err := svc.ListTodos(workbenchTestContext(0), Query{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || res.Pagination.Total < 5 {
		t.Fatalf("pagination mismatch: len=%d total=%d", len(res.Items), res.Pagination.Total)
	}
}

func TestSummaryCounts(t *testing.T) {
	db := newWorkbenchTestDB(t)
	batch := aiproducttext.AIProductTextBatch{BatchNo: "SY", Status: aiproducttext.BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	p := product.Product{Title: "汇总测试", Status: product.StatusDraft, Currency: "CNY"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&aiproducttext.AIProductTextItem{
		BatchID: batch.ID, ProductID: p.ID, OperationType: aiproducttext.OpTitle, Status: aiproducttext.ItemPendingReview,
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db, ProductCheck: &productcheck.Service{DB: db}}
	sum, err := svc.GetSummary(workbenchTestContext(0), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.AITextReviewCount < 1 {
		t.Fatalf("expected ai text review count, got %#v", sum)
	}
	if sum.PublishCheckIssueCount < 1 {
		t.Fatalf("expected publish check issues, got %#v", sum)
	}
}

func TestAppliedItemNotListed(t *testing.T) {
	db := newWorkbenchTestDB(t)
	batch := aiproducttext.AIProductTextBatch{BatchNo: "AP", Status: aiproducttext.BatchSuccess}
	p := product.Product{Title: "已应用", Status: product.StatusDraft, Currency: "CNY"}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&aiproducttext.AIProductTextItem{
		BatchID: batch.ID, ProductID: p.ID, OperationType: aiproducttext.OpTitle,
		Status: aiproducttext.ItemApplied, AppliedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	all, err := svc.collectAllTodos(workbenchTestContext(0), Query{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range all {
		if it.SourceType == SourceAIText {
			t.Fatal("applied item should not appear")
		}
	}
}

func TestKeywordSearch(t *testing.T) {
	all := []TodoItem{
		{ProductTitle: "蓝牙耳机", Message: "待复核", UpdatedAt: time.Now()},
		{ProductTitle: "咖啡机", Message: "待复核", UpdatedAt: time.Now()},
	}
	out := filterTodos(all, Query{Keyword: "蓝牙"})
	if len(out) != 1 {
		t.Fatalf("keyword filter expected 1, got %d", len(out))
	}
}

func TestActionURLs(t *testing.T) {
	bid, iid, pid := uuid.New().String(), uuid.New().String(), uuid.New().String()
	if !strings.Contains(aiTextDetailURL(bid, iid), "itemId=") {
		t.Fatal("ai text url should include itemId")
	}
	if !strings.Contains(productDraftURL(pid, "publish-check"), "tab=readiness") {
		t.Fatal("product draft url")
	}
	if publishBatchURL(bid) != "/product/publish-batches/"+bid {
		t.Fatal("batch url")
	}
}

func TestTenantScopeSeparatesTodosSummaryDetailRefreshAndResolvedCount(t *testing.T) {
	db := newWorkbenchTestDB(t)
	ctxA := workbenchTestContext(101)
	ctxB := workbenchTestContext(202)

	productA := product.Product{TenantID: 101, Title: "tenant-a product", Status: product.StatusDraft, Currency: "CNY"}
	productB := product.Product{TenantID: 202, Title: "tenant-b product", Status: product.StatusDraft, Currency: "CNY"}
	batchA := aiproducttext.AIProductTextBatch{TenantID: 101, BatchNo: "tenant-a-batch", Status: aiproducttext.BatchSuccess}
	batchB := aiproducttext.AIProductTextBatch{TenantID: 202, BatchNo: "tenant-b-batch", Status: aiproducttext.BatchSuccess}
	for _, row := range []any{&productA, &productB, &batchA, &batchB} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	itemA := aiproducttext.AIProductTextItem{BatchID: batchA.ID, ProductID: productA.ID, OperationType: aiproducttext.OpTitle, Status: aiproducttext.ItemPendingReview}
	itemB := aiproducttext.AIProductTextItem{BatchID: batchB.ID, ProductID: productB.ID, OperationType: aiproducttext.OpTitle, Status: aiproducttext.ItemPendingReview}
	if err := db.Create(&itemA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&itemB).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db}
	listA, err := svc.ListTodos(ctxA, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listA.Items) != 1 || listA.Items[0].ProductTitle != productA.Title {
		t.Fatalf("tenant A leaked or missed rows: %#v", listA.Items)
	}
	listB, err := svc.ListTodos(ctxB, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listB.Items) != 1 || listB.Items[0].ProductTitle != productB.Title {
		t.Fatalf("tenant B leaked or missed rows: %#v", listB.Items)
	}
	if _, err := svc.GetTodo(ctxA, listB.Items[0].ID, Query{}); err != gorm.ErrRecordNotFound {
		t.Fatalf("cross-tenant todo detail error = %v, want not found", err)
	}
	summaryA, err := svc.GetSummary(ctxA, Query{})
	if err != nil || summaryA.AITextReviewCount != 1 {
		t.Fatalf("tenant A summary = %#v, err=%v", summaryA, err)
	}
	refreshA, err := svc.RefreshTodos(ctxA, Query{})
	if err != nil || refreshA.Summary.AITextReviewCount != 1 {
		t.Fatalf("tenant A refresh = %#v, err=%v", refreshA, err)
	}

	now := time.Now().UTC()
	if err := db.Model(&aiproducttext.AIProductTextItem{}).Where("id = ?", itemA.ID).Updates(map[string]any{"status": aiproducttext.ItemApplied, "applied_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&aiproducttext.AIProductTextItem{}).Where("id = ?", itemB.ID).Updates(map[string]any{"status": aiproducttext.ItemApplied, "applied_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	resolvedA, err := svc.GetSummary(ctxA, Query{})
	if err != nil || resolvedA.TodayResolvedCount != 1 {
		t.Fatalf("tenant A resolved count = %#v, err=%v", resolvedA, err)
	}
}

func TestWorkbenchFailsClosedWithoutTenantContext(t *testing.T) {
	svc := &Service{DB: newWorkbenchTestDB(t)}
	if _, err := svc.ListTodos(context.Background(), Query{}); err == nil {
		t.Fatal("missing tenant context must fail closed")
	}
}
