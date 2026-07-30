package observabilitymod

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"gorm.io/gorm"
)

func TestSLOErrorBudgetAndBurnRate(t *testing.T) {
	compliance, remaining, burn, status := calculateSLO(1000, 2, 0.995)
	if status != SLOStatusAchieved {
		t.Fatalf("expected achieved got %s", status)
	}
	if compliance < 0.997 || remaining <= 0 || burn <= 0 {
		t.Fatalf("unexpected slo values compliance=%f remaining=%f burn=%f", compliance, remaining, burn)
	}
	_, remaining, burn, status = calculateSLO(100, 10, 0.99)
	if status != SLOStatusViolated || remaining != 0 || burn <= 1 {
		t.Fatalf("expected violated/exhausted got status=%s remaining=%f burn=%f", status, remaining, burn)
	}
	_, _, _, status = calculateSLO(0, 0, 0.99)
	if status != SLOStatusInsufficientData {
		t.Fatalf("expected insufficient_data got %s", status)
	}
}

func TestEvaluateSLOsWritesSnapshots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.SLODefinition{}, &database.SLOSnapshot{}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDefaultSLOs(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	n, err := EvaluateSLOs(context.Background(), db, cat, map[string]float64{
		"slo:api_availability:total":  1000,
		"slo:api_availability:errors": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected snapshots")
	}
	var snap database.SLOSnapshot
	if err := db.Where("slo_id = ?", "api_availability").First(&snap).Error; err != nil {
		t.Fatal(err)
	}
	if snap.Status != SLOStatusAchieved {
		t.Fatalf("expected achieved got %+v", snap)
	}
}
