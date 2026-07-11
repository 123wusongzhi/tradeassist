package security

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// TenantContext carries trusted auth-derived scope for a request.
type TenantContext struct {
	TenantID    int64
	UserID      uuid.UUID
	SessionID   uuid.UUID
	Role        string
	Permissions []string
	ShopScope   []uuid.UUID
	RequestID   string
}

type tenantCtxKey struct{}

var ctxTenantKey = tenantCtxKey{}

// WithTenantContext attaches tenant context to a standard context.
func WithTenantContext(ctx context.Context, tc *TenantContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxTenantKey, tc)
}

// FromContext reads tenant context from context.Context.
func FromContext(ctx context.Context) *TenantContext {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(ctxTenantKey).(*TenantContext)
	return v
}

// FromGin reads tenant context from gin context.
func FromGin(c *gin.Context) *TenantContext {
	if c == nil {
		return nil
	}
	if v, ok := c.Get("security.tenant_context"); ok {
		if tc, ok := v.(*TenantContext); ok {
			return tc
		}
	}
	return nil
}

// SetGin stores tenant context on gin context.
func SetGin(c *gin.Context, tc *TenantContext) {
	if c == nil || tc == nil {
		return
	}
	c.Set("security.tenant_context", tc)
}

// BuildTenantContext constructs tenant context from gin auth keys.
func BuildTenantContext(c *gin.Context, tenantID int64, userID uuid.UUID, sessionID uuid.UUID, role string, perms []string, shopScope []uuid.UUID) *TenantContext {
	reqID, _ := c.Get(ctxkey.TraceID)
	rid, _ := reqID.(string)
	return &TenantContext{
		TenantID:    tenantID,
		UserID:      userID,
		SessionID:   sessionID,
		Role:        role,
		Permissions: perms,
		ShopScope:   shopScope,
		RequestID:   rid,
	}
}

// WorkerTenantContext builds tenant context for background workers.
func WorkerTenantContext(tenantID int64, userID uuid.UUID) *TenantContext {
	return &TenantContext{
		TenantID: tenantID,
		UserID:   userID,
	}
}
