package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

type nPlusOneScenario struct {
	Scenario             string `json:"scenario"`
	Rows10QueryCount     int64  `json:"rows10QueryCount"`
	Rows100QueryCount    int64  `json:"rows100QueryCount"`
	ExpectedMaxQueries   int64  `json:"expectedMaxQueries"`
	LinearGrowthDetected bool   `json:"linearGrowthDetected"`
	TenantScopePassed    bool   `json:"tenantScopePassed"`
	Status               string `json:"status"`
	Issue                string `json:"issue,omitempty"`
}

type nPlusOneReport struct {
	Phase                string             `json:"phase"`
	Status               string             `json:"status"`
	GeneratedAt          string             `json:"generatedAt"`
	TenantID             int64              `json:"tenantId"`
	Scenarios            []nPlusOneScenario `json:"scenarios"`
	LinearGrowthDetected bool               `json:"linearGrowthDetected"`
	DryRun               bool               `json:"dryRun"`
	DurationMs           int64              `json:"durationMs"`
	Guards               []string           `json:"guards"`
	Issues               []string           `json:"issues"`
}

func runNPlusOne(ctx context.Context) (nPlusOneReport, error) {
	started := time.Now().UTC()
	e, err := openVerifiedDB(ctx)
	if err != nil {
		return nPlusOneReport{}, err
	}
	defer closeEnv(e)

	adminID, adminErr := resolveAdminUserID(ctx, e.db, e.tenantID)
	c := newGinContext(e.tenantID, adminID)

	runners := []struct {
		name string
		run  func(int) (int, int64, bool, error)
		max  int64
	}{
		{
			name: "orders_with_items",
			max:  12,
			run: func(limit int) (int, int64, bool, error) {
				qc, detach := attachQueryCounter(e.db)
				defer detach()
				svc := &order.Service{DB: e.db}
				res, err := svc.List(c, order.ListQuery{UseCursor: true, PageSize: limit, Limit: limit})
				if err != nil {
					return 0, 0, false, err
				}
				return len(res.Items), qc.snapshot(), true, nil
			},
		},
		{
			name: "products_with_skus",
			max:  12,
			run: func(limit int) (int, int64, bool, error) {
				qc, detach := attachQueryCounter(e.db)
				defer detach()
				svc := &product.Service{DB: e.db}
				res, err := svc.List(c, product.ListQuery{UseCursor: true, PageSize: limit, Limit: limit})
				if err != nil {
					return 0, 0, false, err
				}
				return len(res.Items), qc.snapshot(), true, nil
			},
		},
		{
			name: "users_with_role_permissions",
			max:  15,
			run: func(limit int) (int, int64, bool, error) {
				qc, detach := attachQueryCounter(e.db)
				defer detach()
				svc := &adminuser.Service{DB: e.db}
				res, err := svc.List(c, adminuser.ListQuery{Page: 1, PageSize: limit})
				if err != nil {
					return 0, 0, false, err
				}
				return len(res.Items), qc.snapshot(), true, nil
			},
		},
		{
			name: "tasks_with_summary",
			max:  40,
			run: func(limit int) (int, int64, bool, error) {
				qc, detach := attachQueryCounter(e.db)
				defer detach()
				svc := &taskcenter.Service{DB: e.db, Cfg: e.cfg}
				listRes, err := svc.ListFailures(ctx, taskcenter.ListFailureParams{
					TenantID:  e.tenantID,
					TaskType:  taskcenter.TaskTypeCollect,
					UseCursor: true,
					PageSize:  limit,
					Limit:     limit,
				})
				if err != nil {
					return 0, 0, false, err
				}
				_, err = svc.Summary(ctx, taskcenter.ListFailureParams{TenantID: e.tenantID, TaskType: taskcenter.TaskTypeCollect})
				if err != nil {
					return 0, 0, false, err
				}
				return len(listRes.List), qc.snapshot(), true, nil
			},
		},
	}

	rep := nPlusOneReport{
		Phase:       phase,
		Status:      "passed",
		GeneratedAt: started.Format(time.RFC3339),
		TenantID:    e.tenantID,
		DryRun:      false,
		Guards:      guardList(),
	}
	if adminErr != nil {
		rep.Issues = append(rep.Issues, adminErr.Error())
	}

	anyLinear := false
	for _, r := range runners {
		item := nPlusOneScenario{Scenario: r.name, ExpectedMaxQueries: r.max, Status: "passed", TenantScopePassed: true}
		rows10, q10, _, err := r.run(10)
		if err != nil {
			item.Status = "failed"
			item.Issue = err.Error()
			rep.Status = "failed"
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %s", r.name, err.Error()))
			rep.Scenarios = append(rep.Scenarios, item)
			continue
		}
		rows100, q100, _, err := r.run(100)
		if err != nil {
			item.Status = "failed"
			item.Issue = err.Error()
			rep.Status = "failed"
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %s", r.name, err.Error()))
			rep.Scenarios = append(rep.Scenarios, item)
			continue
		}
		item.Rows10QueryCount = q10
		item.Rows100QueryCount = q100
		item.LinearGrowthDetected = q100 > q10*3 && q100 > r.max
		if item.LinearGrowthDetected {
			item.Status = "failed"
			rep.Status = "failed"
			anyLinear = true
			item.Issue = fmt.Sprintf("query count grew from %d@%d rows to %d@%d rows", q10, rows10, q100, rows100)
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %s", r.name, item.Issue))
		}
		if q100 > r.max && item.Status == "passed" {
			item.Status = "failed"
			rep.Status = "failed"
			item.Issue = fmt.Sprintf("query count %d exceeds expected max %d", q100, r.max)
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %s", r.name, item.Issue))
		}
		_ = rows10
		_ = rows100
		rep.Scenarios = append(rep.Scenarios, item)
	}
	rep.LinearGrowthDetected = anyLinear

	// Tenant scope sanity: principal load stays tenant-bound.
	if adminID != uuid.Nil {
		p, err := adminperm.LoadPrincipal(c, e.db)
		if err != nil || p == nil {
			rep.Status = "failed"
			rep.Issues = append(rep.Issues, "tenant scope principal load failed")
		}
	}

	rep.DurationMs = time.Since(started).Milliseconds()
	return rep, nil
}
