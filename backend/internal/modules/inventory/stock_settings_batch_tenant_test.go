package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestStockSettingsBatchHTTPEnforcesTenantAndPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_stock_batch_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &product.ProductSKU{}, &product.ProductPlatformPublishConfig{}, &productpublish.ProductPublication{}); err != nil {
		t.Fatal(err)
	}
	stock := 8
	makeSKU := func(tenantID int64, title string) product.ProductSKU {
		p := product.Product{TenantID: tenantID, Source: "test", Status: product.StatusDraft, Title: title}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		sku := product.ProductSKU{ProductID: p.ID, SKUCode: title, Stock: &stock, WarningStock: 1, SafetyStock: 1}
		if err := db.Create(&sku).Error; err != nil {
			t.Fatal(err)
		}
		return sku
	}
	aSKU := makeSKU(1, "tenant-a")
	bSKU := makeSKU(2, "tenant-b")
	zeroSKU := makeSKU(0, "tenant-zero")
	h := &Handler{Svc: &Service{DB: db}}

	call := func(path string, tenantID int64, principal *adminperm.Principal, body any, tenantSet bool) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if tenantSet {
				c.Set(ctxkey.TenantID, tenantID)
			}
			c.Set("adminperm.principal", principal)
		})
		r.POST(path, func(c *gin.Context) {
			if path == "/preview" {
				h.BatchPreviewStockSettings(c)
			} else {
				h.BatchUpdateStockSettings(c)
			}
		})
		payload, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload)))
		return w
	}
	viewer := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin}
	operator := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin}
	reviewer := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleReviewer}
	preview := map[string]any{"includeNormal": true, "pageSize": 100}
	if w := call("/preview", 1, viewer, preview, true); w.Code != http.StatusOK || bytes.Contains(w.Body.Bytes(), []byte(bSKU.ID.String())) || !bytes.Contains(w.Body.Bytes(), []byte(aSKU.ID.String())) {
		t.Fatalf("tenant preview leaked or omitted rows: code=%d body=%s", w.Code, w.Body.String())
	}
	if w := call("/preview", 1, reviewer, preview, true); w.Code != http.StatusForbidden {
		t.Fatalf("reviewer preview code=%d body=%s", w.Code, w.Body.String())
	}
	if w := call("/preview", 1, viewer, preview, false); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant preview code=%d body=%s", w.Code, w.Body.String())
	}
	update := map[string]any{"includeNormal": true, "confirm": true, "confirmAll": true, "warningStock": 4, "safetyStock": 2}
	if w := call("/update", 1, operator, update, true); w.Code != http.StatusOK {
		t.Fatalf("tenant update failed: code=%d body=%s", w.Code, w.Body.String())
	}
	var gotA, gotB product.ProductSKU
	if err := db.First(&gotA, "id = ?", aSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotB, "id = ?", bSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotA.WarningStock != 4 || gotB.WarningStock != 1 {
		t.Fatalf("cross-tenant mutation: A=%d B=%d", gotA.WarningStock, gotB.WarningStock)
	}
	mixed := map[string]any{"includeNormal": true, "productSkuIds": []string{aSKU.ID.String(), bSKU.ID.String()}, "confirm": true, "warningStock": 5, "safetyStock": 3}
	if w := call("/update", 1, operator, mixed, true); w.Code != http.StatusOK {
		t.Fatalf("mixed update code=%d body=%s", w.Code, w.Body.String())
	}
	if err := db.First(&gotB, "id = ?", bSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotB.WarningStock != 1 {
		t.Fatalf("foreign SKU changed through explicit id: %d", gotB.WarningStock)
	}
	zeroViewer := &adminperm.Principal{TenantID: 0, Role: adminperm.RoleAdmin}
	if w := call("/preview", 0, zeroViewer, preview, true); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(zeroSKU.ID.String())) || bytes.Contains(w.Body.Bytes(), []byte(aSKU.ID.String())) {
		t.Fatalf("tenant zero was not exact: code=%d body=%s", w.Code, w.Body.String())
	}
	// A scoped operator may read both granted stores, but a confirmAll write must
	// fail atomically when one shared target is view-only.
	sharedA := makeSKU(1, "shared-a")
	sharedB := makeSKU(1, "shared-b")
	storeA, storeB := uuid.New(), uuid.New()
	if err := db.Create(&product.ProductPlatformPublishConfig{ProductID: sharedA.ProductID, Platform: "a", ShopID: &storeA}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&productpublish.ProductPublication{TenantID: 1, ProductID: sharedB.ProductID, ShopID: storeB, Platform: "b", Status: "ok", PublishStatus: "ok"}).Error; err != nil {
		t.Fatal(err)
	}
	scoped := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleOperator, StoreGrants: []adminperm.StoreGrant{{StoreID: storeA, PermissionScope: "operate"}, {StoreID: storeB, PermissionScope: "view"}}}
	if w := call("/preview", 1, scoped, preview, true); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(sharedA.ID.String())) || !bytes.Contains(w.Body.Bytes(), []byte(sharedB.ID.String())) {
		t.Fatalf("scoped preview should retain view scope: code=%d body=%s", w.Code, w.Body.String())
	}
	if w := call("/update", 1, scoped, update, true); w.Code == http.StatusOK {
		t.Fatalf("confirmAll accepted view-only shared target: %s", w.Body.String())
	}
	var gotSharedA, gotSharedB product.ProductSKU
	if err := db.First(&gotSharedA, "id = ?", sharedA.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotSharedB, "id = ?", sharedB.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotSharedA.WarningStock != 1 || gotSharedB.WarningStock != 1 {
		t.Fatalf("unauthorized confirmAll changed rows: A=%d B=%d", gotSharedA.WarningStock, gotSharedB.WarningStock)
	}
}
