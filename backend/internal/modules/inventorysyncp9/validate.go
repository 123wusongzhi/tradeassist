package inventorysyncp9

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var zeroUUID uuid.UUID

const (
	maxSafeJSONBytes    = 8192
	maxReasonCodesBytes = 4096
)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var allowedRunStatuses = map[string]bool{
	InventorySyncRunStatusPending:   true,
	InventorySyncRunStatusRunning:   true,
	InventorySyncRunStatusSucceeded: true,
	InventorySyncRunStatusFailed:    true,
	InventorySyncRunStatusCancelled: true,
}

var allowedProviderModes = map[string]bool{
	ProviderModeMock:           true,
	ProviderModeSandbox:        true,
	ProviderModeLocalDraftOnly: true,
}

var allowedBindingSources = map[string]bool{
	SKUBindingSourceAutomatic: true,
	SKUBindingSourceManual:    true,
}

var allowedBindingStatuses = map[string]bool{
	SKUBindingStatusProposed:  true,
	SKUBindingStatusConfirmed: true,
	SKUBindingStatusRejected:  true,
	SKUBindingStatusStale:     true,
	SKUBindingStatusConflict:  true,
}

var allowedBindingTransitions = map[string]map[string]bool{
	SKUBindingStatusProposed: {
		SKUBindingStatusConfirmed: true,
		SKUBindingStatusRejected:  true,
	},
	SKUBindingStatusConfirmed: {
		SKUBindingStatusStale: true,
	},
	SKUBindingStatusConflict: {
		SKUBindingStatusConfirmed: true,
	},
}

var allowedMatchStrategies = map[string]bool{
	MatchStrategyExactSKUCode:           true,
	MatchStrategyExactBarcode:           true,
	MatchStrategyNormalizedSKUCode:      true,
	MatchStrategyNormalizedBarcode:      true,
	MatchStrategyNormalizedTitleVariant: true,
	MatchStrategyCompositeMatch:         true,
	MatchStrategyManual:                 true,
}

var allowedCalibrationStatuses = map[string]bool{
	CalibrationStatusCandidate: true,
	CalibrationStatusSelected:  true,
	CalibrationStatusRejected:  true,
	CalibrationStatusConflict:  true,
}

var allowedManualBindingStatuses = map[string]bool{
	ManualBindingStatusPending:   true,
	ManualBindingStatusConfirmed: true,
	ManualBindingStatusRejected:  true,
	ManualBindingStatusCancelled: true,
}

var sensitiveJSONKeys = map[string]bool{
	"access_token":       true,
	"accessToken":        true,
	"app_secret":         true,
	"appSecret":          true,
	"authorization":      true,
	"cookie":             true,
	"cookies":            true,
	"endpointCredential": true,
	"oauth":              true,
	"password":           true,
	"refresh_token":      true,
	"refreshToken":       true,
	"secret":             true,
	"token":              true,
}

func utcNow() time.Time {
	return time.Now().UTC()
}

func normalizeString(value string) string {
	return strings.TrimSpace(value)
}

func normalizeLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeModelJSON(value datatypes.JSON, maxBytes int) (datatypes.JSON, error) {
	if len(value) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}
	if len(value) > maxBytes {
		return nil, ErrValidation
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return nil, ErrValidation
	}
	if jsonHasSensitiveKey(decoded) {
		return nil, ErrValidation
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, ErrValidation
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, encoded); err != nil {
		return nil, ErrValidation
	}
	if compact.Len() > maxBytes {
		return nil, ErrValidation
	}
	return datatypes.JSON(compact.Bytes()), nil
}

func jsonHasSensitiveKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if sensitiveJSONKeys[key] || sensitiveJSONKeys[strings.ToLower(key)] {
				return true
			}
			if jsonHasSensitiveKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if jsonHasSensitiveKey(child) {
				return true
			}
		}
	}
	return false
}

func isSHA256Hex(value string) bool {
	return sha256HexPattern.MatchString(value)
}

func validateTenantID(tenantID int64) error {
	if tenantID <= 0 {
		return ErrValidation
	}
	return nil
}

func validateHashField(value string, required bool) error {
	value = normalizeString(value)
	if value == "" {
		if required {
			return ErrValidation
		}
		return nil
	}
	if !isSHA256Hex(value) {
		return ErrValidation
	}
	return nil
}

