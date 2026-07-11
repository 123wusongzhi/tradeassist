package order_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func openOrderUpsertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_upsert_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{}, &idempotency.Record{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func testPayload(ext, status string, updatedAt time.Time) order.SyncedOrderPayload {
	rev := fmt.Sprintf("t:%d", updatedAt.Unix())
	return order.SyncedOrderPayload{
		ExternalOrderID:   ext,
		Status:            status,
		PaymentStatus:     order.PaymentPaid,
		FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency:          "CNY",
		TotalAmount:       99,
		PlatformUpdatedAt: &updatedAt,
		PlatformRevision:  rev,
	}
}

func TestOrderStaleUpdateIgnored(t *testing.T) {
	db := openOrderUpsertTestDB(t)
	svc := &order.Service{DB: db, Idempotency: &idempotency.Service{DB: db}}
	shopID := uuid.New()
	newer := time.Unix(1700002000, 0).UTC()
	older := time.Unix(1700001000, 0).UTC()

	_, err := svc.UpsertPlatformOrder(context.Background(), order.PlatformOrderUpsertInput{
		Platform: "douyin_shop", ShopID: shopID, PlatformOrderID: "ORD-1",
		Source: order.UpsertSourceWebhook, NormalizedOrder: testPayload("ORD-1", order.StatusPaid, newer),
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.UpsertPlatformOrder(context.Background(), order.PlatformOrderUpsertInput{
		Platform: "douyin_shop", ShopID: shopID, PlatformOrderID: "ORD-1",
		Source: order.UpsertSourceWebhook, NormalizedOrder: testPayload("ORD-1", order.StatusPending, older),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.StaleIgnored {
		t.Fatalf("expected stale ignored, got %+v", res)
	}

	var count int64
	db.Model(&order.Order{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 order, got %d", count)
	}
	var o order.Order
	if err := db.First(&o, "external_order_id = ?", "ORD-1").Error; err != nil {
		t.Fatal(err)
	}
	if o.Status != order.StatusPaid {
		t.Fatalf("stale event overwrote status: %s", o.Status)
	}
}

func TestWebhookPollingConcurrentUpsert(t *testing.T) {
	db := openOrderUpsertTestDB(t)
	svc := &order.Service{DB: db, Idempotency: &idempotency.Service{DB: db}}
	shopID := uuid.New()
	ts := time.Unix(1700003000, 0).UTC()

	var wg sync.WaitGroup
	wg.Add(2)
	for _, src := range []string{order.UpsertSourceWebhook, order.UpsertSourcePolling} {
		src := src
		go func() {
			defer wg.Done()
			p := testPayload("ORD-CONC", order.StatusPaid, ts)
			if src == order.UpsertSourcePolling {
				p.Status = order.StatusShipped
				p.PlatformRevision = fmt.Sprintf("t:%d", ts.Unix()+1)
				t2 := ts.Add(time.Second)
				p.PlatformUpdatedAt = &t2
			}
			_, _ = svc.UpsertPlatformOrder(context.Background(), order.PlatformOrderUpsertInput{
				Platform: "douyin_shop", ShopID: shopID, PlatformOrderID: "ORD-CONC",
				Source: src, NormalizedOrder: p,
			})
		}()
	}
	wg.Wait()

	var count int64
	db.Model(&order.Order{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected single order, got %d", count)
	}
}

func TestUpsertPlatformOrdersUsesShopTenantWhenPayloadTenantMissing(t *testing.T) {
	db := openOrderUpsertTestDB(t)
	svc := &order.Service{DB: db, Idempotency: &idempotency.Service{DB: db}}
	row := shop.Shop{
		TenantID:       42,
		Platform:       "douyin_shop",
		ShopName:       "Tenant Shop",
		ExternalShopID: "ps-tenant",
		Status:         shop.StatusActive,
		AuthStatus:     shop.AuthAuthorized,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	_, success, failed, _, _, err := svc.UpsertPlatformOrders(context.Background(), row.ID, row.Platform, order.UpsertSourcePolling, []order.SyncedOrderPayload{
		testPayload("ORD-TENANT", order.StatusPaid, time.Unix(1700005000, 0).UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if success != 1 || failed != 0 {
		t.Fatalf("unexpected upsert counters success=%d failed=%d", success, failed)
	}

	var got order.Order
	if err := db.First(&got, "external_order_id = ?", "ORD-TENANT").Error; err != nil {
		t.Fatal(err)
	}
	if got.TenantID != row.TenantID {
		t.Fatalf("expected tenant %d, got %d", row.TenantID, got.TenantID)
	}
}

func TestConcurrentSameRevisionOnce(t *testing.T) {
	db := openOrderUpsertTestDB(t)
	svc := &order.Service{DB: db, Idempotency: &idempotency.Service{DB: db}}
	shopID := uuid.New()
	ts := time.Unix(1700004000, 0).UTC()
	p := testPayload("ORD-DUP", order.StatusPaid, ts)

	const n = 20
	var success int32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.UpsertPlatformOrder(context.Background(), order.PlatformOrderUpsertInput{
				Platform: "douyin_shop", ShopID: shopID, PlatformOrderID: "ORD-DUP",
				Source: order.UpsertSourceWebhook, NormalizedOrder: p,
			})
			if err == nil {
				atomic.AddInt32(&success, 1)
			}
		}()
	}
	wg.Wait()

	var count int64
	db.Model(&order.Order{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 order from concurrent upserts, got %d", count)
	}
}
