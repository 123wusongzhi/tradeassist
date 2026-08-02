package files

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/filescanner"
	"gorm.io/gorm"
)

type memoryScanQueue struct {
	lists      map[string][]string
	err        error
	requeueErr error
}

func (q *memoryScanQueue) LPush(_ context.Context, k, v string) error {
	if q.err != nil {
		return q.err
	}
	q.lists[k] = append([]string{v}, q.lists[k]...)
	return nil
}
func (q *memoryScanQueue) LLen(_ context.Context, k string) (int64, error) {
	if q.err != nil {
		return 0, q.err
	}
	return int64(len(q.lists[k])), nil
}
func (q *memoryScanQueue) RPopLPush(_ context.Context, a, b string) (string, error) {
	x := q.lists[a]
	if len(x) == 0 {
		return "", nil
	}
	v := x[len(x)-1]
	q.lists[a] = x[:len(x)-1]
	q.lists[b] = append([]string{v}, q.lists[b]...)
	return v, nil
}
func (q *memoryScanQueue) BRPopLPush(c context.Context, a, b string, _ time.Duration) (string, error) {
	return q.RPopLPush(c, a, b)
}
func (q *memoryScanQueue) LRem(_ context.Context, k string, count int64, v string) error {
	x := q.lists[k]
	if count == 0 {
		out := x[:0]
		for _, item := range x {
			if item != v {
				out = append(out, item)
			}
		}
		q.lists[k] = out
		return nil
	}
	for i, s := range x {
		if s == v {
			q.lists[k] = append(x[:i], x[i+1:]...)
			break
		}
	}
	return nil
}
func (q *memoryScanQueue) Requeue(_ context.Context, old, next string) error {
	if q.requeueErr != nil {
		return q.requeueErr
	}
	_ = q.LRem(context.Background(), fileScanProcessingQueueName, 0, old)
	_ = q.LRem(context.Background(), fileScanQueueName, 0, old)
	if next != old {
		_ = q.LRem(context.Background(), fileScanProcessingQueueName, 0, next)
		_ = q.LRem(context.Background(), fileScanQueueName, 0, next)
	}
	return q.LPush(context.Background(), fileScanQueueName, next)
}
func (q *memoryScanQueue) RestoreRecovery(_ context.Context, payload string) error {
	_ = q.LRem(context.Background(), fileScanQueueName, 0, payload)
	_ = q.LRem(context.Background(), fileScanProcessingQueueName, 0, payload)
	return q.LPush(context.Background(), fileScanProcessingQueueName, payload)
}

func TestEnqueueSecurityScanPropagatesQueueFailure(t *testing.T) {
	s := &Service{scanQueue: &memoryScanQueue{lists: map[string][]string{}, err: errors.New("down")}}
	if err := s.EnqueueSecurityScan(context.Background(), 1, uuid.New()); err == nil {
		t.Fatal("expected queue error")
	}
}

