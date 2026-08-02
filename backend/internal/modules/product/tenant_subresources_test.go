package product

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func tenantProductContext(t *testing.T, tenantID int64) *gin.Context {
	t.Helper()
	c := testGinContext()
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func tenantProductFixture(t *testing.T, tenantID int64) (*Service, *Product) {
	t.Helper()
	db := newImportDraftTenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	product := &Product{TenantID: tenantID, Source: "test", Title: "product", Status: StatusDraft}
	require.NoError(t, db.Create(product).Error)
	return &Service{DB: db}, product
}

func tenantProductAdminContext(t *testing.T, svc *Service, tenantID int64) *gin.Context {
	t.Helper()
	u := &admin.AdminUser{TenantID: tenantID, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: "tenant_admin", Status: admin.StatusActive}
	require.NoError(t, svc.DB.Create(u).Error)
	c := tenantProductContext(t, tenantID)
	c.Set(ctxkey.AdminID, u.ID.String())
	c.Set("adminperm.principal", &adminperm.Principal{UserID: u.ID, TenantID: tenantID, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)})
	return c
}

func TestProductSubresourcesRejectCrossTenantParent(t *testing.T) {
	svc, product := tenantProductFixture(t, 2)
	sku := &ProductSKU{ProductID: product.ID, SKUName: "sku"}
	image := &ProductImage{ProductID: product.ID, ImageType: ImageTypeMain, OriginURL: "https://img.alicdn.com/a.jpg", PublicURL: "https://img.alicdn.com/a.jpg"}
	require.NoError(t, svc.DB.Create(sku).Error)
	require.NoError(t, svc.DB.Create(image).Error)
	c := tenantProductAdminContext(t, svc, 1)

	_, err := svc.CreateSKU(c, product.ID, SKUBody{SKUName: "forbidden"}, nil)
	require.Error(t, err)
	_, err = svc.UpdateSKU(c, product.ID, sku.ID, SKUUpdateBody{}, nil)
	require.Error(t, err)
	require.Error(t, svc.DeleteSKU(c, product.ID, sku.ID, nil))
	_, err = svc.UpdateProductImage(c, product.ID, image.ID, ImageUpdateBody{}, nil)
	require.Error(t, err)
	require.Error(t, svc.DeleteProductImage(c, product.ID, image.ID, nil))
	require.Error(t, svc.ReorderProductImages(c, product.ID, ImageReorderBody{ImageIDs: []uuid.UUID{image.ID}}, nil))
	_, err = svc.SyncImages(c, product.ID, SyncImagesBody{}, nil, &files.Service{})
	require.Error(t, err)

	var remainingSKU ProductSKU
	var remainingImage ProductImage
	require.NoError(t, svc.DB.First(&remainingSKU, "id = ?", sku.ID).Error)
	require.NoError(t, svc.DB.First(&remainingImage, "id = ?", image.ID).Error)
}

func TestCreateProductImageRejectsCrossTenantFile(t *testing.T) {
	svc, product := tenantProductFixture(t, 1)
	require.NoError(t, svc.DB.AutoMigrate(&files.FileRecord{}))
	foreignFile := &files.FileRecord{TenantID: 2, ObjectKey: "foreign.png", PublicURL: "https://cdn.example.test/foreign.png"}
	require.NoError(t, svc.DB.Create(foreignFile).Error)

	_, err := svc.CreateProductImage(tenantProductContext(t, 1), product.ID, ImageCreateBody{
		ImageType: ImageTypeMain,
		FileID:    &foreignFile.ID,
	}, nil)
	require.Error(t, err)
}

func TestProductImageObjectKeyRequiresCleanTenantFile(t *testing.T) {
	svc, product := tenantProductFixture(t, 1)
	require.NoError(t, svc.DB.AutoMigrate(&files.FileRecord{}))
	foreign := &files.FileRecord{TenantID: 2, ObjectKey: "other/secret.png", PublicURL: "https://cdn.example.test/other/secret.png", SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean}
	scanning := &files.FileRecord{TenantID: 1, ObjectKey: "mine/scanning.png", PublicURL: "https://cdn.example.test/mine/scanning.png", SecurityStatus: files.SecurityScanning, ScanStatus: files.SecurityScanning}
	clean := &files.FileRecord{TenantID: 1, ObjectKey: "mine/clean.png", PublicURL: "https://cdn.example.test/mine/clean.png", SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean}
	require.NoError(t, svc.DB.Create(foreign).Error)
	require.NoError(t, svc.DB.Create(scanning).Error)
	require.NoError(t, svc.DB.Create(clean).Error)
	c := tenantProductAdminContext(t, svc, 1)
	for _, key := range []string{foreign.ObjectKey, scanning.ObjectKey} {
		_, err := svc.CreateProductImage(c, product.ID, ImageCreateBody{ImageType: ImageTypeMain, ObjectKey: key, PublicURL: "https://example.test/image.png"}, nil)
		require.Error(t, err, key)
	}
	created, err := svc.CreateProductImage(c, product.ID, ImageCreateBody{ImageType: ImageTypeMain, StorageKey: clean.ObjectKey}, nil)
	require.NoError(t, err)
	require.Equal(t, clean.ObjectKey, created.ObjectKey)
	require.Equal(t, clean.ObjectKey, created.StorageKey)
	require.Equal(t, clean.PublicURL, created.PublicURL)

	_, err = svc.UpdateProductImage(c, product.ID, created.ID, ImageUpdateBody{ObjectKey: &scanning.ObjectKey}, nil)
	require.Error(t, err)
	updated, err := svc.UpdateProductImage(c, product.ID, created.ID, ImageUpdateBody{ObjectKey: &clean.ObjectKey}, nil)
	require.NoError(t, err)
	require.Equal(t, clean.ObjectKey, updated.ObjectKey)
}

