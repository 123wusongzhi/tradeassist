package operationtask

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	DraftFixtureScenarioSuccess            = "success"
	DraftFixtureScenarioValidationRejected = "validation_rejected"
	DraftFixtureScenarioAdapterUnavailable = "adapter_unavailable"
	DraftFixtureScenarioProviderTimeout    = "provider_timeout"
	DraftFixtureScenarioProviderRejected   = "provider_rejected"
	DraftFixtureScenarioContextCancelled   = "context_cancelled"
)

type DraftAdapterCapabilities struct {
	DraftCreation      bool
	Publish            bool
	Listing            bool
	NetworkAccess      bool
	RealCredentials    bool
	AutomaticExecution bool
}

func SafeDraftCreationCapabilities() DraftAdapterCapabilities {
	return DraftAdapterCapabilities{DraftCreation: true}
}

type PlatformDraftAdapterRegistry struct {
	mu       sync.RWMutex
	entries  map[draftAdapterKey]registeredDraftAdapter
	fallback DraftExecutionPort
}

type draftAdapterKey struct {
	platform string
	mode     string
}

type registeredDraftAdapter struct {
	port         DraftExecutionPort
	capabilities DraftAdapterCapabilities
}

func NewSafePlatformDraftAdapterRegistry() *PlatformDraftAdapterRegistry {
	local := NewLocalDraftAdapter()
	registry := &PlatformDraftAdapterRegistry{
		entries:  make(map[draftAdapterKey]registeredDraftAdapter),
		fallback: local,
	}
	_ = registry.Register(PlatformLocal, ExecutionPortModeLocalDraftFixture, local, SafeDraftCreationCapabilities())
	_ = registry.Register(PlatformDouyin, ExecutionPortModeLocalDraftFixture, local, SafeDraftCreationCapabilities())
	_ = registry.Register(PlatformDouyin, ExecutionPortModeMock, NewDouyinDraftFixtureAdapter(ExecutionPortModeMock, DraftFixtureScenarioSuccess), SafeDraftCreationCapabilities())
	_ = registry.Register(PlatformDouyin, ExecutionPortModeSandboxFixture, NewDouyinDraftFixtureAdapter(ExecutionPortModeSandboxFixture, DraftFixtureScenarioSuccess), SafeDraftCreationCapabilities())
	return registry
}

func (r *PlatformDraftAdapterRegistry) Register(platform string, mode string, port DraftExecutionPort, capabilities DraftAdapterCapabilities) error {
	if r == nil {
		return adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "adapter_registry_unavailable", "Adapter registry is unavailable", false)
	}
	platform = normalizeDraftAdapterValue(platform)
	mode = normalizeDraftAdapterValue(mode)
	if port == nil {
		return adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "adapter_unavailable", "Draft adapter is unavailable", true)
	}
	if forbiddenExecutionMode(mode) || !allowedExecutionPortMode(mode) {
		return adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_adapter_mode", "Adapter mode is unsupported", false)
	}
	if err := validateDraftAdapterCapabilities(capabilities); err != nil {
		return err
	}
	if platform == "" || !allowedPlatforms[platform] {
		return adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "unsupported_platform", "Platform is unsupported", false)
	}
	key := draftAdapterKey{platform: platform, mode: mode}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make(map[draftAdapterKey]registeredDraftAdapter)
	}
	if _, exists := r.entries[key]; exists {
		return adapterDomainError(ExecutionErrorCategoryStateConflict, "duplicate_adapter_registration", "Draft adapter registration already exists", false)
	}
	r.entries[key] = registeredDraftAdapter{port: port, capabilities: capabilities}
	return nil
}

