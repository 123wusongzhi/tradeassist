package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestTenantScopedPublicationAndTaskLookup(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	p1, s1 := seedBatchProduct(t, db)
	p2, s2 := seedBatchProduct(t, db)
	if err := db.Model(&product.Product{}).Where("id = ?", p1).Update("tenant_id", 11).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&product.Product{}).Where("id = ?", p2).Update("tenant_id", 22).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ProductPublication{}).Create(&ProductPublication{TenantID: 22, ProductID: p2, ShopID: s2, Platform: "shopee", Status: StatusDraft, PublishStatus: StatusDraft}).Error; err != nil {
		t.Fatal(err)
	}
	if rows, err := svc.ListPublicationsByProduct(context.Background(), 11, p2); err != nil || len(rows) != 0 {
		t.Fatalf("foreign publication leaked: rows=%d err=%v", len(rows), err)
	}
	if rows, err := svc.ListPublicationsByProduct(context.Background(), 22, p2); err != nil || len(rows) != 1 {
		t.Fatalf("owned publication missing: rows=%d err=%v", len(rows), err)
	}
	key := uuid.New().String()
	if err := db.Create(&ProductPublishTask{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, TenantID: 22, ProductID: p2, ShopID: s2, Platform: "shopee", Status: TaskSuccess, TaskType: TaskTypeLocalDraftCreate, Mode: PublishModeSaveAsPlatformDraft, PublishMode: PublishModeSaveAsPlatformDraft, Input: []byte(`{"idempotencyKey":"` + key + `"}`)}).Error; err != nil {
		t.Fatal(err)
	}
	if got, ok := svc.findExistingSuccessfulTask(context.Background(), 11, p2, "shopee", &s2, EffectivePublishConfig{}); ok || got != nil {
		t.Fatal("foreign task dedup leaked")
	}
	_ = p1
	_ = s1
}

func scopedPublishContext(tenantID int64, principal *adminperm.Principal) *gin.Context {
	c := testGinContext()
	c.Set(ctxkey.TenantID, tenantID)
	c.Set("adminperm.principal", principal)
	return c
}

