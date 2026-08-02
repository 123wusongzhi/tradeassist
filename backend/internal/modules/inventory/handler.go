package inventory

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

// Handler serves inventory ledger + outbound sync admin APIs.
type Handler struct {
	Svc *Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func atoiQ(c *gin.Context, key string, def int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func mapInventoryEnqueueErr(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, platformp.ErrManualInventorySyncUnsupported):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return true
	case errors.Is(err, platformp.ErrInventorySyncNotImplemented):
		response.Fail(c, http.StatusNotImplemented, response.CodeBadRequest, err.Error())
		return true
	default:
		return false
	}
}

func parseBoolQuery(c *gin.Context, key string) bool {
	v := strings.TrimSpace(strings.ToLower(c.Query(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (h *Handler) requireInventoryWrite(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return false
	}
	if !adminperm.CanWriteInventory(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "只读账号不可执行库存写操作")
		return false
	}
	return true
}

func (h *Handler) requireInventoryView(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return false
	}
	if !adminperm.CanViewInventory(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "无库存查看权限")
		return false
	}
	return true
}

func requestTenant(c *gin.Context) (int64, error) { return adminperm.TenantIDFromGin(c) }

// requireTaskOperate performs the whole authorization pass before a retry can
// reset, claim, or enqueue any task. Product scope is checked as well as the
// target store: a task can otherwise mutate a shared product through a store
// grant that is narrower than the product's operating scope.
func (h *Handler) requireTaskOperate(c *gin.Context, tenantID int64, taskIDs []uuid.UUID) bool {
	if len(taskIDs) == 0 {
		return true
	}
	var tasks []InventorySyncTask
	if err := h.Svc.DB.WithContext(c.Request.Context()).Where("id IN ? AND tenant_id = ?", taskIDs, tenantID).Find(&tasks).Error; err != nil || len(tasks) != len(taskIDs) {
		response.Fail(c, 404, response.CodeNotFound, "资源不存在")
		return false
	}
	seenProducts := make(map[uuid.UUID]struct{}, len(tasks))
	for _, task := range tasks {
		if _, seen := seenProducts[task.ProductID]; seen {
			continue
		}
		if err := adminperm.EnsureProductOperate(c, h.Svc.DB, task.ProductID); err != nil {
			response.HandleError(c, err)
			return false
		}
		seenProducts[task.ProductID] = struct{}{}
	}
	for _, task := range tasks {
		if !adminperm.RequireStoreOperate(c, h.Svc.DB, task.ShopID) {
			return false
		}
	}
	return true
}

