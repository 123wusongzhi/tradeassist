package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func TestProfileReturnsEffectiveTenantRole(t *testing.T) {
	h, db, _ := newAuthHandlerForBudget(t)
	user := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     42,
		Username:     admin.NewInternalUsername(),
		Email:        "legacy-tenant-admin@example.com",
		PasswordHash: "unused",
		Role:         adminperm.RoleAdmin,
		Status:       admin.StatusActive,
		TokenVersion: 1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
	c.Set(ctxkey.AdminID, user.ID.String())
	h.Profile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data struct {
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Role != adminperm.RoleTenantAdmin {
		t.Fatalf("role=%q, want %q", envelope.Data.Role, adminperm.RoleTenantAdmin)
	}
	for _, permission := range envelope.Data.Permissions {
		if permission == adminperm.PermSettingsManage || permission == adminperm.PermBackupRead {
			t.Fatalf("tenant profile exposed global permission %q", permission)
		}
	}
	if !containsPermission(envelope.Data.Permissions, adminperm.PermProductWrite) {
		t.Fatalf("tenant business permission missing: %v", envelope.Data.Permissions)
	}
}

func containsPermission(permissions []string, want string) bool {
	for _, permission := range permissions {
		if permission == want {
			return true
		}
	}
	return false
}
