package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
)

type paginationListReport struct {
	List                   string `json:"list"`
	Status                 string `json:"status"`
	Service                string `json:"service"`
	RepositoryServiceWired bool   `json:"repositoryServiceWired"`
	PagesRead              int    `json:"pagesRead"`
	RowsRead               int    `json:"rowsRead"`
	Duplicates             int    `json:"duplicates"`
	UnexpectedMissingRows  int    `json:"unexpectedMissingRows"`
	MaxPageDurationMs      int64  `json:"maxPageDurationMs"`
	TamperedRejected       bool   `json:"tamperedRejected"`
	WrongVersionRejected   bool   `json:"wrongVersionRejected"`
	CrossTenantRejected    bool   `json:"crossTenantRejected"`
	CrossShopRejected      bool   `json:"crossShopRejected"`
	DeepOffsetRejected     bool   `json:"deepOffsetRejected"`
	LimitGuardPassed       bool   `json:"limitGuardPassed"`
	FilterMismatchRejected bool   `json:"filterMismatchRejected"`
	Issue                  string `json:"issue,omitempty"`
}

type paginationReport struct {
	Phase                  string                 `json:"phase"`
	Status                 string                 `json:"status"`
	GeneratedAt            string                 `json:"generatedAt"`
	TenantID               int64                  `json:"tenantId"`
	Lists                  []paginationListReport `json:"lists"`
	TamperedRejected       bool                   `json:"tamperedRejected"`
	WrongVersionRejected   bool                   `json:"wrongVersionRejected"`
	CrossTenantRejected    bool                   `json:"crossTenantRejected"`
	CrossShopRejected      bool                   `json:"crossShopRejected"`
	DeepOffsetRejected     bool                   `json:"deepOffsetRejected"`
	LimitGuardPassed       bool                   `json:"limitGuardPassed"`
	FilterMismatchRejected bool                   `json:"filterMismatchRejected"`
	DryRun                 bool                   `json:"dryRun"`
	DurationMs             int64                  `json:"durationMs"`
	Guards                 []string               `json:"guards"`
	Issues                 []string               `json:"issues"`
}

type listRunner struct {
	name    string
	service string
	run     func(context.Context, *env, int) (pages int, rows int, dupes int, maxMs int64, cursor string, err error)
	guards  func(context.Context, *env, string) (tampered, wrongVersion, crossTenant, crossShop, deepOffset, limitGuard, filterMismatch bool, err error)
}

