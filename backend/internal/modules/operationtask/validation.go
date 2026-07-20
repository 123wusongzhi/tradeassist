package operationtask

import (
	"encoding/json"
	"regexp"
	"strings"

	"gorm.io/datatypes"
)

var sha256LowerHex = regexp.MustCompile(`^[0-9a-f]{64}$`)

var allowedOperationTaskSources = map[string]bool{
	OperationTaskSourceManual:         true,
	OperationTaskSourceAISuggestion:   true,
	OperationTaskSourceRuleEngine:     true,
	OperationTaskSourceOrderException: true,
	OperationTaskSourceProductContent: true,
}

var allowedOperationTaskTypes = map[string]bool{
	OperationTaskTypeProductContent: true,
	OperationTaskTypeOrderException: true,
	OperationTaskTypeProductPublish: true,
	OperationTaskTypeInventorySync:  true,
	OperationTaskTypeCustomerReply:  true,
	OperationTaskTypeAIText:         true,
	OperationTaskTypeAIImage:        true,
	OperationTaskTypeManualReview:   true,
}

var allowedPlatforms = map[string]bool{
	PlatformLocal:  true,
	PlatformDouyin: true,
	"amazon":       true,
	"lazada":       true,
	"shopee":       true,
	"tiktok":       true,
	"woocommerce":  true,
	"custom":       true,
}

var allowedOperationTaskStatuses = map[string]bool{
	OperationTaskStatusSuggested:       true,
	OperationTaskStatusDraftPreparing:  true,
	OperationTaskStatusPendingReview:   true,
	OperationTaskStatusApproved:        true,
	OperationTaskStatusRejected:        true,
	OperationTaskStatusExecutionQueued: true,
	OperationTaskStatusExecuting:       true,
	OperationTaskStatusDraftWritten:    true,
	OperationTaskStatusExecutionFailed: true,
	OperationTaskStatusCancelled:       true,
}

var allowedPriorities = map[string]bool{
	OperationTaskPriorityLow:    true,
	OperationTaskPriorityNormal: true,
	OperationTaskPriorityHigh:   true,
	OperationTaskPriorityUrgent: true,
}

var allowedAdapterModes = map[string]bool{
	AdapterModeMock:           true,
	AdapterModeSandbox:        true,
	AdapterModeLocalDraftOnly: true,
}

var allowedPlatformDraftStatuses = map[string]bool{
	PlatformDraftStatusEditable:      true,
	PlatformDraftStatusPendingReview: true,
	PlatformDraftStatusApproved:      true,
	PlatformDraftStatusSuperseded:    true,
	PlatformDraftStatusWritten:       true,
	PlatformDraftStatusFailed:        true,
}

func validateOperationTask(t *OperationTask) error {
	if t == nil {
		return ErrValidation
	}
	normalizeOperationTask(t)
	switch {
	case t.TenantID <= 0:
		return ErrValidation
	case !allowedOperationTaskSources[t.SourceType]:
		return ErrValidation
	case !allowedOperationTaskTypes[t.TaskType]:
		return ErrValidation
	case !allowedPlatforms[t.Platform]:
		return ErrValidation
	case strings.TrimSpace(t.Title) == "":
		return ErrValidation
	case !isValidJSON(t.Payload):
		return ErrValidation
	case payloadHasSecret(t.Payload):
		return ErrValidation
	case !allowedOperationTaskStatuses[t.Status]:
		return ErrValidation
	case !allowedPriorities[t.Priority]:
		return ErrValidation
	case t.Revision < 1:
		return ErrValidation
	}
	return nil
}

func validatePlatformDraft(d *PlatformDraft) error {
	if d == nil {
		return ErrValidation
	}
	normalizePlatformDraft(d)
	switch {
	case d.TenantID <= 0:
		return ErrValidation
	case d.OperationTaskID.String() == "00000000-0000-0000-0000-000000000000":
		return ErrValidation
	case !allowedPlatforms[d.Platform]:
		return ErrValidation
	case !allowedAdapterModes[d.AdapterMode]:
		return ErrValidation
	case d.DraftVersion < 1:
		return ErrValidation
	case !isValidJSON(d.Payload):
		return ErrValidation
	case payloadHasSecret(d.Payload):
		return ErrValidation
	case !sha256LowerHex.MatchString(d.PayloadHash):
		return ErrValidation
	case !allowedPlatformDraftStatuses[d.Status]:
		return ErrValidation
	}
	return nil
}

func payloadHasSecret(raw datatypes.JSON) bool {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return true
	}
	return valueHasSecret(v)
}

func valueHasSecret(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if sensitivePayloadKey(k) {
				return true
			}
			if valueHasSecret(child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if valueHasSecret(child) {
				return true
			}
		}
	}
	return false
}

func sensitivePayloadKey(key string) bool {
	k := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
	for _, needle := range []string{
		"secret", "token", "cookie", "credential", "password", "api_key", "apikey", "access_token", "refresh_token",
	} {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}
