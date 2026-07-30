package inventorysyncp9

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatch1ScopeDoesNotImplementLaterWork(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	forbidden := []string{
		filepath.Join(root, "admin", "src", "pages", "inventorysyncp9"),
		filepath.Join(root, "admin", "src", "services", "inventorysyncp9"),
		filepath.Join(root, "backend", "internal", "api", "inventorysyncp9"),
		filepath.Join(root, "backend", "internal", "modules", "inventorysyncp9", "handler.go"),
		filepath.Join(root, "backend", "internal", "modules", "inventorysyncp9", "service.go"),
		filepath.Join(root, "backend", "internal", "modules", "inventorysyncp9", "worker.go"),
		filepath.Join(root, "backend", "internal", "providers", "douyin", "inventorysyncp9"),
	}
	for _, path := range forbidden {
		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err), "forbidden Batch 1 path exists: %s", path)
	}
}

func TestBatch1ModelsDoNotStoreCredentialFields(t *testing.T) {
	modelSource, err := os.ReadFile("model.go")
	require.NoError(t, err)
	source := strings.ToLower(string(modelSource))
	for _, forbidden := range []string{"accesstoken", "refresh_token", "refreshToken", "appsecret", "cookie", "oauth"} {
		require.NotContains(t, source, strings.ToLower(forbidden))
	}
}
