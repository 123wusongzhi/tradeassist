package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAutoMigrateAgainstIsolatedPostgres(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration migration test")
	}

	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, database.AutoMigrate(db))

	for _, table := range []string{"admin_users", "products", "product_skus", "product_publish_tasks", "inventory_sync_tasks"} {
		require.Truef(t, db.Migrator().HasTable(table), "expected migrated table %s", table)
	}
}