func queuePayload(t *testing.T, attempts int) string {
	t.Helper()
	raw, err := json.Marshal(ScanQueueMessage{TenantID: 7, AssetID: uuid.NewString(), Attempts: attempts})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func failingScanProcessor(context.Context, *slog.Logger, filescanner.FileScanner, string) error {
	return errors.New("scan failed")
}

func successfulScanProcessor(context.Context, *slog.Logger, filescanner.FileScanner, string) error {
	return nil
}

func TestHandleClaimedPayloadRequeuesWithBoundedAttempts(t *testing.T) {
	t.Run("first failure requeues with incremented attempt", func(t *testing.T) {
		payload := queuePayload(t, 0)
		q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
		s := &Service{scanQueue: q}
		s.handleClaimedPayloadWithProcessor(context.Background(), nil, nil, payload, failingScanProcessor)
		if len(q.lists[fileScanProcessingQueueName]) != 0 || len(q.lists[fileScanQueueName]) != 1 {
			t.Fatalf("lists after retry = %+v", q.lists)
		}
		var next ScanQueueMessage
		if err := json.Unmarshal([]byte(q.lists[fileScanQueueName][0]), &next); err != nil || next.Attempts != 1 {
			t.Fatalf("requeued message = %+v, err = %v", next, err)
		}
	})

	t.Run("third failure acknowledges without requeue", func(t *testing.T) {
		payload := queuePayload(t, 2)
		q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
		s := &Service{scanQueue: q}
		s.handleClaimedPayloadWithProcessor(context.Background(), nil, nil, payload, failingScanProcessor)
		if len(q.lists[fileScanProcessingQueueName]) != 0 || len(q.lists[fileScanQueueName]) != 0 {
			t.Fatalf("lists after terminal failure = %+v", q.lists)
		}
	})
}

func TestHandleClaimedPayloadAcknowledgesPoisonAndSuccess(t *testing.T) {
	for _, tc := range []struct {
		name      string
		payload   string
		processor scanPayloadProcessor
	}{
		{name: "malformed", payload: "{", processor: failingScanProcessor},
		{name: "success", payload: queuePayload(t, 0), processor: successfulScanProcessor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {tc.payload}}}
			s := &Service{scanQueue: q}
			s.handleClaimedPayloadWithProcessor(context.Background(), nil, nil, tc.payload, tc.processor)
			if len(q.lists[fileScanProcessingQueueName]) != 0 || len(q.lists[fileScanQueueName]) != 0 {
				t.Fatalf("payload was not acknowledged: %+v", q.lists)
			}
		})
	}
}

func TestHandleClaimedPayloadKeepsProcessingItemWhenRequeueFails(t *testing.T) {
	payload := queuePayload(t, 0)
	q := &memoryScanQueue{
		lists:      map[string][]string{fileScanProcessingQueueName: {payload}},
		requeueErr: errors.New("redis unavailable"),
	}
	s := &Service{scanQueue: q}
	s.handleClaimedPayloadWithProcessor(context.Background(), nil, nil, payload, failingScanProcessor)
	if got := q.lists[fileScanProcessingQueueName]; len(got) != 1 || got[0] != payload {
		t.Fatalf("processing item was lost: %+v", q.lists)
	}
	if len(q.lists[fileScanQueueName]) != 0 {
		t.Fatalf("failed requeue unexpectedly pushed pending item: %+v", q.lists)
	}
}

func TestHandleClaimedPayloadDoesNotAckRecoveryWindowScanning(t *testing.T) {
	payload := queuePayload(t, 0)
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
	s := &Service{scanQueue: q}
	s.handleClaimedPayloadWithProcessor(context.Background(), nil, nil, payload, func(context.Context, *slog.Logger, filescanner.FileScanner, string) error { return errScanInProgress })
	if len(q.lists[fileScanProcessingQueueName]) != 1 || q.lists[fileScanProcessingQueueName][0] != payload || len(q.lists[fileScanQueueName]) != 0 {
		t.Fatalf("recovery-window payload was acknowledged or changed: %+v", q.lists)
	}
}

func TestHandleClaimedPayloadDoesNotMutateNewOwnerAfterClaimLoss(t *testing.T) {
	payload := queuePayload(t, 0)
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
	s := &Service{scanQueue: q}
	s.handleClaimedPayloadWithProcessor(context.Background(), nil, nil, payload, func(context.Context, *slog.Logger, filescanner.FileScanner, string) error { return errScanClaimLost })
	if got := q.lists[fileScanProcessingQueueName]; len(got) != 1 || got[0] != payload {
		t.Fatalf("claim-losing worker mutated new owner's item: %+v", q.lists)
	}
}

func TestMemoryQueueRecoveryAndAckAreIdempotent(t *testing.T) {
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {"same", "same"}}}
	for {
		v, _ := q.RPopLPush(context.Background(), fileScanProcessingQueueName, fileScanQueueName)
		if v == "" {
			break
		}
	}
	v, _ := q.BRPopLPush(context.Background(), fileScanQueueName, fileScanProcessingQueueName, time.Second)
	if v != "same" || len(q.lists[fileScanProcessingQueueName]) != 1 {
		t.Fatal("claim did not preserve processing record")
	}
	if err := q.LRem(context.Background(), fileScanProcessingQueueName, 1, v); err != nil || len(q.lists[fileScanProcessingQueueName]) != 0 {
		t.Fatal("ack failed")
	}
}

