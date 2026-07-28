package inventorysyncp9

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/datatypes"
)

const (
	FixtureScenarioSuccessSinglePage    = "success_single_page"
	FixtureScenarioSuccessMultiPage     = "success_multi_page"
	FixtureScenarioEmptyInventory       = "empty_inventory"
	FixtureScenarioLowConfidenceBinding = "low_confidence_binding"
	FixtureScenarioBindingConflict      = "binding_conflict"
	FixtureScenarioUnmatchedSKU         = "unmatched_sku"
	FixtureScenarioProviderTimeout      = "provider_timeout"
	FixtureScenarioProviderRejected     = "provider_rejected"
	FixtureScenarioMalformedItem        = "malformed_item"
	FixtureScenarioDuplicateExternalSKU = "duplicate_external_sku"
	FixtureScenarioCursorLoop           = "cursor_loop"
	FixtureScenarioCancelledContext     = "cancelled_context"
)

const (
	DefaultInventoryFixtureVersion = "p9-inventory-fixture-v1"
	DefaultInventoryPageSize       = 50
	DefaultMaxPagesPerRun          = 25
	DefaultMaxItemsPerPage         = 100
	DefaultMaxItemsPerRun          = 1000
)

type InventoryProviderKey struct {
	Platform     string `json:"platform"`
	ProviderMode string `json:"providerMode"`
}

func (k InventoryProviderKey) normalized() InventoryProviderKey {
	return InventoryProviderKey{Platform: normalizeLower(k.Platform), ProviderMode: normalizeLower(k.ProviderMode)}
}

func (k InventoryProviderKey) String() string {
	k = k.normalized()
	return k.Platform + "/" + k.ProviderMode
}

type InventoryProviderCapabilities struct {
	FixtureBacked            bool `json:"fixtureBacked"`
	FixtureRead              bool `json:"fixtureRead"`
	MockBacked               bool `json:"mockBacked"`
	NetworkAccess            bool `json:"networkAccess"`
	OAuth                    bool `json:"oauth"`
	Credentials              bool `json:"credentials"`
	RealCredentials          bool `json:"realCredentials"`
	RealPlatformRead         bool `json:"realPlatformRead"`
	RealPlatformWrite        bool `json:"realPlatformWrite"`
	RealInventoryRead        bool `json:"realInventoryRead"`
	RealInventoryWrite       bool `json:"realInventoryWrite"`
	InventoryMutation        bool `json:"inventoryMutation"`
	AutomaticExecution       bool `json:"automaticExecution"`
	AutomaticRetry           bool `json:"automaticRetry"`
	BackgroundWorker         bool `json:"backgroundWorker"`
	AcceptsCredentialPayload bool `json:"acceptsCredentialPayload"`
}

func (c InventoryProviderCapabilities) unsafe() bool {
	return c.NetworkAccess || c.OAuth || c.Credentials || c.RealCredentials || c.RealPlatformRead || c.RealPlatformWrite || c.RealInventoryRead || c.RealInventoryWrite || c.InventoryMutation || c.AutomaticExecution || c.AutomaticRetry || c.BackgroundWorker || c.AcceptsCredentialPayload
}

type InventoryProvider interface {
	Key() InventoryProviderKey
	Capabilities() InventoryProviderCapabilities
	FetchInventoryPage(ctx context.Context, request InventoryFetchRequest) (InventoryFetchPageResult, error)
}

type InventoryFetchRequest struct {
	TenantID         int64
	ShopConnectionID string
	Platform         string
	ProviderMode     string
	FixtureScenario  string
	Cursor           datatypes.JSON
	PageSize         int
	MaxItemsPerPage  int
}

type InventoryProviderItem struct {
	ExternalProductID   string            `json:"externalProductId"`
	ExternalSKUID       string            `json:"externalSkuId"`
	ExternalProductCode string            `json:"externalProductCode,omitempty"`
	ExternalSKUCode     string            `json:"externalSkuCode,omitempty"`
	Barcode             string            `json:"barcode,omitempty"`
	ProductTitle        string            `json:"productTitle,omitempty"`
	VariantTitle        string            `json:"variantTitle,omitempty"`
	AvailableQuantity   int               `json:"availableQuantity"`
	ReservedQuantity    int               `json:"reservedQuantity"`
	TotalQuantity       int               `json:"totalQuantity"`
	SourceUpdatedAt     *time.Time        `json:"sourceUpdatedAt,omitempty"`
	SafeMetadata        map[string]string `json:"safeMetadata,omitempty"`
}

