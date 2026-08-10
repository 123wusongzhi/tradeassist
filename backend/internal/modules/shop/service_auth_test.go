package shop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
	"gorm.io/gorm"
)

type shopConnectionFakeProvider struct {
	result  *platformp.TestConnectionResult
	err     error
	lastReq platformp.TestConnectionRequest
}

func (*shopConnectionFakeProvider) Platform() string { return ozonPlatform }
func (*shopConnectionFakeProvider) Name() string     { return "Ozon test fake" }
func (*shopConnectionFakeProvider) Status() string   { return platformp.StatusAvailable }
func (*shopConnectionFakeProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{platformp.CapProductPublish}
}
func (*shopConnectionFakeProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "api_key",
		Fields: []platformp.AuthField{
			{Name: "appKey", Required: true},
			{Name: "accessToken", Required: true, Sensitive: true},
		},
	}
}
func (*shopConnectionFakeProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PlatformAppConfigSchema{}
}
func (*shopConnectionFakeProvider) PublishConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PlatformAppConfigSchema{}
}
func (p *shopConnectionFakeProvider) TestConnection(_ context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	p.lastReq = req
	return p.result, p.err
}

func TestOzonAuthRequiresVerifiedConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	platformozon.RegisterProvider()
	originalProvider := platformp.Get(ozonPlatform)
	fakeProvider := &shopConnectionFakeProvider{}
	platformp.Register(fakeProvider)
	t.Cleanup(func() {
		if originalProvider != nil {
			platformp.Register(originalProvider)
		}
	})

	db, err := gorm.Open(sqlite.Open("file:shop_auth_truth?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &Shop{}, &ShopAuthToken{}, &operationlog.OperationLog{}))

	adminID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{
		Base:         model.Base{ID: adminID},
		TenantID:     7,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         adminperm.RoleTenantAdmin,
		Status:       admin.StatusActive,
	}).Error)
	shopRow := Shop{
		Base:       model.Base{ID: uuid.New()},
		TenantID:   7,
		Platform:   ozonPlatform,
		ShopName:   "Unverified Ozon",
		Status:     StatusActive,
		AuthStatus: AuthAuthorized,
	}
	require.NoError(t, db.Create(&shopRow).Error)

	encrypter, err := encrypt.NewService("shop-auth-test-master-key")
	require.NoError(t, err)
	svc := &Service{DB: db, Encrypter: encrypter, OpLog: &operationlog.Service{DB: db}}
	newContext := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/shops/"+shopRow.ID.String()+"/auth", nil)
		c.Set(ctxkey.AdminID, adminID.String())
		c.Set(ctxkey.TenantID, int64(7))
		return c
	}

	_, err = svc.UpdateAuth(newContext(), shopRow.ID, UpdateAuthBody{
		AuthType:    "api_key",
		AppKey:      "client-1",
		AccessToken: "test-api-key",
	}, &adminID)
	require.NoError(t, err)
	require.NoError(t, db.First(&shopRow, "id = ?", shopRow.ID).Error)
	require.Equal(t, AuthUnauthorized, shopRow.AuthStatus, "saving credentials must not imply a verified connection")

	var stored ShopAuthToken
	require.NoError(t, db.First(&stored, "shop_id = ?", shopRow.ID).Error)
	require.NotEqual(t, "test-api-key", stored.AccessTokenEnc)
	plain, err := encrypter.Decrypt(stored.AccessTokenEnc)
	require.NoError(t, err)
	require.Equal(t, "test-api-key", string(plain))
	previousCipher := stored.AccessTokenEnc

	// Blank sensitive values mean "leave unchanged" after the first save. The
	// required schema still rejects a missing secret when no stored value exists.
	_, err = svc.UpdateAuth(newContext(), shopRow.ID, UpdateAuthBody{AuthType: "api_key"}, &adminID)
	require.NoError(t, err)
	require.NoError(t, db.First(&stored, "shop_id = ?", shopRow.ID).Error)
	require.Equal(t, "client-1", stored.AppKey)
	require.Equal(t, previousCipher, stored.AccessTokenEnc)
	emptyShop := Shop{
		Base:       model.Base{ID: uuid.New()},
		TenantID:   7,
		Platform:   ozonPlatform,
		ShopName:   "No credentials",
		Status:     StatusActive,
		AuthStatus: AuthUnauthorized,
	}
	require.NoError(t, db.Create(&emptyShop).Error)
	_, err = svc.UpdateAuth(newContext(), emptyShop.ID, UpdateAuthBody{
		AuthType:    "api_key",
		AppKey:      "client-2",
		AccessToken: "****",
	}, &adminID)
	require.ErrorContains(t, err, "field required: accessToken")

	fakeProvider.result = &platformp.TestConnectionResult{OK: false, Message: "credential rejected"}
	result, err := svc.TestConnection(newContext(), shopRow.ID, &adminID)
	require.NoError(t, err)
	require.False(t, result.OK)
	require.Equal(t, "client-1", fakeProvider.lastReq.AppKey)
	require.Equal(t, "test-api-key", fakeProvider.lastReq.AccessToken)
	require.NoError(t, db.First(&shopRow, "id = ?", shopRow.ID).Error)
	require.Equal(t, AuthInvalid, shopRow.AuthStatus)

	var failedLog operationlog.OperationLog
	require.NoError(t, db.Where("resource_id = ? AND action = ?", shopRow.ID.String(), "shop.test_connection.failed").Last(&failedLog).Error)
	require.Equal(t, "failed", failedLog.Status)
	require.NotContains(t, failedLog.Message, "test-api-key")

	// The HTTP contract must also be a failed envelope; a provider-level false
	// result cannot be rendered as a successful Admin toast.
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, adminID.String())
		c.Set(ctxkey.TenantID, int64(7))
	})
	router.POST("/api/v1/shops/:id/test-connection", (&Handler{Svc: svc}).TestConnection)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/shops/"+shopRow.ID.String()+"/test-connection", strings.NewReader(`{}`)))
	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, response.CodeBadRequest, envelope.Code)
	require.Equal(t, "credential rejected", envelope.Message)
	require.Equal(t, false, envelope.Data.(map[string]any)["ok"])

	fakeProvider.result = &platformp.TestConnectionResult{
		OK:             true,
		Message:        "ozon connection ok",
		ShopName:       "Verified Ozon Shop",
		ExternalShopID: "client-1",
		Region:         "RU",
		Currency:       "rub",
	}
	result, err = svc.TestConnection(newContext(), shopRow.ID, &adminID)
	require.NoError(t, err)
	require.True(t, result.OK)
	require.NoError(t, db.First(&shopRow, "id = ?", shopRow.ID).Error)
	require.Equal(t, AuthAuthorized, shopRow.AuthStatus)
	require.Equal(t, "Verified Ozon Shop", shopRow.ShopName)
	require.Equal(t, "client-1", shopRow.ExternalShopID)
	require.Equal(t, "RU", shopRow.Region)
	require.Equal(t, "RUB", shopRow.Currency)

	var successLog operationlog.OperationLog
	require.NoError(t, db.Where("resource_id = ? AND action = ?", shopRow.ID.String(), "shop.test_connection.success").Last(&successLog).Error)
	require.Equal(t, "success", successLog.Status)
}
