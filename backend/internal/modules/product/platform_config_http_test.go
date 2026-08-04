package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestPutOzonPlatformConfigCreatesThenUpdatesStableRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:ozon_config_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&Product{},
		&ProductImage{},
		&ProductSKU{},
		&ProductPlatformPublishConfig{},
		&shop.Shop{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE product_publications (product_id text, shop_id text, deleted_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	productRow := Product{TenantID: 1, Source: "test", Title: "Ozon product", Status: StatusDraft}
	if err := db.Create(&productRow).Error; err != nil {
		t.Fatal(err)
	}
	shops := []shop.Shop{
		{TenantID: 1, Platform: "ozon", ShopName: "Ozon A", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized},
		{TenantID: 1, Platform: "ozon", ShopName: "Ozon B", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized},
	}
	if err := db.Create(&shops).Error; err != nil {
		t.Fatal(err)
	}
	categories := []shop.PlatformCategory{
		{Platform: "ozon", CategoryID: "100:200", Name: "桌子", Level: 2, IsLeaf: true, Status: "active"},
		{Platform: "ozon", CategoryID: "101:201", Name: "椅子", Level: 2, IsLeaf: true, Status: "active"},
	}
	if err := db.Create(&categories).Error; err != nil {
		t.Fatal(err)
	}
	attributes := []shop.PlatformCategoryAttribute{
		{Platform: "ozon", CategoryID: "100:200", AttrID: "85", Name: "品牌", Required: true, ValueType: "string", Raw: datatypes.JSON([]byte(`{"dictionary_id":0}`))},
		{Platform: "ozon", CategoryID: "101:201", AttrID: "86", Name: "材质", Required: true, ValueType: "string", Raw: datatypes.JSON([]byte(`{"dictionary_id":0}`))},
	}
	if err := db.Create(&attributes).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(1))
		c.Set(ctxkey.TraceID, "trace-ozon-config")
		c.Set("adminperm.principal", &adminperm.Principal{
			TenantID:    1,
			Role:        adminperm.RoleTenantAdmin,
			Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin),
		})
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	put := func(payload string) PlatformPublishConfigDTO {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productRow.ID.String()+"/platform-configs/ozon", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var body struct {
			Code    int                      `json:"code"`
			Data    PlatformPublishConfigDTO `json:"data"`
			TraceID string                   `json:"traceId"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != response.CodeOK || body.TraceID != "trace-ozon-config" || body.Data.ID == nil || *body.Data.ID == uuid.Nil {
			t.Fatalf("unexpected response: %+v", body)
		}
		return body.Data
	}

	first := put(fmt.Sprintf(`{"shopId":%q,"categoryId":"100:200","categoryPath":"untrusted","platformAttributes":{"85":{"value":"Acme"}},"sourceCategoryKey":"desk","sourceCategoryName":"桌子"}`, shops[0].ID.String()))
	second := put(fmt.Sprintf(`{"shopId":%q,"categoryId":"101:201","categoryPath":"also-untrusted","platformAttributes":{"86":{"value":"Wood"}},"sourceCategoryKey":"chair","sourceCategoryName":"椅子"}`, shops[1].ID.String()))
	if *first.ID != *second.ID {
		t.Fatalf("upsert returned a different id: first=%s second=%s", first.ID.String(), second.ID.String())
	}
	if second.ShopID == nil || *second.ShopID != shops[1].ID || second.CategoryID != "101:201" || second.CategoryPath != "椅子" {
		t.Fatalf("updated response did not contain persisted values: %+v", second)
	}
	var attrs map[string]map[string]any
	if err := json.Unmarshal(second.PlatformAttributes, &attrs); err != nil || attrs["86"]["value"] != "Wood" {
		t.Fatalf("updated attributes=%s err=%v", second.PlatformAttributes, err)
	}
	var rows []ProductPlatformPublishConfig
	if err := db.Where("product_id = ? AND platform = ?", productRow.ID, "ozon").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != *first.ID || rows[0].ShopID == nil || *rows[0].ShopID != shops[1].ID || rows[0].CategoryID != "101:201" || rows[0].SourceCategoryKey != "chair" {
		t.Fatalf("unexpected persisted row: %+v", rows)
	}

	validUpdate := fmt.Sprintf(`{"shopId":%q,"categoryId":"101:201","platformAttributes":{"86":{"value":"Wood"}},"sourceCategoryKey":"chair","sourceCategoryName":"椅子"}`, shops[1].ID.String())
	tests := []struct {
		name       string
		tenantID   int64
		principal  *adminperm.Principal
		wantStatus int
		wantCode   int
		wantText   string
	}{
		{
			name:       "global admin cross tenant is read only",
			tenantID:   1,
			principal:  &adminperm.Principal{TenantID: 0, Role: adminperm.RoleAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleAdmin)},
			wantStatus: http.StatusForbidden,
			wantCode:   response.CodePermissionDenied,
			wantText:   "全局管理员仅可跨租户查看",
		},
		{
			name:     "operator with operate grant can update",
			tenantID: 1,
			principal: &adminperm.Principal{
				TenantID:    1,
				Role:        adminperm.RoleOperator,
				Permissions: adminperm.PermissionsForRole(adminperm.RoleOperator),
				StoreGrants: []adminperm.StoreGrant{{StoreID: shops[1].ID, PermissionScope: "operate"}},
			},
			wantStatus: http.StatusOK,
			wantCode:   response.CodeOK,
		},
		{
			name:     "operator with view grant is forbidden",
			tenantID: 1,
			principal: &adminperm.Principal{
				TenantID:    1,
				Role:        adminperm.RoleOperator,
				Permissions: adminperm.PermissionsForRole(adminperm.RoleOperator),
				StoreGrants: []adminperm.StoreGrant{{StoreID: shops[1].ID, PermissionScope: "view"}},
			},
			wantStatus: http.StatusForbidden,
			wantCode:   response.CodePermissionDenied,
			wantText:   "仅有查看权限",
		},
		{
			name:       "operator without visibility gets not found",
			tenantID:   1,
			principal:  &adminperm.Principal{TenantID: 1, Role: adminperm.RoleOperator, Permissions: adminperm.PermissionsForRole(adminperm.RoleOperator)},
			wantStatus: http.StatusNotFound,
			wantCode:   response.CodeNotFound,
			wantText:   "not found",
		},
		{
			name:     "cross tenant operator gets not found",
			tenantID: 2,
			principal: &adminperm.Principal{
				TenantID:    2,
				Role:        adminperm.RoleOperator,
				Permissions: adminperm.PermissionsForRole(adminperm.RoleOperator),
				StoreGrants: []adminperm.StoreGrant{{StoreID: shops[1].ID, PermissionScope: "operate"}},
			},
			wantStatus: http.StatusNotFound,
			wantCode:   response.CodeNotFound,
			wantText:   "not found",
		},
		{
			name:       "cross tenant tenant-admin gets not found",
			tenantID:   2,
			principal:  &adminperm.Principal{TenantID: 2, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)},
			wantStatus: http.StatusNotFound,
			wantCode:   response.CodeNotFound,
			wantText:   "not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matrixRouter := gin.New()
			matrixRouter.Use(func(c *gin.Context) {
				c.Set(ctxkey.TenantID, tt.tenantID)
				c.Set(ctxkey.TraceID, "trace-ozon-config-auth")
				c.Set("adminperm.principal", tt.principal)
				c.Next()
			})
			Register(matrixRouter.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})
			req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productRow.ID.String()+"/platform-configs/ozon", bytes.NewBufferString(validUpdate))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			matrixRouter.ServeHTTP(w, req)
			var body struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				TraceID string `json:"traceId"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode status=%d body=%s: %v", w.Code, w.Body.String(), err)
			}
			if w.Code != tt.wantStatus || body.Code != tt.wantCode || body.TraceID != "trace-ozon-config-auth" {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tt.wantText != "" && !strings.Contains(body.Message, tt.wantText) {
				t.Fatalf("message=%q want substring %q", body.Message, tt.wantText)
			}
		})
	}
}
