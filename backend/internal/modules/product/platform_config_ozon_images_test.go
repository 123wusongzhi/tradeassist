package product

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
)

func TestOzonPlatformConfigPersistsAndReloadsSKUImageSelections(t *testing.T) {
	svc, productRow := tenantProductFixture(t, 1)
	require.NoError(t, svc.DB.AutoMigrate(
		&ProductPlatformPublishConfig{},
		&shop.Shop{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
	))
	shopRow := shop.Shop{TenantID: 1, Platform: "ozon", ShopName: "Ozon test", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, svc.DB.Create(&shopRow).Error)
	require.NoError(t, svc.DB.Create(&shop.PlatformCategory{Platform: "ozon", CategoryID: "100:200", Name: "桌子", IsLeaf: true, Status: "active"}).Error)
	require.NoError(t, svc.DB.Create(&shop.PlatformCategoryAttribute{Platform: "ozon", CategoryID: "100:200", AttrID: "optional", Name: "可选属性", Required: false, Raw: datatypes.JSON([]byte(`{}`))}).Error)

	originalSKU := ProductSKU{ProductID: productRow.ID, SKUCode: "RED", SKUName: "红色", ImageURL: "https://img.example/red.jpg"}
	fallbackSKU := ProductSKU{ProductID: productRow.ID, SKUCode: "BLUE", SKUName: "蓝色"}
	shared := ProductImage{ProductID: productRow.ID, ImageType: ImageTypeMain, PublicURL: "https://img.example/shared.jpg", SortOrder: 1}
	detail := ProductImage{ProductID: productRow.ID, ImageType: ImageTypeDetail, PublicURL: "https://img.example/detail.jpg", SortOrder: 2}
	require.NoError(t, svc.DB.Create(&originalSKU).Error)
	require.NoError(t, svc.DB.Create(&fallbackSKU).Error)
	require.NoError(t, svc.DB.Create(&shared).Error)
	require.NoError(t, svc.DB.Create(&detail).Error)

	c := tenantProductAdminContext(t, svc, 1)
	saved, err := svc.PutPlatformPublishConfig(c, productRow.ID, "ozon", PlatformPublishConfigBody{
		ShopID:             shopRow.ID.String(),
		CategoryID:         "100:200",
		PlatformAttributes: json.RawMessage(`{}`),
		OzonImages: &OzonImageConfigInput{Version: OzonImageConfigVersion, SKUSelections: []OzonSKUImageSelection{
			{SKUID: originalSKU.ID, AdditionalImageIDs: []uuid.UUID{detail.ID}},
			{SKUID: fallbackSKU.ID, FallbackMainImageID: &shared.ID, AdditionalImageIDs: []uuid.UUID{detail.ID}},
		}},
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, saved.OzonImages)
	require.True(t, saved.OzonImages.Configured)
	require.Zero(t, saved.OzonImages.ErrorCount)

	var persisted ProductPlatformPublishConfig
	require.NoError(t, svc.DB.Where("product_id = ? AND platform = ?", productRow.ID, "ozon").First(&persisted).Error)
	require.Contains(t, string(persisted.MappedImages), fallbackSKU.ID.String())
	require.Contains(t, string(persisted.MappedImages), shared.ID.String())

	// An older client that only resaves category/attribute fields must not erase
	// the newer SKU image selection.
	_, err = svc.PutPlatformPublishConfig(c, productRow.ID, "ozon", PlatformPublishConfigBody{
		ShopID:             shopRow.ID.String(),
		CategoryID:         "100:200",
		PlatformAttributes: json.RawMessage(`{}`),
	}, nil)
	require.NoError(t, err)

	reloaded, err := svc.GetPlatformPublishConfigForShop(c, productRow.ID, "ozon", shopRow.ID.String())
	require.NoError(t, err)
	require.NotNil(t, reloaded.OzonImages)
	require.True(t, reloaded.OzonImages.Configured)
	require.Zero(t, reloaded.OzonImages.ErrorCount)
	bySKU := map[uuid.UUID]OzonSKUImageDTO{}
	for _, sku := range reloaded.OzonImages.SKUs {
		bySKU[sku.SKUID] = sku
	}
	require.Equal(t, []string{originalSKU.ImageURL, detail.PublicURL}, resolvedOzonURLs(bySKU[originalSKU.ID].FinalImages))
	require.Equal(t, []string{shared.PublicURL, detail.PublicURL}, resolvedOzonURLs(bySKU[fallbackSKU.ID].FinalImages))
}

func TestOzonPlatformConfigPersistsAndReloadsSKUVariantMappings(t *testing.T) {
	svc, productRow := tenantProductFixture(t, 1)
	require.NoError(t, svc.DB.AutoMigrate(
		&ProductPlatformPublishConfig{},
		&shop.Shop{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
	))
	shopRow := shop.Shop{TenantID: 1, Platform: "ozon", ShopName: "Ozon variants", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, svc.DB.Create(&shopRow).Error)
	require.NoError(t, svc.DB.Create(&shop.PlatformCategory{Platform: "ozon", CategoryID: "100:200", Name: "服装", IsLeaf: true, Status: "active"}).Error)
	require.NoError(t, svc.DB.Create(&shop.PlatformCategoryAttribute{
		Platform: "ozon", CategoryID: "100:200", AttrID: "10096", Name: "颜色", Required: true,
		Options: datatypes.JSON([]byte(`[{"id":"1","value":"红色"},{"id":"2","value":"蓝色"}]`)),
		Raw:     datatypes.JSON([]byte(`{"dictionary_id":"7","is_collection":false,"attribute_complex_id":0}`)),
	}).Error)
	red := ProductSKU{ProductID: productRow.ID, SKUCode: "RED", SKUName: "红色"}
	blue := ProductSKU{ProductID: productRow.ID, SKUCode: "BLUE", SKUName: "蓝色"}
	require.NoError(t, svc.DB.Create(&red).Error)
	require.NoError(t, svc.DB.Create(&blue).Error)

	payload := fmt.Sprintf(`{
		"version":3,
		"attributes":{},
		"complexGroups":[],
		"skuVariantAttributeIds":["10096"],
		"skuAttributeOverrides":{
			%q:{"10096":[{"value":"红色","dictionaryValueId":"1"}]},
			%q:{"10096":[{"value":"蓝色","dictionaryValueId":"2"}]}
		}
	}`, red.ID.String(), blue.ID.String())
	c := tenantProductAdminContext(t, svc, 1)
	saved, err := svc.PutPlatformPublishConfig(c, productRow.ID, "ozon", PlatformPublishConfigBody{
		ShopID: shopRow.ID.String(), CategoryID: "100:200", PlatformAttributes: json.RawMessage(payload),
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, saved.OzonPreview)

	reloaded, err := svc.GetPlatformPublishConfigForShop(c, productRow.ID, "ozon", shopRow.ID.String())
	require.NoError(t, err)
	decoded, err := DecodeOzonPlatformAttributes(reloaded.PlatformAttributes)
	require.NoError(t, err)
	require.Equal(t, OzonPlatformAttributesVersion, decoded.Version)
	require.Equal(t, []string{"10096"}, decoded.SKUVariantAttributeIDs)
	require.Equal(t, "红色", decoded.SKUAttributeOverrides[red.ID.String()]["10096"][0].Value)
	require.Equal(t, "蓝色", decoded.SKUAttributeOverrides[blue.ID.String()]["10096"][0].Value)

	previewBySKU := map[uuid.UUID]OzonResolvedSKUListingDTO{}
	for _, sku := range reloaded.OzonPreview.SKUs {
		previewBySKU[sku.SKUID] = sku
	}
	require.Equal(t, "红色", previewBySKU[red.ID].PlatformAttributes.Attributes["10096"][0].Value)
	require.Equal(t, OzonValueSourceSKUShopConfig, previewBySKU[red.ID].AttributeSources["10096"])

	unknownSKU := uuid.New()
	invalid := strings.Replace(payload, red.ID.String(), unknownSKU.String(), 1)
	_, err = svc.PutPlatformPublishConfig(c, productRow.ID, "ozon", PlatformPublishConfigBody{
		ShopID: shopRow.ID.String(), CategoryID: "100:200", PlatformAttributes: json.RawMessage(invalid),
	}, nil)
	require.ErrorContains(t, err, "不存在的 SKU")
}

func resolvedOzonURLs(images []OzonResolvedImageDTO) []string {
	out := make([]string, 0, len(images))
	for _, image := range images {
		out = append(out, image.URL)
	}
	return out
}
