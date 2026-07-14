package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type queryPlanCheck struct {
	Name                string   `json:"name"`
	Status              string   `json:"status"`
	PlanningTimeMs      *float64 `json:"planningTimeMs"`
	ExecutionTimeMs     *float64 `json:"executionTimeMs"`
	ActualRows          *float64 `json:"actualRows"`
	RowsRemovedByFilter *float64 `json:"rowsRemovedByFilter"`
	SharedHitBlocks     *float64 `json:"sharedHitBlocks"`
	SharedReadBlocks    *float64 `json:"sharedReadBlocks"`
	ScanType            string   `json:"scanType"`
	IndexName           string   `json:"indexName"`
	SortMethod          string   `json:"sortMethod"`
	SortSpaceType       string   `json:"sortSpaceType"`
	DiskSpill           *bool    `json:"diskSpill"`
	Issue               string   `json:"issue,omitempty"`
}

type queryPlanReport struct {
	Phase                       string           `json:"phase"`
	Status                      string           `json:"status"`
	GeneratedAt                 string           `json:"generatedAt"`
	TenantID                    int64            `json:"tenantId"`
	PaginationPrerequisite      string           `json:"paginationPrerequisite"`
	UnintendedLargeTableSeqScan *bool            `json:"unintendedLargeTableSeqScan"`
	UnresolvedDiskSpill         *bool            `json:"unresolvedDiskSpill"`
	Checks                      []queryPlanCheck `json:"checks"`
	DryRun                      bool             `json:"dryRun"`
	DurationMs                  int64            `json:"durationMs"`
	Guards                      []string         `json:"guards"`
	Issues                      []string         `json:"issues"`
}

type explainRoot struct {
	Plan          json.RawMessage `json:"Plan"`
	PlanningTime  float64         `json:"Planning Time"`
	ExecutionTime float64         `json:"Execution Time"`
}

type explainPlanNode struct {
	NodeType            string            `json:"Node Type"`
	IndexName           string            `json:"Index Name"`
	ActualRows          float64           `json:"Actual Rows"`
	RowsRemovedByFilter float64           `json:"Rows Removed by Filter"`
	SharedHitBlocks     float64           `json:"Shared Hit Blocks"`
	SharedReadBlocks    float64           `json:"Shared Read Blocks"`
	SortMethod          string            `json:"Sort Method"`
	SortSpaceType       string            `json:"Sort Space Type"`
	Plans               []explainPlanNode `json:"Plans"`
}

