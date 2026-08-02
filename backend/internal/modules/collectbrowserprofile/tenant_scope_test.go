package collectbrowserprofile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func profileTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CollectBrowserProfile{}))
	return &Service{DB: db}, db
}

func profileTestContext(tenantID int64) context.Context {
	return security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: tenantID, AuthSource: security.AuthSourceWorker})
}

func TestProfileServiceScopesListAndInternalProfileResolution(t *testing.T) {
	svc, db := profileTestService(t)
	a := CollectBrowserProfile{TenantID: 11, Name: "a", Domain: "example.com", ProfileKey: "a", Status: StatusActive}
	b := CollectBrowserProfile{TenantID: 22, Name: "b", Domain: "example.com", ProfileKey: "b", Status: StatusActive}
	require.NoError(t, db.Create(&a).Error)
	require.NoError(t, db.Create(&b).Error)

	rows, total, err := svc.List(profileTestContext(11), ListQuery{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, a.ID, rows[0].ID)

	err = svc.EnrichCollectorOptions(profileTestContext(11), map[string]any{}, &b.ID, true, "https://example.com/item")
	require.ErrorIs(t, err, ErrProfileNotFound)
	_, err = svc.MergeIntoRequestOptions(context.Background(), nil, &a.ID, true, "https://example.com/item")
	require.Error(t, err)
}

func TestProfileHTTPFailsClosedAndStampsTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := profileTestService(t)
	h := &Handler{Svc: svc}

	noTenant, _ := gin.CreateTestContext(httptest.NewRecorder())
	noTenant.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	h.List(noTenant)
	require.Equal(t, http.StatusForbidden, noTenant.Writer.Status())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"p","domain":"example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	security.SetGin(c, &security.TenantContext{TenantID: 33, AuthSource: security.AuthSourceAccessToken})
	h.Create(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var row CollectBrowserProfile
	require.NoError(t, db.First(&row).Error)
	require.Equal(t, int64(33), row.TenantID)
}

func TestProfileByIDRejectsForeignTenant(t *testing.T) {
	svc, db := profileTestService(t)
	row := CollectBrowserProfile{TenantID: 8, Name: "p", Domain: "example.com", ProfileKey: uuid.NewString(), Status: StatusActive}
	require.NoError(t, db.Create(&row).Error)
	_, err := svc.byID(profileTestContext(9), row.ID)
	require.True(t, errors.Is(err, ErrProfileNotFound))
}

func TestSystemTenantProfilesStayExplicitlyScoped(t *testing.T) {
	svc, db := profileTestService(t)
	row := CollectBrowserProfile{TenantID: 0, Name: "legacy", Domain: "example.com", ProfileKey: uuid.NewString(), Status: StatusActive}
	require.NoError(t, db.Create(&row).Error)
	got, err := svc.byID(profileTestContext(0), row.ID)
	require.NoError(t, err)
	require.Equal(t, row.ID, got.ID)
	_, err = svc.byID(profileTestContext(9), row.ID)
	require.ErrorIs(t, err, ErrProfileNotFound)
}