func (r *PlatformDraftAdapterRegistry) ExecuteDraft(ctx context.Context, input DraftExecutionInput) (DraftExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return DraftExecutionResult{}, contextExecutionError(err)
	}
	platform := normalizeDraftAdapterValue(input.Platform)
	mode := normalizeDraftAdapterValue(input.AdapterMode)
	input.Platform = platform
	input.AdapterMode = mode
	if err := UnsupportedPlatformGuard(input); err != nil {
		return DraftExecutionResult{}, err
	}
	if err := AutomaticPublishGuard(input); err != nil {
		return DraftExecutionResult{}, err
	}
	if err := CredentialAbsenceGuard(input); err != nil {
		return DraftExecutionResult{}, err
	}
	entry, fallback, err := r.resolve(platform, mode)
	if err != nil {
		return DraftExecutionResult{}, err
	}
	if err := validateDraftAdapterCapabilities(entry.capabilities); err != nil {
		return DraftExecutionResult{}, err
	}
	result, err := entry.port.ExecuteDraft(ctx, input)
	if err != nil || !fallback {
		return result, err
	}
	metadata, metaErr := safeMetadata(map[string]any{
		"adapterMode":    AdapterModeLocalDraftOnly,
		"adapterVersion": "p8-batch-5-local",
		"fallbackReason": "local_draft_only_fallback",
		"payloadHash":    input.DraftPayloadHash,
		"referenceKind":  "local",
	})
	if metaErr != nil {
		return DraftExecutionResult{}, metaErr
	}
	result.SafeMetadata = metadata
	return result, nil
}

func (r *PlatformDraftAdapterRegistry) Capabilities(platform, mode string) (DraftAdapterCapabilities, bool) {
	entry, _, err := r.resolve(normalizeDraftAdapterValue(platform), normalizeDraftAdapterValue(mode))
	if err != nil {
		return DraftAdapterCapabilities{}, false
	}
	return entry.capabilities, true
}

func (r *PlatformDraftAdapterRegistry) resolve(platform, mode string) (registeredDraftAdapter, bool, error) {
	if r == nil {
		return registeredDraftAdapter{}, false, adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "adapter_registry_unavailable", "Adapter registry is unavailable", false)
	}
	r.mu.RLock()
	entry, ok := r.entries[draftAdapterKey{platform: platform, mode: mode}]
	fallback := r.fallback
	r.mu.RUnlock()
	if ok {
		return entry, false, nil
	}
	if mode == ExecutionPortModeLocalDraftFixture && allowedPlatforms[platform] && fallback != nil {
		return registeredDraftAdapter{port: fallback, capabilities: SafeDraftCreationCapabilities()}, true, nil
	}
	if !allowedPlatforms[platform] {
		return registeredDraftAdapter{}, false, adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "unsupported_platform", "Platform is unsupported", false)
	}
	return registeredDraftAdapter{}, false, adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_adapter_mode", "Adapter mode is unsupported", false)
}

func validateDraftAdapterCapabilities(capabilities DraftAdapterCapabilities) error {
	if !capabilities.DraftCreation {
		return adapterDomainError(ExecutionErrorCategoryValidation, "draft_creation_required", "Draft creation capability is required", false)
	}
	if capabilities.Publish || capabilities.Listing || capabilities.NetworkAccess || capabilities.RealCredentials || capabilities.AutomaticExecution {
		return adapterDomainError(ExecutionErrorCategoryPermissionDenied, "production_capability_forbidden", "Production platform capability is forbidden", false)
	}
	return nil
}

func UnsupportedPlatformGuard(input DraftExecutionInput) error {
	platform := normalizeDraftAdapterValue(input.Platform)
	mode := normalizeDraftAdapterValue(input.AdapterMode)
	if platform == "" || !allowedPlatforms[platform] {
		return adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "unsupported_platform", "Platform is unsupported", false)
	}
	if forbiddenExecutionMode(mode) || !allowedExecutionPortMode(mode) {
		return adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_adapter_mode", "Adapter mode is unsupported", false)
	}
	if platform == PlatformDouyin {
		return nil
	}
	if mode != ExecutionPortModeLocalDraftFixture {
		return adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_adapter_mode", "Adapter mode is unsupported", false)
	}
	return nil
}

func AutomaticPublishGuard(input DraftExecutionInput) error {
	if dangerousRuntimeConfigEnabled() || draftPayloadContainsBlockedCapability(input.Payload) {
		return adapterDomainError(ExecutionErrorCategoryPermissionDenied, "production_capability_forbidden", "Production platform capability is forbidden", false)
	}
	return nil
}

