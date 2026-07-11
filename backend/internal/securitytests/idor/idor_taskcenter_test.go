package idor_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
)

// --- Task Center IDOR (6 cases) ---

func TestIDOR_TaskCenterAlertFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	aid := seedTaskAlert(t, db, tenantB, uuid.NewString())
	var row taskcenter.TaskAlert
	assertFindByIDDenied(t, db, tenantA, aid, &row)
	assertNoSensitiveLeak(t, row.Title)
}

func TestIDOR_TaskCenterFailureMarkFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	mid := seedTaskFailureMark(t, db, tenantB, uuid.NewString())
	var row taskcenter.TaskFailureMark
	assertFindByIDDenied(t, db, tenantA, mid, &row)
	assertNoSensitiveLeak(t, row.Remark)
}

func TestIDOR_TaskCenterCollectTaskScopedDenied(t *testing.T) {
	db := openP42DB(t)
	tid := seedCollectTaskFailed(t, db, tenantB)
	var row collect.CollectTask
	assertFindByIDDenied(t, db, tenantA, tid, &row)
	assertNoSensitiveLeak(t, row.ErrorMessage)
}

func TestIDOR_TaskCenterAlertScopedListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	seedTaskAlert(t, db, tenantA, uuid.NewString())
	seedTaskAlert(t, db, tenantB, uuid.NewString())
	c := ginWithTenant(tenantA, uuid.New())
	var rows []taskcenter.TaskAlert
	tx, _, err := applyTenantList(c, db.Model(&taskcenter.TaskAlert{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(rows))
	}
	for _, row := range rows {
		assertNoSensitiveLeak(t, row.Title)
	}
}

func TestIDOR_TaskCenterFailureMarkScopedListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	seedTaskFailureMark(t, db, tenantA, uuid.NewString())
	seedTaskFailureMark(t, db, tenantB, uuid.NewString())
	c := ginWithTenant(tenantA, uuid.New())
	var rows []taskcenter.TaskFailureMark
	tx, _, err := applyTenantList(c, db.Model(&taskcenter.TaskFailureMark{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 failure mark, got %d", len(rows))
	}
}
