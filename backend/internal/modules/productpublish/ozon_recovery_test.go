package productpublish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func seedOzonRecoveryTask(t *testing.T, db *gorm.DB, tenantID int64, taskStatus, publishStatus string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	productID := seedScopedProduct(t, db, tenantID, "ozon recovery")
	shopID := seedScopedShop(t, db, tenantID, "ozon recovery shop")
	if err := db.Model(&struct{ ID uuid.UUID }{}).Table("shops").Where("id = ?", shopID).Update("platform", "ozon").Error; err != nil {
		t.Fatal(err)
	}
	publicationID, taskID := uuid.New(), uuid.New()
	if err := db.Create(&ProductPublication{
		Base: model.Base{ID: publicationID}, TenantID: tenantID, ProductID: productID,
		ShopID: shopID, Platform: "ozon", Status: publishStatus, PublishStatus: publishStatus,
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(publishSnapshot{PublicationID: publicationID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductPublishTask{
		HardDeleteBase: model.HardDeleteBase{ID: taskID}, TenantID: tenantID,
		ProductID: productID, ShopID: shopID, TargetStoreID: shopID,
		Platform: "ozon", TaskType: TaskTypeProductPublish, Status: taskStatus,
		PublishStatus: publishStatus, Mode: ModeManual, PublishMode: ModeManual,
		Input: datatypes.JSON(raw), ErrorCode: ErrorPublishResultUnknown,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return productID, shopID, taskID
}

func ozonTenantAdminContext(tenantID int64) *gin.Context {
	return scopedPublishContext(tenantID, &adminperm.Principal{TenantID: tenantID, Role: adminperm.RoleTenantAdmin})
}

func TestCancelRunningOzonTaskIsRejected(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskRunning, StatusPublishing)
	worker, executionID := "worker-a", uuid.New().String()
	leaseUntil := time.Now().UTC().Add(time.Minute)
	if err := db.Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"locked_by": worker, "execution_id": executionID, "locked_until": leaseUntil, "lock_version": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := (&Service{DB: db}).CancelTask(ozonTenantAdminContext(11), taskID, nil); err == nil || !strings.Contains(err.Error(), "不能取消") {
		t.Fatalf("running Ozon cancellation must be rejected, got %v", err)
	}
	var task ProductPublishTask
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || task.PublishStatus != StatusPublishing {
		t.Fatalf("rejected cancellation changed task: %+v", task)
	}
}

func TestReconcileOzonTaskPlatformCreatedImported(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskFailed, StatusResultUnknown)
	svc := &Service{DB: db}
	out, err := svc.ReconcileOzonTask(ozonTenantAdminContext(11), taskID, ReconcileOzonTaskBody{
		Outcome: OzonReconcileCreated, ExternalProductID: "ozon-product-1",
		PlatformStatus: platformp.PublishStatusImported, Evidence: "Ozon 后台按 offer ID 查询到商品",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != TaskSuccess || out.PublishStatus != StatusImported || out.Retryable {
		t.Fatalf("unexpected reconciled task: %+v", out)
	}
	var publication ProductPublication
	if err := db.First(&publication, "publish_task_id = ?", taskID).Error; err != nil {
		// Older seeded rows do not carry PublishTaskID; resolve from snapshot.
		var task ProductPublishTask
		if loadErr := db.First(&task, "id = ?", taskID).Error; loadErr != nil {
			t.Fatal(loadErr)
		}
		publicationID, _ := snapshotPublicationFromTask(&task)
		if loadErr := db.First(&publication, "id = ?", publicationID).Error; loadErr != nil {
			t.Fatal(loadErr)
		}
	}
	if publication.PublishStatus != StatusImported || publication.ExternalProductID != "ozon-product-1" || publication.PublishedAt != nil {
		t.Fatalf("unexpected reconciled publication: %+v", publication)
	}
}

func TestReconcileOzonTaskNotCreatedOnlyEnablesExplicitRetry(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskFailed, StatusResultUnknown)
	if err := db.Model(&ProductPublishTask{}).Where("id = ?", taskID).Update("platform_product_id", "uncertain-id").Error; err != nil {
		t.Fatal(err)
	}
	out, err := (&Service{DB: db}).ReconcileOzonTask(ozonTenantAdminContext(11), taskID, ReconcileOzonTaskBody{
		Outcome: OzonReconcileNotCreated, Evidence: "Ozon 后台按 offer ID 与任务编号均未查询到商品",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != TaskFailed || !out.Retryable || out.RecoveryState != "confirmed_not_created" || out.PlatformProductID != "" {
		t.Fatalf("reconciliation must enable, but not execute, a retry: %+v", out)
	}
}

func TestReconcileOzonTaskValidatesTenantPermissionAndOutcome(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, shopID, taskID := seedOzonRecoveryTask(t, db, 11, TaskFailed, StatusResultUnknown)
	svc := &Service{DB: db}
	valid := ReconcileOzonTaskBody{Outcome: OzonReconcileNotCreated, Evidence: "checked"}
	if _, err := svc.ReconcileOzonTask(ozonTenantAdminContext(22), taskID, valid, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign tenant must not resolve task, got %v", err)
	}
	operator := scopedPublishContext(11, &adminperm.Principal{TenantID: 11, Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopID, PermissionScope: "view"}}})
	if _, err := svc.ReconcileOzonTask(operator, taskID, valid, nil); err == nil {
		t.Fatal("view-only operator must not reconcile an Ozon task")
	}
	if _, err := svc.ReconcileOzonTask(ozonTenantAdminContext(11), taskID, ReconcileOzonTaskBody{Outcome: "retry", Evidence: "checked"}, nil); err == nil {
		t.Fatal("invalid outcome must be rejected")
	}
	if _, err := svc.ReconcileOzonTask(ozonTenantAdminContext(11), taskID, ReconcileOzonTaskBody{
		Outcome: OzonReconcileCreated, ExternalProductID: "ozon-product-1",
		PlatformStatus: platformp.PublishStatusSellable, Evidence: "checked",
	}, nil); err == nil {
		t.Fatal("published reconciliation must require sellableVerified")
	}
}

func TestReconcileOzonTaskHandlerRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newBatchIntegrationDB(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: uuid.NewString()}}
	c.Request = httptest.NewRequest("POST", "/reconcile", bytes.NewBufferString("{"))
	c.Request.Header.Set("Content-Type", "application/json")
	(&Handler{Svc: &Service{DB: db}}).ReconcileOzonTask(c)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid json body") {
		t.Fatalf("unexpected handler response: code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReconcileOzonTaskHandlerPreservesPermissionEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newBatchIntegrationDB(t)
	_, shopID, taskID := seedOzonRecoveryTask(t, db, 11, TaskFailed, StatusResultUnknown)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: taskID.String()}}
	c.Request = httptest.NewRequest(http.MethodPost, "/reconcile", bytes.NewBufferString(`{"outcome":"platform_not_created","evidence":"checked"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.TenantID, int64(11))
	c.Set("adminperm.principal", &adminperm.Principal{
		TenantID: 11,
		Role:     adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{
			StoreID: shopID, PermissionScope: "view",
		}},
	})

	(&Handler{Svc: &Service{DB: db}}).ReconcileOzonTask(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("permission error status = %d, body=%s", w.Code, w.Body.String())
	}
	var envelope response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("permission response must remain one valid envelope: %v body=%s", err, w.Body.String())
	}
	if envelope.Code != response.CodePermissionDenied {
		t.Fatalf("permission envelope code = %d, body=%s", envelope.Code, w.Body.String())
	}
}

func TestCompleteGenericPublicationResultRollsBackWhenLeaseAlreadyLost(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskCancelled, TaskCancelled)
	var task ProductPublishTask
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	publicationID, _ := snapshotPublicationFromTask(&task)
	claim := &tasklease.ClaimResult{ExecutionID: uuid.New(), LeaseVersion: 1}
	err := (&Service{DB: db}).completeGenericPublicationResult(context.Background(), &task, taskID, "stale-worker", claim, publicationID,
		&platformp.PublishProductResult{ExternalProductID: "must-not-commit", Status: platformp.PublishStatusImported},
		time.Now().UTC(), []byte(`{"status":"imported"}`), StatusImported, TaskSuccess, StatusImported, "", "")
	if !errors.Is(err, tasklease.ErrLeaseLost) {
		t.Fatalf("expected lease loss, got %v", err)
	}
	var publication ProductPublication
	if err := db.First(&publication, "id = ?", publicationID).Error; err != nil {
		t.Fatal(err)
	}
	if publication.ExternalProductID != "" || publication.PublishStatus != TaskCancelled {
		t.Fatalf("publication update escaped rolled-back transaction: %+v", publication)
	}
}

func TestCompleteGenericPublicationResultCommitsTaskAndPublicationAtomically(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskRunning, StatusPublishing)
	executionID := uuid.New()
	worker := "worker-a"
	leaseUntil := time.Now().UTC().Add(time.Minute)
	if err := db.Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"locked_by": worker, "execution_id": executionID.String(), "locked_until": leaseUntil, "lock_version": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var task ProductPublishTask
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	publicationID, _ := snapshotPublicationFromTask(&task)
	res := &platformp.PublishProductResult{
		ExternalProductID: "ozon-imported-id", Status: platformp.PublishStatusImported,
		Warnings: []platformp.PublishWarning{{Stage: "import", Severity: "warning", Code: "DIMENSION", Message: "check dimensions"}},
	}
	if err := (&Service{DB: db}).completeGenericPublicationResult(context.Background(), &task, taskID, worker,
		&tasklease.ClaimResult{ExecutionID: executionID, LeaseVersion: 1}, publicationID, res, time.Now().UTC(),
		[]byte(`{"status":"imported"}`), StatusImported, TaskSuccess, StatusImported, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	var publication ProductPublication
	if err := db.First(&publication, "id = ?", publicationID).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskSuccess || task.PublishStatus != StatusImported || publication.PublishStatus != StatusImported || publication.ExternalProductID != "ozon-imported-id" {
		t.Fatalf("task/publication did not commit one imported outcome: task=%+v publication=%+v", task, publication)
	}
	if !strings.Contains(string(publication.RawData), `"warnings":[{`) {
		t.Fatalf("structured warnings were not persisted: %s", publication.RawData)
	}
}

func TestRecordLateGenericExternalFactPreservesIDWithoutPromotingSuccess(t *testing.T) {
	db := newBatchIntegrationDB(t)
	_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskFailed, StatusResultUnknown)
	var task ProductPublishTask
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	res := &platformp.PublishProductResult{ExternalProductID: "late-ozon-id", ExternalSPUID: "late-spu", Status: platformp.PublishStatusImported}
	if err := (&Service{DB: db}).recordLateGenericExternalFact(context.Background(), &task, taskID, res, "lease lost"); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskFailed || task.PublishStatus != StatusResultUnknown || task.PlatformProductID != "late-ozon-id" || task.Retryable {
		t.Fatalf("late result was not preserved safely: %+v", task)
	}
}

func TestRecoverExpiredOzonLeaseClassifiesMutationBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, stage, wantStatus, wantCode string
		wantRetryable                     bool
	}{
		{name: "before mutation", stage: StatusChecking, wantStatus: StatusPubFailed, wantCode: ErrorPublishNotSent, wantRetryable: true},
		{name: "after mutation", stage: StatusPublishing, wantStatus: StatusResultUnknown, wantCode: ErrorPublishResultUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newBatchIntegrationDB(t)
			_, _, taskID := seedOzonRecoveryTask(t, db, 11, TaskRunning, tc.stage)
			worker, executionID := "expired-worker", uuid.New().String()
			expired := time.Now().UTC().Add(-time.Minute)
			if err := db.Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(map[string]any{
				"locked_by": worker, "execution_id": executionID, "locked_until": expired, "lock_version": 1,
			}).Error; err != nil {
				t.Fatal(err)
			}
			if err := (&Service{DB: db}).RecoverLeaseExpired(context.Background(), taskID); err != nil {
				t.Fatal(err)
			}
			var task ProductPublishTask
			if err := db.First(&task, "id = ?", taskID).Error; err != nil {
				t.Fatal(err)
			}
			if task.Status != TaskFailed || task.PublishStatus != tc.wantStatus || task.ErrorCode != tc.wantCode || task.Retryable != tc.wantRetryable {
				t.Fatalf("unexpected recovered task: %+v", task)
			}
		})
	}
}

func TestOzonPublicationStatusIsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  platformp.PublishProductResult
		want string
	}{
		{name: "imported remains accepted only", res: platformp.PublishProductResult{Status: platformp.PublishStatusImported}, want: StatusImported},
		{name: "needs action remains non success", res: platformp.PublishProductResult{Status: platformp.PublishStatusNeedsAction}, want: StatusNeedsAction},
		{name: "legacy published without verification is unknown", res: platformp.PublishProductResult{Status: StatusPublishedRecord}, want: StatusResultUnknown},
		{name: "explicit sellable is published", res: platformp.PublishProductResult{Status: platformp.PublishStatusSellable}, want: StatusPublishedRecord},
		{name: "unknown provider state is unknown", res: platformp.PublishProductResult{Status: "mystery"}, want: StatusResultUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicationStatusForResult("ozon", &tc.res); got != tc.want {
				t.Fatalf("publicationStatusForResult() = %q, want %q", got, tc.want)
			}
		})
	}
	taskStatus, publishStatus, code, _ := genericTaskOutcome("ozon", StatusImported)
	if taskStatus != TaskSuccess || publishStatus != StatusImported || code != "" {
		t.Fatalf("imported must be a technically complete task without claiming sellable: %s %s %s", taskStatus, publishStatus, code)
	}
	taskStatus, publishStatus, code, _ = genericTaskOutcome("ozon", StatusNeedsAction)
	if taskStatus != TaskFailed || publishStatus != StatusNeedsAction || code != ErrorPublishNeedsAction {
		t.Fatalf("needs_action must remain non-success: %s %s %s", taskStatus, publishStatus, code)
	}
}
