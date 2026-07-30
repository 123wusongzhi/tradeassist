package backupruntime

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// WALSegment is a metadata-only view used for continuity checks.
type WALSegment struct {
	Name      string
	Timeline  string
	StartTime time.Time
	EndTime   time.Time
	Checksum  string
}

// CheckWALContinuity validates a fake or real WAL inventory without exposing object keys.
func CheckWALContinuity(segments []WALSegment) error {
	if len(segments) == 0 {
		return fmt.Errorf("wal archive interrupted: no segments")
	}
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime.Before(segments[j].StartTime)
	})
	for i, s := range segments {
		if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Timeline) == "" || strings.TrimSpace(s.Checksum) == "" {
			return fmt.Errorf("wal archive interrupted: incomplete segment metadata")
		}
		if !s.EndTime.After(s.StartTime) {
			return fmt.Errorf("wal archive interrupted: invalid segment window")
		}
		if i > 0 && !segments[i-1].EndTime.Equal(s.StartTime) {
			return fmt.Errorf("wal archive interrupted: gap detected")
		}
	}
	return nil
}
