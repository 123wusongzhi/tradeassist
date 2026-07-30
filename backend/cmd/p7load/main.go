package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/performance"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type datasetPlan struct {
	Tenants       int `json:"tenants"`
	Shops         int `json:"shops"`
	Users         int `json:"users"`
	Products      int `json:"products"`
	SKUs          int `json:"skus"`
	Orders        int `json:"orders"`
	OrderItems    int `json:"orderItems"`
	InventoryRows int `json:"inventoryRows"`
	Tasks         int `json:"tasks"`
	Webhooks      int `json:"webhooks"`
	OperationLogs int `json:"operationLogs"`
}

type report struct {
	Phase              string         `json:"phase"`
	Status             string         `json:"status"`
	RunID              string         `json:"runId"`
	Profile            string         `json:"profile"`
	DryRun             bool           `json:"dryRun"`
	DatasetPlan        datasetPlan    `json:"datasetPlan"`
	PlannedRows        int64          `json:"plannedRows"`
	InsertedRows       int64          `json:"insertedRows"`
	ExistingRows       int64          `json:"existingRows"`
	SkippedRows        int64          `json:"skippedRows"`
	FailedRows         int64          `json:"failedRows"`
	ActualRows         int64          `json:"actualRows"`
	BatchCount         int            `json:"batchCount"`
	DurationMs         int64          `json:"durationMs"`
	PeakMemoryBytes    uint64         `json:"peakMemoryBytes"`
	CleanupStatus      string         `json:"cleanupStatus"`
	DatasetFingerprint string         `json:"datasetFingerprint"`
	Counts             map[string]int `json:"counts"`
	StartedAt          string         `json:"startedAt"`
	FinishedAt         string         `json:"finishedAt"`
	Guards             []string       `json:"guards"`
	FailureInjection   map[string]int `json:"failureInjection,omitempty"`
	Issues             []string       `json:"issues"`
}

type loader struct {
	db               *gorm.DB
	runID            string
	runKey           string
	plan             datasetPlan
	batchSize        int
	failAfterBatches int
	stopAfterRows    int64
	report           *report
	now              time.Time
}

type skuRef struct {
	ID        uuid.UUID
	ProductID uuid.UUID
}

var errControlledInterruption = errors.New("controlled P7 dataset interruption")

