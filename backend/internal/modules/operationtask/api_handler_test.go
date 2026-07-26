package operationtask_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func newOperationTaskAPITestRouter(t *testing.T) (*gin.Engine, *operationtask.APIService, int64, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	actorID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TraceID, "trace-api-handler-test")
		c.Next()
	})
	operationtask.Register(r.Group("/api/v1"), &operationtask.Handler{Svc: svc})
	return r, svc, tenantID, actorID.String()
}

func TestOperationTaskHandlerRequiresIdempotencyKeyForWrites(t *testing.T) {
	r, _, _, _ := newOperationTaskAPITestRouter(t)
	body := []byte(`{"sourceType":"manual","taskType":"product_content","platform":"douyin","title":"Missing key","payload":{"title":"safe"},"priority":"normal"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/operation-tasks", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, rec.Body.String(), "Idempotency-Key")
}

func TestOperationTaskHandlerListUsesCursorResponse(t *testing.T) {
	r, svc, tenantID, actorIDRaw := newOperationTaskAPITestRouter(t)
	actorID, err := uuid.Parse(actorIDRaw)
	require.NoError(t, err)
	actor := operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}
	for _, title := range []string{"first", "second"} {
		_, err := svc.CreateTask(context.Background(), actor, operationtask.CreateTaskRequest{
			SourceType: operationtask.OperationTaskSourceManual,
			TaskType:   operationtask.OperationTaskTypeProductContent,
			Platform:   operationtask.PlatformDouyin,
			Title:      title,
			Payload:    json.RawMessage(`{"title":"safe"}`),
			Priority:   operationtask.OperationTaskPriorityNormal,
		}, "req-"+title, "idem-list-"+title)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/operation-tasks?limit=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "nextCursor")
	require.Contains(t, rec.Body.String(), "hasMore")
	require.NotContains(t, rec.Body.String(), "offset")
}
