package database

import (
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"gorm.io/gorm"
	"testing"
)

func TestMigrateOrderExceptionTenantBackfillsTrustedParent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &orderexception.OrderExceptionMark{}))
	o := order.Order{TenantID: 9, Platform: "test", OrderNo: "o"}
	require.NoError(t, db.Create(&o).Error)
	i := order.OrderItem{OrderID: o.ID}
	require.NoError(t, db.Create(&i).Error)
	m := orderexception.OrderExceptionMark{ExceptionType: orderexception.TypeSKUUnmatched, SourceType: orderexception.SourceOrderItem, SourceID: i.ID.String(), MarkType: orderexception.MarkHandled}
	require.NoError(t, db.Create(&m).Error)
	require.NoError(t, migrateOrderExceptionTenantScope(db))
	var got orderexception.OrderExceptionMark
	require.NoError(t, db.First(&got, "id = ?", m.ID).Error)
	require.Equal(t, int64(9), got.TenantID)
}
