package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/p7diag"
	"github.com/trademind-ai/trademind/backend/internal/pkg/passwordpolicy"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LoginGuard enforces login rate limits and temporary account lockout.
type LoginGuard struct {
	Cfg *config.Config
	DB  *gorm.DB
}

func (g *LoginGuard) accountKey(account string) string {
	return strings.ToLower(strings.TrimSpace(account))
}

func (g *LoginGuard) compositeKey(account, ip string) string {
	return g.accountKey(account) + "|" + authutil.HashIP(ip)
}

// CheckAllowed returns error if account/IP is locked or rate limited.
func (g *LoginGuard) CheckAllowed(ctx context.Context, account, ip string) error {
	if g == nil || g.Cfg == nil || g.DB == nil {
		return nil
	}
	now := time.Now().UTC()
	keys := []string{g.accountKey(account), g.compositeKey(account, ip)}
	for _, key := range keys {
		if key == "" || key == "|" {
			continue
		}
		var row AuthLoginAttempt
		stageStart := time.Now()
		timing, err := p7diag.TimedGorm(g.DB, func() error {
			return g.DB.WithContext(ctx).Where("account_key = ?", key).First(&row).Error
		})
		outcome := authOutcome(err)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_read", outcome, stageStart)
		p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_read", outcome, stageStart)
		p7diag.ObserveSQL(p7diag.RouteAuthInvalidLogin, "auth", "auth.failed_attempt_read", "select", "auth_login_attempts", outcome, false, timing)
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "failedAttemptReadCount", 1)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if row.LockedUntil != nil && now.Before(*row.LockedUntil) {
			return errors.New(ErrAccountTemporarilyLocked)
		}
	}
	return nil
}

// RecordFailure increments failure counter and may lock the account.
func (g *LoginGuard) RecordFailure(ctx context.Context, account, ip string) error {
	if g == nil || g.Cfg == nil || g.DB == nil {
		return nil
	}
	now := time.Now().UTC()
	window := time.Duration(g.Cfg.AuthLoginWindowMinutes()) * time.Minute
	maxAttempts := g.Cfg.AuthLoginMaxAttempts()
	lockMinutes := g.Cfg.AuthAccountLockMinutes()

	keys := []string{g.accountKey(account), g.compositeKey(account, ip)}
	for _, key := range keys {
		if key == "" || key == "|" {
			continue
		}
		stageStart := time.Now()
		writeTiming, writeErr := p7diag.TimedGormRows(g.DB, func() (int64, error) {
			cutoff := now.Add(-window)
			lockUntil := now.Add(time.Duration(lockMinutes) * time.Minute)
			count := "CASE WHEN auth_login_attempts.last_failed_at IS NULL OR auth_login_attempts.last_failed_at < ? THEN 1 ELSE auth_login_attempts.failed_count + 1 END"
			res := g.DB.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "account_key"}},
				DoUpdates: clause.Assignments(map[string]any{
					"ip_hash":        authutil.HashIP(ip),
					"failed_count":   gorm.Expr(count, cutoff),
					"last_failed_at": now,
					"locked_until":   gorm.Expr("CASE WHEN ? > 0 AND ("+count+") >= ? THEN ? ELSE auth_login_attempts.locked_until END", maxAttempts, cutoff, maxAttempts, lockUntil),
				}),
			}).Create(&AuthLoginAttempt{AccountKey: key, IPHash: authutil.HashIP(ip), FailedCount: 1, LastFailedAt: &now})
			return res.RowsAffected, res.Error
		})
		writeOutcome := authOutcome(writeErr)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", writeOutcome, stageStart)
		p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", writeOutcome, stageStart)
		p7diag.ObserveSQL(p7diag.RouteAuthInvalidLogin, "auth", "auth.failed_attempt_update", "update", "auth_login_attempts", writeOutcome, false, writeTiming)
		if writeErr != nil {
			return writeErr
		}
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "failedAttemptWriteCount", 1)
	}
	return nil
}

// ClearFailures resets counters after successful login.
func (g *LoginGuard) ClearFailures(ctx context.Context, account, ip string) error {
	if g == nil || g.DB == nil {
		return nil
	}
	keys := []string{g.accountKey(account), g.compositeKey(account, ip)}
	for _, key := range keys {
		if key == "" || key == "|" {
			continue
		}
		_ = g.DB.WithContext(ctx).Where("account_key = ?", key).Delete(&AuthLoginAttempt{}).Error
	}
	return nil
}

// IsWeakPassword rejects common example passwords and enforces minimum length.
func IsWeakPassword(cfg *config.Config, password string) bool {
	min := 8
	forbidden := ""
	if cfg != nil {
		min = cfg.AuthPasswordMinLength()
		if config.IsProduction(cfg.AppEnv) {
			forbidden = cfg.BootstrapAdminPassword
		}
	}
	if min <= 0 {
		min = 8
	}
	return passwordpolicy.IsWeakWithForbidden(password, min, forbidden)
}