func main() {
	profile := flag.String("profile", "small", "small|medium|large|stress")
	runID := flag.String("run-id", "", "P7 run id")
	dryRun := flag.Bool("dry-run", true, "only validate guards and print dataset plan")
	execute := flag.Bool("execute", false, "execute dataset generation; equivalent to --dry-run=false")
	cleanupOnly := flag.Bool("cleanup-only", false, "delete only rows that belong to the current run id")
	batchSize := flag.Int("batch-size", 1000, "bounded rows per transaction")
	failAfterBatches := flag.Int("fail-after-batches", 0, "performance-mode only: exit after N successful batches for resume drills")
	stopAfterRows := flag.Int64("stop-after-rows", 0, "performance-mode only: exit after at least N inserted rows for resume drills")
	flag.Parse()

	start := time.Now().UTC()
	id := strings.TrimSpace(*runID)
	if id == "" {
		id = "p7-" + start.Format("20060102T150405Z")
	}
	plan, rows, err := profilePlan(*profile)
	if err != nil {
		write(report{Phase: "P7-V", Status: "invalid_profile", RunID: id, Profile: *profile, DryRun: *dryRun, Issues: []string{err.Error()}})
		os.Exit(2)
	}
	if *execute {
		*dryRun = false
	}
	rep := report{
		Phase:         "P7-V",
		Status:        "planned",
		RunID:         id,
		Profile:       strings.ToLower(strings.TrimSpace(*profile)),
		DryRun:        *dryRun,
		DatasetPlan:   plan,
		PlannedRows:   rows,
		StartedAt:     start.Format(time.RFC3339),
		CleanupStatus: "not_requested",
		Guards: []string{
			"no production datasets",
			"requires APP_ENV=performance",
			"requires PERFORMANCE_TEST_MODE=true",
			"requires ALLOW_PERFORMANCE_DATASET=true",
			"requires EXTERNAL_PROVIDER_MODE=mock",
			"requires DOUYIN_WRITE_ENABLED=false",
			"requires AUTO_LISTING_ENABLED=false",
			"requires PostgreSQL database name prefix trademind_p7_, trademind_p7c2_, trademind_p7c4_, or trademind_p7v2_",
		},
	}
	if *dryRun && !*cleanupOnly {
		rep.Status = "dry_run_passed"
		rep.DatasetFingerprint = fingerprint(plan, nil)
		rep.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		write(rep)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		finish(&rep, "config_failed", err)
		os.Exit(1)
	}
	if err := validateGuards(cfg, rows); err != nil {
		finish(&rep, "guard_rejected", err)
		os.Exit(1)
	}
	db, err := database.Open(cfg)
	if err != nil {
		finish(&rep, "database_failed", err)
		os.Exit(1)
	}
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	defer func() { _ = database.Close(db) }()
	if err := database.AutoMigrate(db); err != nil {
		finish(&rep, "migration_failed", err)
		os.Exit(1)
	}

	ld := &loader{
		db:               db,
		runID:            id,
		runKey:           safeRunKey(id),
		plan:             plan,
		batchSize:        boundedBatchSize(*batchSize),
		failAfterBatches: max(0, *failAfterBatches),
		stopAfterRows:    max64(0, *stopAfterRows),
		report:           &rep,
		now:              start.Truncate(time.Second),
	}
	if ld.failAfterBatches > 0 || ld.stopAfterRows > 0 {
		rep.FailureInjection = map[string]int{
			"failAfterBatches": ld.failAfterBatches,
			"stopAfterRows":    int(ld.stopAfterRows),
		}
	}
	if *cleanupOnly {
		if err := ld.cleanup(context.Background()); err != nil {
			finish(&rep, "cleanup_failed", err)
			os.Exit(1)
		}
		rep.Status = "cleanup_passed"
		rep.CleanupStatus = "passed"
		rep.Counts = map[string]int{}
		rep.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		rep.DurationMs = time.Since(start).Milliseconds()
		write(rep)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()
	before, err := ld.counts(ctx)
	if err != nil {
		finish(&rep, "count_failed", err)
		os.Exit(1)
	}
	rep.ExistingRows = sumCounts(before)
	if err := ld.generate(ctx); err != nil {
		if errors.Is(err, errControlledInterruption) {
			after, countErr := ld.counts(ctx)
			if countErr == nil {
				rep.Counts = after
				rep.ActualRows = sumCounts(after)
				rep.DatasetFingerprint = fingerprint(plan, after)
			}
			finish(&rep, "controlled_interruption", err)
			os.Exit(75)
		}
		rep.FailedRows = rows - rep.InsertedRows - rep.ExistingRows
		finish(&rep, "dataset_generation_failed", err)
		os.Exit(1)
	}
	after, err := ld.counts(ctx)
	if err != nil {
		finish(&rep, "verification_failed", err)
		os.Exit(1)
	}
	rep.Counts = after
	rep.ActualRows = sumCounts(after)
	rep.SkippedRows = rep.ActualRows - rep.InsertedRows
	rep.DatasetFingerprint = fingerprint(plan, after)
	if rep.ActualRows != rows {
		rep.FailedRows = rows - rep.ActualRows
		finish(&rep, "dataset_count_mismatch", fmt.Errorf("actual rows %d do not match planned rows %d", rep.ActualRows, rows))
		os.Exit(1)
	}
	rep.Status = "dataset_generated"
	rep.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	rep.DurationMs = time.Since(start).Milliseconds()
	if err := ld.recordRun(ctx, &rep); err != nil {
		finish(&rep, "record_failed", err)
		os.Exit(1)
	}
	write(rep)
}

func profilePlan(profile string) (datasetPlan, int64, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "small":
		p := datasetPlan{Tenants: 10, Shops: 20, Users: 10, Products: 10000, SKUs: 10000, Orders: 20000, OrderItems: 50000, InventoryRows: 10000, Tasks: 20000, Webhooks: 20000, OperationLogs: 50000}
		return p, physicalRows(p), nil
	case "medium":
		p := datasetPlan{Tenants: 50, Shops: 100, Users: 50, Products: 100000, SKUs: 100000, Orders: 200000, OrderItems: 500000, InventoryRows: 100000, Tasks: 200000, Webhooks: 200000, OperationLogs: 500000}
		return p, physicalRows(p), nil
	case "large", "stress":
		return datasetPlan{}, 0, fmt.Errorf("%s profile requires a separate resource budget and is not enabled by this guarded MVP loader", profile)
	default:
		return datasetPlan{}, 0, fmt.Errorf("profile must be small, medium, large or stress")
	}
}

