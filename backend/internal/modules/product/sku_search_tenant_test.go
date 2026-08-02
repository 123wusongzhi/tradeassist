package product

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

type skuSearchPublication struct {
	model.Base
	ProductID uuid.UUID `gorm:"type:char(36);index;not null"`
	ShopID    uuid.UUID `gorm:"type:char(36);index;not null"`
}

func (skuSearchPublication) TableName() string { return "product_publications" }

func TestSearchSKUsUsesExactTenantAndProductScope(t *testing.T) {
	db := newImportDraftTenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&ProductPlatformPublishConfig{}, &skuSearchPublication{}))

	productA := Product{TenantID: 11, Source: "test", Title: "tenant-a-visible", Status: StatusDraft}
	hiddenA := Product{TenantID: 11, Source: "test", Title: "tenant-a-hidden", Status: StatusDraft}
	productB := Product{TenantID: 22, Source: "test", Title: "tenant-b", Status: StatusDraft}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&hiddenA).Error)
	require.NoError(t, db.Create(&productB).Error)
	stockA, stockHidden, stockB := 3, 4, 99
	require.NoError(t, db.Create(&ProductSKU{ProductID: productA.ID, SKUCode: "A-VISIBLE", Stock: &stockA}).Error)
	require.NoError(t, db.Create(&ProductSKU{ProductID: hiddenA.ID, SKUCode: "A-HIDDEN", Stock: &stockHidden}).Error)
	require.NoError(t, db.Create(&ProductSKU{ProductID: productB.ID, SKUCode: "B-SECRET", Stock: &stockB}).Error)

	svc := &Service{DB: db}
	tenantAdmin := tenantProductContext(t, 11)
	tenantAdmin.Set("adminperm.principal", &adminperm.Principal{
		TenantID: 11,
		Role:     adminperm.RoleTenantAdmin,
	})
	allA, err := svc.SearchSKUs(tenantAdmin, SearchSKUsQuery{Limit: 50})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"A-VISIBLE", "A-HIDDEN"}, skuCodes(allA))

	shopA := uuid.New()
	shopB := uuid.New()
	require.NoError(t, db.Create(&ProductPlatformPublishConfig{ProductID: productA.ID, Platform: "test-a", ShopID: &shopA}).Error)
	require.NoError(t, db.Create(&ProductPlatformPublishConfig{ProductID: productB.ID, Platform: "test-b", ShopID: &shopB}).Error)
	operator := tenantProductContext(t, 11)
	operator.Set(ctxkey.TenantID, int64(11))
	operator.Set("adminperm.principal", &adminperm.Principal{
		TenantID:    11,
		Role:        adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopA, PermissionScope: "view"}},
	})
	visible, err := svc.SearchSKUs(operator, SearchSKUsQuery{Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []string{"A-VISIBLE"}, skuCodes(visible))

	foreignID := productB.ID.String()
	foreign, err := svc.SearchSKUs(operator, SearchSKUsQuery{ProductID: &foreignID, Limit: 50})
	require.NoError(t, err)
	require.Empty(t, foreign)
}

func TestSearchSKUsRequiresTrustedTenant(t *testing.T) {
	db := newImportDraftTenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&ProductPlatformPublishConfig{}, &skuSearchPublication{}))
	c := testGinContext()
	c.Set(ctxkey.TenantID, int64(-1))
	_, err := (&Service{DB: db}).SearchSKUs(c, SearchSKUsQuery{})
	require.Error(t, err)
}

func skuCodes(hits []ProductSKUSearchHit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.SKUCode)
	}
	return out
}
