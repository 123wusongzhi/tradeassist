package restore

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"gorm.io/gorm"
)

func TestRestoreSafetyGateRejectsUnverifiedBackup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&backup.Job{}, &Job{}, &Validation{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&backup.Job{BackupID: "bk_test", BackupType: backup.TypePostgresLogical, Environment: "test", Status: backup.StatusCompleted, VerificationStatus: backup.VerificationPending, StorageProvider: "local", Checksum: "abc"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db, Cfg: &config.Config{AppEnv: config.EnvDevelopment}}
	_, err = svc.Create(context.Background(), CreateRequest{
		BackupID: "bk_test", TargetEnvironment: "isolated", TargetDatabaseName: "restore_db",
		TargetIsIsolated: true, OperatorReauthenticated: true, HighRiskConfirmed: true,
	}, nil)
	if err == nil {
		t.Fatalf("expected unverified backup rejection")
	}
}