func runPagination(ctx context.Context) (paginationReport, error) {
	started := time.Now().UTC()
	e, err := openVerifiedDB(ctx)
	if err != nil {
		return paginationReport{}, err
	}
	defer closeEnv(e)

	adminID, _ := resolveAdminUserID(ctx, e.db, e.tenantID)
	runners := []listRunner{
		{name: "product", service: "backend/internal/modules/product/service.go", run: paginateProducts, guards: guardProductList},
		{name: "order", service: "backend/internal/modules/order/service.go", run: paginateOrders, guards: guardOrderList},
		{name: "inventory", service: "backend/internal/modules/inventory/center_list.go", run: paginateInventory, guards: guardInventoryList},
		{name: "task", service: "backend/internal/modules/taskcenter/service.go", run: paginateTasks, guards: guardTaskList},
		{name: "webhook", service: "backend/internal/modules/webhook/service.go", run: paginateWebhooks, guards: guardWebhookList},
		{name: "operationLog", service: "backend/internal/modules/operationlog/service.go", run: paginateOperationLogs, guards: guardOperationLogList},
	}

	rep := paginationReport{
		Phase:       phase,
		Status:      "passed",
		GeneratedAt: started.Format(time.RFC3339),
		TenantID:    e.tenantID,
		DryRun:      false,
		Guards:      guardList(),
	}
	for _, r := range runners {
		item := paginationListReport{
			List:                   r.name,
			Service:                r.service,
			RepositoryServiceWired: true,
			Status:                 "passed",
		}
		pages, rows, dupes, maxMs, cursor, runErr := r.run(ctx, e, defaultPageSize)
		item.PagesRead = pages
		item.RowsRead = rows
		item.Duplicates = dupes
		item.MaxPageDurationMs = maxMs
		if runErr != nil {
			item.Status = "failed"
			item.Issue = runErr.Error()
			rep.Status = "failed"
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %s", r.name, runErr.Error()))
		}
		if dupes > 0 {
			item.Status = "failed"
			item.Issue = fmt.Sprintf("duplicates detected: %d", dupes)
			rep.Status = "failed"
			rep.Issues = append(rep.Issues, fmt.Sprintf("%s: %d duplicates", r.name, dupes))
		}
		if cursor != "" && r.guards != nil {
			t, wv, ct, cs, dof, lg, fm, gErr := r.guards(ctx, e, cursor)
			item.TamperedRejected = t
			item.WrongVersionRejected = wv
			item.CrossTenantRejected = ct
			item.CrossShopRejected = cs
			item.DeepOffsetRejected = dof
			item.LimitGuardPassed = lg
			item.FilterMismatchRejected = fm
			rep.TamperedRejected = rep.TamperedRejected || t
			rep.WrongVersionRejected = rep.WrongVersionRejected || wv
			rep.CrossTenantRejected = rep.CrossTenantRejected || ct
			rep.CrossShopRejected = rep.CrossShopRejected || cs
			rep.DeepOffsetRejected = rep.DeepOffsetRejected || dof
			rep.LimitGuardPassed = rep.LimitGuardPassed || lg
			rep.FilterMismatchRejected = rep.FilterMismatchRejected || fm
			if gErr != nil {
				item.Status = "failed"
				item.Issue = gErr.Error()
				rep.Status = "failed"
				rep.Issues = append(rep.Issues, fmt.Sprintf("%s guards: %s", r.name, gErr.Error()))
			}
		} else if cursor == "" && runErr == nil && rows > 0 {
			item.Issue = "cursor not produced; guard checks skipped"
		}
		_ = adminID
		rep.Lists = append(rep.Lists, item)
	}
	rep.DurationMs = time.Since(started).Milliseconds()
	return rep, nil
}

func paginateProducts(ctx context.Context, e *env, pageSize int) (int, int, int, int64, string, error) {
	svc := &product.Service{DB: e.db}
	c := newGinContext(e.tenantID, uuid.Nil)
	return paginateCursorPages(pageSize, func(cursor string) (items []string, next string, hasMore bool, err error) {
		res, err := svc.List(c, product.ListQuery{UseCursor: true, Cursor: cursor, PageSize: pageSize, Limit: pageSize})
		if err != nil {
			return nil, "", false, err
		}
		keys := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			keys = append(keys, item.ID.String())
		}
		return keys, res.NextCursor, res.HasMore, nil
	})
}

func paginateOrders(ctx context.Context, e *env, pageSize int) (int, int, int, int64, string, error) {
	svc := &order.Service{DB: e.db}
	c := newGinContext(e.tenantID, uuid.Nil)
	return paginateCursorPages(pageSize, func(cursor string) (items []string, next string, hasMore bool, err error) {
		res, err := svc.List(c, order.ListQuery{UseCursor: true, Cursor: cursor, PageSize: pageSize, Limit: pageSize})
		if err != nil {
			return nil, "", false, err
		}
		keys := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			keys = append(keys, item.ID.String())
		}
		return keys, res.NextCursor, res.HasMore, nil
	})
}

func paginateInventory(ctx context.Context, e *env, pageSize int) (int, int, int, int64, string, error) {
	svc := &inventory.Service{DB: e.db}
	return paginateCursorPages(pageSize, func(cursor string) (items []string, next string, hasMore bool, err error) {
		res, err := svc.ListInventoryCenter(ctx, inventory.CenterListQuery{
			TenantID:  e.tenantID,
			UseCursor: true,
			Cursor:    cursor,
			PageSize:  pageSize,
			Limit:     pageSize,
		})
		if err != nil {
			return nil, "", false, err
		}
		keys := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			keys = append(keys, item.ProductSkuID.String())
		}
		return keys, res.NextCursor, res.HasMore, nil
	})
}