func validateInventorySyncRun(run *InventorySyncRun) error {
	if run == nil || validateTenantID(run.TenantID) != nil || run.ShopConnectionID == zeroUUID {
		return ErrValidation
	}
	run.Platform = normalizeLower(run.Platform)
	run.ProviderMode = normalizeLower(run.ProviderMode)
	run.Status = normalizeLower(run.Status)
	run.ExternalShopReference = normalizeString(run.ExternalShopReference)
	run.RequestID = normalizeString(run.RequestID)
	run.IdempotencyKeyHash = normalizeString(run.IdempotencyKeyHash)
	run.InputFingerprint = normalizeString(run.InputFingerprint)
	if run.Platform == "" || !allowedProviderModes[run.ProviderMode] || !allowedRunStatuses[run.Status] || run.Revision < 1 {
		return ErrValidation
	}
	if run.SnapshotCount < 0 || run.CalibrationCount < 0 || run.ManualRequestCount < 0 || run.RerunSourceRevision < 0 {
		return ErrValidation
	}
	if (run.RerunOfRunID == nil) != (run.RerunSourceRevision == 0) {
		return ErrValidation
	}
	if err := validateHashField(run.IdempotencyKeyHash, false); err != nil {
		return err
	}
	if err := validateHashField(run.InputFingerprint, run.IdempotencyKeyHash != ""); err != nil {
		return err
	}
	if run.StartedAt != nil && run.FinishedAt != nil && run.FinishedAt.Before(*run.StartedAt) {
		return ErrValidation
	}
	var err error
	if run.Cursor, err = normalizeModelJSON(run.Cursor, maxSafeJSONBytes); err != nil {
		return err
	}
	if run.Checkpoint, err = normalizeModelJSON(run.Checkpoint, maxSafeJSONBytes); err != nil {
		return err
	}
	if run.SafeErrorMetadata, err = normalizeModelJSON(run.SafeErrorMetadata, maxSafeJSONBytes); err != nil {
		return err
	}
	return nil
}

func validateInventorySnapshotItem(item *InventorySnapshotItem) error {
	if item == nil || validateTenantID(item.TenantID) != nil || item.InventorySyncRunID == zeroUUID || item.ShopConnectionID == zeroUUID {
		return ErrValidation
	}
	item.Platform = normalizeLower(item.Platform)
	item.ExternalProductID = normalizeString(item.ExternalProductID)
	item.ExternalSKUID = normalizeString(item.ExternalSKUID)
	item.ExternalProductCode = normalizeString(item.ExternalProductCode)
	item.ExternalSKUCode = normalizeString(item.ExternalSKUCode)
	item.Barcode = normalizeString(item.Barcode)
	item.PayloadHash = normalizeString(item.PayloadHash)
	if item.Platform == "" || item.ExternalProductID == "" || item.ExternalSKUID == "" {
		return ErrValidation
	}
	if item.AvailableQuantity < 0 || item.ReservedQuantity < 0 || item.TotalQuantity < 0 {
		return ErrValidation
	}
	if err := validateHashField(item.PayloadHash, true); err != nil {
		return err
	}
	if item.ObservedAt.IsZero() {
		item.ObservedAt = utcNow()
	}
	var err error
	item.SafeMetadata, err = normalizeModelJSON(item.SafeMetadata, maxSafeJSONBytes)
	return err
}

func validateSKUBinding(binding *SKUBinding) error {
	if binding == nil || validateTenantID(binding.TenantID) != nil || binding.ShopConnectionID == zeroUUID || binding.LocalProductID == zeroUUID || binding.LocalSKUID == zeroUUID {
		return ErrValidation
	}
	binding.Platform = normalizeLower(binding.Platform)
	binding.ExternalProductID = normalizeString(binding.ExternalProductID)
	binding.ExternalSKUID = normalizeString(binding.ExternalSKUID)
	binding.ExternalSKUCode = normalizeString(binding.ExternalSKUCode)
	binding.BindingSource = normalizeLower(binding.BindingSource)
	binding.BindingStatus = normalizeLower(binding.BindingStatus)
	if binding.Platform == "" || binding.ExternalProductID == "" || binding.ExternalSKUID == "" {
		return ErrValidation
	}
	if !allowedBindingSources[binding.BindingSource] || !allowedBindingStatuses[binding.BindingStatus] {
		return ErrValidation
	}
	if binding.Confidence < 0 || binding.Confidence > 10000 || binding.CalibrationVersion < 1 || binding.Revision < 1 {
		return ErrValidation
	}
	if binding.BindingStatus == SKUBindingStatusConfirmed && binding.ConfirmedAt == nil {
		now := utcNow()
		binding.ConfirmedAt = &now
	}
	return nil
}