func physicalRows(p datasetPlan) int64 {
	return int64(p.Shops + p.Users + p.Products + p.SKUs + p.Orders + p.OrderItems + p.InventoryRows + p.Tasks + p.Webhooks + p.OperationLogs)
}

func validateGuards(cfg *config.Config, rows int64) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if config.IsProduction(cfg.AppEnv) {
		return fmt.Errorf("production dataset generation is forbidden")
	}
	if cfg.AppEnv != config.EnvPerformance {
		return fmt.Errorf("APP_ENV must be performance")
	}
	if cfg.DB.Driver != "postgres" {
		return fmt.Errorf("P7-V dataset generation requires PostgreSQL")
	}
	if !cfg.P7.PerformanceTestMode || !cfg.P7.AllowPerformanceDataset {
		return fmt.Errorf("PERFORMANCE_TEST_MODE and ALLOW_PERFORMANCE_DATASET must both be true")
	}
	if cfg.P7.ExternalProviderMode != "mock" {
		return fmt.Errorf("EXTERNAL_PROVIDER_MODE must be mock")
	}
	if cfg.P7.DouyinWriteEnabled || cfg.P7.AutoListingEnabled {
		return fmt.Errorf("Douyin writes and auto listing must be disabled")
	}
	if !cfg.P7.PprofInternalOnly {
		return fmt.Errorf("PPROF_INTERNAL_ONLY must be true")
	}
	name := strings.TrimSpace(cfg.DB.Name)
	if !strings.HasPrefix(name, "trademind_p7_") && !strings.HasPrefix(name, "trademind_p7c2_") && !strings.HasPrefix(name, "trademind_p7c4_") && !strings.HasPrefix(name, "trademind_p7v2_") {
		return fmt.Errorf("DB_NAME must start with trademind_p7_, trademind_p7c2_, trademind_p7c4_, or trademind_p7v2_")
	}
	if max := int64(cfg.P7.PerformanceDatasetMaxRows); max > 0 && rows > max {
		return fmt.Errorf("planned rows %d exceed PERFORMANCE_DATASET_MAX_ROWS %d", rows, max)
	}
	return nil
}

