package orderexception

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

func exceptionTenantDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_exception_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderItemSKUMatch{}, &inventory.OrderInventoryEffect{}, &OrderExceptionMark{}))
	return db
}

func TestOrderExceptionTenantListDetailAndMarks(t *testing.T) {
	db := exceptionTenantDB(t)
	ctx := context.Background()
	svc := &Service{DB: db}
	one := order.Order{TenantID: 11, Platform: "test", OrderNo: "one"}
	two := order.Order{TenantID: 22, Platform: "test", OrderNo: "two"}
	require.NoError(t, db.Create(&one).Error)
	require.NoError(t, db.Create(&two).Error)
	i1 := order.OrderItem{OrderID: one.ID, SKUCode: "a", Quantity: 1}
	i2 := order.OrderItem{OrderID: two.ID, SKUCode: "b", Quantity: 1}
	require.NoError(t, db.Create(&i1).Error)
	require.NoError(t, db.Create(&i2).Error)

	// The list aggregation is assembled from several source tables; its final
	// tenant gate must retain only rows whose parent order belongs to tenant 11.
	rows, err := svc.tenantRows(ctx, 11, []aggRow{{orderID: one.ID, sourceID: i1.ID}, {orderID: two.ID, sourceID: i2.ID}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, i1.ID, rows[0].sourceID)
	_, err = svc.GetOrderExceptionDetailForTenant(ctx, 11, SourceOrderItem, i2.ID.String())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, svc.UpsertMarkForTenant(ctx, 11, TypeSKUUnmatched, SourceOrderItem, i1.ID.String(), MarkHandled, "ok", nil))
	var marks []OrderExceptionMark
	require.NoError(t, db.Find(&marks).Error)
	require.Len(t, marks, 1)
	require.Equal(t, int64(11), marks[0].TenantID)
	require.Error(t, svc.UpsertMarkForTenant(ctx, 11, TypeSKUUnmatched, SourceOrderItem, i2.ID.String(), MarkHandled, "foreign", nil))
	require.NoError(t, db.Find(&marks).Error)
	require.Len(t, marks, 1)
}
