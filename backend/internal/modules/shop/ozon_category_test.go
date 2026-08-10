package shop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ozonCategoryFakeAPI struct {
	*httptest.Server
	seenAttrValues bool
}

func newOzonCategoryFakeAPI(t *testing.T) *ozonCategoryFakeAPI {
	t.Helper()
	api := &ozonCategoryFakeAPI{}
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/description-category/tree":
			_, _ = w.Write([]byte(`{"result":[{"description_category_id":100,"category_name":"Мебель","disabled":false,"children":[{"type_id":200,"type_name":"Стол","disabled":false}]}]}`))
		case "/v1/description-category/attribute":
			_, _ = w.Write([]byte(`{"result":[{"id":85,"name":"Бренд","description":"Марка товара","type":"String","is_aspect":false,"is_collection":false,"is_required":true,"dictionary_id":0},{"id":86,"name":"Цвет","description":"Основной цвет","type":"String","is_aspect":true,"is_collection":false,"is_required":true,"dictionary_id":123}]}`))
		case "/v1/description-category/attribute/values":
			api.seenAttrValues = true
			var req map[string]any
			_ = json.Unmarshal(body, &req)
			if stringValue(req["attribute_id"]) == "86" {
				_, _ = w.Write([]byte(`{"result":[{"id":1001,"value":"Белый"},{"id":1002,"value":"Черный"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/v1/description-category/attribute/values/search":
			_, _ = w.Write([]byte(`{"result":[{"id":1001,"value":"Белый"}]}`))
		case "/v2/warehouse/list":
			_, _ = w.Write([]byte(`{"warehouses":[{"warehouse_id":5278166,"name":"测试 FBS 仓","is_rfbs":false}],"has_next":false,"cursor":""}`))
		default:
			t.Fatalf("unexpected ozon path %s", r.URL.Path)
		}
	}))
	t.Cleanup(api.Close)
	return api
}

func seedOzonAuthorizedShop(t *testing.T, db *gorm.DB, enc *encrypt.Service, apiURL string) uuid.UUID {
	t.Helper()
	shop := Shop{Platform: ozonPlatform, ShopName: "Ozon Demo", Status: StatusActive, AuthStatus: AuthAuthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatal(err)
	}
	key, _ := enc.Encrypt([]byte("api-key-test"))
	cfg, _ := json.Marshal(map[string]any{"api_base_url": apiURL, "timeout_sec": "5"})
	tok := ShopAuthToken{
		ShopID:         shop.ID,
		Platform:       ozonPlatform,
		AuthType:       "api_key",
		AppKey:         "client-1",
		AccessTokenEnc: key,
		AuthConfig:     datatypes.JSON(cfg),
	}
	if err := db.Create(&tok).Error; err != nil {
		t.Fatal(err)
	}
	return shop.ID
}

func seedOzonUnauthorizedShop(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	shop := Shop{Platform: ozonPlatform, ShopName: "Ozon NoAuth", Status: StatusActive, AuthStatus: AuthNeedCheck}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatal(err)
	}
	return shop.ID
}

func seedOzonLeafCategory(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	raw, _ := json.Marshal(map[string]any{"description_category_id": "100", "type_id": "200"})
	cat := PlatformCategory{
		Platform:   ozonPlatform,
		CategoryID: "100:200",
		ParentID:   "100",
		Name:       "Стол",
		Level:      2,
		IsLeaf:     true,
		Raw:        datatypes.JSON(raw),
		SyncedAt:   &now,
	}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatal(err)
	}
	return cat.ID
}

func newOzonCategoryTestService(t *testing.T, db *gorm.DB, enc *encrypt.Service) *Service {
	t.Helper()
	return &Service{DB: db, Encrypter: enc}
}

func setOzonTestBaseURL(t *testing.T, svc *Service, baseURL string) {
	t.Helper()
	svc.TrustedProviderRuntimeOverrides = map[string]map[string]string{
		ozonPlatform: {"api_base_url": baseURL, "timeout_sec": "5"},
	}
}

func TestOzonLegacyAuthConfigCannotOverrideRuntime(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	shopID := seedOzonAuthorizedShop(t, db, enc, "http://127.0.0.1:1")
	svc := newOzonCategoryTestService(t, db, enc)

	var shopRow Shop
	if err := db.First(&shopRow, "id = ?", shopID).Error; err != nil {
		t.Fatal(err)
	}
	_, token, auth, err := svc.decryptedAuthForShop(context.Background(), &shopRow)
	if err != nil {
		t.Fatal(err)
	}
	if len(auth.Extra) != 0 {
		t.Fatalf("legacy Ozon authConfig must not reach provider runtime: %+v", auth.Extra)
	}
	if dto := svc.buildAuthPublic(token); len(dto.AuthConfig) != 0 {
		t.Fatalf("legacy Ozon authConfig must not be returned: %s", dto.AuthConfig)
	}

	setOzonTestBaseURL(t, svc, "http://127.0.0.1:2")
	_, _, trustedAuth, err := svc.decryptedAuthForShop(context.Background(), &shopRow)
	if err != nil {
		t.Fatal(err)
	}
	if trustedAuth.Extra["api_base_url"] != "http://127.0.0.1:2" {
		t.Fatalf("trusted process override missing: %+v", trustedAuth.Extra)
	}
}

func TestUpdateAuthRejectsOzonAuthConfig(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	shopID := seedOzonAuthorizedShop(t, db, enc, "http://127.0.0.1:1")
	svc := newOzonCategoryTestService(t, db, enc)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/shops/"+shopID.String()+"/auth", nil)
	c.Set(ctxkey.TenantID, int64(0))

	_, err := svc.UpdateAuth(c, shopID, UpdateAuthBody{AuthConfig: map[string]any{"api_base_url": "http://127.0.0.1:2"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "authConfig is not supported") {
		t.Fatalf("expected Ozon authConfig rejection, got %v", err)
	}
}

func TestNormalizeOzonSourceCategorySupportsUnicodeLetters(t *testing.T) {
	if got := normalizeOzonSourceCategory("  Мебель / 桌子 🪑 42 "); got != "мебель桌子42" {
		t.Fatalf("unexpected normalized category: %q", got)
	}
}

func newOzonCategoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newDouyinShopTestDB(t)
	if err := db.AutoMigrate(&PlatformCategory{}, &PlatformCategoryAttribute{}, &PlatformCategoryAttributeMapping{}); err != nil {
		t.Fatalf("migrate ozon category tables: %v", err)
	}
	return db
}

func TestOzonCategorySyncWritesCacheIdempotently(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	shopID := seedOzonAuthorizedShop(t, db, enc, api.URL)

	stats, err := svc.SyncOzonCategories(context.Background(), 0, shopID)
	if err != nil {
		t.Fatalf("SyncOzonCategories() error = %v", err)
	}
	if stats.Count != 2 || stats.LeafCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, err := svc.SyncOzonCategories(context.Background(), 0, shopID); err != nil {
		t.Fatalf("second sync error = %v", err)
	}
	var count int64
	if err := db.Model(&PlatformCategory{}).Where("platform = ?", ozonPlatform).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("duplicate sync inserted rows, got %d", count)
	}
	var leaf PlatformCategory
	if err := db.Where("platform = ? AND is_leaf = ?", ozonPlatform, true).First(&leaf).Error; err != nil {
		t.Fatal(err)
	}
	if leaf.CategoryID != "100:200" {
		t.Fatalf("leaf composite id = %q, want 100:200", leaf.CategoryID)
	}
	if !strings.Contains(string(leaf.Raw), `"type_id":"200"`) {
		t.Fatalf("leaf raw missing type_id: %s", string(leaf.Raw))
	}
}

func TestListOzonWarehousesUsesAuthorizedTenantShop(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	shopID := seedOzonAuthorizedShop(t, db, enc, api.URL)

	rows, err := svc.ListOzonWarehouses(context.Background(), 0, shopID)
	if err != nil {
		t.Fatalf("ListOzonWarehouses() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "5278166" || rows[0].Name != "测试 FBS 仓" {
		t.Fatalf("unexpected warehouses: %+v", rows)
	}
}

func TestOzonCategorySyncRequiresAuthorizedShop(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	shopID := seedOzonUnauthorizedShop(t, db)
	if _, err := svc.SyncOzonCategories(context.Background(), 0, shopID); err == nil {
		t.Fatalf("expected unauthorized shop failure")
	}
}

func TestOzonCategorySyncResolvesFirstAuthorizedShop(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	seedOzonAuthorizedShop(t, db, enc, api.URL)
	stats, err := svc.SyncOzonCategories(context.Background(), 0, uuid.Nil)
	if err != nil {
		t.Fatalf("SyncOzonCategories() with nil shop error = %v", err)
	}
	if stats.Count != 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestOzonCategoryAttributeSyncWritesCacheAndDictionaryValues(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	seedOzonAuthorizedShop(t, db, enc, api.URL)
	catID := seedOzonLeafCategory(t, db)

	stats, err := svc.SyncOzonCategoryAttributes(context.Background(), 0, catID.String(), uuid.Nil)
	if err != nil {
		t.Fatalf("SyncOzonCategoryAttributes() error = %v", err)
	}
	if stats.Count != 2 {
		t.Fatalf("expected 2 attrs, got %+v", stats)
	}
	if !api.seenAttrValues {
		t.Fatalf("expected dictionary values to be fetched")
	}
	var attrs []PlatformCategoryAttribute
	if err := db.Where("platform = ? AND category_id = ?", ozonPlatform, "100:200").Find(&attrs).Error; err != nil {
		t.Fatal(err)
	}
	if len(attrs) != 2 {
		t.Fatalf("cached attrs = %d, want 2", len(attrs))
	}
	var dictAttr *PlatformCategoryAttribute
	for i := range attrs {
		if attrs[i].AttrID == "86" {
			dictAttr = &attrs[i]
		}
	}
	if dictAttr == nil {
		t.Fatalf("dictionary attr 86 not cached")
	}
	if !strings.Contains(string(dictAttr.Raw), "dictionary_id") || len(dictAttr.Options) == 0 {
		t.Fatalf("dictionary attr missing raw/options: raw=%s options=%s", string(dictAttr.Raw), string(dictAttr.Options))
	}
	dtoList, err := svc.ListOzonCategoryAttributes(context.Background(), catID.String())
	if err != nil {
		t.Fatal(err)
	}
	var dictionaryDTO *OzonAttributeDTO
	for i := range dtoList {
		if dtoList[i].AttrID == "86" {
			dictionaryDTO = &dtoList[i]
		}
	}
	if len(dtoList) != 2 || dictionaryDTO == nil || dictionaryDTO.DictionaryID != "123" || !dictionaryDTO.SKUVariantEligibilityKnown || !dictionaryDTO.SKUVariantEligible || dictionaryDTO.Description != "Основной цвет" {
		t.Fatalf("unexpected dto list: %+v", dtoList)
	}
	policy := OzonVariantPolicy(dtoList)
	if policy.MaxSKUCount != OzonMaxSKUsPerPublish || policy.MaxVariantAttributeCount != 1 || policy.MaxVariantCombinationCount != OzonMaxSKUsPerPublish || !policy.VariantEligibilityFullyKnown {
		t.Fatalf("unexpected variant policy: %+v", policy)
	}
}

func TestOzonCategoryAttributeSyncReturnsActionableCredentialError(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":7,"message":"Api-key is deactivated"}`))
	}))
	t.Cleanup(api.Close)
	setOzonTestBaseURL(t, svc, api.URL)
	shopID := seedOzonAuthorizedShop(t, db, enc, api.URL)
	catID := seedOzonLeafCategory(t, db)

	_, err := svc.SyncOzonCategoryAttributes(context.Background(), 0, catID.String(), shopID)
	var categoryErr *OzonCategoryError
	if !errors.As(err, &categoryErr) {
		t.Fatalf("expected OzonCategoryError, got %v", err)
	}
	if categoryErr.Code != OzonCategoryAttrSyncFailed {
		t.Fatalf("error code = %q", categoryErr.Code)
	}
	if categoryErr.Message != ozonCredentialInvalidMessage {
		t.Fatalf("user message = %q", categoryErr.Message)
	}
	if strings.Contains(categoryErr.Message, "code=7") {
		t.Fatalf("user message leaked upstream detail: %q", categoryErr.Message)
	}
}