type InventoryFetchPageResult struct {
	Items        []InventoryProviderItem `json:"items"`
	Cursor       datatypes.JSON          `json:"cursor"`
	NextCursor   datatypes.JSON          `json:"nextCursor,omitempty"`
	HasMore      bool                    `json:"hasMore"`
	Scenario     string                  `json:"scenario"`
	FixtureHash  string                  `json:"fixtureHash"`
	NetworkCalls int                     `json:"networkCalls"`
}

type inventoryFixtureCursor struct {
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	ProviderMode     string `json:"providerMode"`
	Scenario         string `json:"scenario"`
	FixtureVersion   string `json:"fixtureVersion"`
	FixtureHash      string `json:"fixtureHash"`
	PageIndex        int    `json:"pageIndex"`
	PageSize         int    `json:"pageSize"`
	InputFingerprint string `json:"inputFingerprint"`
}

type InventoryProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]InventoryProvider
}

func NewInventoryProviderRegistry(providers ...InventoryProvider) (*InventoryProviderRegistry, error) {
	registry := &InventoryProviderRegistry{providers: map[string]InventoryProvider{}}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func NewDefaultInventoryProviderRegistry() (*InventoryProviderRegistry, error) {
	return NewInventoryProviderRegistry(
		NewDouyinInventoryMockProvider(),
		NewDouyinInventoryFixtureProvider(FixtureScenarioSuccessSinglePage),
		NewLocalInventoryFixtureProvider(FixtureScenarioSuccessSinglePage),
	)
}

func (r *InventoryProviderRegistry) Register(provider InventoryProvider) error {
	if provider == nil {
		return ErrProviderNotRegistered
	}
	key := provider.Key().normalized()
	if inventoryProviderKeyProductionForbidden(key) {
		return ErrProductionCapabilityForbidden
	}
	if !inventoryProviderKeyAllowed(key) {
		return ErrProviderNotRegistered
	}
	if provider.Capabilities().unsafe() {
		return ErrProviderCapabilityForbidden
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[key.String()] = provider
	return nil
}

func (r *InventoryProviderRegistry) Resolve(platform, providerMode string) (InventoryProvider, error) {
	if r == nil {
		return nil, ErrProviderNotRegistered
	}
	key := InventoryProviderKey{Platform: platform, ProviderMode: providerMode}.normalized()
	if inventoryProviderKeyProductionForbidden(key) {
		return nil, ErrProductionCapabilityForbidden
	}
	if !inventoryProviderKeyAllowed(key) {
		return nil, ErrProviderNotRegistered
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[key.String()]
	if !ok {
		return nil, ErrProviderNotRegistered
	}
	if provider.Capabilities().unsafe() {
		return nil, ErrProviderCapabilityForbidden
	}
	return provider, nil
}

func inventoryProviderKeyAllowed(key InventoryProviderKey) bool {
	if key.Platform != PlatformDouyin {
		return false
	}
	return key.ProviderMode == ProviderModeMock || key.ProviderMode == ProviderModeSandbox || key.ProviderMode == ProviderModeLocalDraftOnly
}

func inventoryProviderKeyProductionForbidden(key InventoryProviderKey) bool {
	mode := normalizeLower(key.ProviderMode)
	switch mode {
	case "production", "prod", "real", "live", "online", "remote", "oauth":
		return true
	default:
		return false
	}
}

type InventoryFixtureProvider struct {
	key             InventoryProviderKey
	capabilities    InventoryProviderCapabilities
	defaultScenario string
	fixtureVersion  string
	networkCalls    int
}

func NewDouyinInventoryMockProvider() *InventoryFixtureProvider {
	return &InventoryFixtureProvider{
		key:             InventoryProviderKey{Platform: PlatformDouyin, ProviderMode: ProviderModeMock},
		capabilities:    InventoryProviderCapabilities{MockBacked: true, FixtureRead: true},
		defaultScenario: FixtureScenarioSuccessSinglePage,
		fixtureVersion:  DefaultInventoryFixtureVersion,
	}
}

func NewDouyinInventoryFixtureProvider(defaultScenario string) *InventoryFixtureProvider {
	return &InventoryFixtureProvider{
		key:             InventoryProviderKey{Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox},
		capabilities:    InventoryProviderCapabilities{FixtureBacked: true, FixtureRead: true},
		defaultScenario: normalizeFixtureScenario(defaultScenario),
		fixtureVersion:  DefaultInventoryFixtureVersion,
	}
}

func NewLocalInventoryFixtureProvider(defaultScenario string) *InventoryFixtureProvider {
	return &InventoryFixtureProvider{
		key:             InventoryProviderKey{Platform: PlatformDouyin, ProviderMode: ProviderModeLocalDraftOnly},
		capabilities:    InventoryProviderCapabilities{FixtureBacked: true, FixtureRead: true},
		defaultScenario: normalizeFixtureScenario(defaultScenario),
		fixtureVersion:  DefaultInventoryFixtureVersion,
	}
}

func (p *InventoryFixtureProvider) Key() InventoryProviderKey { return p.key }

func (p *InventoryFixtureProvider) Capabilities() InventoryProviderCapabilities {
	return p.capabilities
}

func (p *InventoryFixtureProvider) FetchInventoryPage(ctx context.Context, request InventoryFetchRequest) (InventoryFetchPageResult, error) {
	if err := ctx.Err(); err != nil {
		return InventoryFetchPageResult{}, ErrSyncCancelled
	}
	if p == nil {
		return InventoryFetchPageResult{}, ErrProviderNotRegistered
	}
	key := p.key.normalized()
	if request.Platform != "" || request.ProviderMode != "" {
		reqKey := InventoryProviderKey{Platform: request.Platform, ProviderMode: request.ProviderMode}.normalized()
		if reqKey != key {
			return InventoryFetchPageResult{}, ErrProviderNotRegistered
		}
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = DefaultInventoryPageSize
	}
	maxItems := request.MaxItemsPerPage
	if maxItems <= 0 {
		maxItems = DefaultMaxItemsPerPage
	}
	if pageSize <= 0 || pageSize > maxItems {
		return InventoryFetchPageResult{}, ErrProviderPageInvalid
	}
	scenario := normalizeFixtureScenario(request.FixtureScenario)
	if scenario == "" {
		scenario = normalizeFixtureScenario(p.defaultScenario)
	}
	if scenario == FixtureScenarioCancelledContext {
		return InventoryFetchPageResult{}, ErrSyncCancelled
	}
	pages, err := fixturePagesForScenario(scenario)
	if err != nil {
		return InventoryFetchPageResult{}, err
	}
	fixtureHash := fixtureScenarioHash(key, p.fixtureVersion, scenario, pages)
	cursor, err := decodeInventoryFixtureCursor(request.Cursor)
	if err != nil {
		return InventoryFetchPageResult{}, err
	}
	if cursor.Version == "" {
		cursor = inventoryFixtureCursor{Version: "inventory-fixture-cursor-v1", Platform: key.Platform, ProviderMode: key.ProviderMode, Scenario: scenario, FixtureVersion: p.fixtureVersion, FixtureHash: fixtureHash, PageIndex: 0, PageSize: pageSize, InputFingerprint: fixtureHash}
	} else if cursor.Platform != key.Platform || cursor.ProviderMode != key.ProviderMode || cursor.Scenario != scenario || cursor.FixtureVersion != p.fixtureVersion || cursor.FixtureHash != fixtureHash || cursor.PageSize != pageSize {
		return InventoryFetchPageResult{}, ErrProviderCursorInvalid
	}
	if scenario == FixtureScenarioProviderTimeout {
		return InventoryFetchPageResult{}, ErrProviderTimeout
	}
	if scenario == FixtureScenarioProviderRejected {
		return InventoryFetchPageResult{}, ErrProviderRejected
	}
	if cursor.PageIndex < 0 || cursor.PageIndex > len(pages) {
		return InventoryFetchPageResult{}, ErrProviderCursorInvalid
	}
	items := []InventoryProviderItem{}
	if cursor.PageIndex < len(pages) {
		items = append(items, pages[cursor.PageIndex]...)
	}
	for _, item := range items {
		if err := validateInventoryProviderItem(item); err != nil {
			return InventoryFetchPageResult{}, err
		}
	}
	next := cursor
	next.PageIndex++
	hasMore := next.PageIndex < len(pages)
	if scenario == FixtureScenarioCursorLoop {
		next = cursor
		hasMore = true
	}
	nextCursor, err := encodeInventoryFixtureCursor(next)
	if err != nil {
		return InventoryFetchPageResult{}, err
	}
	currentCursor, err := encodeInventoryFixtureCursor(cursor)
	if err != nil {
		return InventoryFetchPageResult{}, err
	}
	return InventoryFetchPageResult{Items: items, Cursor: currentCursor, NextCursor: nextCursor, HasMore: hasMore, Scenario: scenario, FixtureHash: fixtureHash, NetworkCalls: p.networkCalls}, nil
}

func normalizeFixtureScenario(scenario string) string {
	scenario = normalizeString(scenario)
	if scenario == "" {
		return FixtureScenarioSuccessSinglePage
	}
	return scenario
}

func decodeInventoryFixtureCursor(raw datatypes.JSON) (inventoryFixtureCursor, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return inventoryFixtureCursor{}, nil
	}
	var cursor inventoryFixtureCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return inventoryFixtureCursor{}, ErrProviderCursorInvalid
	}
	if cursor.Version != "inventory-fixture-cursor-v1" || cursor.PageIndex < 0 || cursor.Platform == "" || cursor.ProviderMode == "" || cursor.Scenario == "" || cursor.FixtureHash == "" || cursor.PageSize <= 0 {
		return inventoryFixtureCursor{}, ErrProviderCursorInvalid
	}
	return cursor, nil
}

func encodeInventoryFixtureCursor(cursor inventoryFixtureCursor) (datatypes.JSON, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return nil, ErrProviderCursorInvalid
	}
	return normalizeModelJSON(datatypes.JSON(encoded), maxSafeJSONBytes)
}

