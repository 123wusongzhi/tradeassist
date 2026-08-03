package shop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestTenantAdminCannotMutateGlobalOzonAttributeMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:shop_mapping_permissions?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &PlatformCategoryAttributeMapping{}); err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	user := admin.AdminUser{
		Base:         model.Base{ID: userID},
		TenantID:     42,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         adminperm.RoleTenantAdmin,
		Status:       admin.StatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, userID.String())
		c.Set(ctxkey.TenantID, int64(42))
	})
	router.PUT("/platform/ozon/categories/:id/attribute-mappings", (&Handler{Svc: &Service{DB: db}}).PutOzonAttributeMappings)
	req := httptest.NewRequest(http.MethodPut, "/platform/ozon/categories/123/attribute-mappings", strings.NewReader(`{"items":[{"attributeId":"x","localField":"title","enabled":true}]}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
	}
	var count int64
	if err := db.Model(&PlatformCategoryAttributeMapping{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unauthorized mapping mutation count=%d err=%v", count, err)
	}
}

func TestUnknownRoleCannotReadOzonCategoryCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:shop_ozon_read_permissions?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &PlatformCategory{}); err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	user := admin.AdminUser{Base: model.Base{ID: userID}, TenantID: 42, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: "unknown", Status: admin.StatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, userID.String())
		c.Set(ctxkey.TenantID, int64(42))
	})
	router.GET("/platform/ozon/categories", (&Handler{Svc: &Service{DB: db}}).ListOzonCategories)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/platform/ozon/categories", nil))
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
	}
}