// AdjustStock POST /products/:id/skus/:skuId/adjust-stock
func (h *Handler) AdjustStock(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("skuId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid sku id")
		return
	}
	var body AdjustStockBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.AdjustSKUStock(c, pid, sid, body, adminUUID(c))
	if err != nil {
		if mapInventoryEnqueueErr(c, err) {
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ListSKULogs GET /products/:id/skus/:skuId/inventory-logs
func (h *Handler) ListSKULogs(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("skuId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid sku id")
		return
	}
	page := atoiQ(c, "page", 1)
	ps := atoiQ(c, "pageSize", 20)
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	res, err := h.Svc.ListSKUChangeLogs(c.Request.Context(), tid, pid, sid, page, ps)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListPublicationSkuRows GET /products/:id/publication-skus
func (h *Handler) ListPublicationSkuRows(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	var filter *uuid.UUID
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			filter = &u
		}
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	rows, err := h.Svc.ListPublicationSkus(c.Request.Context(), tid, pid, filter)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// SyncPublicationSku POST /product-publication-skus/:id/sync-inventory
func (h *Handler) SyncPublicationSku(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body PublicationSkuSyncBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreatePublicationSKUInventoryTask(c, id, body, adminUUID(c))
	if err != nil {
		if mapInventoryEnqueueErr(c, err) {
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// BatchSyncProduct POST /products/:id/sync-inventory
func (h *Handler) BatchSyncProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	var body ProductBatchInventoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	list, err := h.Svc.CreateProductShopInventoryTasks(c, pid, body, adminUUID(c))
	if err != nil {
		if mapInventoryEnqueueErr(c, err) {
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"list": list})
}

// ListGlobalLogs GET /inventory/logs
func (h *Handler) ListGlobalLogs(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	q := GlobalLogsQuery{
		Page:       atoiQ(c, "page", 1),
		PageSize:   atoiQ(c, "pageSize", 20),
		ChangeType: c.Query("changeType"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("orderId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.RefOrderID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	res, err := h.Svc.ListGlobalLogs(c.Request.Context(), tid, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListGlobalOrderEffects GET /inventory/effects
func (h *Handler) ListGlobalOrderEffects(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	q := OrderEffectsQuery{
		Page:       atoiQ(c, "page", 1),
		PageSize:   atoiQ(c, "pageSize", 20),
		EffectType: c.Query("effectType"),
		Status:     c.Query("status"),
	}
	if raw := strings.TrimSpace(c.Query("orderId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.OrderID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	res, err := h.Svc.ListOrderEffectsGlobal(c.Request.Context(), tid, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListCenter GET /inventory
func (h *Handler) ListCenter(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	q := CenterListQuery{
		Cursor:        strings.TrimSpace(c.Query("cursor")),
		Limit:         atoiQ(c, "limit", 0),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Platform:      strings.TrimSpace(c.Query("platform")),
		StockStatus:   strings.TrimSpace(c.Query("stockStatus")),
		AlertStatus:   strings.TrimSpace(c.Query("alertStatus")),
		SkuBindStatus: strings.TrimSpace(c.Query("skuBindStatus")),
		SyncStatus:    strings.TrimSpace(c.Query("syncStatus")),
		HasException:  parseBoolQuery(c, "hasException"),
		Page:          atoiQ(c, "page", 1),
		PageSize:      atoiQ(c, "pageSize", 20),
	}
	q.UseCursor = q.Cursor != "" || q.Limit > 0
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	q.TenantID = tid
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSkuID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
	}
	res, err := h.Svc.ListInventoryCenter(c.Request.Context(), q)
	if err != nil {
		if code := pagination.ErrorCode(err); code != "" {
			response.JSON(c, 400, response.CodeBadRequest, code, gin.H{"errorCode": code})
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"items":      res.Items,
		"nextCursor": res.NextCursor,
		"hasMore":    res.HasMore,
		"limit":      res.Limit,
		"list":       res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListAlerts GET /inventory/alerts
func (h *Handler) ListAlerts(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	q := AlertsListQuery{
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Platform:      strings.TrimSpace(c.Query("platform")),
		AlertType:     strings.TrimSpace(c.Query("alertType")),
		StockStatus:   strings.TrimSpace(c.Query("stockStatus")),
		OnlyPublished: parseBoolQuery(c, "onlyPublished"),
		IncludeNormal: parseBoolQuery(c, "includeNormal"),
		Page:          atoiQ(c, "page", 1),
		PageSize:      atoiQ(c, "pageSize", 20),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSkuID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	q.TenantID = tid
	res, err := h.Svc.ListInventoryAlertsTenant(c.Request.Context(), tid, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListTasks GET /inventory-sync/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	q := ListQuery{
		Page:     atoiQ(c, "page", 1),
		PageSize: atoiQ(c, "pageSize", 20),
		Status:   c.Query("status"),
		Platform: c.Query("platform"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("batchId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.BatchID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListTasks(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// GetTask GET /inventory-sync/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.GetDTO(c.Request.Context(), tid, id, uuid.Nil, "")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// RetryTask POST /inventory-sync/tasks/:id/retry
func (h *Handler) RetryTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if !h.requireTaskOperate(c, tid, []uuid.UUID{id}) {
		return
	}
	out, err := h.Svc.RetryFailed(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

func parseUUIDQueryPtr(c *gin.Context, key string) *uuid.UUID {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	u, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &u
}

// CreateInventorySyncBatch POST /inventory-sync/batches
func (h *Handler) CreateInventorySyncBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	var body CreateInventorySyncBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.CreateInventorySyncBatch(c.Request.Context(), tid, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ListInventorySyncBatches GET /inventory-sync/batches
func (h *Handler) ListInventorySyncBatches(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	q := InventorySyncBatchListQuery{
		Source:    strings.TrimSpace(strings.ToLower(c.Query("source"))),
		Status:    strings.TrimSpace(strings.ToLower(c.Query("status"))),
		Platform:  strings.TrimSpace(strings.ToLower(c.Query("platform"))),
		ShopID:    parseUUIDQueryPtr(c, "shopId"),
		ProductID: parseUUIDQueryPtr(c, "productId"),
		Page:      atoiQ(c, "page", 1),
		PageSize:  atoiQ(c, "pageSize", 20),
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	res, err := h.Svc.ListInventorySyncBatches(c.Request.Context(), tid, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"items": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// GetInventorySyncBatch GET /inventory-sync/batches/:id
func (h *Handler) GetInventorySyncBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	recent := atoiQ(c, "recentTasks", 15)
	if recent > 50 {
		recent = 50
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.GetInventorySyncBatch(c.Request.Context(), tid, id, recent)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// ListInventorySyncBatchTasks GET /inventory-sync/batches/:id/tasks
func (h *Handler) ListInventorySyncBatchTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid batch id")
		return
	}
	q := ListQuery{
		Page:         atoiQ(c, "page", 1),
		PageSize:     atoiQ(c, "pageSize", 20),
		Status:       c.Query("status"),
		Platform:     c.Query("platform"),
		ProductID:    parseUUIDQueryPtr(c, "productId"),
		ProductSKUID: parseUUIDQueryPtr(c, "productSkuId"),
		ShopID:       parseUUIDQueryPtr(c, "shopId"),
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListInventorySyncBatchTasks(c, id, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// RetryInventorySyncBatchFailed POST /inventory-sync/batches/:id/retry-failed
func (h *Handler) RetryInventorySyncBatchFailed(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	var taskIDs []uuid.UUID
	if err := h.Svc.DB.WithContext(c.Request.Context()).Model(&InventorySyncTask{}).Where("batch_id = ? AND tenant_id = ? AND status = ?", id, tid, StatusFailed).Pluck("id", &taskIDs).Error; err != nil || !h.requireTaskOperate(c, tid, taskIDs) {
		if err != nil {
			response.HandleError(c, err)
		}
		return
	}
	out, err := h.Svc.RetryInventorySyncBatchFailed(c.Request.Context(), tid, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// RetryInventorySyncTasksBatch POST /inventory-sync/batches/retry-failed-tasks
func (h *Handler) RetryInventorySyncTasksBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	var body RetryInventorySyncTasksBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if len(body.TaskIds) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "taskIds required")
		return
	}
	ids := make([]uuid.UUID, 0, len(body.TaskIds))
	for _, raw := range body.TaskIds {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid task id")
			return
		}
		ids = append(ids, u)
	}
	tid, err := requestTenant(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if !h.requireTaskOperate(c, tid, ids) {
		return
	}
	out, err := h.Svc.RetryInventorySyncTasksIntoBatch(c.Request.Context(), tid, ids, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// BatchPreviewStockSettings POST /inventory/stock-settings/batch-preview
func (h *Handler) BatchPreviewStockSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryView(c) {
		return
	}
	tenantID, err := requestTenant(c)
	if err != nil || tenantID < 0 {
		response.Fail(c, 401, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body StockSettingsBatchPreviewBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.PreviewStockSettingsBatch(c, tenantID, body)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// BatchUpdateStockSettings POST /inventory/stock-settings/batch-update
func (h *Handler) BatchUpdateStockSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := requestTenant(c)
	if err != nil || tenantID < 0 {
		response.Fail(c, 401, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body StockSettingsBatchUpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.BatchUpdateStockSettings(c, tenantID, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
