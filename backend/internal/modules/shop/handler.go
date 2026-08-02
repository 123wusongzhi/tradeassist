package shop

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

// Handler serves shop + platform provider metadata routes.
type Handler struct {
	Svc *Service
}

// requireShopOperate applies tenant isolation before the existing store-write RBAC guard.
func (h *Handler) requireShopOperate(c *gin.Context, id uuid.UUID) bool {
	if _, err := h.Svc.tenantShop(c, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		} else {
			response.HandleError(c, err)
		}
		return false
	}
	return adminperm.RequireStoreOperate(c, h.Svc.DB, id)
}

func (h *Handler) requireConfigManage(c *gin.Context) bool {
	return adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermConfigManage)
}

// requireGlobalConfigManage protects instance-wide platform settings, which are
// persisted under tenant 0 and must not disclose their presence to tenant users.
func (h *Handler) requireGlobalConfigManage(c *gin.Context) bool {
	if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
		return false
	}
	return adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermConfigManage)
}

func (h *Handler) requireShopOperatePath(c *gin.Context) bool {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return false
	}
	return h.requireShopOperate(c, id)
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

// ListProviders GET /api/v1/platform/providers
func (h *Handler) ListProviders(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	list := h.Svc.ListPlatformProviders()
	response.OK(c, gin.H{"list": list})
}

// GetPlatformAppSettings GET /api/v1/platform/settings/:platform
func (h *Handler) GetPlatformAppSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireGlobalConfigManage(c) {
		return
	}
	plat := strings.TrimSpace(c.Param("platform"))
	out, err := h.Svc.GetPlatformAppSettings(c.Request.Context(), plat)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

type platformSettingsPutReq struct {
	Values map[string]interface{} `json:"values" binding:"required"`
}

// PutPlatformAppSettings PUT /api/v1/platform/settings/:platform
func (h *Handler) PutPlatformAppSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireGlobalConfigManage(c) {
		return
	}
	plat := strings.TrimSpace(c.Param("platform"))
	var body platformSettingsPutReq
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.PutPlatformAppSettings(c, plat, body.Values)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// TestPlatformAppSettings POST /api/v1/platform/settings/:platform/test-connection
func (h *Handler) TestPlatformAppSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireGlobalConfigManage(c) {
		return
	}
	plat := strings.TrimSpace(c.Param("platform"))
	out, err := h.Svc.TestPlatformAppSettings(c, plat)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// GetPlatformPublishSettings GET /api/v1/platform/publish-settings/:platform
func (h *Handler) GetPlatformPublishSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireGlobalConfigManage(c) {
		return
	}
	plat := strings.TrimSpace(c.Param("platform"))
	out, err := h.Svc.GetPlatformPublishSettings(c.Request.Context(), plat)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// PutPlatformPublishSettings PUT /api/v1/platform/publish-settings/:platform
func (h *Handler) PutPlatformPublishSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireGlobalConfigManage(c) {
		return
	}
	plat := strings.TrimSpace(c.Param("platform"))
	var body platformSettingsPutReq
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.PutPlatformPublishSettings(c, plat, body.Values)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// List GET /api/v1/shops
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	q := ListQuery{
		Page:       atoiQ(c, "page", 1),
		PageSize:   atoiQ(c, "pageSize", 20),
		Platform:   c.Query("platform"),
		Status:     c.Query("status"),
		AuthStatus: c.Query("authStatus"),
		ShopName:   c.Query("shopName"),
	}
	res, err := h.Svc.List(c, q)
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

// Create POST /api/v1/shops
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	// A new shop has no existing store grant to evaluate. Creating it is
	// therefore controlled by the store-operate capability itself.
	if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermStoreOperate) {
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	// A scoped operator may be allowed to create the first shop without having
	// a pre-existing grant for its not-yet-created ID. The new row is already
	// the complete create response (there is no auth token at this point), so
	// do not re-enter a grant-gated detail lookup here.
	response.OK(c, &ShopDetailDTO{Shop: *row})
}

// Get GET /api/v1/shops/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.GetDetail(c, id)
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