func CredentialAbsenceGuard(input DraftExecutionInput) error {
	if payloadHasSecret(input.Payload) {
		return adapterDomainError(ExecutionErrorCategoryPermissionDenied, "real_credentials_forbidden", "Real credentials are forbidden", false)
	}
	return nil
}

type LocalDraftAdapter struct {
	Now func() time.Time

	mu     sync.Mutex
	ledger map[string]string
}

func NewLocalDraftAdapter() *LocalDraftAdapter {
	return &LocalDraftAdapter{ledger: make(map[string]string)}
}

func (a *LocalDraftAdapter) ExecuteDraft(ctx context.Context, input DraftExecutionInput) (DraftExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return DraftExecutionResult{}, contextExecutionError(err)
	}
	if normalizeDraftAdapterValue(input.AdapterMode) != ExecutionPortModeLocalDraftFixture {
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_adapter_mode", "Adapter mode is unsupported", false)
	}
	if err := validatePlatformDraftExecutionInput(input); err != nil {
		return DraftExecutionResult{}, err
	}
	if err := a.recordIdempotency(input); err != nil {
		return DraftExecutionResult{}, err
	}
	reference := fmt.Sprintf("local:%d:%s:%d:%s", input.TenantID, input.OperationTaskID.String(), input.DraftVersion, hashPrefix(input.DraftPayloadHash))
	metadata, err := safeMetadata(map[string]any{
		"adapterMode":    AdapterModeLocalDraftOnly,
		"adapterVersion": "p8-batch-5-local",
		"payloadHash":    input.DraftPayloadHash,
		"referenceKind":  "local",
	})
	if err != nil {
		return DraftExecutionResult{}, err
	}
	return DraftExecutionResult{ResultType: "local_draft", ExternalReference: reference, SafeMetadata: metadata, CompletedAt: adapterNow(a.Now)}, nil
}

func (a *LocalDraftAdapter) recordIdempotency(input DraftExecutionInput) error {
	if a == nil {
		return adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "adapter_unavailable", "Draft adapter is unavailable", true)
	}
	key := draftAdapterIdempotencyKey(input)
	if key == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ledger == nil {
		a.ledger = make(map[string]string)
	}
	if existing, ok := a.ledger[key]; ok && existing != input.DraftPayloadHash {
		return adapterDomainError(ExecutionErrorCategoryIdempotencyConflict, "idempotency_payload_conflict", "Idempotency payload conflict", false)
	}
	a.ledger[key] = input.DraftPayloadHash
	return nil
}

type DouyinDraftFixtureAdapter struct {
	Mode     string
	Scenario string
	Now      func() time.Time

	mu     sync.Mutex
	ledger map[string]string
}

func NewDouyinDraftFixtureAdapter(mode string, scenario string) *DouyinDraftFixtureAdapter {
	mode = normalizeDraftAdapterValue(mode)
	if mode == "" {
		mode = ExecutionPortModeSandboxFixture
	}
	scenario = normalizeDraftAdapterValue(scenario)
	if scenario == "" {
		scenario = DraftFixtureScenarioSuccess
	}
	return &DouyinDraftFixtureAdapter{Mode: mode, Scenario: scenario, ledger: make(map[string]string)}
}

