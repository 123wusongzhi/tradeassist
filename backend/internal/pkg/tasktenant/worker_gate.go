package tasktenant

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BeginWorker builds a tenant-scoped worker context from explicit tenant and optional shop.
// When tenantID <= 0 and shopID is set, tenant is resolved from the shop row.
func BeginWorker(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, operation string) (context.Context, TaskScope, error) {
	tid := tenantID
	if tid <= 0 && shopID != uuid.Nil && db != nil {
		resolved, err := ResolveShopTenant(ctx, db, shopID)
		if err != nil {
			return ctx, TaskScope{}, err
		}
		tid = resolved
	}
	if err := RequireTaskTenant(tid); err != nil {
		return ctx, TaskScope{}, err
	}
	scope := TaskScope{TenantID: tid, ShopID: shopID}
	wctx := BuildWorkerContext(scope, uuid.Nil, operation)
	return wctx, scope, nil
}

// BeginWorkerFromShop resolves tenant from shop and returns worker context.
func BeginWorkerFromShop(ctx context.Context, db *gorm.DB, shopID uuid.UUID, operation string) (context.Context, TaskScope, error) {
	return BeginWorker(ctx, db, 0, shopID, operation)
}
