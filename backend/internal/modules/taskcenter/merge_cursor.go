package taskcenter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
)

const (
	taskMergeCursorVersion = 1
	maxTaskMergeSources    = 12
	taskSortColumn         = "updated_at"
	taskIDColumn           = "id::text"
)

// TaskSourceCursor tracks per-source keyset position during multi-source merge.
type TaskSourceCursor struct {
	SourceID  string `json:"sourceId"`
	SortTime  string `json:"sortTime"`
	ID        string `json:"id"`
	Exhausted bool   `json:"exhausted"`
}

// TaskMergeCursorPayload stores bounded merge state for signed task cursors.
type TaskMergeCursorPayload struct {
	Version           int                `json:"version"`
	TenantID          int64              `json:"tenantId"`
	ShopScopeHash     string             `json:"shopScopeHash"`
	FilterFingerprint string             `json:"filterFingerprint"`
	Sources           []TaskSourceCursor `json:"sources"`
}

func encodeTaskMergeCursor(p TaskMergeCursorPayload) (string, error) {
	p.Version = taskMergeCursorVersion
	if len(p.Sources) > maxTaskMergeSources {
		return "", fmt.Errorf("%w: too many merge sources", pagination.ErrCursorSignatureInvalid)
	}
	return pagination.EncodeSignedJSONMax(p, pagination.MaxMergeCursorLen)
}

func decodeTaskMergeCursor(raw string, tenantID int64, shopScopeHash, filterFingerprint string) (TaskMergeCursorPayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return TaskMergeCursorPayload{}, nil
	}
	var p TaskMergeCursorPayload
	if err := pagination.DecodeSignedJSONMax(raw, &p, pagination.MaxMergeCursorLen); err != nil {
		return TaskMergeCursorPayload{}, err
	}
	if p.Version != taskMergeCursorVersion {
		return TaskMergeCursorPayload{}, pagination.ErrCursorVersionUnsupported
	}
	if p.TenantID != tenantID {
		return TaskMergeCursorPayload{}, pagination.ErrCursorScopeMismatch
	}
	if strings.TrimSpace(p.ShopScopeHash) != strings.TrimSpace(shopScopeHash) {
		return TaskMergeCursorPayload{}, pagination.ErrCursorScopeMismatch
	}
	if strings.TrimSpace(p.FilterFingerprint) != strings.TrimSpace(filterFingerprint) {
		return TaskMergeCursorPayload{}, pagination.ErrCursorFilterMismatch
	}
	if err := validateTaskMergeSources(p.Sources); err != nil {
		return TaskMergeCursorPayload{}, err
	}
	return p, nil
}

func validateTaskMergeSources(sources []TaskSourceCursor) error {
	if len(sources) > maxTaskMergeSources {
		return fmt.Errorf("%w: too many merge sources", pagination.ErrCursorSignatureInvalid)
	}
	seen := map[string]struct{}{}
	for _, s := range sources {
		id := strings.TrimSpace(s.SourceID)
		if id == "" {
			return fmt.Errorf("%w: missing source id", pagination.ErrCursorSignatureInvalid)
		}
		if _, ok := allowedSourceIDs[id]; !ok {
			return fmt.Errorf("%w: unknown source %s", pagination.ErrCursorSignatureInvalid, id)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: duplicate source %s", pagination.ErrCursorSignatureInvalid, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func initSourceCursors(types []string, merge *TaskMergeCursorPayload) map[string]TaskSourceCursor {
	out := map[string]TaskSourceCursor{}
	if merge != nil {
		for _, s := range merge.Sources {
			out[strings.TrimSpace(s.SourceID)] = s
		}
	}
	for _, tt := range types {
		if _, ok := out[tt]; !ok {
			out[tt] = TaskSourceCursor{SourceID: tt}
		}
	}
	return out
}

func buildNextMergeCursor(hasMore bool, tenantID int64, shopScopeHash, filterFingerprint string, sources map[string]TaskSourceCursor, types []string) (string, error) {
	if !hasMore {
		return "", nil
	}
	payload := TaskMergeCursorPayload{
		TenantID:          tenantID,
		ShopScopeHash:     shopScopeHash,
		FilterFingerprint: filterFingerprint,
		Sources:           make([]TaskSourceCursor, 0, len(types)),
	}
	for _, tt := range types {
		cur, ok := sources[tt]
		if !ok {
			cur = TaskSourceCursor{SourceID: tt, Exhausted: true}
		}
		if cur.SourceID == "" {
			cur.SourceID = tt
		}
		payload.Sources = append(payload.Sources, cur)
	}
	sort.Slice(payload.Sources, func(i, j int) bool {
		return payload.Sources[i].SourceID < payload.Sources[j].SourceID
	})
	return encodeTaskMergeCursor(payload)
}

func applyTaskSourceKeyset(cur TaskSourceCursor) pagination.CursorPayload {
	if cur.Exhausted || (strings.TrimSpace(cur.SortTime) == "" && strings.TrimSpace(cur.ID) == "") {
		return pagination.CursorPayload{}
	}
	return pagination.CursorPayload{
		SortValue: strings.TrimSpace(cur.SortTime),
		TieID:     strings.TrimSpace(cur.ID),
	}
}
