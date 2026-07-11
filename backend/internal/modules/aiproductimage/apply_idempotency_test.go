package aiproductimage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func openImageApplyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ai_image_apply_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&product.Product{},
		&product.ProductImage{},
		&product.ProductImageApplication{},
		&imagetask.ImageTask{},
		&AIProductImageBatch{},
		&AIProductImageItem{},
		&idempotency.Record{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c
}

func TestConcurrentReplaceApplySameSlotOnce(t *testing.T) {
	db := openImageApplyTestDB(t)
	svc := &Service{
		DB:          db,
		Idempotency: &idempotency.Service{DB: db},
	}

	p := product.Product{Source: "manual", Title: "Camera", Currency: "USD", Status: product.StatusDraft}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	img := product.ProductImage{
		ProductID: p.ID,
		ImageType: product.ImageTypeMain,
		PublicURL: "https://example.com/old.jpg",
		Source:    product.ImageSourceCollect,
	}
	if err := db.Create(&img).Error; err != nil {
		t.Fatal(err)
	}
	task := imagetask.ImageTask{
		TaskType:  imagetask.TaskTypeRemoveBackground,
		Provider:  "noop",
		Status:    imagetask.StatusSuccess,
		ProductID: &p.ID,
		ResultURL: "https://example.com/new.jpg",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	imgUpdated := img.UpdatedAt.UTC()
	batch := AIProductImageBatch{
		BatchNo: "AITEST0001", BatchType: BatchTypeAIImage, Status: BatchSuccess,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	item := AIProductImageItem{
		BatchID:            batch.ID,
		ProductID:          p.ID,
		ImageID:            &img.ID,
		ImageType:          product.ImageTypeMain,
		OperationType:      OpRemoveWatermark,
		Status:             ItemPendingReview,
		ImageTaskID:        &task.ID,
		SourceImageURL:     img.PublicURL,
		SourceSnapshotHash: imageURLHash(img.PublicURL),
		ResultImageURL:     "https://example.com/new.jpg",
		ResultStorageKey:   "products/ai/new.jpg",
		ImageUpdatedAt:     &imgUpdated,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	const n = 20
	var applied int32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			c := testGinContext()
			c.Set("requestId", fmt.Sprintf("img-req-%d", idx))
			var loaded AIProductImageItem
			if err := db.First(&loaded, "id = ?", item.ID).Error; err != nil {
				t.Errorf("load: %v", err)
				return
			}
			r := svc.applyOneItem(c, &loaded, ApplyReplaceImage, nil)
			if r.Status == ItemApplied {
				atomic.AddInt32(&applied, 1)
			}
		}(i)
	}
	wg.Wait()

	if applied < 1 {
		t.Fatalf("expected at least one applied, got %d", applied)
	}
	var appCount int64
	if err := db.Model(&product.ProductImageApplication{}).
		Where("product_id = ? AND status = ?", p.ID, product.ImageApplyStatusApplied).
		Count(&appCount).Error; err != nil {
		t.Fatal(err)
	}
	if appCount != 1 {
		t.Fatalf("expected 1 application, got %d (applied responses=%d)", appCount, applied)
	}
	var refreshed product.ProductImage
	if err := db.First(&refreshed, "id = ?", img.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.PublicURL != "https://example.com/new.jpg" {
		t.Fatalf("unexpected url: %s", refreshed.PublicURL)
	}
}

func TestApplySlotFormats(t *testing.T) {
	imgID := uuid.New()
	item := &AIProductImageItem{ImageID: &imgID, OperationType: OpWhiteBackground}
	if got := applySlot(item, ApplySetMain); got != "main" {
		t.Fatalf("set_main: %s", got)
	}
	if got := applySlot(item, ApplySaveToGallery); got != "gallery:0" {
		t.Fatalf("gallery: %s", got)
	}
	if got := applySlot(item, ApplyReplaceImage); got != "replace:"+imgID.String() {
		t.Fatalf("replace: %s", got)
	}
	if got := applySlot(item, ""); got != "white_background" {
		t.Fatalf("white_background default: %s", got)
	}
}

func TestImageTargetVersionConflict(t *testing.T) {
	db := openImageApplyTestDB(t)
	svc := &Service{DB: db, Idempotency: &idempotency.Service{DB: db}}
	p := product.Product{Source: "manual", Title: "X", Currency: "USD", Status: product.StatusDraft}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	img := product.ProductImage{ProductID: p.ID, ImageType: product.ImageTypeMain, PublicURL: "https://example.com/a.jpg"}
	if err := db.Create(&img).Error; err != nil {
		t.Fatal(err)
	}
	stale := img.UpdatedAt.Add(-2 * time.Second)
	task := imagetask.ImageTask{TaskType: imagetask.TaskTypeRemoveBackground, Provider: "noop", Status: imagetask.StatusSuccess, ProductID: &p.ID, ResultURL: "https://example.com/b.jpg"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	batch := AIProductImageBatch{BatchNo: "AITEST0002", BatchType: BatchTypeAIImage, Status: BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	item := AIProductImageItem{
		BatchID: batch.ID, ProductID: p.ID, ImageID: &img.ID, OperationType: OpRemoveLogo,
		Status: ItemPendingReview, ImageTaskID: &task.ID,
		SourceSnapshotHash: imageURLHash(img.PublicURL),
		ResultImageURL:     "https://example.com/b.jpg",
		ImageUpdatedAt:     &stale,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&img).Update("public_url", "https://example.com/changed.jpg").Error; err != nil {
		t.Fatal(err)
	}
	r := svc.applyOneItem(testGinContext(), &item, ApplyReplaceImage, nil)
	if r.ErrorCode != ErrCodeTargetVersionConflict {
		t.Fatalf("expected version conflict, got status=%s code=%s msg=%s", r.Status, r.ErrorCode, r.ErrorMessage)
	}
}