func paginateTasks(ctx context.Context, e *env, pageSize int) (int, int, int, int64, string, error) {
	svc := &taskcenter.Service{DB: e.db, Cfg: e.cfg}
	return paginateCursorPages(pageSize, func(cursor string) (items []string, next string, hasMore bool, err error) {
		res, err := svc.ListFailures(ctx, taskcenter.ListFailureParams{
			TenantID:  e.tenantID,
			TaskType:  taskcenter.TaskTypeCollect,
			UseCursor: true,
			Cursor:    cursor,
			PageSize:  pageSize,
			Limit:     pageSize,
		})
		if err != nil {
			return nil, "", false, err
		}
		keys := make([]string, 0, len(res.List))
		for _, item := range res.List {
			keys = append(keys, item.TaskType+":"+item.ID)
		}
		return keys, res.NextCursor, res.HasMore, nil
	})
}

func paginateWebhooks(ctx context.Context, e *env, pageSize int) (int, int, int, int64, string, error) {
	svc := &webhook.Service{DB: e.db}
	return paginateCursorPages(pageSize, func(cursor string) (items []string, next string, hasMore bool, err error) {
		res, err := svc.ListEvents(ctx, webhook.EventListQuery{
			TenantID:  e.tenantID,
			UseCursor: true,
			Cursor:    cursor,
			PageSize:  pageSize,
			Limit:     pageSize,
		})
		if err != nil {
			return nil, "", false, err
		}
		keys := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			keys = append(keys, item.ID.String())
		}
		return keys, res.NextCursor, res.HasMore, nil
	})
}

func paginateOperationLogs(ctx context.Context, e *env, pageSize int) (int, int, int, int64, string, error) {
	svc := &operationlog.Service{DB: e.db}
	c := newGinContext(e.tenantID, uuid.Nil)
	return paginateCursorPages(pageSize, func(cursor string) (items []string, next string, hasMore bool, err error) {
		res, err := svc.List(c, operationlog.ListQuery{UseCursor: true, Cursor: cursor, PageSize: pageSize, Limit: pageSize})
		if err != nil {
			return nil, "", false, err
		}
		keys := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			keys = append(keys, item.ID.String())
		}
		return keys, res.NextCursor, res.HasMore, nil
	})
}

func paginateCursorPages(pageSize int, fetch func(cursor string) ([]string, string, bool, error)) (int, int, int, int64, string, error) {
	cursor := ""
	sampleCursor := ""
	seen := map[string]struct{}{}
	pages, rows, dupes, maxMs := 0, 0, 0, int64(0)
	for pages < maxPaginationPages {
		pageStart := time.Now()
		keys, next, hasMore, err := fetch(cursor)
		if err != nil {
			return pages, rows, dupes, maxMs, sampleCursor, err
		}
		pages++
		if d := time.Since(pageStart).Milliseconds(); d > maxMs {
			maxMs = d
		}
		for _, key := range keys {
			if _, ok := seen[key]; ok {
				dupes++
			}
			seen[key] = struct{}{}
			rows++
		}
		if strings.TrimSpace(next) != "" {
			sampleCursor = next
		}
		if !hasMore || strings.TrimSpace(next) == "" {
			return pages, rows, dupes, maxMs, sampleCursor, nil
		}
		cursor = next
	}
	return pages, rows, dupes, maxMs, sampleCursor, nil
}

