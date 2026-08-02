package ordersync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func TestWriteRoutesFailClosedWithoutDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name    string
		handler *Handler
	}{
		{name: "nil service", handler: &Handler{}},
		{name: "nil database", handler: &Handler{Svc: &Service{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			Register(router.Group("/api/v1"), tc.handler)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/shops/test/sync-orders", nil))

			require.Equal(t, http.StatusInternalServerError, rec.Code)
			var envelope response.Envelope
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			require.Equal(t, response.CodeInternalError, envelope.Code)
			require.Equal(t, "order sync unavailable", envelope.Message)
		})
	}
}

func TestTaskReadRoutesRequireOrderView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:ordersync_read_guard?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	userID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: userID}, TenantID: 1, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReviewer, Status: admin.StatusActive}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(ctxkey.AdminID, userID.String()); c.Set(ctxkey.TenantID, int64(1)) })
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})
	for _, path := range []string{"/api/v1/order-sync/tasks", "/api/v1/order-sync/tasks/" + uuid.NewString()} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusForbidden, rec.Code, path)
	}
}
