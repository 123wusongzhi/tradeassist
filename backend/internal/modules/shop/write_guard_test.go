package shop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

func TestShopWriteGuardsRespectRoleAndStoreScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	platformp.Bootstrap()
	db, err := gorm.Open(sqlite.Open("file:shop_write_guards?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &Shop{}))

	operatorID := uuid.New()
	for _, user := range []admin.AdminUser{
		{Base: model.Base{ID: operatorID}, TenantID: 7, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleOperator, Status: admin.StatusActive},
		{Base: model.Base{ID: uuid.New()}, TenantID: 7, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive},
		{Base: model.Base{ID: uuid.New()}, TenantID: 7, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReviewer, Status: admin.StatusActive},
	} {
		require.NoError(t, db.Create(&user).Error)
	}
	shopA := Shop{Base: model.Base{ID: uuid.New()}, TenantID: 7, Platform: "manual", ShopName: "A", Status: StatusActive, AuthStatus: AuthAuthorized}
	shopB := Shop{Base: model.Base{ID: uuid.New()}, TenantID: 7, Platform: "manual", ShopName: "B", Status: StatusActive, AuthStatus: AuthAuthorized}
	foreign := Shop{Base: model.Base{ID: uuid.New()}, TenantID: 8, Platform: "manual", ShopName: "foreign", Status: StatusActive, AuthStatus: AuthAuthorized}
	require.NoError(t, db.Create(&shopA).Error)
	require.NoError(t, db.Create(&shopB).Error)
	require.NoError(t, db.Create(&foreign).Error)
	require.NoError(t, db.Create(&admin.UserStorePermission{UserID: operatorID, StoreID: shopA.ID, PermissionScope: admin.StorePermScopeOperate}).Error)

	h := &Handler{Svc: &Service{DB: db}}
	request := func(userID uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(ctxkey.AdminID, userID.String())
			c.Set(ctxkey.TenantID, int64(7))
		})
		r.POST("/shops", h.Create)
		r.DELETE("/shops/:id", h.Delete)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}

	// Creating the first shop needs store.operate but cannot require a grant for
	// an ID that does not yet exist.
	createRec := request(operatorID, http.MethodPost, "/shops", `{"platform":"manual","shopName":"new"}`)
	require.Equalf(t, http.StatusOK, createRec.Code, "body=%s", createRec.Body.String())
	var created int64
	require.NoError(t, db.Model(&Shop{}).Where("tenant_id = ? AND shop_name = ?", 7, "new").Count(&created).Error)
	require.Equal(t, int64(1), created)

	var readonlyID, reviewerID uuid.UUID
	var users []admin.AdminUser
	require.NoError(t, db.Where("tenant_id = ?", 7).Find(&users).Error)
	for _, user := range users {
		switch user.Role {
		case adminperm.RoleReadonly:
			readonlyID = user.ID
		case adminperm.RoleReviewer:
			reviewerID = user.ID
		}
	}
	require.Equal(t, http.StatusForbidden, request(readonlyID, http.MethodPost, "/shops", `{"platform":"manual","shopName":"denied"}`).Code)
	require.Equal(t, http.StatusForbidden, request(reviewerID, http.MethodPost, "/shops", `{"platform":"manual","shopName":"denied"}`).Code)

	// The operator can delete only the explicitly operated shop. The inaccessible
	// and cross-tenant IDs reject before the soft delete executes.
	require.Equal(t, http.StatusOK, request(operatorID, http.MethodDelete, "/shops/"+shopA.ID.String(), "").Code)
	require.Equal(t, http.StatusNotFound, request(operatorID, http.MethodDelete, "/shops/"+shopB.ID.String(), "").Code)
	require.Equal(t, http.StatusNotFound, request(operatorID, http.MethodDelete, "/shops/"+foreign.ID.String(), "").Code)
	var remaining int64
	require.NoError(t, db.Unscoped().Model(&Shop{}).Where("id = ?", shopB.ID).Count(&remaining).Error)
	require.Equal(t, int64(1), remaining)
}
