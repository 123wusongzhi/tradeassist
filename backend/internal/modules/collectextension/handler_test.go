package collectextension

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
)

func TestDeviceAuthRequiresCurrentProductWritePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openBrowserExtensionServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}))

	run := func(t *testing.T, role string, wantStatus int) {
		t.Helper()
		user := &admin.AdminUser{
			TenantID:     31,
			Username:     uuid.NewString(),
			PasswordHash: "not-used",
			Role:         role,
			Status:       admin.StatusActive,
			TokenVersion: 1,
		}
		require.NoError(t, db.Create(user).Error)
		token := "tmx_" + uuid.NewString() + uuid.NewString()
		device := &BrowserExtensionDevice{
			TenantID:    31,
			AdminUserID: user.ID,
			Name:        "Permission test",
			TokenHash:   authutil.HashToken(token, ""),
			Status:      DeviceStatusActive,
			ExpiresAt:   time.Now().UTC().Add(time.Hour),
		}
		require.NoError(t, db.Create(device).Error)

		handler := &Handler{Svc: &Service{DB: db}}
		router := gin.New()
		router.POST("/write", handler.DeviceAuth(), func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
		request := httptest.NewRequest(http.MethodPost, "/write", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, wantStatus, recorder.Code)
	}

	t.Run("readonly denied", func(t *testing.T) {
		run(t, adminperm.RoleReadonly, http.StatusForbidden)
	})
	t.Run("operator allowed", func(t *testing.T) {
		run(t, adminperm.RoleOperator, http.StatusNoContent)
	})
}
