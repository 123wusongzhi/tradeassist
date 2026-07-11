package tasklease

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ClaimResult holds execution identity after a successful task claim.
type ClaimResult struct {
	ExecutionID  uuid.UUID
	LeaseVersion int
}

// TryClaim atomically assigns a pending task to workerID with a new execution identity.
// table is the physical table name; pendingStatus and runningStatus are module-specific status values.
func TryClaim(ctx context.Context, db *gorm.DB, table, pendingStatus, runningStatus string, taskID uuid.UUID, workerID string, lease time.Duration) (ClaimResult, bool, error) {
	if db == nil {
		return ClaimResult{}, false, fmt.Errorf("tasklease: no db")
	}
	if table == "" || pendingStatus == "" || runningStatus == "" {
		return ClaimResult{}, false, fmt.Errorf("tasklease: table and statuses required")
	}
	if lease <= 0 {
		lease = 90 * time.Second
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	execID := uuid.New()
	res := db.WithContext(ctx).Table(table).
		Where(`id = ? AND status = ? AND (locked_by IS NULL OR locked_until < ?)`, taskID, pendingStatus, now).
		Updates(map[string]any{
			"status":       runningStatus,
			"locked_by":    workerID,
			"locked_until": &until,
			"lock_version": gorm.Expr("lock_version + 1"),
			"heartbeat_at": &now,
			"execution_id": execID.String(),
			"started_at":   gorm.Expr("COALESCE(started_at, ?)", now),
			"updated_at":   now,
		})
	if res.Error != nil {
		return ClaimResult{}, false, res.Error
	}
	if res.RowsAffected == 0 {
		return ClaimResult{}, false, nil
	}
	var leaseVersion int
	if err := db.WithContext(ctx).Table(table).Where("id = ?", taskID).Select("lock_version").Scan(&leaseVersion).Error; err != nil {
		return ClaimResult{}, false, err
	}
	return ClaimResult{ExecutionID: execID, LeaseVersion: leaseVersion}, true, nil
}

// TryClaimPendingOrRetrying claims a pending task, or a retrying task whose next_retry_at is NULL
// (already due-enqueued). Clears error_message, finished_at, and retry_enqueued_at on claim.
func TryClaimPendingOrRetrying(ctx context.Context, db *gorm.DB, table, pendingStatus, retryingStatus, runningStatus string, taskID uuid.UUID, workerID string, lease time.Duration) (ClaimResult, bool, error) {
	if db == nil {
		return ClaimResult{}, false, fmt.Errorf("tasklease: no db")
	}
	if table == "" || pendingStatus == "" || retryingStatus == "" || runningStatus == "" {
		return ClaimResult{}, false, fmt.Errorf("tasklease: table and statuses required")
	}
	if lease <= 0 {
		lease = 90 * time.Second
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	execID := uuid.New()
	res := db.WithContext(ctx).Table(table).
		Where(`id = ? AND (status = ? OR (status = ? AND next_retry_at IS NULL)) AND (locked_by IS NULL OR locked_until < ?)`,
			taskID, pendingStatus, retryingStatus, now).
		Updates(map[string]any{
			"status":            runningStatus,
			"locked_by":         workerID,
			"locked_until":      &until,
			"lock_version":      gorm.Expr("lock_version + 1"),
			"heartbeat_at":      &now,
			"execution_id":      execID.String(),
			"started_at":        gorm.Expr("COALESCE(started_at, ?)", now),
			"error_message":     "",
			"finished_at":       nil,
			"retry_enqueued_at": nil,
			"updated_at":        now,
		})
	if res.Error != nil {
		return ClaimResult{}, false, res.Error
	}
	if res.RowsAffected == 0 {
		return ClaimResult{}, false, nil
	}
	var leaseVersion int
	if err := db.WithContext(ctx).Table(table).Where("id = ?", taskID).Select("lock_version").Scan(&leaseVersion).Error; err != nil {
		return ClaimResult{}, false, err
	}
	return ClaimResult{ExecutionID: execID, LeaseVersion: leaseVersion}, true, nil
}

// RenewHeartbeat extends lease for the current execution identity.
func RenewHeartbeat(ctx context.Context, db *gorm.DB, table, runningStatus string, taskID uuid.UUID, workerID string, executionID uuid.UUID, leaseVersion int, lease time.Duration) error {
	if db == nil {
		return fmt.Errorf("tasklease: no db")
	}
	if lease <= 0 {
		lease = 90 * time.Second
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	res := db.WithContext(ctx).Table(table).
		Where(`id = ? AND status = ? AND locked_by = ? AND execution_id = ? AND lock_version = ?`,
			taskID, runningStatus, workerID, executionID.String(), leaseVersion).
		Updates(map[string]any{
			"locked_until": &until,
			"heartbeat_at": &now,
			"updated_at":   now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLeaseLost
	}
	return nil
}

// ValidateLease verifies the worker still holds execution rights before writing results.
func ValidateLease(ctx context.Context, db *gorm.DB, table, runningStatus string, taskID uuid.UUID, workerID string, executionID uuid.UUID, leaseVersion int) error {
	if db == nil {
		return fmt.Errorf("tasklease: no db")
	}
	now := time.Now().UTC()
	var n int64
	err := db.WithContext(ctx).Table(table).
		Where(`id = ? AND status = ? AND locked_by = ? AND execution_id = ? AND lock_version = ? AND locked_until >= ?`,
			taskID, runningStatus, workerID, executionID.String(), leaseVersion, now).
		Count(&n).Error
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLeaseLost
	}
	return nil
}

// StartRenewal runs periodic heartbeat renewal until ctx is cancelled.
func StartRenewal(ctx context.Context, db *gorm.DB, table, runningStatus string, taskID uuid.UUID, workerID string, executionID uuid.UUID, leaseVersion int, leaseTTL time.Duration) (stop func()) {
	if db == nil {
		return func() {}
	}
	interval := leaseTTL / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	runCtx, cancel := context.WithCancel(ctx)
	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-tick.C:
				hbCtx, hbCancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = RenewHeartbeat(hbCtx, db, table, runningStatus, taskID, workerID, executionID, leaseVersion, leaseTTL)
				hbCancel()
			}
		}
	}()
	return cancel
}

