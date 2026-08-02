package collect

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var errCustomProfileSnapshotInvalid = errors.New("collect: custom browser profile snapshot is invalid")

// validateCustomProfileSnapshot re-authorizes a persisted task snapshot at
// execution time. The profile id and key are only locators; tenant ownership,
// active status and domain are resolved again from the worker TenantContext.
func (s *Service) validateCustomProfileSnapshot(
	ctx context.Context,
	rawURL string,
	profileIDRaw string,
	profileKeyRaw string,
) (string, error) {
	if s == nil || s.Profiles == nil || s.Profiles.DB == nil {
		return "", errCustomProfileSnapshotInvalid
	}
	profileID, err := uuid.Parse(strings.TrimSpace(profileIDRaw))
	if err != nil || profileID == uuid.Nil {
		return "", errCustomProfileSnapshotInvalid
	}
	snapshotKey := strings.TrimSpace(profileKeyRaw)
	if snapshotKey == "" {
		return "", errCustomProfileSnapshotInvalid
	}
	verified := make(map[string]any, 3)
	if err := s.Profiles.EnrichCollectorOptions(ctx, verified, &profileID, true, rawURL); err != nil {
		return "", errCustomProfileSnapshotInvalid
	}
	resolvedKey, ok := verified["profileKey"].(string)
	if !ok || strings.TrimSpace(resolvedKey) == "" || resolvedKey != snapshotKey {
		return "", errCustomProfileSnapshotInvalid
	}
	return resolvedKey, nil
}