func TestSearchOzonDictionaryValuesUsesSelectedAttributeAndShop(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	api := newOzonCategoryFakeAPI(t)
	setOzonTestBaseURL(t, svc, api.URL)
	shopID := seedOzonAuthorizedShop(t, db, enc, api.URL)
	catID := seedOzonLeafCategory(t, db)
	if _, err := svc.SyncOzonCategoryAttributes(context.Background(), 0, catID.String(), shopID); err != nil {
		t.Fatal(err)
	}
	values, err := svc.SearchOzonDictionaryValues(context.Background(), 0, "100:200", "86", shopID, "Бел")
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].ID != "1001" || values[0].Value != "Белый" {
		t.Fatalf("unexpected dictionary search values: %+v", values)
	}
	if _, err := svc.SearchOzonDictionaryValues(context.Background(), 0, "100:200", "85", shopID, "Бр"); err == nil {
		t.Fatal("expected non-dictionary attribute search to fail")
	}
}

func TestOzonAttributeMappingsRoundTrip(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newOzonCategoryTestService(t, db, enc)
	catID := seedOzonLeafCategory(t, db)

	body := PutOzonAttributeMappingsBody{Items: []OzonAttributeMappingDTO{
		{AttributeID: "85", AttributeName: "Бренд", LocalField: "brand", Enabled: true},
		{AttributeID: "86", AttributeName: "Цвет", LocalField: "color", Enabled: true},
	}}
	out, err := svc.PutOzonAttributeMappings(context.Background(), catID.String(), body)
	if err != nil {
		t.Fatalf("PutOzonAttributeMappings() error = %v", err)
	}
	if len(out) != 2 || out[0].LocalField != "brand" {
		t.Fatalf("unexpected mappings: %+v", out)
	}
	body2 := PutOzonAttributeMappingsBody{Items: []OzonAttributeMappingDTO{
		{AttributeID: "85", AttributeName: "Бренд", LocalField: "brand_name", Enabled: true},
	}}
	if _, err := svc.PutOzonAttributeMappings(context.Background(), catID.String(), body2); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetOzonAttributeMappings(context.Background(), catID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LocalField != "brand_name" {
		t.Fatalf("mapping replace failed: %+v", got)
	}
}

func TestOzonCategoryListFiltersAndStats(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	svc := newOzonCategoryTestService(t, db, &encrypt.Service{})
	seedOzonLeafCategory(t, db)
	_ = db.Create(&PlatformCategory{Platform: ozonPlatform, CategoryID: "100", Name: "Мебель", Level: 1, IsLeaf: false, Status: "active"}).Error

	out, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{OnlyLeaf: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.LeafCount != 1 || len(out.List) != 1 {
		t.Fatalf("unexpected leaf list: %+v", out)
	}
	if out.Total != 2 || out.MatchedTotal != 1 || out.Offset != 0 || out.Limit != 500 {
		t.Fatalf("unexpected list metadata: %+v", out)
	}
	if out.LastSyncedAt == nil || out.CacheStale {
		t.Fatalf("fresh category cache metadata missing: %+v", out)
	}
	if out.List[0].TypeID != "200" || out.List[0].DescriptionCategoryID != "100" {
		t.Fatalf("leaf dto missing ids: %+v", out.List[0])
	}
	if out.List[0].Path != "Мебель / Стол" {
		t.Fatalf("leaf dto path = %q, want canonical hierarchy", out.List[0].Path)
	}
	stats, err := svc.OzonCategoryStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count != 2 || stats.LeafCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestOzonCategoryListSearchesFullPathAndPaginatesMatches(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	svc := newOzonCategoryTestService(t, db, &encrypt.Service{})
	now := time.Now().UTC()
	rows := []PlatformCategory{
		{Platform: ozonPlatform, CategoryID: "home", Name: "住宅和花园", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "storage", ParentID: "home", Name: "收纳", Level: 2, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "17027937:95482", ParentID: "storage", Name: "储物箱", Level: 3, IsLeaf: true, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "17027937:95483", ParentID: "storage", Name: "衣物收纳盒", Level: 3, IsLeaf: true, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "17027937:95484", ParentID: "storage", Name: "旧收纳盒", Level: 3, IsLeaf: true, Status: "inactive", SyncedAt: &now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	first, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		Keyword: "住宅和花园 / 收纳", OnlyLeaf: true, ActiveOnly: true, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.MatchedTotal != 2 || len(first.List) != 1 || first.Offset != 0 || first.Limit != 1 {
		t.Fatalf("first page metadata = %+v", first)
	}
	if !strings.Contains(first.List[0].Path, "住宅和花园 / 收纳 /") {
		t.Fatalf("full path missing from result: %+v", first.List[0])
	}

	second, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		Keyword: "收纳", OnlyLeaf: true, ActiveOnly: true, Limit: 1, Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.MatchedTotal != 2 || len(second.List) != 1 || second.Offset != 1 || second.List[0].CategoryID == first.List[0].CategoryID {
		t.Fatalf("second page = %+v", second)
	}

	allMatches, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		Keyword: "收纳", OnlyLeaf: true, ActiveOnly: true, Limit: 1, Offset: 1, AllMatches: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if allMatches.MatchedTotal != 2 || len(allMatches.List) != 2 || allMatches.Offset != 0 || allMatches.Limit != 2 {
		t.Fatalf("all matches = %+v", allMatches)
	}

	byID, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		Keyword: "17027937:95482", OnlyLeaf: true, ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if byID.MatchedTotal != 1 || len(byID.List) != 1 || byID.List[0].Path != "住宅和花园 / 收纳 / 储物箱" {
		t.Fatalf("id search = %+v", byID)
	}
}

func TestOzonCategoryListSupportsParentByParentNavigation(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	svc := newOzonCategoryTestService(t, db, &encrypt.Service{})
	now := time.Now().UTC()
	rows := []PlatformCategory{
		{Platform: ozonPlatform, CategoryID: "home", Name: "住宅和花园", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "electronics", Name: "电子产品", Level: 1, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "storage", ParentID: "home", Name: "收纳", Level: 2, Status: "active", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "inactive-child", ParentID: "home", Name: "停用分支", Level: 2, Status: "inactive", SyncedAt: &now},
		{Platform: ozonPlatform, CategoryID: "17027937:95482", ParentID: "storage", Name: "储物箱", Level: 3, IsLeaf: true, Status: "active", SyncedAt: &now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	roots, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		RootOnly: true, ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if roots.MatchedTotal != 2 || len(roots.List) != 2 {
		t.Fatalf("roots = %+v", roots)
	}
	if roots.List[0].Level != 1 || len(roots.List[0].Ancestors) != 0 {
		t.Fatalf("root metadata = %+v", roots.List[0])
	}

	parentID := "home"
	children, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		ParentID: &parentID, ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if children.MatchedTotal != 1 || len(children.List) != 1 || children.List[0].CategoryID != "storage" {
		t.Fatalf("children = %+v", children)
	}
	if !children.List[0].HasChildren || children.List[0].ChildCount != 1 {
		t.Fatalf("child count = %+v", children.List[0])
	}

	leafParentID := "storage"
	leaves, err := svc.ListOzonCategories(context.Background(), OzonCategoryListQuery{
		ParentID: &leafParentID, ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves.List) != 1 || !leaves.List[0].IsLeaf || leaves.List[0].HasChildren {
		t.Fatalf("leaf = %+v", leaves)
	}
	if got := leaves.List[0].Ancestors; len(got) != 2 || got[0].CategoryID != "home" || got[1].CategoryID != "storage" {
		t.Fatalf("leaf ancestors = %+v", got)
	}
}

func TestListOzonCategoryChangesReturnsChangeCenterDTO(t *testing.T) {
	db := newOzonCategoryTestDB(t)
	if err := db.AutoMigrate(&OzonCategoryChange{}); err != nil {
		t.Fatal(err)
	}
	svc := newOzonCategoryTestService(t, db, &encrypt.Service{})
	before := datatypes.JSON([]byte(`{"name":"Old category"}`))
	after := datatypes.JSON([]byte(`{"name":"New category"}`))
	change := OzonCategoryChange{TenantID: 9, ShopID: uuid.New(), SyncRunID: uuid.New(), CategoryID: "100:200", ChangeType: "changed", Before: before, After: after}
	if err := db.Create(&change).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListOzonCategoryChanges(context.Background(), 9, OzonCategoryChangesQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	got := rows[0]
	if got.CategoryName != "New category" || got.OccurredAt.IsZero() || got.Detail != "类目名称或层级已变化：New category" || string(got.After) != string(after) {
		t.Fatalf("change DTO = %#v", got)
	}
	reactivated := ozonCategoryChangeDTO(OzonCategoryChange{ChangeType: "reactivated", After: after})
	if reactivated.Detail != "类目已恢复启用：New category" {
		t.Fatalf("reactivated detail = %q", reactivated.Detail)
	}
}
