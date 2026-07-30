package safeenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTestDatabaseURLRequiresBoundedSafeTestDatabaseName(t *testing.T) {
	require.NoError(t, ValidateTestDatabaseURL("postgres://trademind:secret@127.0.0.1:5432/trademind_test?sslmode=disable"))
	require.NoError(t, ValidateTestDatabaseURL("postgresql://trademind:secret@127.0.0.1:5432/e2e_trademind?sslmode=disable"))
	require.NoError(t, ValidateTestDatabaseURL("postgresql://trademind:secret@127.0.0.1:5432/trademind-ci-test?sslmode=disable"))
	for _, raw := range []string{
		"postgres://trademind:secret@127.0.0.1:5432/trademind?sslmode=disable",
		"postgres://trademind:secret@127.0.0.1:5432/contest?sslmode=disable",
		"postgres://trademind:secret@127.0.0.1:5432/trademind_production_test?sslmode=disable",
		"postgres://trademind:secret@127.0.0.1:5432/staging_test?sslmode=disable",
		"mysql://trademind:secret@127.0.0.1:3306/trademind_test",
	} {
		require.Error(t, ValidateTestDatabaseURL(raw), raw)
	}
}

func TestValidateActualTestDatabaseNameMustMatchURLTarget(t *testing.T) {
	require.NoError(t, ValidateActualTestDatabaseName("trademind_test", "trademind_test"))
	require.Error(t, ValidateActualTestDatabaseName("trademind_test", "trademind"))
	require.Error(t, ValidateActualTestDatabaseName("trademind_test", "other_test"))
}

func TestDatabaseConfigRequiresTestEnvironment(t *testing.T) {
	t.Setenv("TEST_DATABASE_URL", "postgresql://trademind:secret@127.0.0.1:5432/trademind_test?sslmode=disable")
	for _, environment := range []string{"", "development", "staging", "production"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv("APP_ENV", environment)
			_, ok, err := TestDatabaseURLFromEnv()
			require.True(t, ok)
			require.EqualError(t, err, "APP_ENV=test is required for PostgreSQL integration tests")
		})
	}
}

func TestValidateTestDatabaseURLRejectsRemoteHostWithoutExplicitOptIn(t *testing.T) {
	const raw = "postgresql://trademind:secret@postgres.test.internal:5432/trademind_test?sslmode=require"
	t.Setenv("P9_POSTGRES_ALLOW_REMOTE_TEST_HOST", "")
	require.EqualError(t, ValidateTestDatabaseURL(raw), "remote PostgreSQL test host requires P9_POSTGRES_ALLOW_REMOTE_TEST_HOST=1")
	t.Setenv("P9_POSTGRES_ALLOW_REMOTE_TEST_HOST", "1")
	require.NoError(t, ValidateTestDatabaseURL(raw))
}

func TestDatabaseConfigContainsOnlySafeMetadata(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("TEST_DATABASE_URL", "postgresql://trademind:secret@127.0.0.1:5432/trademind_test?sslmode=disable")
	cfg, ok, err := TestDatabaseURLFromEnv()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "postgresql", cfg.Driver)
	require.Equal(t, "local", cfg.HostCategory)
	require.Equal(t, "trademind_test", cfg.DatabaseName)
	require.Len(t, cfg.DatabaseNameHash, 64)
	require.NotContains(t, cfg.DatabaseNameHash, "secret")
}

func TestValidateTestRedisURLRequiresIsolatedDB(t *testing.T) {
	require.NoError(t, ValidateTestRedisURL("redis://127.0.0.1:6379/15"))
	require.Error(t, ValidateTestRedisURL("redis://127.0.0.1:6379/0"))
	require.Error(t, ValidateTestRedisURL("redis://127.0.0.1:6379"))
	require.Error(t, ValidateTestRedisURL("http://127.0.0.1:6379/15"))
}
