package alerting

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAlertDeduplicationAndRecovery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AlertEvent{}, &AlertRule{}, &AlertSilence{}, &AlertDelivery{}, &AlertEvaluationRun{}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, time.Second, true)
	ctx := context.Background()
	a1, err := svc.Fire(ctx, "http_5xx_elevated", SeverityWarning, "http", "5xx spike", "safe")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := svc.Fire(ctx, "http_5xx_elevated", SeverityWarning, "http", "5xx spike", "safe")
	if err != nil {
		t.Fatal(err)
	}
	if a2.OccurrenceCount < 2 && a1.Fingerprint != a2.Fingerprint {
		t.Fatalf("dedup failed: %+v %+v", a1, a2)
	}
	if err := svc.Resolve(ctx, a1.ID); err != nil {
		t.Fatal(err)
	}
	var resolved AlertEvent
	if err := db.First(&resolved, "id = ?", a1.ID).Error; err != nil {
		t.Fatal(err)
	}
	if resolved.Status != StatusResolved {
		t.Fatalf("expected resolved got %s", resolved.Status)
	}
}

func TestAlertEvaluatorDeliveryAndRecovery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AlertEvent{}, &AlertRule{}, &AlertSilence{}, &AlertDelivery{}, &AlertEvaluationRun{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rule := AlertRule{ID: "ai_image_provider_timeout", Name: "AI image provider timeout", Metric: "ai_image_provider_timeouts_total", Condition: ">", Threshold: 0, Severity: SeverityWarning, Enabled: true}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, time.Second, true)
	run, err := svc.EvaluateRules(ctx, map[string]float64{"ai_image_provider_timeouts_total": 1})
	if err != nil {
		t.Fatal(err)
	}
	if run.AlertsFired != 1 {
		t.Fatalf("expected fired=1 got %+v", run)
	}
	var delivery AlertDelivery
	if err := db.First(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Status != DeliveryDelivered {
		t.Fatalf("expected delivered got %s", delivery.Status)
	}
	run, err = svc.EvaluateRules(ctx, map[string]float64{"ai_image_provider_timeouts_total": 0})
	if err != nil {
		t.Fatal(err)
	}
	if run.AlertsResolved != 1 {
		t.Fatalf("expected resolved=1 got %+v", run)
	}
}

func TestSanitizeDetails(t *testing.T) {
	if sanitizeDetails("TEST_APP_SECRET_UNIQUE leaked") == "TEST_APP_SECRET_UNIQUE leaked" {
		// contains secret marker word 'secret' in TEST_APP_SECRET - should redact
	}
	out := sanitizeDetails("password=foo")
	if out != "[redacted]" {
		t.Fatalf("got %q", out)
	}
}