func TestRecoverProcessingQueueRestoresScanningRecord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FileRecord{}); err != nil {
		t.Fatal(err)
	}
	row := FileRecord{
		TenantID:       7,
		OriginalName:   "recovered.png",
		ObjectKey:      "quarantine/t7/recovered.png",
		StorageKind:    "local",
		SecurityStatus: SecurityScanning,
		ScanStatus:     SecurityScanning,
	}
	past := time.Now().Add(-time.Minute)
	row.ScanClaimID = uuid.NewString()
	row.ScanLeaseUntil = &past
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	payload := string(raw)
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
	s := &Service{DB: db, scanQueue: q}
	if err := s.recoverProcessingQueue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(q.lists[fileScanProcessingQueueName]) != 0 || len(q.lists[fileScanQueueName]) != 1 {
		t.Fatalf("recovered queue state = %+v", q.lists)
	}
	var recovered FileRecord
	if err := db.First(&recovered, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if recovered.SecurityStatus != SecurityScanFailed || recovered.ScanStatus != SecurityScanFailed {
		t.Fatalf("recovered statuses = %s/%s", recovered.SecurityStatus, recovered.ScanStatus)
	}
	if recovered.ScanClaimID != "" || recovered.ScanLeaseUntil != nil {
		t.Fatalf("recovered claim was not cleared: %+v", recovered)
	}
}

func TestRecoverProcessingQueuePreservesActiveLease(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FileRecord{}); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	row := FileRecord{
		TenantID:       7,
		OriginalName:   "active.png",
		ObjectKey:      "quarantine/t7/active.png",
		StorageKind:    "local",
		SecurityStatus: SecurityScanning,
		ScanStatus:     SecurityScanning,
		ScanClaimID:    uuid.NewString(),
		ScanLeaseUntil: &future,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	payload := string(raw)
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
	s := &Service{DB: db, scanQueue: q}
	if err := s.recoverProcessingQueueAt(context.Background(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if len(q.lists[fileScanProcessingQueueName]) != 1 || len(q.lists[fileScanQueueName]) != 0 {
		t.Fatalf("active lease was made claimable: %+v", q.lists)
	}
	var got FileRecord
	if err := db.First(&got, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.SecurityStatus != SecurityScanning || got.ScanClaimID != row.ScanClaimID {
		t.Fatalf("active lease was reset: %+v", got)
	}
}

func TestRecoverProcessingQueueDefersLegacyUnleasedScanningRow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FileRecord{}); err != nil {
		t.Fatal(err)
	}
	row := FileRecord{TenantID: 7, OriginalName: "legacy.png", ObjectKey: "quarantine/t7/legacy.png", StorageKind: "local", SecurityStatus: SecurityScanning, ScanStatus: SecurityScanning}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	payload := string(raw)
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
	s := &Service{DB: db, scanQueue: q}
	if err := s.recoverProcessingQueueAt(context.Background(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if len(q.lists[fileScanProcessingQueueName]) != 1 || len(q.lists[fileScanQueueName]) != 0 {
		t.Fatalf("legacy active row was not deferred: %+v", q.lists)
	}
}

func TestRecoverProcessingQueueRestoresPayloadWhenDatabaseUnavailable(t *testing.T) {
	payload := queuePayload(t, 0)
	q := &memoryScanQueue{lists: map[string][]string{fileScanProcessingQueueName: {payload}}}
	s := &Service{scanQueue: q}
	if err := s.recoverProcessingQueue(context.Background()); err == nil {
		t.Fatal("expected database recovery error")
	}
	if got := q.lists[fileScanProcessingQueueName]; len(got) != 1 || got[0] != payload {
		t.Fatalf("payload was not restored to processing: %+v", q.lists)
	}
	if len(q.lists[fileScanQueueName]) != 0 {
		t.Fatalf("pending queue should stay empty: %+v", q.lists)
	}
}
