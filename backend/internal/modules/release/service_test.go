package release

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

func TestRollbackNeverRestoresDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&Run{}, &Step{}, &Rollback{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db, Cfg: &config.Config{AppEnv: config.EnvDevelopment, Release: config.ReleaseConfig{Strategy: "blue_green", HealthTimeoutSeconds: 30, KeepCount: 2}}}
	run, err := svc.Create(context.Background(), CreateRequest{Version: "v-test", GitCommit: "abc"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	run.State = StateCompleted
	if err := db.Save(run).Error; err != nil {
		t.Fatal(err)
	}
	rb, err := svc.Rollback(context.Background(), run.ReleaseID, RollbackRequest{Reason: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rb.DatabaseRestore {
		t.Fatalf("application rollback must not restore database")
	}
}
