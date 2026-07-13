package backupruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
)

func TestEncryptFileRoundTripAndTamperReject(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "plain.dump")
	encPath := filepath.Join(dir, "plain.dump.enc")
	out := filepath.Join(dir, "plain.out")
	if err := os.WriteFile(src, []byte(strings.Repeat("backup-data-", 10000)), 0o600); err != nil {
		t.Fatal(err)
	}
	kek, err := encrypt.NewService("test-master-key-unique-p6")
	if err != nil {
		t.Fatal(err)
	}
	env, err := EncryptFile(src, encPath, "test-key", kek)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if env.WrappedDataKey == "" {
		t.Fatalf("wrapped data key missing")
	}
	if err := DecryptFile(encPath, out, *env, kek); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	plain, _ := os.ReadFile(src)
	roundTrip, _ := os.ReadFile(out)
	if string(plain) != string(roundTrip) {
		t.Fatalf("roundtrip mismatch")
	}
	f, err := os.OpenFile(encPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{0x7f}, int64(len(encryptionMagic)+8)); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := DecryptFile(encPath, filepath.Join(dir, "tampered.out"), *env, kek); err == nil {
		t.Fatalf("expected tamper rejection")
	}
}

func TestPITRValidationAndWALContinuity(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	if err := ValidateRecoveryTargetTime(now.Add(time.Minute), now.Add(-time.Hour), now); err == nil {
		t.Fatalf("future target should fail")
	}
	if err := ValidateRecoveryTargetTime(now.Add(-2*time.Hour), now.Add(-time.Hour), now); err == nil {
		t.Fatalf("too old target should fail")
	}
	segs := []WALSegment{
		{Name: "0001", Timeline: "1", StartTime: now.Add(-2 * time.Minute), EndTime: now.Add(-time.Minute), Checksum: "a"},
		{Name: "0002", Timeline: "1", StartTime: now.Add(-time.Minute), EndTime: now, Checksum: "b"},
	}
	if err := CheckWALContinuity(segs); err != nil {
		t.Fatalf("continuous wal should pass: %v", err)
	}
	segs[1].StartTime = now.Add(-30 * time.Second)
	if err := CheckWALContinuity(segs); err == nil {
		t.Fatalf("wal gap should fail")
	}
}