// Update PUT /api/v1/shops/:id
func (h *Handler) Update(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if _, err := h.Svc.Update(c, id, body, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.GetDetail(c, id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// Delete DELETE /api/v1/shops/:id
func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	// Resolve the exact tenant shop before any delete side effect, then apply
	// the store-level operate grant (including its 404 anti-enumeration rule).
	if !h.requireShopOperate(c, id) {
		return
	}
	if err := h.Svc.Delete(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// PutAuth PUT /api/v1/shops/:id/auth
func (h *Handler) PutAuth(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	var body UpdateAuthBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.UpdateAuth(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"auth": out})
}

// TestConnection POST /api/v1/shops/:id/test-connection
func (h *Handler) TestConnection(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	res, err := h.Svc.TestConnection(c, id, adminUUID(c))
	if err != nil {
		if errors.Is(err, platformp.ErrNotImplemented) {
			response.Fail(c, http.StatusNotImplemented, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

func failDouyin(c *gin.Context, err error) {
	var ce *DouyinCategoryError
	if errors.As(err, &ce) {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, ce.Message, gin.H{"errorCode": ce.Code})
		return
	}
	var de *DouyinAuthError
	if errors.As(err, &de) {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, de.Message, gin.H{"errorCode": de.Code})
		return
	}
	response.Fail(c, 400, response.CodeBadRequest, err.Error())
}

// ListDouyinCategories GET /api/v1/platform/douyin/categories
func (h *Handler) ListDouyinCategories(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	var shopID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
		shopID = &u
	}
	out, err := h.Svc.ListDouyinCategories(c, DouyinCategoryListQuery{
		Keyword:  c.Query("keyword"),
		ParentID: c.Query("parentId"),
		OnlyLeaf: queryBoolShop(c, "onlyLeaf"),
		Refresh:  queryBoolShop(c, "refresh"),
		ShopID:   shopID,
	}, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// SyncDouyinCategories POST /api/v1/platform/douyin/categories/sync
func (h *Handler) SyncDouyinCategories(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	var body struct {
		ShopID string `json:"shopId"`
	}
	if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&body)
	}
	sid, err := uuid.Parse(strings.TrimSpace(firstNonEmptyDouyin(body.ShopID, c.Query("shopId"))))
	if err != nil || sid == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
		return
	}
	out, err := h.Svc.SyncDouyinCategories(c, sid, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// ListDouyinCategoryAttributes GET /api/v1/platform/douyin/categories/:categoryId/attributes
func (h *Handler) ListDouyinCategoryAttributes(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	out, err := h.Svc.ListDouyinCategoryAttributes(c.Request.Context(), strings.TrimSpace(c.Param("categoryId")))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}

// SyncDouyinCategoryAttributes POST /api/v1/platform/douyin/categories/:categoryId/attributes/sync
func (h *Handler) SyncDouyinCategoryAttributes(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	var body struct {
		ShopID string `json:"shopId"`
	}
	if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&body)
	}
	sid, err := uuid.Parse(strings.TrimSpace(firstNonEmptyDouyin(body.ShopID, c.Query("shopId"))))
	if err != nil || sid == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
		return
	}
	out, err := h.Svc.SyncDouyinCategoryAttributes(c, sid, strings.TrimSpace(c.Param("categoryId")), adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}

func failOzon(c *gin.Context, err error) {
	var ce *OzonCategoryError
	if errors.As(err, &ce) {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, ce.Message, gin.H{"errorCode": ce.Code})
		return
	}
	response.Fail(c, 400, response.CodeBadRequest, err.Error())
}

// ListOzonCategories GET /api/v1/platform/ozon/categories
func (h *Handler) ListOzonCategories(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	out, err := h.Svc.ListOzonCategories(c.Request.Context(), OzonCategoryListQuery{
		Keyword:  c.Query("keyword"),
		OnlyLeaf: queryBoolShop(c, "onlyLeaf"),
		Limit:    atoiQ(c, "limit", 500),
	})
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, out)
}

// SyncOzonCategories POST /api/v1/platform/ozon/categories/sync
func (h *Handler) SyncOzonCategories(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	var body struct {
		ShopID string `json:"shopId"`
	}
	if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&body)
	}
	raw := strings.TrimSpace(body.ShopID)
	if raw == "" {
		raw = strings.TrimSpace(c.Query("shopId"))
	}
	sid := uuid.Nil
	if raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
		sid = u
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.SyncOzonCategories(c.Request.Context(), tenantID, sid)
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, out)
}

// OzonCategoryStats GET /api/v1/platform/ozon/categories/stats
func (h *Handler) OzonCategoryStats(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	out, err := h.Svc.OzonCategoryStats(c.Request.Context())
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, out)
}

func ozonCategoryIDParam(c *gin.Context) (uuid.UUID, bool) {
	u, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || u == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid category id")
		return uuid.Nil, false
	}
	return u, true
}

// ListOzonCategoryAttributes GET /api/v1/platform/ozon/categories/:id/attributes
func (h *Handler) ListOzonCategoryAttributes(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, ok := ozonCategoryIDParam(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListOzonCategoryAttributes(c.Request.Context(), id)
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}

// SyncOzonCategoryAttributes POST /api/v1/platform/ozon/categories/:id/attributes/sync
func (h *Handler) SyncOzonCategoryAttributes(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, ok := ozonCategoryIDParam(c)
	if !ok {
		return
	}
	var body struct {
		ShopID string `json:"shopId"`
	}
	if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&body)
	}
	raw := strings.TrimSpace(body.ShopID)
	if raw == "" {
		raw = strings.TrimSpace(c.Query("shopId"))
	}
	sid := uuid.Nil
	if raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
		sid = u
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.SyncOzonCategoryAttributes(c.Request.Context(), tenantID, id, sid)
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, out)
}

// GetOzonAttributeMappings GET /api/v1/platform/ozon/categories/:id/attribute-mappings
func (h *Handler) GetOzonAttributeMappings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	id, ok := ozonCategoryIDParam(c)
	if !ok {
		return
	}
	out, err := h.Svc.GetOzonAttributeMappings(c.Request.Context(), id)
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}