func validateSKUBindingCalibration(calibration *SKUBindingCalibration) error {
	if calibration == nil || validateTenantID(calibration.TenantID) != nil || calibration.InventorySyncRunID == zeroUUID || calibration.InventorySnapshotItemID == zeroUUID || calibration.CandidateLocalProductID == zeroUUID || calibration.CandidateLocalSKUID == zeroUUID {
		return ErrValidation
	}
	calibration.ExternalSKUID = normalizeString(calibration.ExternalSKUID)
	calibration.MatchStrategy = normalizeLower(calibration.MatchStrategy)
	calibration.Status = normalizeLower(calibration.Status)
	calibration.InputFingerprint = normalizeString(calibration.InputFingerprint)
	if calibration.ExternalSKUID == "" || !allowedMatchStrategies[calibration.MatchStrategy] || !allowedCalibrationStatuses[calibration.Status] {
		return ErrValidation
	}
	if calibration.Confidence < 0 || calibration.Confidence > 10000 || calibration.CalibrationVersion < 1 {
		return ErrValidation
	}
	if err := validateHashField(calibration.InputFingerprint, true); err != nil {
		return err
	}
	var err error
	if calibration.ScoreBreakdown, err = normalizeModelJSON(calibration.ScoreBreakdown, maxSafeJSONBytes); err != nil {
		return err
	}
	calibration.ReasonCodes, err = normalizeModelJSON(calibration.ReasonCodes, maxReasonCodesBytes)
	return err
}

func validateManualBindingRequest(request *ManualBindingRequest) error {
	if request == nil || validateTenantID(request.TenantID) != nil || request.InventorySyncRunID == zeroUUID || request.InventorySnapshotItemID == zeroUUID || request.ShopConnectionID == zeroUUID {
		return ErrValidation
	}
	request.ExternalSKUID = normalizeString(request.ExternalSKUID)
	request.Status = normalizeLower(request.Status)
	request.ReasonCode = normalizeString(request.ReasonCode)
	request.RequestID = normalizeString(request.RequestID)
	request.IdempotencyKeyHash = normalizeString(request.IdempotencyKeyHash)
	request.InputFingerprint = normalizeString(request.InputFingerprint)
	if request.ExternalSKUID == "" || request.ReasonCode == "" || request.RequestID == "" || !allowedManualBindingStatuses[request.Status] {
		return ErrValidation
	}
	if request.CandidateCount < 0 || request.Revision < 1 {
		return ErrValidation
	}
	if err := validateHashField(request.IdempotencyKeyHash, false); err != nil {
		return err
	}
	if err := validateHashField(request.InputFingerprint, true); err != nil {
		return err
	}
	if request.IdempotencyKeyHash == "" && request.InputFingerprint == "" {
		return ErrValidation
	}
	if request.Status != ManualBindingStatusPending && request.ResolvedAt == nil {
		now := utcNow()
		request.ResolvedAt = &now
	}
	return nil
}

func validateManualBindingDecision(decision *ManualBindingDecision) error {
	if decision == nil || validateTenantID(decision.TenantID) != nil || decision.ManualBindingRequestID == zeroUUID || decision.ActorID == zeroUUID {
		return ErrValidation
	}
	decision.Operation = normalizeLower(decision.Operation)
	decision.IdempotencyKeyHash = normalizeString(decision.IdempotencyKeyHash)
	decision.PayloadFingerprint = normalizeString(decision.PayloadFingerprint)
	decision.ReasonCode = normalizeString(decision.ReasonCode)
	decision.Comment = normalizeString(decision.Comment)
	if decision.Operation != ManualBindingStatusConfirmed && decision.Operation != ManualBindingStatusRejected {
		return ErrValidation
	}
	if decision.IdempotencyKeyHash == "" || decision.PayloadFingerprint == "" || decision.ReasonCode == "" || decision.RequestRevision < 1 {
		return ErrValidation
	}
	if err := validateHashField(decision.IdempotencyKeyHash, true); err != nil {
		return err
	}
	return validateHashField(decision.PayloadFingerprint, true)
}

func bindingTransitionAllowed(from string, to string) bool {
	return allowedBindingTransitions[from][to]
}

func manualBindingResolutionStatus(status string) bool {
	return status == ManualBindingStatusConfirmed || status == ManualBindingStatusRejected || status == ManualBindingStatusCancelled
}