func (a *DouyinDraftFixtureAdapter) ExecuteDraft(ctx context.Context, input DraftExecutionInput) (DraftExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return DraftExecutionResult{}, contextExecutionError(err)
	}
	if a == nil {
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "adapter_unavailable", "Draft adapter is unavailable", true)
	}
	input.Platform = normalizeDraftAdapterValue(input.Platform)
	input.AdapterMode = normalizeDraftAdapterValue(input.AdapterMode)
	if input.Platform != PlatformDouyin {
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "unsupported_platform", "Platform is unsupported", false)
	}
	if input.AdapterMode != a.Mode || (a.Mode != ExecutionPortModeMock && a.Mode != ExecutionPortModeSandboxFixture) {
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_adapter_mode", "Adapter mode is unsupported", false)
	}
	if err := validatePlatformDraftExecutionInput(input); err != nil {
		return DraftExecutionResult{}, err
	}
	if err := validateDouyinDraftPayload(input.Payload); err != nil {
		return DraftExecutionResult{}, err
	}
	if err := a.recordIdempotency(input); err != nil {
		return DraftExecutionResult{}, err
	}
	switch normalizeDraftAdapterValue(a.Scenario) {
	case DraftFixtureScenarioSuccess:
	case DraftFixtureScenarioValidationRejected:
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryValidation, "validation_rejected", "Draft validation was rejected", false)
	case DraftFixtureScenarioAdapterUnavailable:
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryAdapterUnavailable, "adapter_unavailable", "Draft adapter is unavailable", true)
	case DraftFixtureScenarioProviderTimeout:
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryProviderTimeout, "provider_timeout", "Fixture provider timed out", true)
	case DraftFixtureScenarioProviderRejected:
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryProviderRejected, "provider_rejected", "Fixture provider rejected the draft", false)
	case DraftFixtureScenarioContextCancelled:
		return DraftExecutionResult{}, contextExecutionError(context.Canceled)
	default:
		return DraftExecutionResult{}, adapterDomainError(ExecutionErrorCategoryValidation, "unsupported_fixture_scenario", "Fixture scenario is unsupported", false)
	}
	resultType := "sandbox_fixture"
	prefix := "sandbox:douyin"
	if a.Mode == ExecutionPortModeMock {
		resultType = "mock_draft"
		prefix = "mock:douyin"
	}
	reference := fmt.Sprintf("%s:%d:%s:%d:%s", prefix, input.TenantID, input.OperationTaskID.String(), input.DraftVersion, hashPrefix(input.DraftPayloadHash))
	metadata, err := safeMetadata(map[string]any{
		"adapterMode":         a.Mode,
		"adapterVersion":      "p8-batch-5-douyin-fixture",
		"fixtureScenario":     a.Scenario,
		"payloadHash":         input.DraftPayloadHash,
		"referenceKind":       resultType,
		"validatedFieldCount": 6,
	})
	if err != nil {
		return DraftExecutionResult{}, err
	}
	return DraftExecutionResult{ResultType: resultType, ExternalReference: reference, SafeMetadata: metadata, CompletedAt: adapterNow(a.Now)}, nil
}

func (a *DouyinDraftFixtureAdapter) recordIdempotency(input DraftExecutionInput) error {
	key := draftAdapterIdempotencyKey(input)
	if key == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ledger == nil {
		a.ledger = make(map[string]string)
	}
	if existing, ok := a.ledger[key]; ok && existing != input.DraftPayloadHash {
		return adapterDomainError(ExecutionErrorCategoryIdempotencyConflict, "idempotency_payload_conflict", "Idempotency payload conflict", false)
	}
	a.ledger[key] = input.DraftPayloadHash
	return nil
}

func validatePlatformDraftExecutionInput(input DraftExecutionInput) error {
	if input.TenantID <= 0 || input.OperationTaskID == uuid.Nil || input.PlatformDraftID == uuid.Nil || input.ActorID == uuid.Nil || input.DraftVersion < 1 || strings.TrimSpace(input.RequestID) == "" {
		return adapterDomainError(ExecutionErrorCategoryValidation, ErrCodeValidation, "Draft execution input is invalid", false)
	}
	if !sha256LowerHex.MatchString(strings.TrimSpace(input.DraftPayloadHash)) {
		return adapterDomainError(ExecutionErrorCategoryValidation, ErrCodeValidation, "Draft payload hash is invalid", false)
	}
	if len(input.Payload) == 0 || !json.Valid(input.Payload) {
		return adapterDomainError(ExecutionErrorCategoryValidation, ErrCodeValidation, "Draft payload JSON is invalid", false)
	}
	if payloadHasSecret(input.Payload) {
		return adapterDomainError(ExecutionErrorCategoryPermissionDenied, "real_credentials_forbidden", "Real credentials are forbidden", false)
	}
	return nil
}

