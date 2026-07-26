package config

import (
	"fmt"
	"os"
	"strings"
)

// BackupConfig holds P6 backup, verification and retention settings.
type BackupConfig struct {
	Enabled               bool
	Mode                  string
	Schedule              string
	StorageProvider       string
	StorageBucket         string
	StoragePrefix         string
	EncryptionEnabled     bool
	EncryptionKeyID       string
	RetentionDaily        int
	RetentionWeekly       int
	RetentionMonthly      int
	MaxAgeHours           int
	CommandTimeoutSeconds int
	VerifyEnabled         bool
	RestoreDrillEnabled   bool
	RestoreDrillSchedule  string
}

// PostgresBackupConfig holds PostgreSQL backup/PITR command settings.
type PostgresBackupConfig struct {
	Format            string
	PGDumpPath        string
	PGRestorePath     string
	PSQLPath          string
	WALArchiveEnabled bool
	WALArchivePath    string
	PITREnabled       bool
}

// ReleaseConfig holds P6 controlled deployment settings.
type ReleaseConfig struct {
	Enabled              bool
	Root                 string
	ArtifactDir          string
	CurrentLink          string
	PreviousLink         string
	KeepCount            int
	HealthTimeoutSeconds int
	RollbackOnFailure    bool
	RequirePreBackup     bool
	Strategy             string
	TrafficSwitchMode    string
}

