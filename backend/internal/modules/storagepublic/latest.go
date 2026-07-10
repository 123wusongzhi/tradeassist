package storagepublic

import (
	"sync"
	"time"

	storagepub "github.com/trademind-ai/trademind/backend/internal/pkg/storagepublic"
)

var (
	latestMu     sync.RWMutex
	latestResult *PublicCheckSnapshot
)

// PublicCheckSnapshot is the sanitized latest public-check result.
type PublicCheckSnapshot struct {
	Status           string                       `json:"status"`
	Provider         string                       `json:"provider,omitempty"`
	Checks           []storagepub.ValidationIssue `json:"checks,omitempty"`
	DouyinImageReady bool                         `json:"douyinImageReady"`
	Message          string                       `json:"message,omitempty"`
	ErrorCode        string                       `json:"errorCode,omitempty"`
	CheckedAt        string                       `json:"checkedAt"`
}

func saveLatest(res storagepub.EndToEndResult) {
	st := "passed"
	if !res.OK {
		st = "failed"
	} else if res.ErrorCode != "" {
		st = "passed_with_warning"
	}
	snap := &PublicCheckSnapshot{
		Status:           st,
		Provider:         res.StorageKind,
		DouyinImageReady: res.OK,
		Message:          res.Message,
		ErrorCode:        res.ErrorCode,
		CheckedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	latestMu.Lock()
	latestResult = snap
	latestMu.Unlock()
}

func getLatest() *PublicCheckSnapshot {
	latestMu.RLock()
	defer latestMu.RUnlock()
	if latestResult == nil {
		return nil
	}
	cp := *latestResult
	return &cp
}
