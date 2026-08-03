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
			_, _ = w.Write([]byte(`{"result":[{"id":85,"name":"Бренд","type":"String","is_collection":false,"is_required":true,"dictionary_id":0},{"id":86,"name":"Цвет","type":"Option","is_collection":false,"is_required":true,"dictionary_id":123}]}`))
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
	if len(dtoList) != 2 || dictionaryDTO == nil || dictionaryDTO.DictionaryID != "123" {
		t.Fatalf("unexpected dto list: %+v", dtoList)
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
	if out.List[0].TypeID != "200" || out.List[0].DescriptionCategoryID != "100" {
		t.Fatalf("leaf dto missing ids: %+v", out.List[0])
	}
	stats, err := svc.OzonCategoryStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count != 2 || stats.LeafCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
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
