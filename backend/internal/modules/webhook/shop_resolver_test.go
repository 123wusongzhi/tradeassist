package webhook_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"gorm.io/gorm"
)

func createDouyinBinding(t *testing.T, db *gorm.DB, tenantID int64, platformShopID, appKey string) (shop.Shop, shop.ShopAuthToken) {
	t.Helper()
	s := shop.Shop{
		TenantID:       tenantID,
		Platform:       "douyin_shop",
		ShopName:       "Douyin " + platformShopID,
		ExternalShopID: platformShopID,
		Status:         shop.StatusActive,
		AuthStatus:     shop.AuthAuthorized,
	}
	require.NoError(t, db.Create(&s).Error)
	tok := shop.ShopAuthToken{
		ShopID:    s.ID,
		Platform:  "douyin_shop",
		AuthType:  "oauth2",
		AppKey:    appKey,
		SellerID:  platformShopID,
		ExpiresAt: ptrTime(time.Now().UTC().Add(24 * time.Hour)),
	}
	require.NoError(t, db.Create(&tok).Error)
	return s, tok
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestWebhookIngestSameEventIDDifferentShops(t *testing.T) {
	svc, db := testService(t, true)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &shop.ShopAuthToken{}))
	shopA, tokA := createDouyinBinding(t, db, 10, "ps-a", "app-a")
	shopB, tokB := createDouyinBinding(t, db, 20, "ps-b", "app-b")

	body := json.RawMessage(`{"eventId":"same-event","content":{"shop_id":"ps-a"}}`)
	_, err := svc.Ingest(context.Background(), webhook.IngestRequest{
		Platform:  "douyin_shop",
		EventID:   "same-event",
		Payload:   body,
		Timestamp: svc.Now(),
		ResolvedShop: &webhook.ResolvedWebhookShop{
			TenantID:            shopA.TenantID,
			InternalShopID:      shopA.ID,
			Platform:            shopA.Platform,
			PlatformShopID:      shopA.ExternalShopID,
			AppID:               tokA.AppKey,
			BindingID:           tokA.ID,
			AuthorizationStatus: shopA.AuthStatus,
		},
	})
	require.NoError(t, err)
	_, err = svc.Ingest(context.Background(), webhook.IngestRequest{
		Platform:  "douyin_shop",
		EventID:   "same-event",
		Payload:   body,
		Timestamp: svc.Now(),
		ResolvedShop: &webhook.ResolvedWebhookShop{
			TenantID:            shopB.TenantID,
			InternalShopID:      shopB.ID,
			Platform:            shopB.Platform,
			PlatformShopID:      shopB.ExternalShopID,
			AppID:               tokB.AppKey,
			BindingID:           tokB.ID,
			AuthorizationStatus: shopB.AuthStatus,
		},
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&webhook.Event{}).Where("platform = ? AND event_id = ?", "douyin_shop", "same-event").Count(&count).Error)
	require.Equal(t, int64(2), count)

	n, err := svc.ProcessQueuedEvents(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.NoError(t, db.Model(&webhook.Event{}).Where("event_id = ? AND status = ?", "same-event", webhook.StatusProcessed).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestWebhookShopResolverByAppAndPlatformShopID(t *testing.T) {
	_, db := testService(t, true)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &shop.ShopAuthToken{}, &idempotency.Record{}, &webhook.Event{}))
	shopA, tokA := createDouyinBinding(t, db, 10, "ps-a", "app-a")
	_, _ = createDouyinBinding(t, db, 20, "ps-b", "app-a")
	resolver := &webhook.DBWebhookShopResolver{DB: db, AppEnv: "test"}

	res, err := resolver.Resolve(context.Background(), webhook.ResolveWebhookShopInput{
		Platform:       "douyin_shop",
		AppID:          "app-a",
		PlatformShopID: "ps-a",
	})
	require.NoError(t, err)
	require.Equal(t, shopA.ID, res.InternalShopID)
	require.Equal(t, tokA.ID, res.BindingID)
	require.Equal(t, int64(10), res.TenantID)
}

func TestWebhookShopResolverRejectsAmbiguousBinding(t *testing.T) {
	_, db := testService(t, true)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &shop.ShopAuthToken{}))
	_, _ = createDouyinBinding(t, db, 10, "ps-a", "app-a")
	_, _ = createDouyinBinding(t, db, 20, "ps-a", "app-a")
	resolver := &webhook.DBWebhookShopResolver{DB: db, AppEnv: "test"}

	_, err := resolver.Resolve(context.Background(), webhook.ResolveWebhookShopInput{
		Platform:       "douyin_shop",
		AppID:          "app-a",
		PlatformShopID: "ps-a",
	})
	require.Error(t, err)
	ce, ok := webhook.AsCodeError(err)
	require.True(t, ok)
	require.Equal(t, webhook.CodeDouyinWebhookShopAmbiguous, ce.Code)
}

func TestWebhookShopResolverRejectsNeedCheckAuthorization(t *testing.T) {
	_, db := testService(t, true)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &shop.ShopAuthToken{}))
	row, _ := createDouyinBinding(t, db, 10, "ps-a", "app-a")
	require.NoError(t, db.Model(&shop.Shop{}).Where("id = ?", row.ID).Update("auth_status", shop.AuthNeedCheck).Error)
	resolver := &webhook.DBWebhookShopResolver{DB: db, AppEnv: "test"}

	_, err := resolver.Resolve(context.Background(), webhook.ResolveWebhookShopInput{
		Platform:       "douyin_shop",
		AppID:          "app-a",
		PlatformShopID: "ps-a",
	})
	require.Error(t, err)
	ce, ok := webhook.AsCodeError(err)
	require.True(t, ok)
	require.Equal(t, webhook.CodeDouyinWebhookBindingRevoked, ce.Code)
}
