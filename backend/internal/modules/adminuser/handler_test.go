package adminuser

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

type adminUserEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newAdminUserHTTPRouter(t *testing.T) (*gin.Engine, *Service, int64) {
	t.Helper()
	svc, _, db := newAdminUserTestService(t)
	tenant := auth.Tenant{Name: "HTTP 测试租户"}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	actor := admin.AdminUser{
		TenantID:     0,
		Username:     admin.NewInternalUsername(),
		Email:        "global-admin@example.test",
		PasswordHash: "test-hash",
		Role:         adminperm.RoleAdmin,
		Status:       "active",
		TokenVersion: 1,
	}
	if err := db.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	api := router.Group("/api/v1")
	api.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Next()
	})
	Register(api, &Handler{Svc: svc})
	return router, svc, tenant.ID
}

func TestAdminUserHTTPSerializesTenantAndRejectsMissingTenant(t *testing.T) {
	router, _, tenantID := newAdminUserHTTPRouter(t)

	missingBody := []byte(`{"email":"missing@example.test","password":"SafePassphrase42!","role":"tenant_admin"}`)
	missingRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(missingBody))
	missingRequest.Header.Set("Content-Type", "application/json")
	missingResponse := httptest.NewRecorder()
	router.ServeHTTP(missingResponse, missingRequest)
	if missingResponse.Code != http.StatusBadRequest {
		t.Fatalf("missing tenant status=%d body=%s", missingResponse.Code, missingResponse.Body.String())
	}

	validBody, err := json.Marshal(map[string]any{
		"email":    "valid-tenant-admin@example.test",
		"password": "SafePassphrase42!",
		"role":     adminperm.RoleTenantAdmin,
		"tenantId": tenantID,
	})
	if err != nil {
		t.Fatal(err)
	}
	validRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(validBody))
	validRequest.Header.Set("Content-Type", "application/json")
	validResponse := httptest.NewRecorder()
	router.ServeHTTP(validResponse, validRequest)
	if validResponse.Code != http.StatusOK {
		t.Fatalf("valid tenant status=%d body=%s", validResponse.Code, validResponse.Body.String())
	}
	var envelope adminUserEnvelope
	if err := json.Unmarshal(validResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var row UserRow
	if err := json.Unmarshal(envelope.Data, &row); err != nil {
		t.Fatal(err)
	}
	if row.TenantID != tenantID || row.Role != adminperm.RoleTenantAdmin {
		t.Fatalf("serialized row=%+v", row)
	}
}

func TestAdminTenantOptionsHTTPIncludesLegacySources(t *testing.T) {
	router, _, tenantID := newAdminUserHTTPRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			List []TenantOption `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 || len(envelope.Data.List) != 1 || envelope.Data.List[0].ID != tenantID {
		t.Fatalf("tenant options envelope=%+v", envelope)
	}
}
