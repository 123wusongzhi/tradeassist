package webhook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:webhook_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&idempotency.Record{}, &webhook.Event{}))
	return db
}

func testService(t *testing.T, enableTest bool) (*webhook.Service, *gorm.DB) {
	t.Helper()
	db := openWebhookTestDB(t)
	cfg := &config.Config{
		AppEnv:                     config.EnvDevelopment,
		WebhookEnableTestVerifier:  enableTest,
		WebhookMaxBodyKB:           512,
		WebhookMaxClockSkewSeconds: 300,
	}
	svc := &webhook.Service{
		DB:              db,
		Idempotency:     &idempotency.Service{DB: db},
		Verifiers:       webhook.NewRegistry(cfg),
		MaxPayloadBytes: cfg.WebhookMaxBodyBytes(),
		MaxClockSkew:    cfg.WebhookMaxClockSkew(),
		AppEnv:          cfg.AppEnv,
		Now:             func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	}
	return svc, db
}

func signedRequest(t *testing.T, svc *webhook.Service, platform, eventType string, body []byte, ts time.Time, sig string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &webhook.Handler{Svc: svc}
	webhook.RegisterPublic(r.Group("/api/v1"), h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/"+platform+"/"+eventType, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if !ts.IsZero() {
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", ts.Unix()))
	}
	if sig != "" {
		req.Header.Set("X-Webhook-Signature", sig)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWebhookValidSignature(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"eventId":"evt-1","hello":"world"}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "order.created", body, ts, sig)
	require.Equal(t, http.StatusOK, w.Code)
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, float64(0), env["code"])
	require.Equal(t, "accepted", env["message"])
	data := env["data"].(map[string]any)
	require.Equal(t, "evt-1", data["eventId"])
	require.Equal(t, false, data["duplicate"])
}

func TestWebhookMissingSignature(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"eventId":"evt-miss"}`)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeSignatureMissing)
}

func TestWebhookInvalidSignature(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"eventId":"evt-bad"}`)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, "deadbeef")
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeSignatureInvalid)
}

func TestWebhookExpiredTimestamp(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now().Add(-10 * time.Minute)
	body := []byte(`{"eventId":"evt-old"}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeTimestampExpired)
}

func TestWebhookFutureTimestamp(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now().Add(10 * time.Minute)
	body := []byte(`{"eventId":"evt-future"}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeTimestampExpired)
}

func TestWebhookDuplicateEventID(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"eventId":"evt-dup","n":1}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w1 := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusOK, w1.Code)
	w2 := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusOK, w2.Code)
	var env map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &env))
	data := env["data"].(map[string]any)
	require.Equal(t, true, data["duplicate"])
}

func TestWebhookSamePayloadNoEventID(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"hello":"no-id"}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w1 := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusOK, w1.Code)
	w2 := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusOK, w2.Code)
	var env map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &env))
	data := env["data"].(map[string]any)
	require.Equal(t, true, data["duplicate"])
	require.NotEmpty(t, data["eventId"])
}

func TestWebhookOversizedBody(t *testing.T) {
	svc, _ := testService(t, true)
	svc.MaxPayloadBytes = 64
	ts := svc.Now()
	body := []byte(`{"eventId":"big","pad":"` + strings.Repeat("x", 200) + `"}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodePayloadTooLarge)
}

func TestWebhookInvalidJSON(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{not-json`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeInvalidJSON)
}

func TestWebhookVerifierNotConfigured(t *testing.T) {
	svc, _ := testService(t, false)
	ts := svc.Now()
	body := []byte(`{"eventId":"x"}`)
	w := signedRequest(t, svc, "unknown-platform", "ping", body, ts, "abc")
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeVerifierNotConfigured)
}

func TestWebhookProductionBlocksTestVerifier(t *testing.T) {
	db := openWebhookTestDB(t)
	cfg := &config.Config{
		AppEnv:                    config.EnvProduction,
		WebhookEnableTestVerifier: true,
	}
	reg := webhook.NewRegistry(cfg)
	err := reg.Verify(context.Background(), webhook.VerifyInput{
		Platform:  webhook.PlatformInternalTest,
		RawBody:   []byte(`{}`),
		Timestamp: time.Now(),
		Signature: "x",
	})
	require.Error(t, err)
	ce, ok := webhook.AsCodeError(err)
	require.True(t, ok)
	require.Equal(t, webhook.CodeSignatureBypassForbidden, ce.Code)
	_ = db
}

func TestWebhookConcurrentSameEvent(t *testing.T) {
	svc, db := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"eventId":"evt-concurrent","v":1}`)
	sig := webhook.SignTestPayload(nil, ts, body)

	const n = 20
	var wg sync.WaitGroup
	var okCount int32
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
			if w.Code == http.StatusOK {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	require.GreaterOrEqual(t, okCount, int32(1))

	var count int64
	require.NoError(t, db.Model(&webhook.Event{}).Where("platform = ? AND event_id = ?", webhook.PlatformInternalTest, "evt-concurrent").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestWebhookPersistFailureDoesNotACK(t *testing.T) {
	svc, db := testService(t, true)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close()) // force persist failure

	ts := svc.Now()
	body := []byte(`{"eventId":"evt-persist-fail"}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NotContains(t, w.Body.String(), `"message":"accepted"`)
}

func TestWebhookProcessQueuedEvents(t *testing.T) {
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"eventId":"evt-proc","x":1}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusOK, w.Code)

	n, err := svc.ProcessQueuedEvents(context.Background(), 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)

	ev, err := svc.LoadEventByPlatformEventID(context.Background(), webhook.PlatformInternalTest, "evt-proc")
	require.NoError(t, err)
	require.Equal(t, webhook.StatusProcessed, ev.Status)
}

func TestWebhookIngestServiceTimestamp(t *testing.T) {
	svc, _ := testService(t, true)
	_, err := svc.Ingest(context.Background(), webhook.IngestRequest{
		Platform:  "manual",
		EventID:   "e1",
		Payload:   json.RawMessage(`{"a":1}`),
		Timestamp: svc.Now().Add(-1 * time.Hour),
	})
	require.Error(t, err)
	ce, ok := webhook.AsCodeError(err)
	require.True(t, ok)
	require.Equal(t, webhook.CodeTimestampExpired, ce.Code)
}

func TestExtractEventIDFallbacks(t *testing.T) {
	// covered indirectly via HTTP; also ensure body without eventId hashes stably
	svc, _ := testService(t, true)
	ts := svc.Now()
	body := []byte(`{"foo":1}`)
	sig := webhook.SignTestPayload(nil, ts, body)
	w := signedRequest(t, svc, webhook.PlatformInternalTest, "ping", body, ts, sig)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Data struct {
			EventID string `json:"eventId"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Len(t, env.Data.EventID, 64)
}

func TestWebhookInvalidContentType(t *testing.T) {
	svc, _ := testService(t, true)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	webhook.RegisterPublic(r.Group("/api/v1"), &webhook.Handler{Svc: svc})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/internal-test/ping", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	require.Contains(t, w.Body.String(), webhook.CodeInvalidContentType)
	_, _ = io.Copy(io.Discard, w.Result().Body)
}
