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

var allowedApprovalDecisions = map[string]bool{
	ApprovalDecisionApproved: true,
	ApprovalDecisionRejected: true,
}

var allowedReviewerRoles = map[string]bool{
	ReviewerRoleReviewer: true,
	ReviewerRoleAdmin:    true,
}

var allowedExecutionAttemptStatuses = map[string]bool{
	ExecutionAttemptStatusQueued:    true,
	ExecutionAttemptStatusRunning:   true,
	ExecutionAttemptStatusSucceeded: true,
	ExecutionAttemptStatusFailed:    true,
	ExecutionAttemptStatusCancelled: true,
}

var allowedExecutionErrorCategories = map[string]bool{
	ExecutionErrorCategoryValidation:          true,
	ExecutionErrorCategoryPermissionDenied:    true,
	ExecutionErrorCategoryStateConflict:       true,
	ExecutionErrorCategoryAdapterUnavailable:  true,
	ExecutionErrorCategoryProviderTimeout:     true,
	ExecutionErrorCategoryProviderRejected:    true,
	ExecutionErrorCategoryIdempotencyConflict: true,
	ExecutionErrorCategoryInternal:            true,
}

var allowedExecutionResultTypes = map[string]bool{
	"":                true,
	"local_draft":     true,
	"mock_draft":      true,
	"sandbox_fixture": true,
}

var allowedOperationTaskEventTypes = map[string]bool{
	OperationTaskEventTypeTaskCreated:      true,
	OperationTaskEventTypeDraftGenerated:   true,
	OperationTaskEventTypeDraftUpdated:     true,
	OperationTaskEventTypeReviewRequested:  true,
	OperationTaskEventTypeApproved:         true,
	OperationTaskEventTypeRejected:         true,
	OperationTaskEventTypeExecutionQueued:  true,
	OperationTaskEventTypeExecutionStarted: true,
	OperationTaskEventTypeDraftWritten:     true,
	OperationTaskEventTypeExecutionFailed:  true,
	OperationTaskEventTypeRetryRequested:   true,
	OperationTaskEventTypeCancelled:        true,
}

var allowedOperationTaskEventActors = map[string]bool{
	OperationTaskEventActorUser:   true,
	OperationTaskEventActorSystem: true,
	OperationTaskEventActorAI:     true,
	OperationTaskEventActorRule:   true,
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

func validateApprovalRecord(a *ApprovalRecord) error {
	if a == nil {
		return ErrValidation
	}
	normalizeApprovalRecord(a)
	switch {
	case a.TenantID <= 0:
		return ErrValidation
	case a.OperationTaskID == uuidNil:
		return ErrValidation
	case a.PlatformDraftID == uuidNil:
		return ErrValidation
	case !allowedApprovalDecisions[a.Decision]:
		return ErrValidation
	case a.DraftVersion < 1:
		return ErrValidation
	case !sha256LowerHex.MatchString(a.DraftPayloadHash):
		return ErrValidation
	case a.ReviewerID == uuidNil:
		return ErrValidation
	case !allowedReviewerRoles[a.ReviewerRole]:
		return ErrValidation
	case a.Decision == ApprovalDecisionRejected && a.Reason == "":
		return ErrValidation
	}
	return nil
}

func validateExecutionAttempt(a *ExecutionAttempt) error {
	if a == nil {
		return ErrValidation
	}
	normalizeExecutionAttempt(a)
	switch {
	case a.TenantID <= 0:
		return ErrValidation
	case a.OperationTaskID == uuidNil:
		return ErrValidation
	case a.PlatformDraftID == uuidNil:
		return ErrValidation
	case a.ApprovalRecordID == uuidNil:
		return ErrValidation
	case a.AttemptNumber < 1:
		return ErrValidation
	case !allowedExecutionAttemptStatuses[a.Status]:
		return ErrValidation
	case !allowedAdapterModes[a.AdapterMode]:
		return ErrValidation
	case !allowedPlatforms[a.Platform]:
		return ErrValidation
	case a.ApprovedDraftVersion < 1 || a.ExecutedDraftVersion < 1:
		return ErrValidation
	case !sha256LowerHex.MatchString(a.ApprovedDraftPayloadHash):
		return ErrValidation
	case !sha256LowerHex.MatchString(a.ExecutedDraftPayloadHash):
		return ErrValidation
	case !allowedExecutionResultTypes[a.ResultType]:
		return ErrValidation
	case len(a.SafeMetadata) > 8192:
		return ErrValidation
	case len(a.SafeMetadata) > 0 && !isValidJSON(a.SafeMetadata):
		return ErrValidation
	case len(a.SafeMetadata) > 0 && payloadHasSecret(a.SafeMetadata):
		return ErrValidation
	case a.Revision < 1:
		return ErrValidation
	}
	return nil
}

func validateExecutionError(e *ExecutionError) error {
	if e == nil {
		return ErrValidation
	}
	normalizeExecutionError(e)
	switch {
	case e.TenantID <= 0:
		return ErrValidation
	case e.ExecutionAttemptID == uuidNil:
		return ErrValidation
	case e.Sequence < 1:
		return ErrValidation
	case !allowedExecutionErrorCategories[e.Category]:
		return ErrValidation
	case e.Code == "" || len(e.Code) > 128:
		return ErrValidation
	case e.SafeMessage == "" || len(e.SafeMessage) > 2048:
		return ErrValidation
	case safeTextHasSecret(e.SafeMessage):
		return ErrValidation
	case len(e.Details) > 8192:
		return ErrValidation
	case !isValidJSON(e.Details):
		return ErrValidation
	case payloadHasSecret(e.Details):
		return ErrValidation
	}
	return nil
}

func validateOperationTaskEvent(e *OperationTaskEvent) error {
	if e == nil {
		return ErrValidation
	}
	normalizeOperationTaskEvent(e)
	switch {
	case e.TenantID <= 0:
		return ErrValidation
	case e.OperationTaskID == uuidNil:
		return ErrValidation
	case e.Sequence < 1:
		return ErrValidation
	case !allowedOperationTaskEventTypes[e.EventType]:
		return ErrValidation
	case !allowedOperationTaskEventActors[e.ActorType]:
		return ErrValidation
	case e.ActorType == OperationTaskEventActorUser && (e.ActorID == nil || *e.ActorID == uuidNil):
		return ErrValidation
	case e.BeforeState != "" && !allowedOperationTaskStatuses[e.BeforeState]:
		return ErrValidation
	case e.AfterState != "" && !allowedOperationTaskStatuses[e.AfterState]:
		return ErrValidation
	case e.PlatformDraftID != nil && *e.PlatformDraftID == uuidNil:
		return ErrValidation
	case e.PlatformDraftID != nil && e.DraftVersion < 1:
		return ErrValidation
	case len(e.Metadata) > 8192:
		return ErrValidation
	case !isValidJSON(e.Metadata):
		return ErrValidation
	case payloadHasSecret(e.Metadata):
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

func safeTextHasSecret(message string) bool {
	normalized := strings.ToLower(message)
	for _, needle := range []string{
		"authorization:", "bearer ", "access token", "refresh token", "cookie:", "password",
		"postgres://", "mysql://", "provider response secret",
	} {
		if strings.Contains(normalized, needle) {
			return true
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

var uuidNil = [16]byte{}