// TakeoverExpired reclaims a running task whose lease has expired.
func TakeoverExpired(ctx context.Context, db *gorm.DB, table, runningStatus string, taskID uuid.UUID, workerID string, lease time.Duration, staleHeartbeatBefore time.Time) (ClaimResult, bool, error) {
	if db == nil {
		return ClaimResult{}, false, fmt.Errorf("tasklease: no db")
	}
	if lease <= 0 {
		lease = 90 * time.Second
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	execID := uuid.New()
	res := db.WithContext(ctx).Table(table).
		Where(`id = ? AND status = ? AND locked_until < ? AND (heartbeat_at IS NULL OR heartbeat_at < ?)`,
			taskID, runningStatus, now, staleHeartbeatBefore).
		Updates(map[string]any{
			"locked_by":    workerID,
			"locked_until": &until,
			"lock_version": gorm.Expr("lock_version + 1"),
			"heartbeat_at": &now,
			"execution_id": execID.String(),
			"updated_at":   now,
		})
	if res.Error != nil {
		return ClaimResult{}, false, res.Error
	}
	if res.RowsAffected == 0 {
		return ClaimResult{}, false, nil
	}
	var leaseVersion int
	if err := db.WithContext(ctx).Table(table).Where("id = ?", taskID).Select("lock_version").Scan(&leaseVersion).Error; err != nil {
		return ClaimResult{}, false, err
	}
	return ClaimResult{ExecutionID: execID, LeaseVersion: leaseVersion}, true, nil
}