func (l *loader) generate(ctx context.Context) error {
	steps := []func(context.Context) error{
		l.ensureShops,
		l.ensureUsers,
		l.ensureProducts,
		l.ensureSKUs,
		l.ensureOrders,
		l.ensureOrderItems,
		l.ensureInventoryRows,
		l.ensureTasks,
		l.ensureWebhooks,
		l.ensureOperationLogs,
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (l *loader) ensureShops(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&shop.Shop{}).Where("shop_code LIKE ?", l.shopCodePrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	return l.batchFrom(int(count), l.plan.Shops, func(start, end int) error {
		rows := make([]shop.Shop, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, shop.Shop{
				TenantID:       l.tenant(i),
				Platform:       "mock",
				ShopName:       fmt.Sprintf("P7 Mock Shop %05d", i),
				ShopCode:       fmt.Sprintf("%s%05d", l.shopCodePrefix(), i),
				ExternalShopID: fmt.Sprintf("mock-shop-%s-%05d", l.runKey, i),
				Status:         "active",
				AuthStatus:     "mock_authorized",
				Region:         "CN",
				Currency:       "CNY",
				Timezone:       "Asia/Shanghai",
				Capabilities:   jsonRaw(map[string]any{"p7RunId": l.runID, "mock": true}),
				PlatformConfig: jsonRaw(map[string]any{"provider": "mock", "p7RunId": l.runID}),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureUsers(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&admin.AdminUser{}).Where("username LIKE ?", l.userPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	return l.batchFrom(int(count), l.plan.Users, func(start, end int) error {
		rows := make([]admin.AdminUser, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, admin.AdminUser{
				TenantID:     l.tenant(i),
				Username:     fmt.Sprintf("%s%05d", l.userPrefix(), i),
				Email:        fmt.Sprintf("p7-%s-%05d@example.invalid", l.runKey, i),
				PasswordHash: "p7-mock-password-hash-not-a-secret",
				DisplayName:  fmt.Sprintf("P7 Mock User %05d", i),
				Role:         roleFor(i),
				Status:       "active",
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureProducts(ctx context.Context) error {
	count, err := l.countJSON(ctx, "products", "raw_data")
	if err != nil {
		return err
	}
	return l.batchFrom(count, l.plan.Products, func(start, end int) error {
		rows := make([]product.Product, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, product.Product{
				TenantID:      l.tenant(i),
				Source:        "p7-mock",
				SourceURL:     fmt.Sprintf("https://example.invalid/p7/%s/products/%d", l.runKey, i),
				OriginalTitle: fmt.Sprintf("P7 Mock Product Original %06d", i),
				Title:         fmt.Sprintf("P7 Mock Product %06d", i),
				Description:   "P7 mock product for isolated performance dataset.",
				Currency:      "CNY",
				Status:        product.StatusReady,
				RawData:       jsonRaw(map[string]any{"p7RunId": l.runID, "ordinal": i}),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureSKUs(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&product.ProductSKU{}).Where("sku_code LIKE ?", l.skuPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	productIDs, err := l.productIDs(ctx)
	if err != nil {
		return err
	}
	if len(productIDs) == 0 && l.plan.SKUs > 0 {
		return fmt.Errorf("cannot create SKUs without products")
	}
	return l.batchFrom(int(count), l.plan.SKUs, func(start, end int) error {
		rows := make([]product.ProductSKU, 0, end-start)
		for i := start; i < end; i++ {
			stock := 20 + (i % 500)
			price := float64(1000+i%9000) / 100
			rows = append(rows, product.ProductSKU{
				ProductID:    productIDs[i%len(productIDs)],
				SKUCode:      fmt.Sprintf("%s%06d", l.skuPrefix(), i),
				SKUName:      fmt.Sprintf("P7 SKU %06d", i),
				Attrs:        jsonRaw(map[string]any{"p7RunId": l.runID, "color": fmt.Sprintf("mock-%d", i%12)}),
				Price:        &price,
				Stock:        &stock,
				WarningStock: 5,
				SafetyStock:  2,
				StockStatus:  "normal",
				RawData:      jsonRaw(map[string]any{"p7RunId": l.runID, "ordinal": i}),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureOrders(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&order.Order{}).Where("order_no LIKE ?", l.orderPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	shopIDs, err := l.shopIDs(ctx)
	if err != nil {
		return err
	}
	return l.batchFrom(int(count), l.plan.Orders, func(start, end int) error {
		rows := make([]order.Order, 0, end-start)
		for i := start; i < end; i++ {
			var sid *uuid.UUID
			if len(shopIDs) > 0 {
				v := shopIDs[i%len(shopIDs)]
				sid = &v
			}
			ordered := l.now.Add(-time.Duration(i%1440) * time.Minute)
			external := fmt.Sprintf("mock-ext-order-%s-%06d", l.runKey, i)
			rows = append(rows, order.Order{
				TenantID:          l.tenant(i),
				Platform:          "mock",
				ShopID:            sid,
				ExternalOrderID:   &external,
				OrderNo:           fmt.Sprintf("%s%06d", l.orderPrefix(), i),
				CustomerName:      fmt.Sprintf("P7 Mock Customer %06d", i),
				CustomerEmail:     fmt.Sprintf("p7-customer-%s-%06d@example.invalid", l.runKey, i),
				CustomerPhone:     fmt.Sprintf("199%08d", i%100000000),
				Status:            orderStatus(i),
				PaymentStatus:     "paid",
				FulfillmentStatus: fulfillmentStatus(i),
				Currency:          "CNY",
				TotalAmount:       float64(3000+i%20000) / 100,
				OrderedAt:         &ordered,
				PlatformUpdatedAt: &ordered,
				RawData:           jsonRaw(map[string]any{"p7RunId": l.runID, "ordinal": i}),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureOrderItems(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&order.OrderItem{}).Where("external_item_id LIKE ?", l.itemPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	orderIDs, err := l.orderIDs(ctx)
	if err != nil {
		return err
	}
	skus, err := l.skuRefs(ctx)
	if err != nil {
		return err
	}
	if len(orderIDs) == 0 || len(skus) == 0 {
		return fmt.Errorf("cannot create order items without orders and skus")
	}
	return l.batchFrom(int(count), l.plan.OrderItems, func(start, end int) error {
		rows := make([]order.OrderItem, 0, end-start)
		for i := start; i < end; i++ {
			s := skus[i%len(skus)]
			externalItem := fmt.Sprintf("%s%07d", l.itemPrefix(), i)
			externalSku := fmt.Sprintf("external-sku-%s-%07d", l.runKey, i)
			qty := 1 + i%4
			unit := float64(1000+i%5000) / 100
			rows = append(rows, order.OrderItem{
				OrderID:        orderIDs[i%len(orderIDs)],
				ProductID:      &s.ProductID,
				ProductSKUID:   &s.ID,
				ExternalItemID: &externalItem,
				ExternalSKUID:  &externalSku,
				SellerSKU:      fmt.Sprintf("%s%06d", l.skuPrefix(), i%len(skus)),
				ProductTitle:   fmt.Sprintf("P7 Mock Product %06d", i%l.plan.Products),
				SKUName:        fmt.Sprintf("P7 SKU %06d", i%len(skus)),
				SKUCode:        fmt.Sprintf("%s%06d", l.skuPrefix(), i%len(skus)),
				Quantity:       qty,
				UnitPrice:      unit,
				TotalPrice:     unit * float64(qty),
				Attrs:          jsonRaw(map[string]any{"p7RunId": l.runID, "ordinal": i}),
				RawData:        jsonRaw(map[string]any{"p7RunId": l.runID}),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureInventoryRows(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&inventory.InventoryChangeLog{}).Where("business_event_key LIKE ?", l.inventoryPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	skus, err := l.skuRefs(ctx)
	if err != nil {
		return err
	}
	if len(skus) == 0 && l.plan.InventoryRows > 0 {
		return fmt.Errorf("cannot create inventory rows without skus")
	}
	return l.batchFrom(int(count), l.plan.InventoryRows, func(start, end int) error {
		rows := make([]inventory.InventoryChangeLog, 0, end-start)
		for i := start; i < end; i++ {
			s := skus[i%len(skus)]
			before := 100 + i%500
			delta := -1 * (1 + i%3)
			rows = append(rows, inventory.InventoryChangeLog{
				TenantID:         l.tenant(i),
				ProductID:        s.ProductID,
				ProductSKUID:     s.ID,
				ChangeType:       "p7_mock_adjustment",
				BeforeStock:      before,
				AfterStock:       before + delta,
				Delta:            delta,
				Reason:           "p7_dataset",
				Remark:           "P7 isolated performance dataset row",
				BusinessEventKey: fmt.Sprintf("%s%07d", l.inventoryPrefix(), i),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureTasks(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&collect.CollectTask{}).Where("source_url LIKE ?", l.taskURLPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	return l.batchFrom(int(count), l.plan.Tasks, func(start, end int) error {
		rows := make([]collect.CollectTask, 0, end-start)
		for i := start; i < end; i++ {
			status := collect.StatusPending
			if i%10 == 0 {
				status = collect.StatusFailed
			} else if i%4 == 0 {
				status = collect.StatusSuccess
			}
			rows = append(rows, collect.CollectTask{
				TenantID:       l.tenant(i),
				Source:         "p7-mock",
				SourceURL:      fmt.Sprintf("%s%07d", l.taskURLPrefix(), i),
				Status:         status,
				RawResult:      jsonRaw(map[string]any{"p7RunId": l.runID, "ordinal": i}),
				RequestOptions: jsonRaw(map[string]any{"provider": "mock", "p7RunId": l.runID}),
				RetryCount:     i % 3,
				MaxRetries:     3,
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureWebhooks(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&webhook.Event{}).Where("event_id LIKE ?", l.webhookPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	shopIDs, err := l.shopIDs(ctx)
	if err != nil {
		return err
	}
	return l.batchFrom(int(count), l.plan.Webhooks, func(start, end int) error {
		rows := make([]webhook.Event, 0, end-start)
		for i := start; i < end; i++ {
			var sid *uuid.UUID
			if len(shopIDs) > 0 {
				v := shopIDs[i%len(shopIDs)]
				sid = &v
			}
			eventID := fmt.Sprintf("%s%07d", l.webhookPrefix(), i)
			rows = append(rows, webhook.Event{
				Platform:       "mock",
				TenantID:       l.tenant(i),
				InternalShopID: sid,
				PlatformShopID: fmt.Sprintf("mock-shop-%s-%05d", l.runKey, i%max(1, l.plan.Shops)),
				AppID:          "p7-mock-app",
				EventID:        eventID,
				EventType:      webhookType(i),
				PayloadHash:    hashShort(eventID),
				PayloadBody:    fmt.Sprintf(`{"p7RunId":%q,"eventId":%q}`, l.runID, eventID),
				Status:         webhookStatus(i),
				RawSummary:     "P7 mock webhook summary",
				Metadata:       jsonRaw(map[string]any{"p7RunId": l.runID, "ordinal": i, "duplicatePlanned": i%25 == 0}),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) ensureOperationLogs(ctx context.Context) error {
	var count int64
	if err := l.db.WithContext(ctx).Model(&operationlog.OperationLog{}).Where("request_id LIKE ?", l.logPrefix()+"%").Count(&count).Error; err != nil {
		return err
	}
	shopIDs, _ := l.shopIDs(ctx)
	return l.batchFrom(int(count), l.plan.OperationLogs, func(start, end int) error {
		rows := make([]operationlog.OperationLog, 0, end-start)
		for i := start; i < end; i++ {
			var sid *uuid.UUID
			if len(shopIDs) > 0 {
				v := shopIDs[i%len(shopIDs)]
				sid = &v
			}
			req := fmt.Sprintf("%s%07d", l.logPrefix(), i)
			rows = append(rows, operationlog.OperationLog{
				TenantID:         l.tenant(i),
				AdminRole:        roleFor(i),
				Username:         fmt.Sprintf("p7-user-%s-%05d", l.runKey, i%max(1, l.plan.Users)),
				Action:           operationAction(i),
				Resource:         operationResource(i),
				ResourceID:       fmt.Sprintf("p7-resource-%s-%07d", l.runKey, i),
				ShopID:           sid,
				Platform:         "mock",
				Permission:       "p7.performance.read",
				Method:           "GET",
				Path:             "/api/v1/p7/mock",
				IPHash:           hashShort(fmt.Sprintf("ip-%s-%d", l.runKey, i)),
				UserAgentSummary: "p7-mock-agent",
				RequestID:        req,
				Status:           "success",
				Message:          "P7 isolated operation log",
				PrevHash:         hashShort(fmt.Sprintf("prev-%s-%d", l.runKey, i)),
				EntryHash:        hashShort(fmt.Sprintf("entry-%s-%d", l.runKey, i)),
				HashVersion:      1,
				ChainPartition:   fmt.Sprintf("tenant:%d", l.tenant(i)),
				CreatedAt:        l.now.Add(time.Duration(i) * time.Millisecond),
			})
		}
		return l.create(ctx, rows)
	})
}

func (l *loader) cleanup(ctx context.Context) error {
	deletes := []func() error{
		func() error {
			return l.db.WithContext(ctx).Where("external_item_id LIKE ?", l.itemPrefix()+"%").Delete(&order.OrderItem{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Where("business_event_key LIKE ?", l.inventoryPrefix()+"%").Delete(&inventory.InventoryChangeLog{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Where("sku_code LIKE ?", l.skuPrefix()+"%").Delete(&product.ProductSKU{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Unscoped().Where("raw_data->>'p7RunId' = ?", l.runID).Delete(&product.Product{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Unscoped().Where("order_no LIKE ?", l.orderPrefix()+"%").Delete(&order.Order{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Where("source_url LIKE ?", l.taskURLPrefix()+"%").Delete(&collect.CollectTask{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Unscoped().Where("event_id LIKE ?", l.webhookPrefix()+"%").Delete(&webhook.Event{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Where("request_id LIKE ?", l.logPrefix()+"%").Delete(&operationlog.OperationLog{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Unscoped().Where("shop_code LIKE ?", l.shopCodePrefix()+"%").Delete(&shop.Shop{}).Error
		},
		func() error {
			return l.db.WithContext(ctx).Unscoped().Where("username LIKE ?", l.userPrefix()+"%").Delete(&admin.AdminUser{}).Error
		},
	}
	for _, del := range deletes {
		if err := del(); err != nil {
			return err
		}
	}
	return nil
}

func (l *loader) counts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	count := func(name string, model any, query string, args ...any) error {
		var n int64
		if err := l.db.WithContext(ctx).Model(model).Where(query, args...).Count(&n).Error; err != nil {
			return err
		}
		out[name] = int(n)
		return nil
	}
	if err := count("shops", &shop.Shop{}, "shop_code LIKE ?", l.shopCodePrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("users", &admin.AdminUser{}, "username LIKE ?", l.userPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("products", &product.Product{}, "raw_data->>'p7RunId' = ?", l.runID); err != nil {
		return nil, err
	}
	if err := count("skus", &product.ProductSKU{}, "sku_code LIKE ?", l.skuPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("orders", &order.Order{}, "order_no LIKE ?", l.orderPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("orderItems", &order.OrderItem{}, "external_item_id LIKE ?", l.itemPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("inventoryRows", &inventory.InventoryChangeLog{}, "business_event_key LIKE ?", l.inventoryPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("tasks", &collect.CollectTask{}, "source_url LIKE ?", l.taskURLPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("webhooks", &webhook.Event{}, "event_id LIKE ?", l.webhookPrefix()+"%"); err != nil {
		return nil, err
	}
	if err := count("operationLogs", &operationlog.OperationLog{}, "request_id LIKE ?", l.logPrefix()+"%"); err != nil {
		return nil, err
	}
	out["tenants"] = l.plan.Tenants
	return out, nil
}

func (l *loader) recordRun(ctx context.Context, rep *report) error {
	summary, _ := json.Marshal(rep)
	now := time.Now().UTC()
	row := performance.TestRun{
		RunID:       l.runID,
		Profile:     rep.Profile,
		Status:      rep.Status,
		DatasetRows: rep.ActualRows,
		StartedAt:   parseTime(rep.StartedAt),
		FinishedAt:  &now,
		Summary:     datatypes.JSON(summary),
	}
	return l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "run_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"profile":      row.Profile,
			"status":       row.Status,
			"dataset_rows": row.DatasetRows,
			"finished_at":  row.FinishedAt,
			"summary":      row.Summary,
			"updated_at":   now,
		}),
	}).Create(&row).Error
}

func (l *loader) batchFrom(existing int, target int, fn func(start, end int) error) error {
	if existing > target {
		return fmt.Errorf("existing rows %d exceed target %d for run %s", existing, target, l.runID)
	}
	for start := existing; start < target; start += l.batchSize {
		end := start + l.batchSize
		if end > target {
			end = target
		}
		if err := fn(start, end); err != nil {
			return err
		}
		l.report.InsertedRows += int64(end - start)
		l.report.BatchCount++
		if l.failAfterBatches > 0 && l.report.BatchCount >= l.failAfterBatches {
			return fmt.Errorf("%w after %d batches", errControlledInterruption, l.report.BatchCount)
		}
		if l.stopAfterRows > 0 && l.report.InsertedRows >= l.stopAfterRows {
			return fmt.Errorf("%w after %d inserted rows", errControlledInterruption, l.report.InsertedRows)
		}
	}
	return nil
}

func (l *loader) create(ctx context.Context, rows any) error {
	return l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.CreateInBatches(rows, l.batchSize).Error
	})
}

func (l *loader) countJSON(ctx context.Context, table string, column string) (int, error) {
	var count int64
	if err := l.db.WithContext(ctx).Table(table).Where(fmt.Sprintf("%s->>'p7RunId' = ?", column), l.runID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (l *loader) productIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := l.db.WithContext(ctx).Model(&product.Product{}).Where("raw_data->>'p7RunId' = ?", l.runID).Order("created_at ASC, id ASC").Pluck("id", &ids).Error
	return ids, err
}

func (l *loader) shopIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := l.db.WithContext(ctx).Model(&shop.Shop{}).Where("shop_code LIKE ?", l.shopCodePrefix()+"%").Order("created_at ASC, id ASC").Pluck("id", &ids).Error
	return ids, err
}

func (l *loader) orderIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := l.db.WithContext(ctx).Model(&order.Order{}).Where("order_no LIKE ?", l.orderPrefix()+"%").Order("created_at ASC, id ASC").Pluck("id", &ids).Error
	return ids, err
}

func (l *loader) skuRefs(ctx context.Context) ([]skuRef, error) {
	var refs []skuRef
	err := l.db.WithContext(ctx).Model(&product.ProductSKU{}).Select("id, product_id").Where("sku_code LIKE ?", l.skuPrefix()+"%").Order("created_at ASC, id ASC").Scan(&refs).Error
	return refs, err
}

func (l *loader) tenant(i int) int64 {
	return int64(1 + (i % max(1, l.plan.Tenants)))
}

func (l *loader) shopCodePrefix() string  { return "p7-" + l.runKey + "-shop-" }
func (l *loader) userPrefix() string      { return "p7" + l.runKey }
func (l *loader) skuPrefix() string       { return "p7-" + l.runKey + "-sku-" }
func (l *loader) orderPrefix() string     { return "P7-" + strings.ToUpper(l.runKey) + "-" }
func (l *loader) itemPrefix() string      { return "p7-" + l.runKey + "-item-" }
func (l *loader) inventoryPrefix() string { return "p7-" + l.runKey + "-inventory-" }
func (l *loader) webhookPrefix() string   { return "p7-" + l.runKey + "-webhook-" }
func (l *loader) logPrefix() string       { return "p7-" + l.runKey + "-log-" }
func (l *loader) taskURLPrefix() string   { return "https://example.invalid/p7/" + l.runKey + "/tasks/" }

func finish(rep *report, status string, err error) {
	rep.Status = status
	if err != nil {
		rep.Issues = append(rep.Issues, err.Error())
	}
	rep.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if rep.StartedAt != "" {
		rep.DurationMs = time.Since(parseTime(rep.StartedAt)).Milliseconds()
	}
	write(*rep)
}

func write(rep report) {
	b, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(b))
}

func jsonRaw(v any) datatypes.JSON {
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

func fingerprint(plan datasetPlan, counts map[string]int) string {
	// The run ID only scopes generated records to an isolated database. It must
	// not affect the dataset-comparability contract for baseline/current runs.
	payload, _ := json.Marshal(map[string]any{"plan": plan, "counts": counts})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func hashShort(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func safeRunKey(runID string) string {
	lower := strings.ToLower(strings.TrimSpace(runID))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	key := strings.Trim(re.ReplaceAllString(lower, ""), "")
	if key == "" {
		key = "run"
	}
	if len(key) > 18 {
		sum := sha256.Sum256([]byte(runID))
		key = key[:10] + hex.EncodeToString(sum[:])[:8]
	}
	return key
}

func boundedBatchSize(v int) int {
	if v < 1 {
		return 500
	}
	if v > 5000 {
		return 5000
	}
	return v
}

func sumCounts(counts map[string]int) int64 {
	var total int64
	for k, v := range counts {
		if k == "tenants" {
			continue
		}
		total += int64(v)
	}
	return total
}

func parseTime(v string) time.Time {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}

func roleFor(i int) string {
	switch i % 3 {
	case 0:
		return "admin"
	case 1:
		return "operator"
	default:
		return "readonly"
	}
}

func orderStatus(i int) string {
	switch i % 5 {
	case 0:
		return "pending"
	case 1:
		return "paid"
	case 2:
		return "shipped"
	case 3:
		return "completed"
	default:
		return "cancelled"
	}
}

func fulfillmentStatus(i int) string {
	switch i % 4 {
	case 0:
		return "unfulfilled"
	case 1:
		return "processing"
	case 2:
		return "shipped"
	default:
		return "delivered"
	}
}

func webhookType(i int) string {
	if i%25 == 0 {
		return "duplicate_order_created"
	}
	if i%3 == 0 {
		return "inventory_updated"
	}
	return "order_created"
}

func webhookStatus(i int) string {
	if i%25 == 0 {
		return webhook.StatusDuplicate
	}
	if i%11 == 0 {
		return webhook.StatusFailedRetryable
	}
	return webhook.StatusQueued
}

func operationAction(i int) string {
	switch i % 4 {
	case 0:
		return "list"
	case 1:
		return "view"
	case 2:
		return "export"
	default:
		return "update"
	}
}

func operationResource(i int) string {
	switch i % 6 {
	case 0:
		return "product"
	case 1:
		return "order"
	case 2:
		return "inventory"
	case 3:
		return "task"
	case 4:
		return "webhook"
	default:
		return "operation_log"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
