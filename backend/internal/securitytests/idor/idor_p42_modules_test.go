package idor_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
)

// --- Export IDOR (3 cases) ---

func TestIDOR_ExportGetCrossTenant(t *testing.T) {
	db := openP42DB(t)
	svc := &exportmod.Service{DB: db}
	jid := seedExportJob(t, db, tenantB, exportmod.ExportTypeOrders)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.GetJob(c, jid)
	assertCrossTenantDenied(t, err)
}

func TestIDOR_ExportListTenantScoped(t *testing.T) {
	db := openP42DB(t)
	svc := &exportmod.Service{DB: db}
	seedExportJob(t, db, tenantA, exportmod.ExportTypeOrders)
	seedExportJob(t, db, tenantB, exportmod.ExportTypeProducts)
	c := ginWithTenant(tenantA, uuid.New())
	rows, total, err := svc.ListJobs(c, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 export job, got total=%d len=%d", total, len(rows))
	}
	if rows[0].TenantID != tenantA {
		t.Fatalf("expected tenant %d, got %d", tenantA, rows[0].TenantID)
	}
	for _, row := range rows {
		assertNoSensitiveLeak(t, row.ErrorMessage)
	}
}

func TestIDOR_ExportCreateStampsTenant(t *testing.T) {
	db := openP42DB(t)
	svc := &exportmod.Service{DB: db}
	c := ginWithTenant(tenantA, uuid.New())
	row, err := svc.CreateJob(c, exportmod.CreateJobInput{ExportType: exportmod.ExportTypeOrders, MaskedPII: true})
	if err != nil {
		t.Fatal(err)
	}
	if row.TenantID != tenantA {
		t.Fatalf("create must stamp JWT tenant, got %d", row.TenantID)
	}
}

// --- Operation Log IDOR (2 cases) ---

func TestIDOR_OpLogListTenantScoped(t *testing.T) {
	db := openP42DB(t)
	svc := &operationlog.Service{DB: db}
	seedOpLog(t, db, tenantA, nil, "log-a")
	seedOpLog(t, db, tenantB, nil, secretTenantB)
	c := ginWithTenant(tenantA, uuid.New())
	res, err := svc.List(c, operationlog.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 log, got %d", res.Total)
	}
	for _, item := range res.Items {
		assertNoSensitiveLeak(t, item.Message)
	}
}

func TestIDOR_OpLogCrossTenantNotInList(t *testing.T) {
	db := openP42DB(t)
	svc := &operationlog.Service{DB: db}
	seedOpLog(t, db, tenantB, nil, secretTenantB)
	c := ginWithTenant(tenantA, uuid.New())
	res, err := svc.List(c, operationlog.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Fatalf("expected 0 logs for tenant A, got %d", res.Total)
	}
}

// --- Files IDOR extended (3 cases) ---

func TestIDOR_FileDeleteCrossTenantViaDelete(t *testing.T) {
	db := openP42DB(t)
	svc := &files.Service{DB: db}
	fid := seedFile(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	err := svc.Delete(c, fid)
	assertCrossTenantDenied(t, err)
	var count int64
	db.Model(&files.FileRecord{}).Where("id = ?", fid).Count(&count)
	if count != 1 {
		t.Fatal("cross-tenant delete removed row")
	}
}

func TestIDOR_WebhookEventListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	seedWebhookEvent(t, db, tenantA)
	seedWebhookEvent(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []webhook.Event
	tx, _, err := applyTenantList(c, db.Model(&webhook.Event{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 webhook event, got %d", len(rows))
	}
	for _, row := range rows {
		assertNoSensitiveLeak(t, row.RawSummary)
	}
}

func TestIDOR_FileScanCrossTenantLoadDenied(t *testing.T) {
	db := openP42DB(t)
	fid := seedFile(t, db, tenantB)
	var row files.FileRecord
	assertFindByIDDenied(t, db, tenantA, fid, &row)
}

// --- Task tenant worker gate (2 cases) ---

func TestIDOR_TaskTenantRequireZeroDenied(t *testing.T) {
	assertTaskTenantMissing(t, tasktenant.RequireTaskTenant(0))
}

func TestIDOR_TaskTenantResourceMismatchDenied(t *testing.T) {
	err := tasktenant.EnsureResourceTenantMatch(tenantA, tenantB)
	if err == nil {
		t.Fatal("expected tenant mismatch error")
	}
	if err != security.ErrTaskTenantMismatch {
		t.Fatalf("expected ErrTaskTenantMismatch, got %v", err)
	}
}

// --- Security rotation tenant isolation (2 cases) ---

func TestIDOR_SecurityRotationFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	rid := seedRotationJob(t, db, tenantB)
	var row securitymod.KeyRotationJob
	assertFindByIDDenied(t, db, tenantA, rid, &row)
}

func TestIDOR_SecurityRotationNoLeakOnDeniedLoad(t *testing.T) {
	db := openP42DB(t)
	rid := seedRotationJob(t, db, tenantB)
	ctx := context.Background()
	var row securitymod.KeyRotationJob
	err := repository.FindByID(ctx, db, &row, tenantA, rid)
	assertCrossTenantDenied(t, err)
	if strings.Contains(row.ActiveKeyID, secretTenantB) {
		t.Fatal("leaked rotation data on denied load")
	}
}
