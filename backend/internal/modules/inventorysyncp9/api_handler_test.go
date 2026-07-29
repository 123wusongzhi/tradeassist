package inventorysyncp9

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func newInventorySyncAPITestRouter(t *testing.T, role string, tenantID int64) (*gin.Engine, *APIService, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &idempotency.Record{}))
	shopID, _, _ := seedShopAndSKU(t, db, tenantID)
	actorID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: role, Status: admin.StatusActive}).Error)
	svc := NewAPIService(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TraceID, "trace-p9-api-test")
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: svc})
	return r, svc, actorID, shopID.ID
}

func p9Request(t *testing.T, method, path string, body any, idem string) *http.Request {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	return req
}

func TestInventorySyncAPIRejectsUnknownFieldsAndCredentials(t *testing.T) {
	r, _, _, shopID := newInventorySyncAPITestRouter(t, admin.RoleAdmin, 101)
	req := p9Request(t, http.MethodPost, "/api/v1/inventory-sync/runs", map[string]any{
		"shopConnectionId": shopID, "platform": PlatformDouyin, "providerMode": ProviderModeMock,
		"accessToken": "should-never-be-accepted",
	}, "p9-unknown-field")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, rec.Body.String(), "should-never-be-accepted")
}

func TestInventorySyncAPICreateReplayConflictAndSafeDTO(t *testing.T) {
	r, svc, _, shopID := newInventorySyncAPITestRouter(t, admin.RoleAdmin, 101)
	payload := map[string]any{"shopConnectionId": shopID, "platform": PlatformDouyin, "providerMode": ProviderModeMock, "fixtureScenario": FixtureScenarioSuccessSinglePage}
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, p9Request(t, http.MethodPost, "/api/v1/inventory-sync/runs", payload, "p9-create-replay"))
		require.Equal(t, http.StatusCreated, rec.Code)
		require.NotContains(t, rec.Body.String(), "cursorAfter")
		require.NotContains(t, rec.Body.String(), "idempotencyKeyHash")
	}
	var count int64
	require.NoError(t, svc.DB.Model(&InventorySyncRun{}).Where("tenant_id = ?", 101).Count(&count).Error)
	require.Equal(t, int64(1), count)
	conflict := payload
	conflict["fixtureScenario"] = FixtureScenarioEmptyInventory
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, p9Request(t, http.MethodPost, "/api/v1/inventory-sync/runs", conflict, "p9-create-replay"))
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestInventorySyncAPIKeysetAndTenantIsolation(t *testing.T) {
	r, svc, actorID, shopID := newInventorySyncAPITestRouter(t, admin.RoleAdmin, 101)
	for idx, scenario := range []string{FixtureScenarioSuccessSinglePage, FixtureScenarioEmptyInventory} {
		_, err := svc.CreateRun(context.Background(), APIActor{TenantID: 101, ActorID: actorID, Role: admin.RoleAdmin}, CreateInventorySyncRunRequest{ShopConnectionID: shopID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: scenario}, "req-"+string(rune('a'+idx)), sha256Hex("p9-list-"+string(rune('a'+idx))))
		require.NoError(t, err)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/inventory-sync/runs?limit=1", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "nextCursor")
	require.Contains(t, rec.Body.String(), "hasMore")
	require.NotContains(t, rec.Body.String(), "offset")
	require.NotContains(t, rec.Body.String(), "checkpoint")

	otherRouter, _, _, _ := newInventorySyncAPITestRouter(t, admin.RoleAdmin, 202)
	var run InventorySyncRun
	require.NoError(t, svc.DB.Where("tenant_id = ?", 101).First(&run).Error)
	rec = httptest.NewRecorder()
	otherRouter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/inventory-sync/runs/"+run.ID.String(), nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestInventorySyncAPIRoleAndProductionCaps(t *testing.T) {
	readonly, _, _, shopID := newInventorySyncAPITestRouter(t, adminperm.RoleReadonly, 301)
	rec := httptest.NewRecorder()
	readonly.ServeHTTP(rec, p9Request(t, http.MethodPost, "/api/v1/inventory-sync/runs", map[string]any{"shopConnectionId": shopID, "platform": PlatformDouyin, "providerMode": ProviderModeMock}, "p9-readonly-run"))
	require.Equal(t, http.StatusForbidden, rec.Code)

	adminRouter, _, _, adminShopID := newInventorySyncAPITestRouter(t, admin.RoleAdmin, 302)
	rec = httptest.NewRecorder()
	adminRouter.ServeHTTP(rec, p9Request(t, http.MethodPost, "/api/v1/inventory-sync/runs", map[string]any{"shopConnectionId": adminShopID, "platform": PlatformDouyin, "providerMode": "prod"}, "p9-prod-run"))
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestInventorySyncAPINoMutationRoutesRegistered(t *testing.T) {
	r, _, _, _ := newInventorySyncAPITestRouter(t, admin.RoleAdmin, 401)
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/inventory-sync/snapshots/"+uuid.New().String(), nil)
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	}
}

func TestInventorySyncAPIAuthRequired(t *testing.T) {
	db := newTestDB(t)
	r := gin.New()
	Register(r.Group("/api/v1"), &Handler{Svc: NewAPIService(db)})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/inventory-sync/runs", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.True(t, strings.Contains(rec.Body.String(), "authentication_required"))
}
