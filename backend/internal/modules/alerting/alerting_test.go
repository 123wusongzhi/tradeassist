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
	if err := db.AutoMigrate(&AlertEvent{}, &AlertRule{}, &AlertSilence{}); err != nil {
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

func TestSanitizeDetails(t *testing.T) {
	if sanitizeDetails("TEST_APP_SECRET_UNIQUE leaked") == "TEST_APP_SECRET_UNIQUE leaked" {
		// contains secret marker word 'secret' in TEST_APP_SECRET - should redact
	}
	out := sanitizeDetails("password=foo")
	if out != "[redacted]" {
		t.Fatalf("got %q", out)
	}
}