func validateDouyinDraftPayload(payload datatypes.JSON) error {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return adapterDomainError(ExecutionErrorCategoryValidation, ErrCodeValidation, "Draft payload JSON is invalid", false)
	}
	if nested, ok := raw["draft"].(map[string]any); ok {
		raw = nested
	}
	if strings.TrimSpace(stringField(raw, "title")) == "" ||
		strings.TrimSpace(stringField(raw, "description")) == "" ||
		strings.TrimSpace(stringField(raw, "category")) == "" ||
		!positiveNumber(raw["price"]) ||
		!nonNegativeNumber(raw["inventory"]) ||
		!nonEmptyArray(raw["media"]) {
		return adapterDomainError(ExecutionErrorCategoryValidation, ErrCodeValidation, "Draft payload validation failed", false)
	}
	return nil
}

func dangerousRuntimeConfigEnabled() bool {
	for _, key := range []string{"AUTO_PUBLISH", "AUTO_LISTING", "REAL_PLATFORM_WRITE", "DOUYIN_REAL_WRITE", "PRODUCTION_ADAPTER"} {
		if truthy(os.Getenv(key)) {
			return true
		}
	}
	return false
}

func draftPayloadContainsBlockedCapability(payload datatypes.JSON) bool {
	if len(payload) == 0 || !json.Valid(payload) {
		return false
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return false
	}
	return containsBlockedCapability(value)
}

func containsBlockedCapability(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			key = strings.TrimSpace(strings.ToLower(key))
			switch key {
			case "publish", "listing", "go_live", "auto_publish", "auto_listing", "direct_write", "production_write":
				if truthy(child) {
					return true
				}
			}
			if containsBlockedCapability(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsBlockedCapability(child) {
				return true
			}
		}
	}
	return false
}

func stringField(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func positiveNumber(value any) bool {
	switch typed := value.(type) {
	case float64:
		return typed > 0
	case int:
		return typed > 0
	case json.Number:
		parsed, err := typed.Float64()
		return err == nil && parsed > 0
	default:
		return false
	}
}

func nonNegativeNumber(value any) bool {
	switch typed := value.(type) {
	case float64:
		return typed >= 0
	case int:
		return typed >= 0
	case json.Number:
		parsed, err := typed.Float64()
		return err == nil && parsed >= 0
	default:
		return false
	}
}

func nonEmptyArray(value any) bool {
	items, ok := value.([]any)
	return ok && len(items) > 0
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.TrimSpace(strings.ToLower(typed)) {
		case "true", "1", "yes", "enabled", "allow", "allowed":
			return true
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return false
}

func safeMetadata(value map[string]any) (datatypes.JSON, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, adapterDomainError(ExecutionErrorCategoryInternal, "safe_metadata_invalid", "Safe metadata is invalid", false)
	}
	metadata := redactSafeJSON(datatypes.JSON(data))
	if payloadHasSecret(metadata) {
		return nil, adapterDomainError(ExecutionErrorCategoryPermissionDenied, "real_credentials_forbidden", "Real credentials are forbidden", false)
	}
	return metadata, nil
}

func draftAdapterIdempotencyKey(input DraftExecutionInput) string {
	key := strings.TrimSpace(input.IdempotencyKey)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s:%d:%s", input.TenantID, input.OperationTaskID.String(), input.DraftVersion, key)
}

func hashPrefix(hash string) string {
	hash = strings.TrimSpace(strings.ToLower(hash))
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func adapterNow(now func() time.Time) time.Time {
	if now != nil {
		return now().UTC()
	}
	return utcNow()
}

func normalizeDraftAdapterValue(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func adapterDomainError(category, code, message string, retryable bool) *ExecutionDomainError {
	return &ExecutionDomainError{
		Category:    category,
		Code:        code,
		SafeMessage: message,
		Retryable:   retryable,
		Details:     datatypes.JSON([]byte(`{}`)),
	}
}

func contextExecutionError(err error) *ExecutionDomainError {
	if err == context.Canceled {
		return adapterDomainError(ExecutionErrorCategoryProviderTimeout, "context_cancelled", "Context was cancelled", false)
	}
	return adapterDomainError(ExecutionErrorCategoryProviderTimeout, "provider_timeout", "Context deadline exceeded", true)
}