func seedScopedShop(t *testing.T, db *gorm.DB, tenantID int64, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Create(&shop.Shop{
		Base:       model.Base{ID: id},
		TenantID:   tenantID,
		Platform:   "shopee",
		ShopName:   name,
		Status:     shop.StatusActive,
		AuthStatus: shop.AuthAuthorized,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func seedScopedProduct(t *testing.T, db *gorm.DB, tenantID int64, title string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Create(&product.Product{
		Base:     model.Base{ID: id},
		TenantID: tenantID,
		Source:   "test",
		Title:    title,
		Status:   product.StatusDraft,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGetPublishBatchDetailScopedHidesForeignTenantBatch(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	batchID := uuid.New()
	if err := db.Create(&ProductPublishBatch{
		HardDeleteBase: model.HardDeleteBase{ID: batchID},
		TenantID:       22,
		BatchType:      BatchTypeMultiProduct,
		Status:         BatchSuccess,
	}).Error; err != nil {
		t.Fatal(err)
	}
	c := scopedPublishContext(11, &adminperm.Principal{TenantID: 11, Role: adminperm.RoleTenantAdmin})
	if _, err := svc.GetPublishBatchDetailScoped(c, batchID, nil); err == nil || err != gorm.ErrRecordNotFound {
		t.Fatalf("foreign batch must be indistinguishable from not found, got %v", err)
	}
}

func TestGetPublishBatchDetailScopedFailsClosedForMixedStoreChildrenAndInput(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	const tenantID int64 = 11
	productID := seedScopedProduct(t, db, tenantID, "owned product")
	allowedShopID := seedScopedShop(t, db, tenantID, "allowed")
	foreignShopID := seedScopedShop(t, db, tenantID, "restricted")
	batchID := uuid.New()
	allowedShopRaw := allowedShopID.String()
	foreignShopRaw := foreignShopID.String()
	input, err := json.Marshal(BatchTargetsCreateDraftsRequest{
		ProductIDs: []string{productID.String()},
		Targets: []PublishTargetRef{
			{Platform: "shopee", ShopID: &allowedShopRaw},
			{Platform: "shopee", ShopID: &foreignShopRaw},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductPublishBatch{
		HardDeleteBase: model.HardDeleteBase{ID: batchID}, TenantID: tenantID,
		BatchType: BatchTypeMultiProduct, Status: BatchSuccess, Input: datatypes.JSON(input),
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, shopID := range []uuid.UUID{allowedShopID, foreignShopID} {
		if err := db.Create(&ProductPublishTask{
			HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, TenantID: tenantID, ProductID: productID,
			ShopID: shopID, TargetStoreID: shopID, BatchID: &batchID, Platform: "shopee",
			TaskType: TaskTypeLocalDraftCreate, Status: TaskSuccess, Mode: PublishModeSaveAsPlatformDraft,
			PublishMode: PublishModeSaveAsPlatformDraft,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	c := scopedPublishContext(tenantID, &adminperm.Principal{TenantID: tenantID, Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: allowedShopID, PermissionScope: "operate"}}})
	out, err := svc.GetPublishBatchDetailScoped(c, batchID, nil)
	if !errors.Is(err, gorm.ErrRecordNotFound) || out != nil {
		t.Fatalf("mixed-store batch must fail closed without partial detail, out=%+v err=%v", out, err)
	}
}

func TestTaskToDTODoesNotResolveForeignTenantChildren(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	productID := seedScopedProduct(t, db, 22, "foreign product")
	shopID := seedScopedShop(t, db, 22, "foreign shop")
	task := &ProductPublishTask{TenantID: 11, ProductID: productID, ShopID: shopID, Platform: "shopee"}
	dto := svc.taskToDTO(context.Background(), task)
	if dto.ProductTitle != "" || dto.ShopName != "" {
		t.Fatalf("tenant-mismatched task leaked child data: %+v", dto)
	}
}

func TestOperatorCannotPublishSharedProductWithoutAllOperateGrants(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	const tenantID int64 = 11
	productID := seedScopedProduct(t, db, tenantID, "shared")
	allowedShopID := seedScopedShop(t, db, tenantID, "allowed")
	sharedShopID := seedScopedShop(t, db, tenantID, "shared-view-only")
	for _, shopID := range []uuid.UUID{allowedShopID, sharedShopID} {
		if err := db.Create(&ProductPublication{TenantID: tenantID, ProductID: productID, ShopID: shopID, Platform: "shopee", Status: StatusDraft, PublishStatus: StatusDraft}).Error; err != nil {
			t.Fatal(err)
		}
	}
	c := scopedPublishContext(tenantID, &adminperm.Principal{TenantID: tenantID, Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: allowedShopID, PermissionScope: "operate"}, {StoreID: sharedShopID, PermissionScope: "view"}}})
	allowedRaw := allowedShopID.String()
	if _, err := svc.CreateBatchTargetDrafts(c, BatchTargetsCreateDraftsRequest{
		ProductIDs: []string{productID.String()}, Targets: []PublishTargetRef{{Platform: "shopee", ShopID: &allowedRaw}},
	}, nil); err == nil {
		t.Fatal("shared product must not allow batch publish with only partial operate grants")
	}
	if _, err := svc.CreateDraftsForTargets(c, productID, PublishTargetsCreateDraftsRequest{
		Targets: []PublishTargetRef{{Platform: "shopee", ShopID: &allowedRaw}},
	}, nil); err == nil {
		t.Fatal("shared product must not allow single publish with only partial operate grants")
	}
	var batches, tasks, publications int64
	db.Model(&ProductPublishBatch{}).Count(&batches)
	db.Model(&ProductPublishTask{}).Count(&tasks)
	db.Model(&ProductPublication{}).Count(&publications)
	if batches != 0 || tasks != 0 || publications != 2 {
		t.Fatalf("permission rejection must have no publish side effects: batches=%d tasks=%d publications=%d", batches, tasks, publications)
	}
}

func TestTenantZeroScopeIsExact(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	foreignProductID := seedScopedProduct(t, db, 1, "tenant one")
	if _, err := svc.productForTenant(context.Background(), 0, foreignProductID); err != gorm.ErrRecordNotFound {
		t.Fatalf("tenant zero must not resolve a nonzero tenant product, got %v", err)
	}
	legacyProductID := seedScopedProduct(t, db, 0, "legacy")
	if got, err := svc.productForTenant(context.Background(), 0, legacyProductID); err != nil || got.ID != legacyProductID {
		t.Fatalf("tenant zero must resolve only tenant zero product: got=%+v err=%v", got, err)
	}
}

func TestFailedTargetsFromBatchUsesExactTenantAndProductScope(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	productA := seedScopedProduct(t, db, 11, "tenant-a")
	productB := seedScopedProduct(t, db, 22, "tenant-b")
	shopB := seedScopedShop(t, db, 22, "tenant-b-shop")
	foreignTarget := shopB.String()
	foreignInput, err := json.Marshal(PublishTargetsCreateDraftsRequest{Targets: []PublishTargetRef{{Platform: "shopee", ShopID: &foreignTarget}}})
	if err != nil {
		t.Fatal(err)
	}
	foreignBatchID := uuid.New()
	requireCreate := func(row *ProductPublishBatch) {
		t.Helper()
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	requireCreate(&ProductPublishBatch{HardDeleteBase: model.HardDeleteBase{ID: foreignBatchID}, TenantID: 22, BatchType: BatchTypeSingleProduct, ProductID: &productB, Status: BatchFailed, Input: datatypes.JSON(foreignInput)})
	if _, err := svc.failedTargetsFromBatch(context.Background(), 11, productA, foreignBatchID.String()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant A must not read tenant B fallback input, got %v", err)
	}
	if _, err := svc.failedTargetsFromBatch(context.Background(), 0, productA, foreignBatchID.String()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant zero must not bypass explicit tenant scope, got %v", err)
	}

	otherProduct := seedScopedProduct(t, db, 11, "tenant-a-other")
	otherShop := seedScopedShop(t, db, 11, "tenant-a-other-shop")
	otherBatchID := uuid.New()
	requireCreate(&ProductPublishBatch{HardDeleteBase: model.HardDeleteBase{ID: otherBatchID}, TenantID: 11, BatchType: BatchTypeSingleProduct, ProductID: &otherProduct, Status: BatchFailed})
	if err := db.Create(&ProductPublishTask{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, TenantID: 11, ProductID: otherProduct, ShopID: otherShop, TargetStoreID: otherShop, BatchID: &otherBatchID, Platform: "shopee", TaskType: TaskTypeLocalDraftCreate, Mode: PublishModeSaveAsPlatformDraft, Status: TaskFailed}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.failedTargetsFromBatch(context.Background(), 11, productA, otherBatchID.String()); err == nil {
		t.Fatal("a batch for another product must not be reusable")
	}
}

func TestCancelTaskDoesNotOverwriteConcurrentWorkerCompletion(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	const tenantID int64 = 11
	productID := seedScopedProduct(t, db, tenantID, "cancel-race")
	shopID := seedScopedShop(t, db, tenantID, "cancel-race-shop")
	publicationID, taskID := uuid.New(), uuid.New()
	if err := db.Create(&ProductPublication{Base: model.Base{ID: publicationID}, TenantID: tenantID, ProductID: productID, ShopID: shopID, Platform: "shopee", Status: StatusDraft, PublishStatus: StatusDraft}).Error; err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(publishSnapshot{PublicationID: publicationID})
	if err != nil {
		t.Fatal(err)
	}
	worker, executionID := "worker-a", uuid.New().String()
	leaseUntil := time.Now().UTC().Add(time.Minute)
	if err := db.Create(&ProductPublishTask{HardDeleteBase: model.HardDeleteBase{ID: taskID}, TenantID: tenantID, ProductID: productID, ShopID: shopID, TargetStoreID: shopID, Platform: "shopee", TaskType: TaskTypeLocalDraftCreate, Mode: PublishModeSaveAsPlatformDraft, Status: TaskRunning, LockedBy: &worker, LockedUntil: &leaseUntil, ExecutionID: &executionID, LockVersion: 1, Input: datatypes.JSON(input)}).Error; err != nil {
		t.Fatal(err)
	}
	var complete sync.Once
	requireNoError := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Complete the task immediately after CancelTask's initial read, before its
	// transaction starts. This keeps the simulated worker commit independent of
	// the cancel transaction, so a rejected cancel cannot roll it back.
	requireNoError(db.Callback().Query().After("gorm:query").Register("publish_complete_before_cancel", func(tx *gorm.DB) {
		complete.Do(func() {
			result := tx.Session(&gorm.Session{NewDB: true, SkipHooks: true}).Exec("UPDATE product_publish_tasks SET status = ?, publish_status = ?, lock_version = lock_version + 1, locked_by = NULL, locked_until = NULL, updated_at = ? WHERE id = ?", TaskSuccess, TaskSuccess, time.Now().UTC(), taskID)
			if result.Error != nil || result.RowsAffected != 1 {
				t.Fatalf("worker completion callback failed: rows=%d err=%v", result.RowsAffected, result.Error)
			}
		})
	}))
	c := scopedPublishContext(tenantID, &adminperm.Principal{TenantID: tenantID, Role: adminperm.RoleTenantAdmin})
	if _, err := svc.CancelTask(c, taskID, nil); err == nil {
		t.Fatal("cancel must reject a task completed after it was read")
	}
	var task ProductPublishTask
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskSuccess {
		t.Fatalf("cancel overwrote worker completion: %s", task.Status)
	}
	var publication ProductPublication
	if err := db.First(&publication, "id = ?", publicationID).Error; err != nil {
		t.Fatal(err)
	}
	if publication.Status != StatusDraft || publication.PublishStatus != StatusDraft {
		t.Fatalf("publication was cancelled after task conflict: %+v", publication)
	}

	foreignPublicationID, foreignTaskID := uuid.New(), uuid.New()
	if err := db.Create(&ProductPublication{Base: model.Base{ID: foreignPublicationID}, TenantID: 22, ProductID: productID, ShopID: shopID, Platform: "shopee", Status: StatusDraft, PublishStatus: StatusDraft}).Error; err != nil {
		t.Fatal(err)
	}
	foreignInput, err := json.Marshal(publishSnapshot{PublicationID: foreignPublicationID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductPublishTask{HardDeleteBase: model.HardDeleteBase{ID: foreignTaskID}, TenantID: tenantID, ProductID: productID, ShopID: shopID, TargetStoreID: shopID, Platform: "shopee", TaskType: TaskTypeLocalDraftCreate, Mode: PublishModeSaveAsPlatformDraft, Status: TaskPending, Input: datatypes.JSON(foreignInput)}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelTask(c, foreignTaskID, nil); err != nil {
		t.Fatalf("owned task cancellation failed: %v", err)
	}
	publication = ProductPublication{}
	if err := db.First(&publication, "id = ?", foreignPublicationID).Error; err != nil {
		t.Fatal(err)
	}
	if publication.Status != StatusDraft || publication.PublishStatus != StatusDraft {
		t.Fatalf("tenant-scoped cancel changed foreign publication: %+v", publication)
	}
}

func TestDouyinSuccessDoesNotWritePublicationWhenCancelWins(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	const tenantID int64 = 11
	productID := seedScopedProduct(t, db, tenantID, "cancel-wins")
	shopID, publicationID, taskID := seedScopedShop(t, db, tenantID, "cancel-wins-shop"), uuid.New(), uuid.New()
	if err := db.Create(&ProductPublication{Base: model.Base{ID: publicationID}, TenantID: tenantID, ProductID: productID, ShopID: shopID, Platform: "douyin_shop", Status: StatusPublishing, PublishStatus: StatusPublishing}).Error; err != nil {
		t.Fatal(err)
	}
	worker, executionID := "worker-a", uuid.New()
	executionIDRaw := executionID.String()
	leaseUntil := time.Now().UTC().Add(time.Minute)
	if err := db.Create(&ProductPublishTask{HardDeleteBase: model.HardDeleteBase{ID: taskID}, TenantID: tenantID, ProductID: productID, ShopID: shopID, TargetStoreID: shopID, Platform: "douyin_shop", Status: TaskRunning, LockedBy: &worker, LockedUntil: &leaseUntil, ExecutionID: &executionIDRaw, LockVersion: 1}).Error; err != nil {
		t.Fatal(err)
	}
	var cancelOnce sync.Once
	if err := db.Callback().Update().Before("gorm:update").Register("douyin_cancel_before_finish", func(tx *gorm.DB) {
		cancelOnce.Do(func() {
			result := tx.Exec("UPDATE product_publish_tasks SET status = ?, publish_status = ?, locked_by = NULL, locked_until = NULL, updated_at = ? WHERE id = ?", TaskCancelled, TaskCancelled, time.Now().UTC(), taskID)
			if result.Error != nil || result.RowsAffected != 1 {
				t.Fatalf("cancel callback failed: rows=%d err=%v", result.RowsAffected, result.Error)
			}
		})
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.completeDouyinDraftSuccess(context.Background(), &ProductPublishTask{TenantID: tenantID, ProductID: productID, ShopID: shopID}, taskID, worker, &tasklease.ClaimResult{ExecutionID: executionID, LeaseVersion: 1}, douyinDraftSnapshot{PublicationID: publicationID}, &DouyinPayloadBuildResult{Payload: &DouyinProductPayload{}}, &platformdouyin.PlatformProductResult{PlatformProductID: "external-id"})
	if !errors.Is(err, tasklease.ErrLeaseLost) {
		t.Fatalf("expected cancel to win lease race, got %v", err)
	}
	var publication ProductPublication
	if err := db.First(&publication, "id = ?", publicationID).Error; err != nil {
		t.Fatal(err)
	}
	if publication.Status != StatusPublishing || publication.PublishStatus != StatusPublishing || publication.ExternalProductID != "" {
		t.Fatalf("cancelled worker changed publication: %+v", publication)
	}
}
