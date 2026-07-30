package security

import (
	"context"

	"github.com/google/uuid"
)

// Auth source constants.
const (
	AuthSourceAccessToken = "access_token"
	AuthSourceWebhook     = "webhook"
	AuthSourceWorker      = "worker"
	AuthSourceSystem      = "system"
	AuthSourceDevFallback = "dev_tenant_fallback"
)

// SystemContext identifies privileged internal operations (not a tenant impersonation).
type SystemContext struct {
	Operation string
	ActorID   uuid.UUID
	RequestID string
}

type systemCtxKey struct{}

var ctxSystemKey = systemCtxKey{}

// WithSystemContext attaches an explicit system context.
func WithSystemContext(ctx context.Context, sc *SystemContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxSystemKey, sc)
}

// SystemFromContext reads system context.
func SystemFromContext(ctx context.Context) *SystemContext {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(ctxSystemKey).(*SystemContext)
	return v
}

// WorkerSystemContext builds a worker tenant context with system auth source.
func WorkerSystemContext(tenantID int64, userID uuid.UUID, operation string) context.Context {
	tc := &TenantContext{
		TenantID:   tenantID,
		UserID:     userID,
		AuthSource: AuthSourceWorker,
	}
	ctx := WithTenantContext(context.Background(), tc)
	return WithSystemContext(ctx, &SystemContext{Operation: operation})
}