func runQueryPlan(ctx context.Context) (queryPlanReport, error) {
	started := time.Now().UTC()
	e, err := openVerifiedDB(ctx)
	if err != nil {
		return queryPlanReport{}, err
	}
	defer closeEnv(e)

	scenarios := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "Product Cursor List",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT id FROM products WHERE tenant_id = ? ORDER BY created_at DESC, id DESC LIMIT 50`,
			args: []any{e.tenantID},
		},
		{
			name: "Order Cursor List",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT id FROM orders WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY created_at DESC, id DESC LIMIT 50`,
			args: []any{e.tenantID},
		},
		{
			name: "Inventory Cursor List",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT sk.id FROM product_skus sk INNER JOIN products p ON p.id = sk.product_id WHERE p.tenant_id = ? ORDER BY sk.updated_at DESC, sk.id DESC LIMIT 50`,
			args: []any{e.tenantID},
		},
		{
			name: "Task Claim",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT id FROM collect_tasks WHERE tenant_id = ? AND status = 'failed' ORDER BY updated_at DESC, id DESC LIMIT 50`,
			args: []any{e.tenantID},
		},
		{
			name: "Webhook Dedup",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT id FROM webhook_events WHERE tenant_id = ? ORDER BY created_at DESC, id DESC LIMIT 50`,
			args: []any{e.tenantID},
		},
		{
			name: "Operation Log Cursor List",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT id FROM operation_logs WHERE tenant_id = ? ORDER BY created_at DESC, id DESC LIMIT 50`,
			args: []any{e.tenantID},
		},
		{
			name: "Role/Permission Batch Read",
			sql:  `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT user_id, store_id FROM user_store_permissions WHERE user_id IN (SELECT id FROM admin_users WHERE tenant_id = ? LIMIT 50)`,
			args: []any{e.tenantID},
		},
	}

	rep := queryPlanReport{
		Phase:                  phase,
		Status:                 "passed",
		GeneratedAt:            started.Format(time.RFC3339),
		TenantID:               e.tenantID,
		PaginationPrerequisite: "not_required_for_explain",
		DryRun:                 false,
		Guards:                 guardList(),
	}
	seqScan := false
	diskSpill := false
	for _, sc := range scenarios {
		check := queryPlanCheck{Name: sc.name, Status: "passed"}
		raw, runErr := explainJSON(ctx, e, sc.sql, sc.args...)
		if runErr != nil {
			check.Status = "failed"
			check.Issue = runErr.Error()
			rep.Status = "failed"
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %s", sc.name, runErr.Error()))
			rep.Checks = append(rep.Checks, check)
			continue
		}
		fillExplainCheck(&check, raw)
		if strings.EqualFold(check.ScanType, "Seq Scan") && check.ActualRows != nil && *check.ActualRows > 1000 {
			seqScan = true
		}
		if check.DiskSpill != nil && *check.DiskSpill {
			diskSpill = true
		}
		rep.Checks = append(rep.Checks, check)
	}
	falseVal := false
	rep.UnintendedLargeTableSeqScan = &seqScan
	rep.UnresolvedDiskSpill = &falseVal
	if diskSpill {
		trueVal := true
		rep.UnresolvedDiskSpill = &trueVal
	}
	rep.DurationMs = time.Since(started).Milliseconds()
	return rep, nil
}

func explainJSON(ctx context.Context, e *env, sql string, args ...any) (explainRoot, error) {
	var zero explainRoot
	var payload string
	if err := e.db.WithContext(ctx).Raw(sql, args...).Scan(&payload).Error; err != nil {
		return zero, err
	}
	payload = strings.TrimSpace(payload)
	var roots []explainRoot
	if err := json.Unmarshal([]byte(payload), &roots); err != nil {
		return zero, err
	}
	if len(roots) == 0 {
		return zero, fmt.Errorf("empty explain payload")
	}
	return roots[0], nil
}

func fillExplainCheck(check *queryPlanCheck, root explainRoot) {
	if check == nil {
		return
	}
	pt := root.PlanningTime
	et := root.ExecutionTime
	check.PlanningTimeMs = &pt
	check.ExecutionTimeMs = &et
	var plan explainPlanNode
	if err := json.Unmarshal(root.Plan, &plan); err != nil {
		check.Status = "failed"
		check.Issue = err.Error()
		return
	}
	top := pickPlanNode(plan)
	check.ScanType = top.NodeType
	check.IndexName = top.IndexName
	if top.ActualRows > 0 {
		v := top.ActualRows
		check.ActualRows = &v
	}
	if top.RowsRemovedByFilter > 0 {
		v := top.RowsRemovedByFilter
		check.RowsRemovedByFilter = &v
	}
	if top.SharedHitBlocks > 0 {
		v := top.SharedHitBlocks
		check.SharedHitBlocks = &v
	}
	if top.SharedReadBlocks > 0 {
		v := top.SharedReadBlocks
		check.SharedReadBlocks = &v
	}
	check.SortMethod = top.SortMethod
	check.SortSpaceType = top.SortSpaceType
	spill := strings.EqualFold(top.SortSpaceType, "Disk") || strings.Contains(strings.ToLower(top.NodeType), "hash")
	check.DiskSpill = &spill
}

func pickPlanNode(plan explainPlanNode) explainPlanNode {
	if len(plan.Plans) == 0 {
		return plan
	}
	return pickPlanNode(plan.Plans[0])
}
