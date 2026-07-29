package inventorysyncp9

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type Handler struct {
	Svc *APIService
}

func (h *Handler) actor(c *gin.Context) (APIActor, error) {
	if h == nil || h.Svc == nil {
		return APIActor{}, ErrStateConflict
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return APIActor{}, ErrAuthenticationRequired
	}
	actorID, err := apiActorID(c)
	if err != nil {
		return APIActor{}, err
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || principal.Disabled {
		return APIActor{}, ErrAuthenticationRequired
	}
	return APIActor{TenantID: tenantID, ActorID: actorID, Role: principal.Role}, nil
}

func (h *Handler) requireActor(c *gin.Context) (APIActor, bool) {
	actor, err := h.actor(c)
	if err != nil {
		apiRespondError(c, err)
		return APIActor{}, false
	}
	return actor, true
}

func (h *Handler) CreateRun(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	idem, err := apiIdempotencyKeyHash(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	var req CreateInventorySyncRunRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.CreateRun(c.Request.Context(), actor, req, apiRequestID(c), idem)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, response.CodeOK, "ok", out)
}

func (h *Handler) ListRuns(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	limit, cursor, err := apiLimitCursor(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	shopID, err := apiOptionalUUIDQuery(c, "shopConnectionId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedRunStatuses[status] {
		apiRespondError(c, ErrValidation)
		return
	}
	mode := strings.TrimSpace(c.Query("providerMode"))
	if mode != "" && !allowedProviderModes[mode] {
		apiRespondError(c, ErrValidation)
		return
	}
	out, err := h.Svc.ListRuns(c.Request.Context(), actor, InventorySyncRunListParams{ShopConnectionID: shopID, Status: status, ProviderMode: mode, Limit: limit, Cursor: cursor})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GetRun(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	runID, err := apiUUIDParam(c, "runId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.GetRun(c.Request.Context(), actor, runID)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) Rerun(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	runID, err := apiUUIDParam(c, "runId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	idem, err := apiIdempotencyKeyHash(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	var req RerunInventorySyncRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.Rerun(c.Request.Context(), actor, runID, req, apiRequestID(c), idem)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListSnapshots(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	runID, err := apiUUIDParam(c, "runId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, cursor, err := apiLimitCursor(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	result := strings.TrimSpace(c.Query("bindingResult"))
	if result != "" && result != "matched" && result != "manual_review" && result != "unmatched" {
		apiRespondError(c, ErrValidation)
		return
	}
	out, err := h.Svc.ListSnapshots(c.Request.Context(), actor, runID, SnapshotListParams{BindingResult: result, Limit: limit, Cursor: cursor})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GetSnapshot(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	snapshotID, err := apiUUIDParam(c, "snapshotId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.GetSnapshot(c.Request.Context(), actor, snapshotID)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListBindings(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	limit, cursor, err := apiLimitCursor(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	shopID, err := apiOptionalUUIDQuery(c, "shopConnectionId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	status := strings.TrimSpace(c.Query("bindingStatus"))
	if status != "" && !allowedBindingStatuses[status] {
		apiRespondError(c, ErrValidation)
		return
	}
	source := strings.TrimSpace(c.Query("bindingSource"))
	if source != "" && source != SKUBindingSourceAutomatic && source != SKUBindingSourceManual {
		apiRespondError(c, ErrValidation)
		return
	}
	out, err := h.Svc.ListBindings(c.Request.Context(), actor, BindingListParams{ShopConnectionID: shopID, BindingStatus: status, BindingSource: source, Limit: limit, Cursor: cursor})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GetBinding(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	bindingID, err := apiUUIDParam(c, "bindingId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.GetBinding(c.Request.Context(), actor, bindingID)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GetBindingHistory(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	bindingID, err := apiUUIDParam(c, "bindingId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.GetBindingHistory(c.Request.Context(), actor, bindingID)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListCalibrations(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	snapshotID, err := apiUUIDParam(c, "snapshotId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, cursor, err := apiLimitCursor(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.ListCalibrations(c.Request.Context(), actor, snapshotID, CalibrationListParams{Limit: limit, Cursor: cursor})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) Recalibrate(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	snapshotID, err := apiUUIDParam(c, "snapshotId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	rawKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if !apiIdempotencyKeyPattern.MatchString(rawKey) {
		apiRespondError(c, ErrValidation)
		return
	}
	var req RecalibrateSnapshotRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.Recalibrate(c.Request.Context(), actor, snapshotID, req, apiRequestID(c), rawKey)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListManualRequests(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	limit, cursor, err := apiLimitCursor(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	shopID, err := apiOptionalUUIDQuery(c, "shopConnectionId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !allowedManualStatuses[status] {
		apiRespondError(c, ErrValidation)
		return
	}
	out, err := h.Svc.ListManualRequests(c.Request.Context(), actor, ManualBindingListParams{ShopConnectionID: shopID, Status: status, Limit: limit, Cursor: cursor})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GetManualRequest(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	requestID, err := apiUUIDParam(c, "requestId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.GetManualRequest(c.Request.Context(), actor, requestID)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ConfirmManual(c *gin.Context) {
	h.decideManual(c, true)
}

func (h *Handler) RejectManual(c *gin.Context) {
	h.decideManual(c, false)
}

func (h *Handler) decideManual(c *gin.Context, confirm bool) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	requestID, err := apiUUIDParam(c, "requestId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	idem, err := apiIdempotencyKeyHash(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	if confirm {
		var req ConfirmManualBindingRequest
		if err := apiBindJSON(c, &req); err != nil {
			apiRespondError(c, err)
			return
		}
		out, err := h.Svc.ConfirmManual(c.Request.Context(), actor, requestID, req, apiRequestID(c), idem)
		if err != nil {
			apiRespondError(c, err)
			return
		}
		response.OK(c, out)
		return
	}
	var req RejectManualBindingRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.RejectManual(c.Request.Context(), actor, requestID, req, apiRequestID(c), idem)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListRunAudit(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	runID, err := apiUUIDParam(c, "runId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, cursor, err := apiLimitCursor(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.ListRunAudit(c.Request.Context(), actor, runID, AuditEventListParams{Limit: limit, Cursor: cursor})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}
