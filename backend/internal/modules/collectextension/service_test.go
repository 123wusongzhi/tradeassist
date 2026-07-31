package collectextension

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"gorm.io/gorm"
)

func openBrowserExtensionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:collect_extension_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&BrowserExtensionPairing{}, &BrowserExtensionDevice{}))
	return db
}

func TestPairingExchangeIsOneTimeAndDeviceIsRevocable(t *testing.T) {
	db := openBrowserExtensionServiceTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	adminID := uuid.New()

	pairing, err := svc.CreatePairing(ctx, 42, adminID)
	require.NoError(t, err)
	require.Len(t, pairing.Code, 11)

	exchanged, err := svc.ExchangePairing(ctx, pairing.Code, "My Chrome")
	require.NoError(t, err)
	require.Equal(t, "My Chrome", exchanged.Device.Name)
	require.Equal(t, DeviceStatusActive, exchanged.Device.Status)
	require.Contains(t, exchanged.DeviceToken, "tmx_")

	_, err = svc.ExchangePairing(ctx, pairing.Code, "Second browser")
	require.ErrorIs(t, err, ErrPairingInvalid)

	device, err := svc.AuthenticateDevice(ctx, exchanged.DeviceToken)
	require.NoError(t, err)
	require.Equal(t, int64(42), device.TenantID)
	require.Equal(t, adminID, device.AdminUserID)

	devices, err := svc.ListDevices(ctx, 42)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, exchanged.Device.ID, devices[0].ID)

	require.NoError(t, svc.RevokeDevice(ctx, 42, exchanged.Device.ID))
	_, err = svc.AuthenticateDevice(ctx, exchanged.DeviceToken)
	require.ErrorIs(t, err, ErrDeviceUnauthorized)
	require.ErrorIs(t, svc.RevokeDevice(ctx, 42, exchanged.Device.ID), ErrDeviceAlreadyRevoked)
}

func TestPairingAndRevokeStayTenantScoped(t *testing.T) {
	db := openBrowserExtensionServiceTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	pairing, err := svc.CreatePairing(ctx, 7, uuid.New())
	require.NoError(t, err)
	exchanged, err := svc.ExchangePairing(ctx, pairing.Code, "Tenant seven")
	require.NoError(t, err)

	require.ErrorIs(t, svc.RevokeDevice(ctx, 8, exchanged.Device.ID), gorm.ErrRecordNotFound)
	devices, err := svc.ListDevices(ctx, 8)
	require.NoError(t, err)
	require.Empty(t, devices)

	_, err = svc.AuthenticateDevice(ctx, exchanged.DeviceToken)
	require.NoError(t, err)
}

func TestExpiredDeviceIsReportedAndCannotAuthenticate(t *testing.T) {
	db := openBrowserExtensionServiceTestDB(t)
	svc := &Service{DB: db}
	rawToken := "tmx_expired_device_token_abcdefghijklmnopqrstuvwxyz"
	device := &BrowserExtensionDevice{
		TenantID:    5,
		AdminUserID: uuid.New(),
		Name:        "Expired Chrome",
		TokenHash:   authutil.HashToken(rawToken, ""),
		Status:      DeviceStatusActive,
		ExpiresAt:   time.Now().UTC().Add(-time.Minute),
	}
	require.NoError(t, db.Create(device).Error)

	devices, err := svc.ListDevices(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, DeviceStatusExpired, devices[0].Status)

	_, err = svc.AuthenticateDevice(context.Background(), rawToken)
	require.ErrorIs(t, err, ErrDeviceUnauthorized)
}
