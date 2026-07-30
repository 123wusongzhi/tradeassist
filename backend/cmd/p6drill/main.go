package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/release"
	"github.com/trademind-ai/trademind/backend/internal/modules/restore"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		writeJSON(map[string]any{"status": "failed", "error": sanitize(err.Error())})
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: p6drill <seed|backup|verify|restore|validate|release>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if config.IsProduction(cfg.AppEnv) || strings.EqualFold(os.Getenv("TARGET_ENVIRONMENT"), "production") {
		return fmt.Errorf("P6-V drill refuses production environment")
	}
	enc, err := encrypt.NewService(cfg.MasterKey)
	if err != nil {
		return err
	}
	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	switch os.Args[1] {
	case "seed":
		return seed(ctx, db, cfg, enc)
	case "backup":
		return doBackup(ctx, db, cfg, enc)
	case "verify":
		return doVerify(ctx, db, cfg, enc, arg("--backup-id"))
	case "restore":
		return doRestore(ctx, db, cfg, enc, arg("--backup-id"), arg("--target-db"))
	case "validate":
		return doValidate(ctx, db)
	case "release":
		return doRelease(ctx, db, cfg, enc)
	case "negative":
		return doNegative(ctx, db, cfg, enc)
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func seed(ctx context.Context, db *gorm.DB, cfg *config.Config, enc *encrypt.Service) error {
	if err := database.AutoMigrate(db); err != nil {
		return err
	}
	marker := strings.TrimSpace(os.Getenv("P6V_MARKER"))
	if marker == "" {
		marker = "p6v-marker-" + uuid.NewString()
	}
	markerHash := hash(marker)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS p6v_markers (
		id bigserial PRIMARY KEY,
		marker_hash text NOT NULL,
		created_at timestamptz NOT NULL DEFAULT now()
	)`).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO p6v_markers(marker_hash) VALUES (?)`, markerHash).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO admin_users (id, tenant_id, username, email, password_hash, display_name, role, status, token_version, must_change_password, created_at, updated_at)
		VALUES (?, 1001, ?, 'p6v-admin@example.invalid', 'p6v-password-hash-only', 'P6V Admin', 'admin', 'active', 1, false, now(), now())
		ON CONFLICT (username) DO NOTHING`, uuid.NewString(), "p6v_admin_"+short(markerHash)).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO shops (id, tenant_id, platform, shop_name, shop_code, external_shop_id, status, auth_status, region, currency, created_at, updated_at)
		VALUES (?, 1001, 'demo', 'P6V Tenant A Shop', ?, ?, 'active', 'authorized', 'CN', 'CNY', now(), now()),
		       (?, 1002, 'demo', 'P6V Tenant B Shop', ?, ?, 'active', 'authorized', 'CN', 'CNY', now(), now())`,
		uuid.NewString(), "p6v-a-"+short(markerHash), "p6v-ext-a-"+short(markerHash),
		uuid.NewString(), "p6v-b-"+short(markerHash), "p6v-ext-b-"+short(markerHash)).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO products (id, tenant_id, source, source_url, original_title, title, currency, status, created_at, updated_at)
		VALUES (?, 1001, 'p6v', '', 'P6V source product', 'P6V restored product', 'CNY', 'draft', now(), now())`, uuid.NewString()).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO orders (id, tenant_id, platform, order_no, customer_name, status, payment_status, fulfillment_status, currency, total_amount, created_at, updated_at)
		VALUES (?, 1001, 'demo', ?, 'P6V Customer', 'pending', 'unpaid', 'unfulfilled', 'CNY', 12.34, now(), now())`, uuid.NewString(), "P6V-"+short(markerHash)).Error; err != nil {
		return err
	}
	secretValue := "p6v-secret-placeholder"
	if enc != nil {
		if encrypted, err := enc.Encrypt([]byte(secretValue)); err == nil {
			secretValue = encrypted
		}
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO settings (tenant_id, group_key, item_key, item_value, value_type, is_encrypted, remark, created_at, updated_at)
		VALUES (1001, 'p6v', ?, ?, 'string', true, 'P6-V encrypted marker', now(), now())
		ON CONFLICT (tenant_id, group_key, item_key) DO UPDATE SET item_value = EXCLUDED.item_value, updated_at = now()`,
		"secret_"+short(markerHash), secretValue).Error; err != nil {
		return err
	}
	oplog := &operationlog.Service{DB: db}
	_ = oplog.WriteBackground(ctx, operationlog.WriteOpts{TenantID: 1001, Username: "p6v-drill", Action: "p6v.seed", Resource: "p6v", ResourceID: markerHash, Status: "passed", Message: "P6-V isolated seed completed"})
	summary, err := summary(ctx, db)
	if err != nil {
		return err
	}
	writeJSON(map[string]any{"status": "passed", "markerHash": markerHash, "summary": summary, "environment": cfg.AppEnv})
	return nil
}

func doBackup(ctx context.Context, db *gorm.DB, cfg *config.Config, enc *encrypt.Service) error {
	svc := &backup.Service{DB: db, Cfg: cfg, Enc: enc, OpLog: &operationlog.Service{DB: db}}
	row, err := svc.CreateDatabaseBackup(ctx, backup.CreateRequest{Reason: "P6-V isolated restore drill"}, nil)
	if err != nil {
		return err
	}
	writeJSON(map[string]any{"status": row.Status, "backupId": row.BackupID, "encrypted": row.Encrypted, "checksum": row.Checksum, "artifactSize": row.ArtifactSize})
	return nil
}

func doVerify(ctx context.Context, db *gorm.DB, cfg *config.Config, enc *encrypt.Service, backupID string) error {
	if backupID == "" {
		return fmt.Errorf("--backup-id is required")
	}
	svc := &backup.Service{DB: db, Cfg: cfg, Enc: enc, OpLog: &operationlog.Service{DB: db}}
	row, err := svc.Verify(ctx, backupID)
	if err != nil {
		return err
	}
	writeJSON(map[string]any{"status": row.Status, "backupId": row.BackupID, "checksum": row.ChecksumPassed, "manifest": row.ManifestPassed, "encryption": row.EncryptionPassed, "pgRestoreList": row.PGRestoreListed})
	return nil
}

func doRestore(ctx context.Context, db *gorm.DB, cfg *config.Config, enc *encrypt.Service, backupID, targetDB string) error {
	if backupID == "" || targetDB == "" {
		return fmt.Errorf("--backup-id and --target-db are required")
	}
	bsvc := &backup.Service{DB: db, Cfg: cfg, Enc: enc, OpLog: &operationlog.Service{DB: db}}
	rsvc := &restore.Service{DB: db, Cfg: cfg, Enc: enc, Backup: bsvc, OpLog: &operationlog.Service{DB: db}}
	row, err := rsvc.Create(ctx, restore.CreateRequest{
		BackupID: backupID, TargetEnvironment: "isolated", TargetDatabaseName: targetDB,
		TargetIsIsolated: true, OperatorReauthenticated: true, HighRiskConfirmed: true,
	}, nil)
	if err != nil {
		return err
	}
	writeJSON(map[string]any{"status": row.Status, "restoreId": row.RestoreID, "targetHash": row.TargetDatabaseHash, "safetyGate": row.SafetyGateStatus})
	return nil
}

func doValidate(ctx context.Context, db *gorm.DB) error {
	s, err := summary(ctx, db)
	if err != nil {
		return err
	}
	oplog := &operationlog.Service{DB: db}
	_, _, chainErr := oplog.VerifyChain(ctx, 1001, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	writeJSON(map[string]any{"status": "passed", "summary": s, "auditChain": chainErr == nil})
	if chainErr != nil {
		return chainErr
	}
	return nil
}

func doRelease(ctx context.Context, db *gorm.DB, cfg *config.Config, enc *encrypt.Service) error {
	bsvc := &backup.Service{DB: db, Cfg: cfg, Enc: enc, OpLog: &operationlog.Service{DB: db}}
	rsvc := &release.Service{DB: db, Cfg: cfg, Backup: bsvc, OpLog: &operationlog.Service{DB: db}}
	run, err := rsvc.Create(ctx, release.CreateRequest{Version: "p6v-b", GitCommit: strings.TrimSpace(os.Getenv("GIT_COMMIT"))}, nil)
	if err != nil {
		return err
	}
	executed, err := rsvc.Execute(ctx, run.ReleaseID)
	if err != nil {
		return err
	}
	rb, err := rsvc.Rollback(ctx, executed.ReleaseID, release.RollbackRequest{Reason: "P6-V controlled post-switch failure simulation"}, nil)
	if err != nil {
		return err
	}
	writeJSON(map[string]any{"status": "passed", "releaseId": executed.ReleaseID, "state": release.StateRolledBack, "preBackupId": executed.PreBackupID, "databaseRestore": rb.DatabaseRestore})
	return nil
}

func doNegative(ctx context.Context, db *gorm.DB, cfg *config.Config, enc *encrypt.Service) error {
	backupID := arg("--backup-id")
	unverifiedID := arg("--unverified-backup-id")
	if backupID == "" || unverifiedID == "" {
		return fmt.Errorf("--backup-id and --unverified-backup-id are required")
	}
	bsvc := &backup.Service{DB: db, Cfg: cfg, Enc: enc, OpLog: &operationlog.Service{DB: db}}
	rsvc := &restore.Service{DB: db, Cfg: cfg, Enc: enc, Backup: bsvc, OpLog: &operationlog.Service{DB: db}}
	results := map[string]bool{}
	results["unverifiedRejected"] = expectRestoreReject(ctx, rsvc, unverifiedID, arg("--unverified-db"), "isolated", true, "RESTORE_BACKUP_NOT_VERIFIED")
	results["productionTargetRejected"] = expectRestoreReject(ctx, rsvc, backupID, "production", "production", true, "RESTORE_TARGET_FORBIDDEN")
	results["nonEmptyTargetRejected"] = expectRestoreReject(ctx, rsvc, backupID, arg("--non-empty-db"), "isolated", true, "RESTORE_TARGET_NOT_EMPTY")
	results["manifestRejected"] = withCorruptManifest(ctx, db, rsvc, backupID, arg("--manifest-db"))
	results["checksumRejected"] = withCorruptArtifact(ctx, db, rsvc, backupID, arg("--checksum-db"), false)
	results["ciphertextRejected"] = withCorruptArtifact(ctx, db, rsvc, backupID, arg("--cipher-db"), true)
	writeJSON(map[string]any{"status": "passed", "negativeTests": results})
	if !allTrue(results) {
		return fmt.Errorf("one or more negative tests did not reject as expected")
	}
	return nil
}

func expectRestoreReject(ctx context.Context, svc *restore.Service, backupID, targetDB, targetEnv string, isolated bool, code string) bool {
	_, err := svc.Create(ctx, restore.CreateRequest{
		BackupID: backupID, TargetEnvironment: targetEnv, TargetDatabaseName: targetDB,
		TargetIsIsolated: isolated, OperatorReauthenticated: true, HighRiskConfirmed: true,
	}, nil)
	return err != nil && strings.Contains(err.Error(), code)
}

func withCorruptManifest(ctx context.Context, db *gorm.DB, svc *restore.Service, backupID, targetDB string) bool {
	var row backup.Job
	if err := db.WithContext(ctx).Where("backup_id = ?", backupID).First(&row).Error; err != nil {
		return false
	}
	original := append([]byte(nil), row.ManifestJSON...)
	var payload map[string]any
	if err := json.Unmarshal(original, &payload); err != nil {
		return false
	}
	payload["environment"] = "tampered"
	corrupt, _ := json.Marshal(payload)
	if err := db.WithContext(ctx).Model(&backup.Job{}).Where("backup_id = ?", backupID).Update("manifest_json", corrupt).Error; err != nil {
		return false
	}
	ok := expectRestoreReject(ctx, svc, backupID, targetDB, "isolated", true, "RESTORE_MANIFEST_CHECKSUM_MISMATCH")
	_ = db.WithContext(ctx).Model(&backup.Job{}).Where("backup_id = ?", backupID).Update("manifest_json", original).Error
	return ok
}

func withCorruptArtifact(ctx context.Context, db *gorm.DB, svc *restore.Service, backupID, targetDB string, recomputeChecksum bool) bool {
	var artifact backup.Artifact
	if err := db.WithContext(ctx).Where("backup_id = ?", backupID).Order("created_at DESC").First(&artifact).Error; err != nil {
		return false
	}
	raw, err := os.ReadFile(artifact.LocalPath)
	if err != nil || len(raw) == 0 {
		return false
	}
	raw[len(raw)-1] ^= 0xff
	corruptPath := filepath.Join(os.TempDir(), "trademind-p6v-corrupt-"+uuid.NewString()+filepath.Ext(artifact.LocalPath))
	if err := os.WriteFile(corruptPath, raw, 0o600); err != nil {
		return false
	}
	defer func() { _ = os.Remove(corruptPath) }()
	sha := artifact.SHA256
	if recomputeChecksum {
		sha = fileHash(raw)
	}
	updates := map[string]any{"local_path": corruptPath, "sha256": sha}
	if err := db.WithContext(ctx).Model(&backup.Artifact{}).Where("id = ?", artifact.ID).Updates(updates).Error; err != nil {
		return false
	}
	expected := "RESTORE_CHECKSUM_MISMATCH"
	if recomputeChecksum {
		expected = "RESTORE_DECRYPT_INTEGRITY_FAILED"
	}
	ok := expectRestoreReject(ctx, svc, backupID, targetDB, "isolated", true, expected)
	_ = db.WithContext(ctx).Model(&backup.Artifact{}).Where("id = ?", artifact.ID).Updates(map[string]any{"local_path": artifact.LocalPath, "sha256": artifact.SHA256}).Error
	return ok
}

func allTrue(values map[string]bool) bool {
	for _, v := range values {
		if !v {
			return false
		}
	}
	return true
}

func fileHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func summary(ctx context.Context, db *gorm.DB) (map[string]any, error) {
	out := map[string]any{}
	for key, table := range map[string]string{
		"tenantCount": "shops", "shopCount": "shops", "userCount": "admin_users",
		"productCount": "products", "orderCount": "orders", "operationLogCount": "operation_logs",
	} {
		var n int64
		q := fmt.Sprintf("SELECT count(*) FROM %s", table)
		if key == "tenantCount" {
			q = "SELECT count(DISTINCT tenant_id) FROM shops"
		}
		if err := db.WithContext(ctx).Raw(q).Scan(&n).Error; err != nil {
			return nil, err
		}
		out[key] = n
	}
	var marker string
	_ = db.WithContext(ctx).Raw(`SELECT marker_hash FROM p6v_markers ORDER BY id DESC LIMIT 1`).Scan(&marker).Error
	out["p6vMarkerHash"] = marker
	return out, nil
}

func arg(name string) string {
	for i := 2; i < len(os.Args)-1; i++ {
		if os.Args[i] == name {
			return os.Args[i+1]
		}
	}
	return ""
}

func writeJSON(v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(raw))
}

func hash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func short(v string) string {
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

func sanitize(v string) string {
	replacer := strings.NewReplacer(os.Getenv("DB_PASSWORD"), "[redacted]", os.Getenv("APP_MASTER_KEY"), "[redacted]")
	return replacer.Replace(v)
}