func fixtureScenarioHash(key InventoryProviderKey, version string, scenario string, pages [][]InventoryProviderItem) string {
	type fixtureHashInput struct {
		Key      InventoryProviderKey      `json:"key"`
		Version  string                    `json:"version"`
		Scenario string                    `json:"scenario"`
		Pages    [][]InventoryProviderItem `json:"pages"`
	}
	payload, _ := json.Marshal(fixtureHashInput{Key: key, Version: version, Scenario: scenario, Pages: pages})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func validateInventoryProviderItem(item InventoryProviderItem) error {
	if normalizeString(item.ExternalProductID) == "" || normalizeString(item.ExternalSKUID) == "" {
		return ErrProviderPageInvalid
	}
	if item.AvailableQuantity < 0 || item.ReservedQuantity < 0 || item.TotalQuantity < 0 {
		return ErrProviderPageInvalid
	}
	if len(item.ExternalProductID) > 255 || len(item.ExternalSKUID) > 255 || len(item.ExternalProductCode) > 255 || len(item.ExternalSKUCode) > 255 || len(item.Barcode) > 255 || len(item.ProductTitle) > 512 || len(item.VariantTitle) > 512 {
		return ErrProviderPageInvalid
	}
	if len(item.SafeMetadata) > 0 {
		encoded, err := json.Marshal(item.SafeMetadata)
		if err != nil {
			return ErrProviderPageInvalid
		}
		if _, err := normalizeModelJSON(datatypes.JSON(encoded), maxSafeJSONBytes); err != nil {
			return ErrProviderPageInvalid
		}
	}
	return nil
}

func fixturePagesForScenario(scenario string) ([][]InventoryProviderItem, error) {
	base := func(suffix string, code string, barcode string) InventoryProviderItem {
		updated := time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC)
		return InventoryProviderItem{ExternalProductID: "douyin-product-" + suffix, ExternalSKUID: "douyin-sku-" + suffix, ExternalProductCode: "DP-" + suffix, ExternalSKUCode: code, Barcode: barcode, ProductTitle: "Fixture Product " + suffix, VariantTitle: "Fixture Variant " + suffix, AvailableQuantity: 10, ReservedQuantity: 1, TotalQuantity: 11, SourceUpdatedAt: &updated, SafeMetadata: map[string]string{"fixtureScenario": scenario}}
	}
	switch normalizeFixtureScenario(scenario) {
	case FixtureScenarioSuccessSinglePage:
		return [][]InventoryProviderItem{{base("001", "SKU-701", "B701")}}, nil
	case FixtureScenarioSuccessMultiPage:
		return [][]InventoryProviderItem{{base("001", "SKU-701", "B701")}, {base("002", "SKU-702", "B702")}}, nil
	case FixtureScenarioEmptyInventory:
		return [][]InventoryProviderItem{{}}, nil
	case FixtureScenarioLowConfidenceBinding:
		return [][]InventoryProviderItem{{base("low", "sku low", "")}}, nil
	case FixtureScenarioBindingConflict:
		return [][]InventoryProviderItem{{base("conflict", "SKU-CONFLICT", "")}}, nil
	case FixtureScenarioUnmatchedSKU:
		return [][]InventoryProviderItem{{base("unmatched", "NO-MATCH", "")}}, nil
	case FixtureScenarioMalformedItem:
		item := base("malformed", "SKU-MALFORMED", "")
		item.ExternalSKUID = ""
		return [][]InventoryProviderItem{{item}}, nil
	case FixtureScenarioDuplicateExternalSKU:
		item := base("duplicate", "SKU-DUP", "")
		return [][]InventoryProviderItem{{item, item}}, nil
	case FixtureScenarioCursorLoop:
		return [][]InventoryProviderItem{{base("loop", "SKU-LOOP", "")}}, nil
	case FixtureScenarioProviderTimeout, FixtureScenarioProviderRejected, FixtureScenarioCancelledContext:
		return [][]InventoryProviderItem{{base("error", "SKU-ERROR", "")}}, nil
	default:
		return nil, ErrProviderPageInvalid
	}
}