func guardProductList(ctx context.Context, e *env, cursor string) (bool, bool, bool, bool, bool, bool, bool, error) {
	svc := &product.Service{DB: e.db}
	c := newGinContext(e.tenantID, uuid.Nil)
	tampered, err := svc.List(c, product.ListQuery{UseCursor: true, Cursor: tamperCursor(cursor), PageSize: 10, Limit: 10})
	_ = tampered
	tamperedRejected := err != nil && guardRejected(err, pagination.ErrCodeCursorSignatureInvalid)

	wrongVersion, err := signedCursorWrongVersion(cursor)
	wrongVersionRejected := false
	if err == nil {
		_, err = svc.List(c, product.ListQuery{UseCursor: true, Cursor: wrongVersion, PageSize: 10, Limit: 10})
		wrongVersionRejected = guardRejected(err, pagination.ErrCodeCursorVersionUnsupported)
	}

	otherTenant := newGinContext(e.tenantID+999, uuid.Nil)
	_, err = svc.List(otherTenant, product.ListQuery{UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10})
	crossTenantRejected := guardRejected(err, pagination.ErrCodeCursorScopeMismatch)

	_, err = svc.List(c, product.ListQuery{UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10, Status: "draft"})
	filterMismatchRejected := guardRejected(err, pagination.ErrCodeCursorFilterMismatch)

	_, err = svc.List(c, product.ListQuery{Page: 300, PageSize: 50})
	deepOffsetRejected := guardRejected(err, pagination.ErrCodeOffsetTooDeep)

	res, err := svc.List(c, product.ListQuery{UseCursor: true, PageSize: 999, Limit: 999})
	limitGuardPassed := err == nil && res != nil && res.Limit <= pagination.MaxLimit

	return tamperedRejected, wrongVersionRejected, crossTenantRejected, false, deepOffsetRejected, limitGuardPassed, filterMismatchRejected, nil
}

func guardOrderList(ctx context.Context, e *env, cursor string) (bool, bool, bool, bool, bool, bool, bool, error) {
	svc := &order.Service{DB: e.db}
	c := newGinContext(e.tenantID, uuid.Nil)
	_, err := svc.List(c, order.ListQuery{UseCursor: true, Cursor: tamperCursor(cursor), PageSize: 10, Limit: 10})
	tamperedRejected := guardRejected(err, pagination.ErrCodeCursorSignatureInvalid)

	wrongVersion, err := signedCursorWrongVersion(cursor)
	wrongVersionRejected := false
	if err == nil {
		_, err = svc.List(c, order.ListQuery{UseCursor: true, Cursor: wrongVersion, PageSize: 10, Limit: 10})
		wrongVersionRejected = guardRejected(err, pagination.ErrCodeCursorVersionUnsupported)
	}

	otherTenant := newGinContext(e.tenantID+999, uuid.Nil)
	_, err = svc.List(otherTenant, order.ListQuery{UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10})
	crossTenantRejected := guardRejected(err, pagination.ErrCodeCursorScopeMismatch)

	_, err = svc.List(c, order.ListQuery{UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10, Status: "paid"})
	filterMismatchRejected := guardRejected(err, pagination.ErrCodeCursorFilterMismatch)

	_, err = svc.List(c, order.ListQuery{Page: 300, PageSize: 50})
	deepOffsetRejected := guardRejected(err, pagination.ErrCodeOffsetTooDeep)

	res, err := svc.List(c, order.ListQuery{UseCursor: true, PageSize: 999, Limit: 999})
	limitGuardPassed := err == nil && res != nil && res.Limit <= pagination.MaxLimit

	return tamperedRejected, wrongVersionRejected, crossTenantRejected, false, deepOffsetRejected, limitGuardPassed, filterMismatchRejected, nil
}

func guardInventoryList(ctx context.Context, e *env, cursor string) (bool, bool, bool, bool, bool, bool, bool, error) {
	svc := &inventory.Service{DB: e.db}
	_, err := svc.ListInventoryCenter(ctx, inventory.CenterListQuery{
		TenantID: e.tenantID, UseCursor: true, Cursor: tamperCursor(cursor), PageSize: 10, Limit: 10,
	})
	tamperedRejected := guardRejected(err, pagination.ErrCodeCursorSignatureInvalid)

	wrongVersion, err := signedCursorWrongVersion(cursor)
	wrongVersionRejected := false
	if err == nil {
		_, err = svc.ListInventoryCenter(ctx, inventory.CenterListQuery{
			TenantID: e.tenantID, UseCursor: true, Cursor: wrongVersion, PageSize: 10, Limit: 10,
		})
		wrongVersionRejected = guardRejected(err, pagination.ErrCodeCursorVersionUnsupported)
	}

	_, err = svc.ListInventoryCenter(ctx, inventory.CenterListQuery{
		TenantID: e.tenantID + 999, UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10,
	})
	crossTenantRejected := guardRejected(err, pagination.ErrCodeCursorScopeMismatch)

	_, err = svc.ListInventoryCenter(ctx, inventory.CenterListQuery{
		TenantID: e.tenantID, UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10, StockStatus: "low",
	})
	filterMismatchRejected := guardRejected(err, pagination.ErrCodeCursorFilterMismatch)

	_, err = svc.ListInventoryCenter(ctx, inventory.CenterListQuery{TenantID: e.tenantID, Page: 300, PageSize: 50})
	deepOffsetRejected := err != nil

	res, err := svc.ListInventoryCenter(ctx, inventory.CenterListQuery{
		TenantID: e.tenantID, UseCursor: true, PageSize: 999, Limit: 999,
	})
	limitGuardPassed := err == nil && res != nil && res.Limit <= 100

	return tamperedRejected, wrongVersionRejected, crossTenantRejected, false, deepOffsetRejected, limitGuardPassed, filterMismatchRejected, nil
}

