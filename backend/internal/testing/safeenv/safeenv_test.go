package safeenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTestDatabaseURLRequiresTestDatabaseName(t *testing.T) {
	require.NoError(t, ValidateTestDatabaseURL("postgres://trademind:secret@127.0.0.1:5432/trademind_test?sslmode=disable"))
	require.NoError(t, ValidateTestDatabaseURL("postgresql://trademind:secret@127.0.0.1:5432/e2e_trademind?sslmode=disable"))
	require.Error(t, ValidateTestDatabaseURL("postgres://trademind:secret@127.0.0.1:5432/trademind?sslmode=disable"))
	require.Error(t, ValidateTestDatabaseURL("mysql://trademind:secret@127.0.0.1:3306/trademind_test"))
}

func TestValidateTestRedisURLRequiresIsolatedDB(t *testing.T) {
	require.NoError(t, ValidateTestRedisURL("redis://127.0.0.1:6379/15"))
	require.Error(t, ValidateTestRedisURL("redis://127.0.0.1:6379/0"))
	require.Error(t, ValidateTestRedisURL("redis://127.0.0.1:6379"))
	require.Error(t, ValidateTestRedisURL("http://127.0.0.1:6379/15"))
}
