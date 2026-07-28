package inventorysyncp9

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

func seedAdminUser(t *testing.T, db *gorm.DB, tenantID int64, role string) uuid.UUID {
	t.Helper()
	user := admin.AdminUser{TenantID: tenantID, Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", PasswordHash: "hash", Role: role, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

func TestRBACAuthorizerInventorySyncRunPermissions(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	store, _, _ := seedShopAndSKU(t, db, 801)
	authorizer := NewRBACAuthorizer(db)

	operatorID := seedAdminUser(t, db, 801, adminperm.RoleOperator)
	require.NoError(t, authorizer.CanRunInventorySync(ctx, 801, operatorID, store.ID))

	readonlyID := seedAdminUser(t, db, 801, adminperm.RoleReadonly)
	require.ErrorIs(t, authorizer.CanRunInventorySync(ctx, 801, readonlyID, store.ID), ErrPermissionDenied)
	require.ErrorIs(t, authorizer.CanRunInventorySync(ctx, 802, operatorID, store.ID), ErrPermissionDenied)
	require.ErrorIs(t, authorizer.CanRunInventorySync(ctx, 801, uuid.Nil, store.ID), ErrPermissionDenied)
	require.ErrorIs(t, authorizer.CanRunInventorySync(ctx, 801, operatorID, uuid.New()), ErrNotFound)
}

func TestRBACAuthorizerRerunAndManualBindingPermissions(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	store, _, sku := seedShopAndSKU(t, db, 802)
	run := createRun(t, ctx, db, 802, store.ID)
	snapshot := validSnapshot(802, run, "manual-rbac-sku")
	require.NoError(t, NewInventorySnapshotRepository(db).CreateBatch(ctx, 802, []InventorySnapshotItem{snapshot}))
	stored, err := NewInventorySnapshotRepository(db).GetByRunAndExternalSKU(ctx, 802, run.ID, "manual-rbac-sku")
	require.NoError(t, err)
	request, err := NewManualBindingRequestRepository(db).Create(ctx, &ManualBindingRequest{TenantID: 802, InventorySyncRunID: run.ID, InventorySnapshotItemID: stored.ID, ShopConnectionID: store.ID, ExternalSKUID: stored.ExternalSKUID, Status: ManualBindingStatusPending, ReasonCode: ReasonNoBindingCandidate, CandidateCount: 1, SuggestedLocalSKUID: &sku.ID, RequestID: "manual-rbac-request", IdempotencyKeyHash: testHashB, InputFingerprint: testHashC, Revision: 1})
	require.NoError(t, err)

	authorizer := NewRBACAuthorizer(db)
	operatorID := seedAdminUser(t, db, 802, adminperm.RoleOperator)
	reviewerID := seedAdminUser(t, db, 802, adminperm.RoleReviewer)

	require.NoError(t, authorizer.CanRerunInventorySync(ctx, 802, operatorID, run.ID))
	require.ErrorIs(t, authorizer.CanResolveManualBinding(ctx, 802, operatorID, request.ID), ErrPermissionDenied)
	require.NoError(t, authorizer.CanResolveManualBinding(ctx, 802, reviewerID, request.ID))
	require.ErrorIs(t, authorizer.CanRerunInventorySync(ctx, 803, operatorID, run.ID), ErrPermissionDenied)
	require.ErrorIs(t, authorizer.CanResolveManualBinding(ctx, 803, reviewerID, request.ID), ErrPermissionDenied)
}