func loadBackupConfig(appEnv string) BackupConfig {
	return BackupConfig{
		Enabled:               envBool(os.Getenv("BACKUP_ENABLED"), false),
		Mode:                  strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_MODE"), "disabled"))),
		Schedule:              strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_SCHEDULE"), "0 3 * * *")),
		StorageProvider:       strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_STORAGE_PROVIDER"), "local"))),
		StorageBucket:         strings.TrimSpace(os.Getenv("BACKUP_STORAGE_BUCKET")),
		StoragePrefix:         strings.Trim(strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_STORAGE_PREFIX"), "backups/"+NormalizeEnv(appEnv))), "/"),
		EncryptionEnabled:     envBool(os.Getenv("BACKUP_ENCRYPTION_ENABLED"), IsStagingOrProduction(appEnv)),
		EncryptionKeyID:       strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_ENCRYPTION_KEY_ID"), "app-master-key")),
		RetentionDaily:        atoiOrDefault(os.Getenv("BACKUP_RETENTION_DAILY"), 7),
		RetentionWeekly:       atoiOrDefault(os.Getenv("BACKUP_RETENTION_WEEKLY"), 4),
		RetentionMonthly:      atoiOrDefault(os.Getenv("BACKUP_RETENTION_MONTHLY"), 6),
		MaxAgeHours:           atoiOrDefault(os.Getenv("BACKUP_MAX_AGE_HOURS"), 30),
		CommandTimeoutSeconds: atoiOrDefault(os.Getenv("BACKUP_COMMAND_TIMEOUT_SECONDS"), 900),
		VerifyEnabled:         envBool(os.Getenv("BACKUP_VERIFY_ENABLED"), true),
		RestoreDrillEnabled:   envBool(os.Getenv("BACKUP_RESTORE_DRILL_ENABLED"), false),
		RestoreDrillSchedule:  strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_RESTORE_DRILL_SCHEDULE"), "0 4 * * 0")),
	}
}

func loadPostgresBackupConfig() PostgresBackupConfig {
	return PostgresBackupConfig{
		Format:            strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_BACKUP_FORMAT"), "custom"))),
		PGDumpPath:        strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_PG_DUMP_PATH"), "pg_dump")),
		PGRestorePath:     strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_PG_RESTORE_PATH"), "pg_restore")),
		PSQLPath:          strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_PSQL_PATH"), "psql")),
		WALArchiveEnabled: envBool(os.Getenv("POSTGRES_WAL_ARCHIVE_ENABLED"), false),
		WALArchivePath:    strings.TrimSpace(os.Getenv("POSTGRES_WAL_ARCHIVE_PATH")),
		PITREnabled:       envBool(os.Getenv("POSTGRES_PITR_ENABLED"), false),
	}
}

func loadReleaseConfig(appEnv string) ReleaseConfig {
	root := strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_ROOT"), "/opt/trademind/releases"))
	return ReleaseConfig{
		Enabled:              envBool(os.Getenv("RELEASE_ENABLED"), false),
		Root:                 root,
		ArtifactDir:          strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_ARTIFACT_DIR"), "artifacts/releases")),
		CurrentLink:          strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_CURRENT_LINK"), root+"/current")),
		PreviousLink:         strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_PREVIOUS_LINK"), root+"/previous")),
		KeepCount:            atoiOrDefault(os.Getenv("RELEASE_KEEP_COUNT"), 5),
		HealthTimeoutSeconds: atoiOrDefault(os.Getenv("RELEASE_HEALTH_TIMEOUT_SECONDS"), 120),
		RollbackOnFailure:    envBool(os.Getenv("RELEASE_ROLLBACK_ON_FAILURE"), true),
		RequirePreBackup:     envBool(os.Getenv("RELEASE_REQUIRE_PRE_BACKUP"), IsStagingOrProduction(appEnv)),
		Strategy:             strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_STRATEGY"), "blue_green"))),
		TrafficSwitchMode:    strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_TRAFFIC_SWITCH_MODE"), "manual"))),
	}
}

func (c *Config) validateP6ProductionGuards() error {
	if c == nil {
		return nil
	}
	if strings.TrimSpace(c.Backup.Mode) == "" {
		c.Backup.Mode = "disabled"
	}
	if strings.TrimSpace(c.Backup.StorageProvider) == "" {
		c.Backup.StorageProvider = "local"
	}
	if strings.TrimSpace(c.Release.Strategy) == "" {
		c.Release.Strategy = "blue_green"
	}
	if c.Backup.CommandTimeoutSeconds == 0 {
		c.Backup.CommandTimeoutSeconds = 900
	}
	if !validBackupMode(c.Backup.Mode) {
		return fmt.Errorf("%s: BACKUP_MODE must be disabled, local, object_storage, or hybrid", ErrCodeConfigInvalid)
	}
	if !validReleaseStrategy(c.Release.Strategy) {
		return fmt.Errorf("%s: RELEASE_STRATEGY must be in_place, rolling, or blue_green", ErrCodeConfigInvalid)
	}
	if c.Backup.CommandTimeoutSeconds <= 0 {
		return fmt.Errorf("%s: BACKUP_COMMAND_TIMEOUT_SECONDS must be positive", ErrCodeConfigInvalid)
	}
	if c.Backup.RetentionDaily < 0 || c.Backup.RetentionWeekly < 0 || c.Backup.RetentionMonthly < 0 {
		return fmt.Errorf("%s: backup retention counts cannot be negative", ErrCodeConfigInvalid)
	}
	if c.Backup.Enabled && c.Backup.RetentionDaily == 0 && c.Backup.RetentionWeekly == 0 && c.Backup.RetentionMonthly == 0 {
		return fmt.Errorf("%s: backup retention cannot be unlimited or empty", ErrCodeConfigInvalid)
	}
	if IsProduction(c.AppEnv) {
		if !c.Backup.Enabled {
			return fmt.Errorf("%s: BACKUP_ENABLED=true is required in production", ErrCodeConfigRequired)
		}
		if c.Backup.Mode == "disabled" || c.Backup.Mode == "local" {
			return fmt.Errorf("%s: production backups require object_storage or hybrid mode", ErrCodeConfigInvalid)
		}
		if !c.Backup.EncryptionEnabled {
			return fmt.Errorf("%s: BACKUP_ENCRYPTION_ENABLED=true is required in production", ErrCodeConfigRequired)
		}
		if c.Release.Enabled && !c.Release.RequirePreBackup {
			return fmt.Errorf("%s: RELEASE_REQUIRE_PRE_BACKUP=true is required for production release", ErrCodeConfigRequired)
		}
	}
	return nil
}

func validBackupMode(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "disabled", "local", "object_storage", "hybrid":
		return true
	default:
		return false
	}
}

func validReleaseStrategy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "in_place", "rolling", "blue_green":
		return true
	default:
		return false
	}
}
