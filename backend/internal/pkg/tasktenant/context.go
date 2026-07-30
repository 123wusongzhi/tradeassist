package tasktenant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// TaskScope carries tenant and optional shop for worker tasks.
type TaskScope struct {
	TenantID     int64
	ShopID       uuid.UUID
	ResourceType string
	ResourceID   string
}

// RequireTaskTenant validates task has a positive tenant id.
func RequireTaskTenant(tenantID int64) error {
	if tenantID <= 0 {
		return security.ErrTaskTenantMissing
	}
	return nil
}

// BuildWorkerContext creates tenant context from task scope.
func BuildWorkerContext(scope TaskScope, actorID uuid.UUID, operation string) context.Context {
	ctx := security.WorkerSystemContext(scope.TenantID, actorID, operation)
	tc := security.FromContext(ctx)
	if tc != nil {
		tc.AuthSource = security.AuthSourceWorker
		if scope.ShopID != uuid.Nil {
			tc.ShopScope = []uuid.UUID{scope.ShopID}
		}
		ctx = security.WithTenantContext(ctx, tc)
	}
	return ctx
}

// EnsureResourceTenantMatch rejects when resource tenant differs from task tenant.
func EnsureResourceTenantMatch(taskTenantID int64, resourceTenantID int64) error {
	if err := RequireTaskTenant(taskTenantID); err != nil {
		return err
	}
	if resourceTenantID <= 0 {
		return security.ErrTaskResourceMismatch
	}
	if taskTenantID != resourceTenantID {
		return security.ErrTaskTenantMismatch
	}
	return nil
}

// EnsureShopInScope validates shop is allowed for worker task.
func EnsureShopInScope(taskShopID, resourceShopID uuid.UUID) error {
	if taskShopID == uuid.Nil {
		return nil
	}
	if resourceShopID == uuid.Nil || taskShopID != resourceShopID {
		return security.ErrTaskShopScopeMismatch
	}
	return nil
}

// WrapError returns user-safe task tenant error message.
func WrapError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case err == security.ErrTaskTenantMissing:
		return "任务缺少租户上下文，已停止处理"
	case err == security.ErrTaskTenantMismatch, err == security.ErrTaskResourceMismatch:
		return "资源不属于当前租户"
	case err == security.ErrTaskShopScopeMismatch:
		return "当前账号无权访问此店铺"
	default:
		return fmt.Sprintf("%v", err)
	}
}
