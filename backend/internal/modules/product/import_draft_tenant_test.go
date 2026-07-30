package product

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func newImportDraftTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Product{}, &ProductImage{}, &ProductSKU{}))
	return db
}

func importDraftTenantParams() ImportDraftParams {
	return ImportDraftParams{
		Source:     "taobao_tmall",
		SourceURL:  "https://detail.tmall.com/item.htm?id=1",
		Title:      "租户归属回归测试商品",
		MainImages: []string{"https://img.example.com/main-1.jpg"},
	}
}

func requirePersistedTenantID(t *testing.T, db *gorm.DB, id uuid.UUID, want int64) {
	t.Helper()
	var got Product
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, want, got.TenantID)
}

// 回归：HTTP 入口必须按 gin 上下文租户写入商品，避免落库 tenant_id=0 后被租户隔离查询 404。
func TestImportDraftStampsTenantFromGinContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newImportDraftTenantTestDB(t)
	svc := &Service{DB: db}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/products/import", nil)
	c.Set(ctxkey.TenantID, int64(5))

	out, err := svc.ImportDraft(c, nil, importDraftTenantParams())
	require.NoError(t, err)
	requirePersistedTenantID(t, db, out.ID, 5)
}

// 回归：worker 入口必须从任务租户上下文写入商品（采集 worker 经 tasktenant.BeginWorker 注入）。
func TestImportDraftWithContextStampsTenantFromWorkerContext(t *testing.T) {
	db := newImportDraftTenantTestDB(t)
	svc := &Service{DB: db}
	ctx := security.WorkerSystemContext(9, uuid.Nil, "collect")

	out, err := svc.ImportDraftWithContext(ctx, nil, importDraftTenantParams())
	require.NoError(t, err)
	requirePersistedTenantID(t, db, out.ID, 9)
}

// 显式传入的 TenantID 优先于上下文，不允许被覆盖。
func TestImportDraftWithContextKeepsExplicitTenant(t *testing.T) {
	db := newImportDraftTenantTestDB(t)
	svc := &Service{DB: db}
	ctx := security.WorkerSystemContext(9, uuid.Nil, "collect")

	p := importDraftTenantParams()
	p.TenantID = 3
	out, err := svc.ImportDraftWithContext(ctx, nil, p)
	require.NoError(t, err)
	requirePersistedTenantID(t, db, out.ID, 3)
}

// HTTP 入口缺少租户上下文时必须报错，而不是静默落库 tenant_id=0。
func TestImportDraftFailsWithoutTenantContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newImportDraftTenantTestDB(t)
	svc := &Service{DB: db}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/products/import", nil)

	_, err := svc.ImportDraft(c, nil, importDraftTenantParams())
	require.Error(t, err)
}