// PutOzonAttributeMappings PUT /api/v1/platform/ozon/categories/:id/attribute-mappings
func (h *Handler) PutOzonAttributeMappings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	// Attribute mappings are instance-wide publishing configuration. Restrict
	// their mutation to the global configuration administrator.
	if !h.requireConfigManage(c) {
		return
	}
	id, ok := ozonCategoryIDParam(c)
	if !ok {
		return
	}
	var body PutOzonAttributeMappingsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.PutOzonAttributeMappings(c.Request.Context(), id, body)
	if err != nil {
		failOzon(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}

// DouyinCategoryStats GET /api/v1/platform/douyin/categories/stats
func (h *Handler) DouyinCategoryStats(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	out, err := h.Svc.DouyinCategoryStats(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func queryBoolShop(c *gin.Context, key string) bool {
	v := strings.TrimSpace(strings.ToLower(c.Query(key)))
	return v == "1" || v == "true" || v == "yes"
}

// DouyinOAuthStart GET /api/v1/shops/oauth/douyin/start
func (h *Handler) DouyinOAuthStart(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	var sid *uuid.UUID
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
		sid = &u
		if !h.requireShopOperate(c, u) {
			return
		}
	}
	out, err := h.Svc.DouyinOAuthStart(c, sid, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// DouyinOAuthAuthorizeURL GET /api/v1/shops/:id/oauth/douyin/authorize-url
func (h *Handler) DouyinOAuthAuthorizeURL(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.DouyinOAuthStart(c, &id, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// DouyinOAuthCallback GET /api/v1/shops/oauth/douyin/callback
func (h *Handler) DouyinOAuthCallback(c *gin.Context) {
	if h == nil || h.Svc == nil {
		c.Redirect(http.StatusFound, "/settings/platforms?platform=douyin_shop&auth=failed&reason=UNKNOWN_DOUYIN_AUTH_ERROR")
		return
	}
	h.Svc.DouyinOAuthCallbackRedirect(c)
}

// DouyinOAuthRefresh POST /api/v1/shops/:id/oauth/douyin/refresh
func (h *Handler) DouyinOAuthRefresh(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.DouyinOAuthRefresh(c, id, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// DouyinOAuthRevoke POST /api/v1/shops/:id/oauth/douyin/revoke
func (h *Handler) DouyinOAuthRevoke(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.DouyinOAuthRevoke(c, id, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// DouyinOAuthTest POST /api/v1/shops/:id/oauth/douyin/test
func (h *Handler) DouyinOAuthTest(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.DouyinOAuthTest(c, id, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// DouyinSyncShopInfo POST /api/v1/shops/:id/oauth/douyin/sync-shop-info
func (h *Handler) DouyinSyncShopInfo(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.DouyinSyncShopInfo(c, id, adminUUID(c))
	if err != nil {
		failDouyin(c, err)
		return
	}
	response.OK(c, out)
}

// TikTokOAuthAuthorizeURL GET /api/v1/shops/:id/oauth/tiktok/authorize-url
func (h *Handler) TikTokOAuthAuthorizeURL(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	redirect := strings.TrimSpace(c.Query("redirectUri"))
	out, err := h.Svc.TikTokOAuthAuthorizeURL(c, id, redirect, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// TikTokOAuthCallback POST /api/v1/shops/:id/oauth/tiktok/callback
func (h *Handler) TikTokOAuthCallback(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body TikTokOAuthCallbackBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.TikTokOAuthCallback(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ShopeeOAuthAuthorizeURL GET /api/v1/shops/:id/oauth/shopee/authorize-url
func (h *Handler) ShopeeOAuthAuthorizeURL(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	redirect := strings.TrimSpace(c.Query("redirectUri"))
	out, err := h.Svc.ShopeeOAuthAuthorizeURL(c, id, redirect, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ShopeeOAuthCallback POST /api/v1/shops/:id/oauth/shopee/callback
func (h *Handler) ShopeeOAuthCallback(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body ShopeeOAuthCallbackBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.ShopeeOAuthCallback(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// LazadaOAuthAuthorizeURL GET /api/v1/shops/:id/oauth/lazada/authorize-url
func (h *Handler) LazadaOAuthAuthorizeURL(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	redirect := strings.TrimSpace(c.Query("redirectUri"))
	out, err := h.Svc.LazadaOAuthAuthorizeURL(c, id, redirect, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// LazadaOAuthCallback POST /api/v1/shops/:id/oauth/lazada/callback
func (h *Handler) LazadaOAuthCallback(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body LazadaOAuthCallbackBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.LazadaOAuthCallback(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// AmazonOAuthAuthorizeURL GET /api/v1/shops/:id/oauth/amazon/authorize-url
func (h *Handler) AmazonOAuthAuthorizeURL(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	redirect := strings.TrimSpace(c.Query("redirectUri"))
	out, err := h.Svc.AmazonOAuthAuthorizeURL(c, id, redirect, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// AmazonOAuthCallback POST /api/v1/shops/:id/oauth/amazon/callback
func (h *Handler) AmazonOAuthCallback(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
		return
	}
	if !h.requireShopOperatePath(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body AmazonOAuthCallbackBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.AmazonOAuthCallback(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
