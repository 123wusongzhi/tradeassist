package pricing

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func pricingTenantTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pricing_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&product.Product{}, &product.ProductSKU{}, &admin.AdminUser{}))
	return &Service{DB: db}
}

func pricingTenantContext(t *testing.T, svc *Service, tenantID int64) *gin.Context {
	t.Helper()
	u := &admin.AdminUser{TenantID: tenantID, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: "tenant_admin", Status: admin.StatusActive}
	require.NoError(t, svc.DB.Create(u).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Set(ctxkey.AdminID, u.ID.String())
	return c
}

func seedPricingTenantProduct(t *testing.T, svc *Service, tenantID int64, price, cost float64) (*product.Product, *product.ProductSKU) {
	t.Helper()
	p := &product.Product{TenantID: tenantID, Source: "test", Title: "pricing product", Currency: "CNY", Status: product.StatusReady}
	require.NoError(t, svc.DB.Create(p).Error)
	sku := &product.ProductSKU{ProductID: p.ID, SKUCode: uuid.NewString(), Price: &price, CostPrice: &cost}
	require.NoError(t, svc.DB.Create(sku).Error)
	return p, sku
}

func tenantPricingRule() Rule {
	return Rule{MarkupType: MarkupPercent, MarkupPercent: 20, RoundingMode: "none"}
}

func TestPricingRejectsCrossTenantCalculatePreviewAndApply(t *testing.T) {
	svc := pricingTenantTestService(t)
	c := pricingTenantContext(t, svc, 1)
	foreignProduct, foreignSKU := seedPricingTenantProduct(t, svc, 2, 100, 50)

	_, err := svc.Calculate(c, 1, CalculateBody{ProductSkuID: &foreignSKU.ID, Rule: tenantPricingRule()})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.PreviewProduct(c, 1, foreignProduct.ID, ProductApplyBody{Rule: tenantPricingRule()})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.ApplyProduct(c, 1, foreignProduct.ID, ProductApplyBody{Confirm: true, Rule: tenantPricingRule()}, nil)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var got product.ProductSKU
	require.NoError(t, svc.DB.First(&got, "id = ?", foreignSKU.ID).Error)
	require.Equal(t, 100.0, *got.Price)

	updated, err := svc.commitPlans(context.Background(), 1, []applyPlan{{skuID: foreignSKU.ID, productID: foreignProduct.ID, newPrice: 999}})
	require.NoError(t, err)
	require.Zero(t, updated, "final write must retain the tenant predicate")
	require.NoError(t, svc.DB.First(&got, "id = ?", foreignSKU.ID).Error)
	require.Equal(t, 100.0, *got.Price)
}

func TestPricingBatchConfirmAllOnlyTouchesCurrentTenant(t *testing.T) {
	svc := pricingTenantTestService(t)
	c := pricingTenantContext(t, svc, 1)
	_, localSKU := seedPricingTenantProduct(t, svc, 1, 100, 50)
	_, foreignSKU := seedPricingTenantProduct(t, svc, 2, 100, 50)

	summary, err := svc.BatchApply(c, 1, BatchApplyBody{Confirm: true, ConfirmAll: true, Rule: tenantPricingRule()}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, summary.ProductCount)
	require.Equal(t, 1, summary.Updated)

	var local, foreign product.ProductSKU
	require.NoError(t, svc.DB.First(&local, "id = ?", localSKU.ID).Error)
	require.NoError(t, svc.DB.First(&foreign, "id = ?", foreignSKU.ID).Error)
	require.Equal(t, 60.0, *local.Price)
	require.Equal(t, 100.0, *foreign.Price)
}

func TestPricingBatchExplicitMixedIDsReturnsOnlyCurrentTenant(t *testing.T) {
	svc := pricingTenantTestService(t)
	c := pricingTenantContext(t, svc, 1)
	localProduct, localSKU := seedPricingTenantProduct(t, svc, 1, 100, 50)
	foreignProduct, foreignSKU := seedPricingTenantProduct(t, svc, 2, 100, 50)

	productSummary, err := svc.ApplyProduct(c, 1, localProduct.ID, ProductApplyBody{
		SkuIDs:  []uuid.UUID{localSKU.ID, foreignSKU.ID},
		Confirm: true,
		Rule:    tenantPricingRule(),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, productSummary.Updated)

	summary, err := svc.BatchPreview(c, 1, BatchApplyBody{
		ProductIDs: []uuid.UUID{localProduct.ID, foreignProduct.ID},
		Rule:       tenantPricingRule(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.ProductCount)
	require.Len(t, summary.Preview, 1)
	require.Equal(t, localSKU.ID.String(), summary.Preview[0].ProductSkuID)

	summary, err = svc.BatchApply(c, 1, BatchApplyBody{
		ProductIDs: []uuid.UUID{localProduct.ID, foreignProduct.ID},
		Confirm:    true,
		Rule:       tenantPricingRule(),
	}, nil)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.Nil(t, summary)
	var foreign product.ProductSKU
	require.NoError(t, svc.DB.First(&foreign, "id = ?", foreignSKU.ID).Error)
	require.Equal(t, 100.0, *foreign.Price)
}
