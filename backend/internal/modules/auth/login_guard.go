package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/p7diag"
	"gorm.io/gorm"
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
		err := g.DB.WithContext(ctx).Where("account_key = ?", key).First(&row).Error
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_read", authOutcome(err), stageStart)
		p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_read", authOutcome(err), stageStart)
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
		var row AuthLoginAttempt
		stageStart := time.Now()
		err := g.DB.WithContext(ctx).Where("account_key = ?", key).First(&row).Error
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_read", authOutcome(err), stageStart)
		p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_read", authOutcome(err), stageStart)
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "failedAttemptReadCount", 1)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = AuthLoginAttempt{
				AccountKey:   key,
				IPHash:       authutil.HashIP(ip),
				FailedCount:  1,
				LastFailedAt: &now,
			}
			stageStart = time.Now()
			if err := g.DB.WithContext(ctx).Create(&row).Error; err != nil {
				p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", authOutcome(err), stageStart)
				p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", authOutcome(err), stageStart)
				return err
			}
			p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", p7diag.OutcomeSuccess, stageStart)
			p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", p7diag.OutcomeSuccess, stageStart)
			p7diag.Count(p7diag.RouteAuthInvalidLogin, "failedAttemptWriteCount", 1)
			continue
		}
		if err != nil {
			return err
		}
		if row.LastFailedAt != nil && now.Sub(*row.LastFailedAt) > window {
			row.FailedCount = 0
		}
		row.FailedCount++
		row.LastFailedAt = &now
		if maxAttempts > 0 && row.FailedCount >= maxAttempts {
			lockUntil := now.Add(time.Duration(lockMinutes) * time.Minute)
			row.LockedUntil = &lockUntil
		}
		stageStart = time.Now()
		if err := g.DB.WithContext(ctx).Save(&row).Error; err != nil {
			p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", authOutcome(err), stageStart)
			p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", authOutcome(err), stageStart)
			return err
		}
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", p7diag.OutcomeSuccess, stageStart)
		p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "failed_attempt_write", p7diag.OutcomeSuccess, stageStart)
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
	if cfg == nil {
		return len(password) < 8
	}
	min := cfg.AuthPasswordMinLength()
	if min <= 0 {
		min = 8
	}
	if len(password) < min {
		return true
	}
	low := strings.ToLower(strings.TrimSpace(password))
	weak := []string{
		"password", "12345678", "admin123", "changeme", "trademind",
		"admin@123", "test1234", "qwerty123", "11111111",
	}
	for _, w := range weak {
		if low == w {
			return true
		}
	}
	if cfg.BootstrapAdminPassword != "" && password == cfg.BootstrapAdminPassword && config.IsProduction(cfg.AppEnv) {
		return true
	}
	return false
}