func TestReadonlyProductWritesAreDeniedBeforeMutation(t *testing.T) {
	svc, product := tenantProductFixture(t, 1)
	h := &Handler{Svc: svc, Files: &files.Service{}}

	for _, call := range []func(*gin.Context){h.Put, h.PostSKU, h.SyncImages} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/products/"+product.ID.String(), bytes.NewBufferString(`{}`))
		c.Params = gin.Params{{Key: "id", Value: product.ID.String()}}
		c.Set(ctxkey.TenantID, int64(1))
		c.Set("adminperm.principal", &adminperm.Principal{Role: adminperm.RoleReadonly, Permissions: adminperm.PermissionsForRole(adminperm.RoleReadonly)})
		call(c)
		require.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestProductAIHTTPRejectsForeignTenantBeforeProviderOrMutation(t *testing.T) {
	svc, foreign := tenantProductFixture(t, 2)
	require.NoError(t, svc.DB.AutoMigrate(&aitask.AITask{}, &ProductAIContentApplication{}))
	h := &Handler{Svc: svc}
	principal := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)}

	call := func(method, path, body string, fn func(*gin.Context)) {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		c.Params = gin.Params{{Key: "id", Value: foreign.ID.String()}}
		c.Set(ctxkey.TenantID, int64(1))
		c.Set("adminperm.principal", principal)
		fn(c)
		require.Equal(t, http.StatusNotFound, w.Code)
	}

	call(http.MethodPost, "/products/"+foreign.ID.String()+"/ai/optimize-title", `{}`, h.OptimizeTitle)
	call(http.MethodPost, "/products/"+foreign.ID.String()+"/ai/generate-description", `{}`, h.GenerateDescription)
	call(http.MethodGet, "/products/"+foreign.ID.String()+"/ai/tasks", "", h.ListAITasks)
	call(http.MethodPost, "/products/"+foreign.ID.String()+"/apply-ai-title", `{"aiTitle":"x","taskId":"`+uuid.NewString()+`"}`, h.ApplyAITitle)
	call(http.MethodPost, "/products/"+foreign.ID.String()+"/apply-ai-description", `{"aiDescription":"x","taskId":"`+uuid.NewString()+`"}`, h.ApplyAIDescription)
	call(http.MethodPost, "/products/"+foreign.ID.String()+"/undo-ai-title", `{}`, h.UndoAITitle)

	var taskCount, applicationCount int64
	require.NoError(t, svc.DB.Table("ai_tasks").Count(&taskCount).Error)
	require.NoError(t, svc.DB.Table("product_ai_content_applications").Count(&applicationCount).Error)
	require.Zero(t, taskCount)
	require.Zero(t, applicationCount)

}

func TestDouyinProductSubresourcesRejectForeignTenantBeforeSideEffects(t *testing.T) {
	svc, foreign := tenantProductFixture(t, 2)
	h := &Handler{Svc: svc, Files: &files.Service{}}
	principal := &adminperm.Principal{TenantID: 1, Role: adminperm.RoleTenantAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleTenantAdmin)}

	call := func(method, body string, fn func(*gin.Context)) {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/products/"+foreign.ID.String(), bytes.NewBufferString(body))
		c.Params = gin.Params{{Key: "id", Value: foreign.ID.String()}, {Key: "platform", Value: "douyin_shop"}, {Key: "imageKey", Value: "main-0"}}
		c.Set(ctxkey.TenantID, int64(1))
		c.Set("adminperm.principal", principal)
		fn(c)
		require.Equal(t, http.StatusNotFound, w.Code)
	}

	call(http.MethodGet, "", h.GetOperationProgress)
	call(http.MethodGet, "", h.GetPlatformPublishConfig)
	call(http.MethodPut, `{}`, h.PutPlatformPublishConfig)
	call(http.MethodPost, `{}`, h.BuildDouyinDraftMapping)
	call(http.MethodGet, "", h.GetDouyinDraftMapping)
	call(http.MethodPut, `{}`, h.PutDouyinDraftMapping)
	call(http.MethodPost, `{}`, h.ValidateDouyinDraftMapping)
	call(http.MethodPost, `{}`, h.UploadDouyinImages)
	call(http.MethodPost, `{}`, h.RetryDouyinImage)
	call(http.MethodGet, "", h.GetDouyinImageStatus)
}
