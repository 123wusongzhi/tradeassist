package operationlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// hashChainVersion is the audit integrity schema version.
const hashChainVersion = 1

// appendHashChain computes prev_hash / entry_hash for a new log row.
func (s *Service) appendHashChain(tx *gorm.DB, row *OperationLog) error {
	if row == nil {
		return nil
	}
	partition := chainPartition(row.TenantID, row.CreatedAt)
	row.ChainPartition = partition
	row.HashVersion = hashChainVersion

	var prev OperationLog
	err := tx.Where("chain_partition = ?", partition).Order("created_at DESC").First(&prev).Error
	prevHash := ""
	if err == nil {
		prevHash = strings.TrimSpace(prev.EntryHash)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// first entry in partition
	} else {
		return err
	}
	row.PrevHash = prevHash
	row.EntryHash = computeEntryHash(prevHash, row)
	return nil
}

func chainPartition(tenantID int64, at time.Time) string {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return fmt.Sprintf("t%d:%s", tenantID, at.UTC().Format("2006-01-02"))
}

func computeEntryHash(prevHash string, row *OperationLog) string {
	summary := sha256.Sum256([]byte(strings.TrimSpace(row.Message)))
	payload := strings.Join([]string{
		prevHash,
		fmt.Sprintf("%d", row.TenantID),
		row.Action,
		row.Resource,
		row.ResourceID,
		row.Status,
		row.RequestID,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
		hex.EncodeToString(summary[:]),
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// VerifyChain checks hash chain integrity for a partition/date range.
func (s *Service) VerifyChain(ctx context.Context, tenantID int64, from, to time.Time) (int, *time.Time, error) {
	if s == nil || s.DB == nil {
		return 0, nil, fmt.Errorf("operationlog: no db")
	}
	tx := s.DB.WithContext(ctx).Model(&OperationLog{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at <= ?", tenantID, from, to).
		Order("created_at ASC")
	var rows []OperationLog
	if err := tx.Find(&rows).Error; err != nil {
		return 0, nil, err
	}
	prev := ""
	for i, row := range rows {
		expected := computeEntryHash(prev, &row)
		if strings.TrimSpace(row.EntryHash) == "" {
			continue
		}
		if row.EntryHash != expected || (i > 0 && row.PrevHash != prev) {
			t := row.CreatedAt
			return i, &t, fmt.Errorf("audit chain mismatch at index %d", i)
		}
		prev = row.EntryHash
	}
	return len(rows), nil, nil
}