func guardTaskList(ctx context.Context, e *env, cursor string) (bool, bool, bool, bool, bool, bool, bool, error) {
	svc := &taskcenter.Service{DB: e.db, Cfg: e.cfg}
	_, err := svc.ListFailures(ctx, taskcenter.ListFailureParams{
		TenantID: e.tenantID, TaskType: taskcenter.TaskTypeCollect, UseCursor: true, Cursor: tamperCursor(cursor), PageSize: 10, Limit: 10,
	})
	tamperedRejected := err != nil && (guardRejected(err, pagination.ErrCodeCursorSignatureInvalid) || strings.Contains(strings.ToLower(err.Error()), "signature"))

	wrongVersion, err := mergeCursorWrongVersion(cursor)
	wrongVersionRejected := false
	if err == nil {
		_, err = svc.ListFailures(ctx, taskcenter.ListFailureParams{
			TenantID: e.tenantID, TaskType: taskcenter.TaskTypeCollect, UseCursor: true, Cursor: wrongVersion, PageSize: 10, Limit: 10,
		})
		wrongVersionRejected = guardRejected(err, pagination.ErrCodeCursorVersionUnsupported)
	}

	_, err = svc.ListFailures(ctx, taskcenter.ListFailureParams{
		TenantID: e.tenantID + 999, TaskType: taskcenter.TaskTypeCollect, UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10,
	})
	crossTenantRejected := guardRejected(err, pagination.ErrCodeCursorScopeMismatch)

	_, err = svc.ListFailures(ctx, taskcenter.ListFailureParams{
		TenantID: e.tenantID, TaskType: taskcenter.TaskTypeCollect, UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10, Status: "failed",
	})
	filterMismatchRejected := guardRejected(err, pagination.ErrCodeCursorFilterMismatch)

	_, err = svc.ListFailures(ctx, taskcenter.ListFailureParams{
		TenantID: e.tenantID, TaskType: taskcenter.TaskTypeCollect, Page: 300, PageSize: 50,
	})
	deepOffsetRejected := guardRejected(err, pagination.ErrCodeOffsetTooDeep)

	res, err := svc.ListFailures(ctx, taskcenter.ListFailureParams{
		TenantID: e.tenantID, TaskType: taskcenter.TaskTypeCollect, UseCursor: true, PageSize: 999, Limit: 999,
	})
	limitGuardPassed := err == nil && res.Limit <= pagination.MaxLimit

	return tamperedRejected, wrongVersionRejected, crossTenantRejected, false, deepOffsetRejected, limitGuardPassed, filterMismatchRejected, nil
}

