package postgrestest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var safeIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Harness struct {
	DB               *gorm.DB
	Driver           string
	HostCategory     string
	DatabaseNameHash string
	ServerVersion    string
	SchemaName       string
	SQLiteFallback   bool
}

// Require opens a fail-closed PostgreSQL test connection in an isolated schema.
// It never falls back to SQLite and never logs the connection URL.
func Require(t testing.TB) *Harness {
	t.Helper()
	cfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	require.True(t, ok, "TEST_DATABASE_URL is required for PostgreSQL integration tests")

	schema := uniqueSchemaName()
	isolatedURL, err := databaseURLWithSearchPath(cfg.URL, schema)
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(isolatedURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err, "PostgreSQL connection failed")
	require.Equal(t, "postgres", db.Dialector.Name())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(12)
	sqlDB.SetMaxIdleConns(12)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	var actualDatabase string
	require.NoError(t, db.Raw("SELECT current_database()").Scan(&actualDatabase).Error)
	require.NoError(t, safeenv.ValidateActualTestDatabaseName(cfg.DatabaseName, actualDatabase))

	var serverVersion string
	require.NoError(t, db.Raw("SHOW server_version").Scan(&serverVersion).Error)
	serverVersion = safeServerVersion(serverVersion)

	var readOnly string
	require.NoError(t, db.Raw("SHOW transaction_read_only").Scan(&readOnly).Error)
	require.Equal(t, "off", strings.ToLower(strings.TrimSpace(readOnly)), "PostgreSQL test database must allow isolated test writes")

	require.True(t, safeIdentifier.MatchString(schema))
	require.NoError(t, db.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, quoteIdentifier(schema))).Error)
	require.NoError(t, db.Exec(fmt.Sprintf(`SET search_path TO %s`, quoteIdentifier(schema))).Error)

	t.Cleanup(func() {
		cleanupDB, openErr := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if openErr != nil {
			t.Errorf("PostgreSQL test schema cleanup connection failed: %v", openErr)
		} else {
			if dropErr := cleanupDB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, quoteIdentifier(schema))).Error; dropErr != nil {
				t.Errorf("PostgreSQL test schema cleanup failed: %v", dropErr)
			}
			if cleanupSQL, dbErr := cleanupDB.DB(); dbErr != nil {
				t.Errorf("PostgreSQL cleanup handle failed: %v", dbErr)
			} else if closeErr := cleanupSQL.Close(); closeErr != nil {
				t.Errorf("PostgreSQL cleanup connection close failed: %v", closeErr)
			}
		}
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Errorf("PostgreSQL test connection close failed: %v", closeErr)
		}
	})

	return &Harness{
		DB:               db,
		Driver:           "postgresql",
		HostCategory:     cfg.HostCategory,
		DatabaseNameHash: cfg.DatabaseNameHash,
		ServerVersion:    serverVersion,
		SchemaName:       schema,
		SQLiteFallback:   false,
	}
}

func (h *Harness) EmitMetadata(t testing.TB) {
	t.Helper()
	fmt.Printf("P9PG_META driver=%s hostCategory=%s databaseNameHash=%s serverVersion=%s sqliteFallbackUsed=%t schemaIsolated=%t\n",
		h.Driver,
		h.HostCategory,
		h.DatabaseNameHash,
		h.ServerVersion,
		h.SQLiteFallback,
		h.SchemaName != "",
	)
}

func uniqueSchemaName() string {
	var entropy [8]byte
	_, _ = rand.Read(entropy[:])
	return "p9pg_" + hex.EncodeToString(entropy[:])
}

func databaseURLWithSearchPath(raw, schema string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid PostgreSQL test URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func quoteIdentifier(value string) string {
	if !safeIdentifier.MatchString(value) {
		panic("unsafe PostgreSQL test schema identifier")
	}
	return `"` + value + `"`
}

func safeServerVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ".")
	if len(parts) == 0 {
		return "unknown"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + "." + parts[1]
}
