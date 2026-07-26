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

type webhookEventSelectCounterLogger struct {
	logger.Interface
	count             atomic.Int64
	conflictDB        *gorm.DB
	conflictEventID   string
	conflictInserted  atomic.Bool
	conflictInsertSQL atomic.Int64
	conflictDelete    bool
	conflictDeleted   atomic.Bool
}

func (l *webhookEventSelectCounterLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	normalized := strings.ToLower(strings.TrimSpace(sql))
	if strings.HasPrefix(normalized, "select") && strings.Contains(normalized, "webhook_events") && strings.Contains(normalized, "event_id") {
		l.count.Add(1)
		if l.conflictDB != nil && l.conflictEventID != "" && !l.conflictInserted.Swap(true) {
			now := time.Now().UTC()
			_ = l.conflictDB.Exec(`INSERT INTO webhook_events (id, created_at, updated_at, platform, tenant_id, platform_shop_id, event_id, event_type, payload_hash, payload_body, status, raw_summary, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), now, now, webhook.PlatformInternalTest, int64(1), "", l.conflictEventID, "ping", strings.Repeat("a", 64), `{"seeded":true}`, webhook.StatusQueued, `{"seeded":true}`, `{}`).Error
		}
	}
	if strings.HasPrefix(normalized, "insert") && strings.Contains(normalized, "webhook_events") && l.conflictDB != nil && l.conflictDelete {
		if l.conflictInsertSQL.Add(1) >= 2 && !l.conflictDeleted.Swap(true) {
			_ = l.conflictDB.Exec(`DELETE FROM webhook_events WHERE platform = ? AND tenant_id = ? AND platform_shop_id = ? AND event_id = ?`,
				webhook.PlatformInternalTest, int64(1), "", l.conflictEventID).Error
		}
	}
	l.Interface.Trace(ctx, begin, func() (string, int64) { return sql, rows }, err)
}

func openWebhookTestDB(t *testing.T) *gorm.DB {
	return openWebhookTestDBWithLogger(t, logger.Default.LogMode(logger.Silent))
}

func openWebhookTestDBWithLogger(t *testing.T, log logger.Interface) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:webhook_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 log,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&idempotency.Record{}, &webhook.Event{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Serialize SQLite connections so concurrent ingest tests exercise app idempotency,
	// not driver "database is locked" races.
	sqlDB.SetMaxOpenConns(1)
	return db
}

func testService(t *testing.T, enableTest bool) (*webhook.Service, *gorm.DB) {
	return testServiceWithLogger(t, enableTest, logger.Default.LogMode(logger.Silent))
}

func testServiceWithLogger(t *testing.T, enableTest bool, log logger.Interface) (*webhook.Service, *gorm.DB) {
	t.Helper()
	db := openWebhookTestDBWithLogger(t, log)
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

func TestWebhookNormalInsertDoesNotReloadEvent(t *testing.T) {
	counter := &webhookEventSelectCounterLogger{Interface: logger.Default.LogMode(logger.Silent)}
	svc, _ := testServiceWithLogger(t, true, counter)
	counter.count.Store(0)

	res, err := svc.Ingest(context.Background(), webhook.IngestRequest{
		Platform:  webhook.PlatformInternalTest,
		EventID:   "evt-query-budget-normal",
		EventType: "ping",
		Payload:   json.RawMessage(`{"eventId":"evt-query-budget-normal","n":1}`),
		Timestamp: svc.Now(),
	})
	require.NoError(t, err)
	require.False(t, res.Duplicate)
	require.Equal(t, webhook.StatusQueued, res.Status)
	require.Equal(t, int64(1), counter.count.Load(), "normal insert should only run the initial event existence query")
}

func TestWebhookConflictDuplicateReloadsExistingEventOnce(t *testing.T) {
	counter := &webhookEventSelectCounterLogger{Interface: logger.Default.LogMode(logger.Silent)}
	svc, db := testServiceWithLogger(t, true, counter)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)
	counter.conflictDB = db
	counter.conflictEventID = "evt-query-budget-conflict"
	counter.count.Store(0)

	res, err := svc.Ingest(context.Background(), webhook.IngestRequest{
		Platform:  webhook.PlatformInternalTest,
		EventID:   "evt-query-budget-conflict",
		EventType: "ping",
		Payload:   json.RawMessage(`{"eventId":"evt-query-budget-conflict","n":1}`),
		Timestamp: svc.Now(),
	})
	require.NoError(t, err)
	require.True(t, res.Duplicate)
	require.Equal(t, webhook.StatusQueued, res.Status)
	require.Equal(t, int64(2), counter.count.Load(), "conflict duplicate should run initial lookup plus one event reload")

	var count int64
	require.NoError(t, db.Model(&webhook.Event{}).Where("event_id = ?", "evt-query-budget-conflict").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestWebhookConflictReloadMissingReturnsConsistencyError(t *testing.T) {
	counter := &webhookEventSelectCounterLogger{Interface: logger.Default.LogMode(logger.Silent)}
	svc, db := testServiceWithLogger(t, true, counter)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)
	counter.conflictDB = db
	counter.conflictEventID = "evt-query-budget-missing"
	counter.conflictDelete = true
	counter.count.Store(0)

	res, err := svc.Ingest(context.Background(), webhook.IngestRequest{
		Platform:  webhook.PlatformInternalTest,
		EventID:   "evt-query-budget-missing",
		EventType: "ping",
		Payload:   json.RawMessage(`{"eventId":"evt-query-budget-missing","n":1}`),
		Timestamp: svc.Now(),
	})
	require.Error(t, err)
	require.Nil(t, res)
	require.Contains(t, err.Error(), "conflict reload consistency error")
	require.Equal(t, int64(2), counter.count.Load(), "missing duplicate still attempts exactly one conflict reload before failing")
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
