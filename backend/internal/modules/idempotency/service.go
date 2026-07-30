package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DefaultLease is how long a processing record holds execution rights.
const DefaultLease = 2 * time.Minute

// DefaultTTL is how long completed records are kept for replay.
const DefaultTTL = 7 * 24 * time.Hour

// AcquireResult describes the outcome of an Acquire call.
type AcquireResult struct {
	Record        *Record
	Acquired      bool
	Replay        bool
	ReplaySummary string
	ResourceID    string
}

// Service provides unified idempotency acquire/complete/fail operations.
type Service struct {
	DB *gorm.DB
}

func (s *Service) WithDB(db *gorm.DB) *Service {
	if db == nil {
		return s
	}
	return &Service{DB: db}
}

// Acquire attempts to obtain execution rights for scope+key.
func (s *Service) Acquire(ctx context.Context, scope, key, requestHash, owner string, lease time.Duration) (*AcquireResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("idempotency: unavailable")
	}
	scope = norm(scope)
	key = norm(key)
	requestHash = norm(requestHash)
	if scope == "" || key == "" || requestHash == "" {
		return nil, fmt.Errorf("idempotency: scope, key and requestHash are required")
	}
	if lease <= 0 {
		lease = DefaultLease
	}
	now := time.Now().UTC()
	expires := now.Add(DefaultTTL)
	lockedUntil := now.Add(lease)

	var out AcquireResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rec Record
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("scope = ? AND idempotency_key = ?", scope, key).
			First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rec = Record{
				Scope:          scope,
				IdempotencyKey: key,
				RequestHash:    requestHash,
				Status:         StatusProcessing,
				Owner:          owner,
				LockedUntil:    &lockedUntil,
				ExpiresAt:      &expires,
			}
			if err := tx.Create(&rec).Error; err != nil {
				return err
			}
			out.Record = &rec
			out.Acquired = true
			return nil
		}

		if rec.RequestHash != requestHash {
			return newOpErr(ErrCodeKeyConflict, "idempotency key reused with different payload", &rec)
		}
		if rec.ExpiresAt != nil && rec.ExpiresAt.Before(now) {
			return newOpErr(ErrCodeRecordExpired, "idempotency record expired", &rec)
		}

		switch rec.Status {
		case StatusSucceeded:
			out.Record = &rec
			out.Replay = true
			out.ReplaySummary = rec.ResponseSummary
			out.ResourceID = rec.ResourceID
			return newOpErr(ErrCodeAlreadySucceeded, "already succeeded", &rec)
		case StatusProcessing:
			if rec.LockedUntil != nil && rec.LockedUntil.After(now) && rec.Owner != owner {
				return newOpErr(ErrCodeInProgress, "another worker is processing", &rec)
			}
			// Lease expired or same owner — take over.
			if err := tx.Model(&rec).Updates(map[string]any{
				"status":       StatusProcessing,
				"owner":        owner,
				"locked_until": &lockedUntil,
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
			rec.Owner = owner
			rec.LockedUntil = &lockedUntil
			rec.Status = StatusProcessing
			out.Record = &rec
			out.Acquired = true
			return nil
		case StatusFailedRetryable:
			if err := tx.Model(&rec).Updates(map[string]any{
				"status":       StatusProcessing,
				"owner":        owner,
				"locked_until": &lockedUntil,
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
			rec.Status = StatusProcessing
			rec.Owner = owner
			rec.LockedUntil = &lockedUntil
			out.Record = &rec
			out.Acquired = true
			return nil
		case StatusFailedPermanent:
			return newOpErr(ErrCodeKeyConflict, "permanent failure — manual intervention required", &rec)
		default:
			if err := tx.Model(&rec).Updates(map[string]any{
				"status":       StatusProcessing,
				"owner":        owner,
				"locked_until": &lockedUntil,
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
			out.Record = &rec
			out.Acquired = true
			return nil
		}
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Heartbeat extends the lease for a processing record owned by owner.
func (s *Service) Heartbeat(ctx context.Context, recordID uuid.UUID, owner string, lease time.Duration) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("idempotency: unavailable")
	}
	if lease <= 0 {
		lease = DefaultLease
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	res := s.DB.WithContext(ctx).Model(&Record{}).
		Where("id = ? AND status = ? AND owner = ?", recordID, StatusProcessing, owner).
		Updates(map[string]any{
			"locked_until": &until,
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

// CompleteResult holds summary fields for a successful operation.
type CompleteResult struct {
	ResponseCode    string
	ResponseSummary string
	ResourceType    string
	ResourceID      string
}

// Complete marks a record succeeded; verifies lease ownership.
func (s *Service) Complete(ctx context.Context, recordID uuid.UUID, owner string, result CompleteResult) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("idempotency: unavailable")
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&Record{}).
		Where("id = ? AND status = ? AND owner = ? AND (locked_until IS NULL OR locked_until >= ?)",
			recordID, StatusProcessing, owner, now).
		Updates(map[string]any{
			"status":           StatusSucceeded,
			"response_code":    result.ResponseCode,
			"response_summary": result.ResponseSummary,
			"resource_type":    result.ResourceType,
			"resource_id":      result.ResourceID,
			"completed_at":     &now,
			"locked_until":     nil,
			"updated_at":       now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLeaseLost
	}
	return nil
}

// Fail marks a record failed with retry classification.
func (s *Service) Fail(ctx context.Context, recordID uuid.UUID, owner, errorCode string, retryable bool) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("idempotency: unavailable")
	}
	status := StatusFailedPermanent
	if retryable {
		status = StatusFailedRetryable
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&Record{}).
		Where("id = ? AND status = ? AND owner = ?", recordID, StatusProcessing, owner).
		Updates(map[string]any{
			"status":       status,
			"error_code":   errorCode,
			"retryable":    retryable,
			"locked_until": nil,
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

// Get returns the latest record for scope+key.
func (s *Service) Get(ctx context.Context, scope, key string) (*Record, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("idempotency: unavailable")
	}
	var rec Record
	err := s.DB.WithContext(ctx).
		Where("scope = ? AND idempotency_key = ?", norm(scope), norm(key)).
		First(&rec).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ReleaseExpired marks expired records and clears stale processing leases.
func (s *Service) ReleaseExpired(ctx context.Context, limit int) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("idempotency: unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&Record{}).
		Where("(expires_at IS NOT NULL AND expires_at < ?) OR (status = ? AND locked_until IS NOT NULL AND locked_until < ?)",
			now, StatusProcessing, now).
		Limit(limit).
		Updates(map[string]any{
			"status":       StatusExpired,
			"locked_until": nil,
			"updated_at":   now,
		})
	return res.RowsAffected, res.Error
}
