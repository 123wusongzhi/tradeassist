package files

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

func TestStaticOnlyServesCleanRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FileRecord{}, &settings.Setting{}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for k, v := range map[string]string{"kind": "local", "local_root": root} {
		if err := db.Create(&settings.Setting{GroupKey: "storage", ItemKey: k, ItemValue: v, ValueType: "string"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	key := "t7/image.png"
	path := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{SecurityPendingScan, SecurityRejected} {
		p := filepath.Join(root, filepath.FromSlash(key+suffix))
		if err := os.WriteFile(p, []byte("unscanned"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{SecurityPendingScan, SecurityRejected} {
		if err := db.Create(&FileRecord{TenantID: 7, ObjectKey: key + status, OriginalName: "x", PublicURL: "", SecurityStatus: status, ScanStatus: status, StorageKind: "local"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&FileRecord{TenantID: 7, ObjectKey: key, OriginalName: "x", PublicURL: "/static/" + key, SecurityStatus: SecurityClean, ScanStatus: SecurityClean, StorageKind: "local"}).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/static/*filepath", (&StaticHandler{Settings: &settings.Service{DB: db}, DB: db}).Serve)
	for _, got := range []struct {
		path string
		want int
	}{{"/static/t7/image.pngpending_scan", 404}, {"/static/t7/image.pngrejected", 404}, {"/static/" + key, 200}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, got.path, nil))
		if w.Code != got.want {
			t.Fatalf("%s: got %d want %d", got.path, w.Code, got.want)
		}
	}
}
