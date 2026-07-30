//go:build p9postgres

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
)

func TestP9PostgresAutoMigrateAgainstIsolatedDatabase(t *testing.T) {
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB

	require.NoError(t, database.AutoMigrate(db))
	require.NoError(t, database.AutoMigrate(db))

	for _, table := range []string{
		"admin_users",
		"products",
		"product_skus",
		"p9_inventory_sync_runs",
		"p9_inventory_snapshot_items",
		"p9_sku_bindings",
		"p9_sku_binding_calibrations",
		"p9_manual_binding_requests",
		"p9_manual_binding_decisions",
	} {
		require.Truef(t, db.Migrator().HasTable(table), "expected migrated table %s", table)
	}
}
