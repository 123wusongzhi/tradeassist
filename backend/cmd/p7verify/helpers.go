package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const phase = "P7-C4"

type env struct {
	cfg      *config.Config
	db       *gorm.DB
	tenantID int64
}

type queryCounter struct {
	count atomic.Int64
	token string
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func failReport(status string, issues ...string) {
	writeJSON(map[string]any{
		"phase":       phase,
		"status":      status,
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"guards":      guardList(),
		"issues":      issues,
	})
}

func newGinContext(tenantID int64, userID uuid.UUID) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	if userID != uuid.Nil {
		c.Set(ctxkey.AdminID, userID.String())
	}
	security.SetGin(c, &security.TenantContext{
		TenantID:   tenantID,
		UserID:     userID,
		AuthSource: security.AuthSourceAccessToken,
	})
	return c
}

func resolveTenantID(ctx context.Context, db *gorm.DB) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database unavailable")
	}
	type row struct {
		TenantID int64 `gorm:"column:tenant_id"`
	}
	var r row
	err := db.WithContext(ctx).
		Raw(`SELECT tenant_id FROM products WHERE tenant_id > 0 ORDER BY tenant_id ASC LIMIT 1`).
		Scan(&r).Error
	if err == nil && r.TenantID > 0 {
		return r.TenantID, nil
	}
	err = db.WithContext(ctx).
		Raw(`SELECT tenant_id FROM admin_users WHERE tenant_id > 0 ORDER BY tenant_id ASC LIMIT 1`).
		Scan(&r).Error
	if err == nil && r.TenantID > 0 {
		return r.TenantID, nil
	}
	return 1, nil
}

func resolveAdminUserID(ctx context.Context, db *gorm.DB, tenantID int64) (uuid.UUID, error) {
	type row struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	var r row
	err := db.WithContext(ctx).
		Raw(`SELECT id FROM admin_users WHERE tenant_id = ? ORDER BY created_at ASC LIMIT 1`, tenantID).
		Scan(&r).Error
	if err != nil || r.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("no admin user for tenant %d", tenantID)
	}
	return r.ID, nil
}

func attachQueryCounter(db *gorm.DB) (*queryCounter, func()) {
	if db == nil {
		return &queryCounter{}, func() {}
	}
	qc := &queryCounter{token: uuid.NewString()}
	cb := db.Callback().Query().Before("gorm:query")
	_ = cb.Register("p7verify:count:"+qc.token, func(tx *gorm.DB) {
		if tx != nil && tx.Statement != nil && strings.TrimSpace(tx.Statement.SQL.String()) != "" {
			qc.count.Add(1)
		}
	})
	return qc, func() { _ = cb.Remove("p7verify:count:" + qc.token) }
}

func (q *queryCounter) snapshot() int64 {
	if q == nil {
		return 0
	}
	return q.count.Load()
}

func (q *queryCounter) reset() {
	if q == nil {
		return
	}
	q.count.Store(0)
}

func isGuardError(err error, code string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(code))
}

func openEnv(ctx context.Context) (*env, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if err := validateGuards(cfg); err != nil {
		return nil, err
	}
	db, err := database.Open(cfg)
	if err != nil {
		return nil, err
	}
	tenantID, err := resolveTenantID(ctx, db)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	return &env{cfg: cfg, db: db, tenantID: tenantID}, nil
}
