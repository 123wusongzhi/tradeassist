package backupruntime

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// PostgresTarget is the safe subset needed by pg_dump/pg_restore.
type PostgresTarget struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// DumpCommand builds pg_dump args without exposing the database password.
func DumpCommand(binary, format, output string, target PostgresTarget) (string, []string, []string, error) {
	if strings.TrimSpace(binary) == "" {
		binary = "pg_dump"
	}
	if strings.TrimSpace(format) == "" {
		format = "custom"
	}
	if strings.TrimSpace(output) == "" || strings.TrimSpace(target.Database) == "" {
		return "", nil, nil, fmt.Errorf("pg_dump: output and database are required")
	}
	args := []string{
		"--format=" + format,
		"--no-owner",
		"--no-acl",
		"--file", output,
		"--host", target.Host,
		"--port", fmt.Sprintf("%d", target.Port),
		"--username", target.User,
		target.Database,
	}
	env := []string{}
	if target.Password != "" {
		env = append(env, "PGPASSWORD="+target.Password)
	}
	return binary, args, env, nil
}

// RestoreListCommand builds pg_restore --list args for a logical backup file.
func RestoreListCommand(binary, backupFile string) (string, []string, error) {
	if strings.TrimSpace(binary) == "" {
		binary = "pg_restore"
	}
	if strings.TrimSpace(backupFile) == "" {
		return "", nil, fmt.Errorf("pg_restore --list: backup file is required")
	}
	return binary, []string{"--list", backupFile}, nil
}

// RestoreCommand builds pg_restore args for an explicitly selected isolated target.
func RestoreCommand(binary, backupFile string, target PostgresTarget) (string, []string, []string, error) {
	if strings.TrimSpace(binary) == "" {
		binary = "pg_restore"
	}
	if strings.TrimSpace(backupFile) == "" || strings.TrimSpace(target.Database) == "" {
		return "", nil, nil, fmt.Errorf("pg_restore: backup file and target database are required")
	}
	args := []string{
		"--no-owner",
		"--no-acl",
		"--dbname", target.Database,
		"--host", target.Host,
		"--port", fmt.Sprintf("%d", target.Port),
		"--username", target.User,
		backupFile,
	}
	env := []string{}
	if target.Password != "" {
		env = append(env, "PGPASSWORD="+target.Password)
	}
	return binary, args, env, nil
}

// RunCommand executes a safe argv command with timeout and a redacted summary.
func RunCommand(ctx context.Context, timeout time.Duration, binary string, args []string, env []string) error {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, binary, args...)
	cmd.Env = append(cmd.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timeout: %s", binary)
	}
	if err != nil {
		return fmt.Errorf("command failed: %s: %s", binary, RedactCommandOutput(string(out)))
	}
	return nil
}

// RedactCommandOutput strips common credential-bearing fragments.
func RedactCommandOutput(s string) string {
	s = strings.TrimSpace(s)
	replacers := []string{"password=", "secret=", "token=", "PGPASSWORD=", "Authorization:"}
	for _, r := range replacers {
		idx := strings.Index(strings.ToLower(s), strings.ToLower(r))
		if idx >= 0 {
			end := strings.IndexAny(s[idx:], " \n\r\t")
			if end < 0 {
				s = s[:idx] + r + "[redacted]"
			} else {
				s = s[:idx] + r + "[redacted]" + s[idx+end:]
			}
		}
	}
	if len(s) > 512 {
		return s[:512]
	}
	return s
}

// ValidateRecoveryTargetTime rejects unsafe PITR targets.
func ValidateRecoveryTargetTime(target, earliest, now time.Time) error {
	if target.IsZero() {
		return fmt.Errorf("recovery target time is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if target.After(now) {
		return fmt.Errorf("recovery target time cannot be in the future")
	}
	if !earliest.IsZero() && target.Before(earliest) {
		return fmt.Errorf("recovery target time is before earliest recoverable time")
	}
	return nil
}

// BuildRestoreCommand creates a safe restore_command template for PostgreSQL PITR docs.
func BuildRestoreCommand(archivePath string) (string, error) {
	u, err := url.Parse(archivePath)
	if err != nil || strings.TrimSpace(archivePath) == "" {
		return "", fmt.Errorf("invalid WAL archive path")
	}
	if u.Scheme != "" && u.Scheme != "file" && u.Scheme != "s3" && u.Scheme != "oss" && u.Scheme != "cos" {
		return "", fmt.Errorf("unsupported WAL archive scheme")
	}
	return "restore_wal --source='" + strings.ReplaceAll(archivePath, "'", "") + "' --wal='%f' --dest='%p'", nil
}
