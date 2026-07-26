package operationtask

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type Handler struct {
	Svc *APIService
}

func (h *Handler) actor(c *gin.Context) (APIActor, error) {
	var actor APIActor
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		return actor, ErrConflict
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID <= 0 {
		return actor, ErrPermissionDenied
	}
	actorID, err := apiActorID(c)
	if err != nil {
		return actor, err
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || principal.Disabled {
		return actor, ErrPermissionDenied
	}
	return APIActor{TenantID: tenantID, ActorID: actorID, Role: principal.Role}, nil
}

func (h *Handler) CreateTask(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	key, err := apiIdempotencyKey(c)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	var req CreateTaskRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.CreateTask(c.Request.Context(), actor, req, apiRequestID(c), key)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.JSON(c, 201, response.CodeOK, "ok", out)
}

func (h *Handler) ListTasks(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	limit, err := parseLimit(c.DefaultQuery("limit", "50"))
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, err = apiValidateLimitCursor(limit, c.Query("cursor"))
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.ListTasks(c.Request.Context(), actor, OperationTaskListParams{Status: strings.TrimSpace(c.Query("status")), Platform: strings.TrimSpace(c.Query("platform")), TaskType: strings.TrimSpace(c.Query("taskType")), Limit: limit, Cursor: strings.TrimSpace(c.Query("cursor"))})
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GetTask(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	taskID, err := apiUUIDParam(c, "taskId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.GetTask(c.Request.Context(), actor, taskID)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) CancelTask(c *gin.Context) {
	actor, taskID, key, ok := h.requireWrite(c)
	if !ok {
		return
	}
	var req CancelTaskRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.Cancel(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) CreateDraft(c *gin.Context) {
	actor, taskID, key, ok := h.requireWrite(c)
	if !ok {
		return
	}
	var req CreateDraftRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.CreateInitialDraft(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) EditLatestDraft(c *gin.Context) {
	actor, taskID, key, ok := h.requireWrite(c)
	if !ok {
		return
	}
	var req EditDraftRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.EditLatestDraft(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListDrafts(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	taskID, err := apiUUIDParam(c, "taskId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, err := parseLimit(c.DefaultQuery("limit", "50"))
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.ListDrafts(c.Request.Context(), actor, taskID, limit)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) Approve(c *gin.Context) {
	h.decide(c, true)
}

func (h *Handler) Reject(c *gin.Context) {
	h.decide(c, false)
}

func (h *Handler) decide(c *gin.Context, approve bool) {
	actor, taskID, key, ok := h.requireWrite(c)
	if !ok {
		return
	}
	var req ApprovalRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	var out *ApprovalResponse
	var err error
	if approve {
		out, err = h.Svc.Approve(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	} else {
		out, err = h.Svc.Reject(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	}
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) Execute(c *gin.Context) {
	actor, taskID, key, ok := h.requireWrite(c)
	if !ok {
		return
	}
	var req ExecuteRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.Execute(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) Retry(c *gin.Context) {
	actor, taskID, key, ok := h.requireWrite(c)
	if !ok {
		return
	}
	var req RetryRequest
	if err := apiBindJSON(c, &req); err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.RetryExecution(c.Request.Context(), actor, taskID, req, apiRequestID(c), key)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListAttempts(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	taskID, err := apiUUIDParam(c, "taskId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, err := parseLimit(c.DefaultQuery("limit", "50"))
	if err != nil {
		apiRespondError(c, err)
		return
	}
	out, err := h.Svc.ListAttempts(c.Request.Context(), actor, taskID, limit, strings.TrimSpace(c.Query("cursor")))
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListEvents(c *gin.Context) {
	actor, ok := h.requireActor(c)
	if !ok {
		return
	}
	taskID, err := apiUUIDParam(c, "taskId")
	if err != nil {
		apiRespondError(c, err)
		return
	}
	limit, err := parseLimit(c.DefaultQuery("limit", "50"))
	if err != nil {
		apiRespondError(c, err)
		return
	}
	after := 0
	if raw := strings.TrimSpace(c.Query("afterSequence")); raw != "" {
		after, err = strconv.Atoi(raw)
		if err != nil || after < 0 {
			apiRespondError(c, ErrValidation)
			return
		}
	}
	out, err := h.Svc.ListEvents(c.Request.Context(), actor, taskID, limit, after)
	if err != nil {
		apiRespondError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) requireActor(c *gin.Context) (APIActor, bool) {
	actor, err := h.actor(c)
	if err != nil {
		apiRespondError(c, err)
		return APIActor{}, false
	}
	return actor, true
}

func (h *Handler) requireWrite(c *gin.Context) (APIActor, uuid.UUID, string, bool) {
	actor, ok := h.requireActor(c)
	if !ok {
		return APIActor{}, uuid.Nil, "", false
	}
	taskID, err := apiUUIDParam(c, "taskId")
	if err != nil {
		apiRespondError(c, err)
		return APIActor{}, uuid.Nil, "", false
	}
	key, err := apiIdempotencyKey(c)
	if err != nil {
		apiRespondError(c, err)
		return APIActor{}, uuid.Nil, "", false
	}
	return actor, taskID, key, true
}

func parseLimit(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return 0, ErrValidation
	}
	return n, nil
}
