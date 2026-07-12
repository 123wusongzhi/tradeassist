package securitymod_test

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testSecurityCfg(kr *crypto.KeyRing) *config.Config {
	return &config.Config{
		MasterKey: "0123456789abcdef0123456789abcdef",
		Auth: config.AuthConfig{
			AppMasterActiveKeyID:  kr.ActiveID,
			AppMasterActiveKey:    "0123456789abcdef0123456789abcdef",
			AppMasterPreviousKeys: `{"old1":"fedcba9876543210fedcba9876543210"}`,
		},
	}
}

func openRotationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL DEFAULT 0,
		group_key TEXT,
		item_key TEXT,
		item_value TEXT,
		is_encrypted INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("settings ddl: %v", err)
	}
	if err := db.Exec(`CREATE TABLE shops (
		id TEXT PRIMARY KEY,
		tenant_id INTEGER NOT NULL DEFAULT 0,
		shop_name TEXT
	)`).Error; err != nil {
		t.Fatalf("shops ddl: %v", err)
	}
	if err := db.Exec(`CREATE TABLE shop_auth_tokens (
		id TEXT PRIMARY KEY,
		shop_id TEXT NOT NULL,
		app_secret_enc TEXT,
		access_token_enc TEXT,
		refresh_token_enc TEXT
	)`).Error; err != nil {
		t.Fatalf("shop_auth_tokens ddl: %v", err)
	}
	if err := db.Exec(`CREATE TABLE key_rotation_jobs (
		id TEXT PRIMARY KEY,
		active_key_id TEXT,
		source_key_ids TEXT,
		scope TEXT,
		dry_run INTEGER,
		status TEXT,
		total_records INTEGER,
		processed_records INTEGER,
		reencrypted_records INTEGER,
		skipped_records INTEGER,
		failed_records INTEGER,
		last_cursor TEXT,
		table_scope TEXT,
		started_by TEXT,
		verification_status TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("rotation jobs ddl: %v", err)
	}
	if err := db.Exec(`CREATE TABLE key_rotation_item_failures (
		id TEXT PRIMARY KEY,
		rotation_id TEXT,
		target_table TEXT,
		record_id TEXT,
		tenant_id INTEGER,
		key_id TEXT,
		reason_code TEXT,
		safe_summary TEXT,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("rotation failures ddl: %v", err)
	}
	return db
}

func testKeyRing(t *testing.T) *crypto.KeyRing {
	t.Helper()
	kr, err := crypto.NewKeyRing("active", "0123456789abcdef0123456789abcdef", `{"old1":"fedcba9876543210fedcba9876543210"}`)
	if err != nil {
		t.Fatalf("keyring: %v", err)
	}
	return kr
}

func TestVerifyRotationFailsWithOldKeyReferences(t *testing.T) {
	db := openRotationTestDB(t)
	kr := testKeyRing(t)
	cipher, err := kr.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Simulate old-key ciphertext by encrypting with previous key material directly.
	prevKR, _ := crypto.NewKeyRing("old1", "fedcba9876543210fedcba9876543210", "")
	oldCipher, err := prevKR.Encrypt([]byte("old-secret"))
	if err != nil {
		t.Fatalf("prev encrypt: %v", err)
	}
	if err := db.Exec(`INSERT INTO settings (tenant_id, group_key, item_key, item_value, is_encrypted) VALUES (1,'ai','api_key',?,1)`, oldCipher).Error; err != nil {
		t.Fatalf("insert settings: %v", err)
	}
	_ = cipher

	svc := &securitymod.Service{DB: db}
	svc.Cfg = testSecurityCfg(kr)
	counts, err := svc.CountSecretReferencesByKeyID(context.Background(), kr.PreviousKeyIDs())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	var refs int64
	for _, c := range counts {
		refs += c.ReferenceCount
	}
	if refs == 0 {
		t.Fatalf("expected old key references > 0, counts=%+v", counts)
	}
}

func TestVerifyRotationPassesWhenNoOldReferences(t *testing.T) {
	db := openRotationTestDB(t)
	kr := testKeyRing(t)
	cipher, err := kr.Encrypt([]byte("active-secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := db.Exec(`INSERT INTO settings (tenant_id, group_key, item_key, item_value, is_encrypted) VALUES (1,'ai','api_key',?,1)`, cipher).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	svc := &securitymod.Service{DB: db}
	svc.Cfg = testSecurityCfg(kr)
	counts, err := svc.CountSecretReferencesByKeyID(context.Background(), kr.PreviousKeyIDs())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	for _, c := range counts {
		if c.ReferenceCount > 0 {
			t.Fatalf("unexpected old refs: %+v", c)
		}
	}
}

func TestCountIncludesShopAuthTokens(t *testing.T) {
	db := openRotationTestDB(t)
	kr := testKeyRing(t)
	prevKR, _ := crypto.NewKeyRing("old1", "fedcba9876543210fedcba9876543210", "")
	tokenCipher, err := prevKR.Encrypt([]byte("douyin-access-token"))
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	shopID := "11111111-1111-1111-1111-111111111111"
	if err := db.Exec(`INSERT INTO shops (id, tenant_id, shop_name) VALUES (?,1,'s')`, shopID).Error; err != nil {
		t.Fatalf("shop: %v", err)
	}
	if err := db.Exec(`INSERT INTO shop_auth_tokens (id, shop_id, access_token_enc) VALUES (?,?,?)`,
		"22222222-2222-2222-2222-222222222222", shopID, tokenCipher).Error; err != nil {
		t.Fatalf("token: %v", err)
	}
	svc := &securitymod.Service{DB: db}
	svc.Cfg = testSecurityCfg(kr)
	counts, err := svc.CountSecretReferencesByKeyID(context.Background(), kr.PreviousKeyIDs())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	found := false
	for _, c := range counts {
		if c.TableName == "shop_auth_tokens" && c.FieldName == "access_token_enc" && c.ReferenceCount > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("shop_auth_tokens not counted: %+v", counts)
	}
}

func TestAllReencryptTargetsRegistered(t *testing.T) {
	db := openRotationTestDB(t)
	kr := testKeyRing(t)
	targets := securitymod.AllReencryptTargets(db, kr)
	if len(targets) < 2 {
		t.Fatalf("expected >=2 targets, got %d", len(targets))
	}
	names := map[string]bool{}
	for _, t := range targets {
		names[t.Name()] = true
	}
	if !names["settings_encrypted"] || !names["shop_auth_tokens"] {
		t.Fatalf("missing targets: %v", names)
	}
}