func validateProviderPageNoDuplicateExternalSKU(items []InventoryProviderItem) error {
	seen := map[string]bool{}
	for _, item := range items {
		key := normalizeString(item.ExternalSKUID)
		if key == "" {
			return ErrProviderPageInvalid
		}
		if seen[key] {
			return ErrDuplicateExternalSKU
		}
		seen[key] = true
	}
	return nil
}

func providerCursorEqual(a, b datatypes.JSON) bool {
	return strings.TrimSpace(string(a)) == strings.TrimSpace(string(b))
}

func providerSafeMetadata(meta map[string]string) (datatypes.JSON, error) {
	return safeProviderMetadataJSON(meta)
}

func providerErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrProviderNotRegistered):
		return ErrCodeProviderNotRegistered
	case errors.Is(err, ErrProviderCapabilityForbidden):
		return ErrCodeProviderCapabilityForbidden
	case errors.Is(err, ErrProductionCapabilityForbidden):
		return ErrCodeProductionCapabilityForbidden
	case errors.Is(err, ErrProviderCursorInvalid):
		return ErrCodeProviderCursorInvalid
	case errors.Is(err, ErrProviderCursorLoop):
		return ErrCodeProviderCursorLoop
	case errors.Is(err, ErrProviderPageLimitExceeded):
		return ErrCodeProviderPageLimitExceeded
	case errors.Is(err, ErrProviderPageInvalid):
		return ErrCodeProviderPageInvalid
	case errors.Is(err, ErrProviderRejected):
		return ErrCodeProviderRejected
	case errors.Is(err, ErrProviderTimeout):
		return ErrCodeProviderTimeout
	case errors.Is(err, ErrSyncRunAlreadyRunning):
		return ErrCodeSyncRunAlreadyRunning
	case errors.Is(err, ErrSyncCancelled):
		return ErrCodeSyncCancelled
	case errors.Is(err, context.Canceled):
		return ErrCodeSyncCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrCodeProviderTimeout
	case errors.Is(err, ErrIdempotencyPayloadConflict):
		return ErrCodeIdempotencyPayloadConflict
	case errors.Is(err, ErrRevisionConflict):
		return ErrCodeRevisionConflict
	case errors.Is(err, ErrDuplicateExternalSKU):
		return ErrCodeDuplicateExternalSKU
	case errors.Is(err, ErrPermissionDenied):
		return ErrCodePermissionDenied
	default:
		return ErrCodeStateConflict
	}
}

func hashInventoryProviderItem(item InventoryProviderItem) string {
	payload, _ := json.Marshal(item)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func errWithCode(code string) error {
	switch code {
	case ErrCodeProviderNotRegistered:
		return ErrProviderNotRegistered
	case ErrCodeProviderCapabilityForbidden:
		return ErrProviderCapabilityForbidden
	case ErrCodeProductionCapabilityForbidden:
		return ErrProductionCapabilityForbidden
	case ErrCodeProviderCursorInvalid:
		return ErrProviderCursorInvalid
	case ErrCodeProviderCursorLoop:
		return ErrProviderCursorLoop
	case ErrCodeProviderPageLimitExceeded:
		return ErrProviderPageLimitExceeded
	case ErrCodeProviderPageInvalid:
		return ErrProviderPageInvalid
	case ErrCodeProviderRejected:
		return ErrProviderRejected
	case ErrCodeProviderTimeout:
		return ErrProviderTimeout
	case ErrCodeSyncRunAlreadyRunning:
		return ErrSyncRunAlreadyRunning
	case ErrCodeSyncCancelled:
		return ErrSyncCancelled
	default:
		return fmt.Errorf("inventory sync error: %s", code)
	}
}