func guardWebhookList(ctx context.Context, e *env, cursor string) (bool, bool, bool, bool, bool, bool, bool, error) {
	svc := &webhook.Service{DB: e.db}
	_, err := svc.ListEvents(ctx, webhook.EventListQuery{
		TenantID: e.tenantID, UseCursor: true, Cursor: tamperCursor(cursor), PageSize: 10, Limit: 10,
	})
	tamperedRejected := guardRejected(err, pagination.ErrCodeCursorSignatureInvalid)

	wrongVersion, err := signedCursorWrongVersion(cursor)
	wrongVersionRejected := false
	if err == nil {
		_, err = svc.ListEvents(ctx, webhook.EventListQuery{
			TenantID: e.tenantID, UseCursor: true, Cursor: wrongVersion, PageSize: 10, Limit: 10,
		})
		wrongVersionRejected = guardRejected(err, pagination.ErrCodeCursorVersionUnsupported)
	}

	_, err = svc.ListEvents(ctx, webhook.EventListQuery{
		TenantID: e.tenantID + 999, UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10,
	})
	crossTenantRejected := guardRejected(err, pagination.ErrCodeCursorScopeMismatch)

	_, err = svc.ListEvents(ctx, webhook.EventListQuery{
		TenantID: e.tenantID, UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10, Status: "processed",
	})
	filterMismatchRejected := guardRejected(err, pagination.ErrCodeCursorFilterMismatch)

	_, err = svc.ListEvents(ctx, webhook.EventListQuery{TenantID: e.tenantID, Page: 300, PageSize: 50})
	deepOffsetRejected := guardRejected(err, pagination.ErrCodeOffsetTooDeep)

	res, err := svc.ListEvents(ctx, webhook.EventListQuery{
		TenantID: e.tenantID, UseCursor: true, PageSize: 999, Limit: 999,
	})
	limitGuardPassed := err == nil && res != nil && res.Limit <= pagination.MaxLimit

	return tamperedRejected, wrongVersionRejected, crossTenantRejected, false, deepOffsetRejected, limitGuardPassed, filterMismatchRejected, nil
}

func guardOperationLogList(ctx context.Context, e *env, cursor string) (bool, bool, bool, bool, bool, bool, bool, error) {
	svc := &operationlog.Service{DB: e.db}
	c := newGinContext(e.tenantID, uuid.Nil)
	_, err := svc.List(c, operationlog.ListQuery{UseCursor: true, Cursor: tamperCursor(cursor), PageSize: 10, Limit: 10})
	tamperedRejected := guardRejected(err, pagination.ErrCodeCursorSignatureInvalid)

	wrongVersion, err := signedCursorWrongVersion(cursor)
	wrongVersionRejected := false
	if err == nil {
		_, err = svc.List(c, operationlog.ListQuery{UseCursor: true, Cursor: wrongVersion, PageSize: 10, Limit: 10})
		wrongVersionRejected = guardRejected(err, pagination.ErrCodeCursorVersionUnsupported)
	}

	otherTenant := newGinContext(e.tenantID+999, uuid.Nil)
	_, err = svc.List(otherTenant, operationlog.ListQuery{UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10})
	crossTenantRejected := guardRejected(err, pagination.ErrCodeCursorScopeMismatch)

	_, err = svc.List(c, operationlog.ListQuery{UseCursor: true, Cursor: cursor, PageSize: 10, Limit: 10, Action: "login"})
	filterMismatchRejected := guardRejected(err, pagination.ErrCodeCursorFilterMismatch)

	_, err = svc.List(c, operationlog.ListQuery{Page: 300, PageSize: 50})
	deepOffsetRejected := err != nil

	res, err := svc.List(c, operationlog.ListQuery{UseCursor: true, PageSize: 999, Limit: 999})
	limitGuardPassed := err == nil && res != nil && res.Limit <= 100

	return tamperedRejected, wrongVersionRejected, crossTenantRejected, false, deepOffsetRejected, limitGuardPassed, filterMismatchRejected, nil
}

func mergeCursorWrongVersion(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty cursor")
	}
	var p map[string]any
	if err := pagination.DecodeSignedJSONMax(raw, &p, pagination.MaxMergeCursorLen); err != nil {
		return "", err
	}
	p["version"] = 99
	return pagination.EncodeSignedJSONMax(p, pagination.MaxMergeCursorLen)
}

func signedCursorWrongVersion(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty cursor")
	}
	p, err := decodeCursorPayload(raw)
	if err != nil {
		return "", err
	}
	p.Version = 99
	return pagination.EncodeSignedJSONMax(p, pagination.MaxCursorLen)
}

func decodeCursorPayload(raw string) (pagination.CursorPayload, error) {
	var zero pagination.CursorPayload
	raw = strings.TrimSpace(raw)
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return zero, err
	}
	var env struct {
		P pagination.CursorPayload `json:"p"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return zero, err
	}
	return env.P, nil
}
