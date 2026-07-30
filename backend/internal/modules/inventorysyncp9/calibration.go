package inventorysyncp9

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	CalibrationVersionV1     = 1
	ThresholdPolicyVersionV1 = "calibration-threshold-policy-v1"
)

const (
	ReasonExactSKUCodeMatch             = "exact_sku_code_match"
	ReasonExactBarcodeMatch             = "exact_barcode_match"
	ReasonNormalizedSKUCodeMatch        = "normalized_sku_code_match"
	ReasonNormalizedBarcodeMatch        = "normalized_barcode_match"
	ReasonMissingExternalIdentifier     = "missing_external_identifier"
	ReasonMissingLocalIdentifier        = "missing_local_identifier"
	ReasonMultipleExactMatches          = "multiple_exact_matches"
	ReasonMultipleNormalizedMatches     = "multiple_normalized_matches"
	ReasonExistingConfirmedBinding      = "existing_confirmed_binding"
	ReasonExistingBindingConflict       = "existing_binding_conflict"
	ReasonCrossTenantCandidateRejected  = "cross_tenant_candidate_rejected"
	ReasonCalibrationThresholdNotMet    = "calibration_threshold_not_met"
	ReasonManualReviewRequired          = "manual_review_required"
	ReasonAutoConfirmationNotAuthorized = "auto_confirmation_not_authorized"
	ReasonNoBindingCandidate            = "no_binding_candidate"
)

const (
	MatchResultNoCandidate        = "no_candidate"
	MatchResultSingleCandidate    = "single_candidate"
	MatchResultMultipleCandidates = "multiple_candidates"
	MatchResultConflict           = "conflict"
)

type LocalSKUCandidate struct {
	TenantID       int64
	LocalProductID uuid.UUID
	LocalSKUID     uuid.UUID
	SKUCode        string
	Barcode        string
}

type CalibrationCandidate struct {
	TenantID                int64                `json:"tenantId"`
	CandidateLocalProductID uuid.UUID            `json:"candidateLocalProductId"`
	CandidateLocalSKUID     uuid.UUID            `json:"candidateLocalSkuId"`
	MatchStrategy           string               `json:"matchStrategy"`
	Confidence              int                  `json:"confidence"`
	ScoreBreakdown          []ScoreBreakdownItem `json:"scoreBreakdown"`
	ReasonCodes             []string             `json:"reasonCodes"`
	CalibrationVersion      int                  `json:"calibrationVersion"`
	NormalizationVersion    string               `json:"normalizationVersion"`
	InputFingerprint        string               `json:"inputFingerprint"`
	Status                  string               `json:"status"`
}

type ScoreBreakdownItem struct {
	Code   string `json:"code"`
	Points int    `json:"points"`
	Reason string `json:"reason"`
}

type IdentifierMatchResult struct {
	Status      string                 `json:"status"`
	Candidates  []CalibrationCandidate `json:"candidates"`
	ReasonCodes []string               `json:"reasonCodes"`
}

type ExactIdentifierMatcher struct {
	Normalizer SKUIdentifierNormalizer
}

func NewExactIdentifierMatcher(normalizer SKUIdentifierNormalizer) ExactIdentifierMatcher {
	if normalizer == nil {
		n := NewDefaultSKUIdentifierNormalizer()
		normalizer = n
	}
	return ExactIdentifierMatcher{Normalizer: normalizer}
}

func (m ExactIdentifierMatcher) Match(snapshot InventorySnapshotItem, candidates []LocalSKUCandidate) IdentifierMatchResult {
	normalizer := m.Normalizer
	if normalizer == nil {
		n := NewDefaultSKUIdentifierNormalizer()
		normalizer = n
	}
	fingerprint := inputFingerprint(snapshot, candidates)
	externalSKU := normalizer.NormalizeSKUCode(snapshot.ExternalSKUCode)
	externalBarcode := normalizer.NormalizeBarcode(snapshot.Barcode)
	matches := make(map[uuid.UUID]CalibrationCandidate)
	for _, local := range stableLocalCandidates(candidates) {
		if local.TenantID != snapshot.TenantID {
			continue
		}
		if local.LocalProductID == zeroUUID || local.LocalSKUID == zeroUUID {
			continue
		}
		localSKU := normalizer.NormalizeSKUCode(local.SKUCode)
		localBarcode := normalizer.NormalizeBarcode(local.Barcode)
		strategy, reasons, ok := bestMatchStrategy(snapshot, local, externalSKU, externalBarcode, localSKU, localBarcode)
		if !ok {
			continue
		}
		matches[local.LocalSKUID] = CalibrationCandidate{
			TenantID:                snapshot.TenantID,
			CandidateLocalProductID: local.LocalProductID,
			CandidateLocalSKUID:     local.LocalSKUID,
			MatchStrategy:           strategy,
			ReasonCodes:             reasons,
			CalibrationVersion:      CalibrationVersionV1,
			NormalizationVersion:    NormalizationVersionV1,
			InputFingerprint:        fingerprint,
			Status:                  CalibrationStatusCandidate,
		}
	}
	result := IdentifierMatchResult{Candidates: mapCandidates(matches)}
	if len(result.Candidates) == 0 {
		result.Status = MatchResultNoCandidate
		result.ReasonCodes = []string{ReasonNoBindingCandidate}
		if strings.TrimSpace(snapshot.ExternalSKUCode) == "" && strings.TrimSpace(snapshot.Barcode) == "" {
			result.ReasonCodes = append(result.ReasonCodes, ReasonMissingExternalIdentifier)
		}
		return result
	}
	bestPriority := strategyPriority(result.Candidates[0].MatchStrategy)
	bestCount := 0
	for _, candidate := range result.Candidates {
		if strategyPriority(candidate.MatchStrategy) == bestPriority {
			bestCount++
		}
	}
	if bestCount > 1 {
		result.Status = MatchResultConflict
		if bestPriority <= strategyPriority(MatchStrategyExactSKUCode) {
			result.ReasonCodes = []string{ReasonMultipleExactMatches}
		} else {
			result.ReasonCodes = []string{ReasonMultipleNormalizedMatches}
		}
		for idx := range result.Candidates {
			if strategyPriority(result.Candidates[idx].MatchStrategy) == bestPriority {
				result.Candidates[idx].Status = CalibrationStatusConflict
				result.Candidates[idx].ReasonCodes = appendUnique(result.Candidates[idx].ReasonCodes, result.ReasonCodes...)
			}
		}
		return result
	}
	if len(result.Candidates) == 1 {
		result.Status = MatchResultSingleCandidate
	} else {
		result.Status = MatchResultMultipleCandidates
	}
	return result
}

func bestMatchStrategy(snapshot InventorySnapshotItem, local LocalSKUCandidate, externalSKU NormalizedIdentifier, externalBarcode NormalizedIdentifier, localSKU NormalizedIdentifier, localBarcode NormalizedIdentifier) (string, []string, bool) {
	if strings.TrimSpace(snapshot.Barcode) != "" && strings.TrimSpace(local.Barcode) != "" && snapshot.Barcode == local.Barcode {
		return MatchStrategyExactBarcode, []string{ReasonExactBarcodeMatch}, true
	}
	if strings.TrimSpace(snapshot.ExternalSKUCode) != "" && strings.TrimSpace(local.SKUCode) != "" && snapshot.ExternalSKUCode == local.SKUCode {
		return MatchStrategyExactSKUCode, []string{ReasonExactSKUCodeMatch}, true
	}
	if externalBarcode.Valid && localBarcode.Valid && externalBarcode.NormalizedValue != "" && externalBarcode.NormalizedValue == localBarcode.NormalizedValue {
		return MatchStrategyNormalizedBarcode, []string{ReasonNormalizedBarcodeMatch}, true
	}
	if externalSKU.Valid && localSKU.Valid && externalSKU.NormalizedValue != "" && externalSKU.NormalizedValue == localSKU.NormalizedValue {
		return MatchStrategyNormalizedSKUCode, []string{ReasonNormalizedSKUCodeMatch}, true
	}
	return "", nil, false
}

func mapCandidates(values map[uuid.UUID]CalibrationCandidate) []CalibrationCandidate {
	rows := make([]CalibrationCandidate, 0, len(values))
	for _, candidate := range values {
		rows = append(rows, candidate)
	}
	sortCalibrationCandidates(rows)
	return rows
}

func stableLocalCandidates(candidates []LocalSKUCandidate) []LocalSKUCandidate {
	rows := append([]LocalSKUCandidate(nil), candidates...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TenantID != rows[j].TenantID {
			return rows[i].TenantID < rows[j].TenantID
		}
		if rows[i].LocalSKUID != rows[j].LocalSKUID {
			return rows[i].LocalSKUID.String() < rows[j].LocalSKUID.String()
		}
		return rows[i].LocalProductID.String() < rows[j].LocalProductID.String()
	})
	return rows
}

type CandidateScoringService struct{}

func NewCandidateScoringService() CandidateScoringService {
	return CandidateScoringService{}
}

func (CandidateScoringService) Score(result IdentifierMatchResult) ([]CalibrationCandidate, error) {
	candidates := append([]CalibrationCandidate(nil), result.Candidates...)
	for idx := range candidates {
		score, breakdown, reason, err := scoreForStrategy(candidates[idx].MatchStrategy)
		if err != nil {
			return nil, err
		}
		candidates[idx].Confidence = score
		candidates[idx].ScoreBreakdown = breakdown
		candidates[idx].ReasonCodes = appendUnique(candidates[idx].ReasonCodes, reason)
		if result.Status == MatchResultConflict {
			candidates[idx].ReasonCodes = appendUnique(candidates[idx].ReasonCodes, result.ReasonCodes...)
			if candidates[idx].Status == "" {
				candidates[idx].Status = CalibrationStatusConflict
			}
		}
		if candidates[idx].Status == "" {
			candidates[idx].Status = CalibrationStatusCandidate
		}
	}
	sortCalibrationCandidates(candidates)
	return candidates, nil
}

func scoreForStrategy(strategy string) (int, []ScoreBreakdownItem, string, error) {
	switch strategy {
	case MatchStrategyExactBarcode:
		return 10000, []ScoreBreakdownItem{{Code: "exactBarcodeScore", Points: 10000, Reason: ReasonExactBarcodeMatch}}, ReasonExactBarcodeMatch, nil
	case MatchStrategyExactSKUCode:
		return 9500, []ScoreBreakdownItem{{Code: "exactSKUCodeScore", Points: 9500, Reason: ReasonExactSKUCodeMatch}}, ReasonExactSKUCodeMatch, nil
	case MatchStrategyNormalizedBarcode:
		return 9000, []ScoreBreakdownItem{{Code: "normalizedBarcodeScore", Points: 9000, Reason: ReasonNormalizedBarcodeMatch}}, ReasonNormalizedBarcodeMatch, nil
	case MatchStrategyNormalizedSKUCode:
		return 8500, []ScoreBreakdownItem{{Code: "normalizedSKUCodeScore", Points: 8500, Reason: ReasonNormalizedSKUCodeMatch}}, ReasonNormalizedSKUCodeMatch, nil
	default:
		return 0, nil, "", ErrValidation
	}
}

type CalibrationThresholdConfig struct {
	HighConfidenceThreshold int
	AutoConfirmationEnabled bool
	PolicyVersion           string
}

type CalibrationThresholdPolicy struct {
	Config CalibrationThresholdConfig
}

type CalibrationPolicyResult struct {
	AutoConfirmEligible  bool     `json:"autoConfirmEligible"`
	ManualReviewRequired bool     `json:"manualReviewRequired"`
	ConflictDetected     bool     `json:"conflictDetected"`
	NoCandidate          bool     `json:"noCandidate"`
	ReasonCodes          []string `json:"reasonCodes"`
	PolicyVersion        string   `json:"policyVersion"`
}

func NewCalibrationThresholdPolicy(config CalibrationThresholdConfig) (CalibrationThresholdPolicy, error) {
	if config.HighConfidenceThreshold == 0 {
		config.HighConfidenceThreshold = 9500
	}
	if config.HighConfidenceThreshold < 1 || config.HighConfidenceThreshold > 10000 {
		return CalibrationThresholdPolicy{}, ErrCalibrationPolicyInvalid
	}
	if config.PolicyVersion == "" {
		config.PolicyVersion = ThresholdPolicyVersionV1
	}
	return CalibrationThresholdPolicy{Config: config}, nil
}

func (p CalibrationThresholdPolicy) Evaluate(matchStatus string, candidates []CalibrationCandidate) CalibrationPolicyResult {
	result := CalibrationPolicyResult{PolicyVersion: p.Config.PolicyVersion}
	if matchStatus == MatchResultNoCandidate || len(candidates) == 0 {
		result.NoCandidate = true
		result.ManualReviewRequired = true
		result.ReasonCodes = []string{ReasonNoBindingCandidate, ReasonManualReviewRequired}
		return result
	}
	if matchStatus == MatchResultConflict || hasConflictCandidate(candidates) {
		result.ConflictDetected = true
		result.ManualReviewRequired = true
		result.ReasonCodes = []string{ReasonExistingBindingConflict, ReasonManualReviewRequired}
		return result
	}
	if len(candidates) != 1 {
		result.ManualReviewRequired = true
		result.ReasonCodes = []string{ReasonMultipleNormalizedMatches, ReasonManualReviewRequired}
		return result
	}
	candidate := candidates[0]
	if candidate.Confidence < p.Config.HighConfidenceThreshold {
		result.ManualReviewRequired = true
		result.ReasonCodes = []string{ReasonCalibrationThresholdNotMet, ReasonManualReviewRequired}
		return result
	}
	if !p.Config.AutoConfirmationEnabled {
		result.ManualReviewRequired = true
		result.ReasonCodes = []string{ReasonAutoConfirmationNotAuthorized, ReasonManualReviewRequired}
		return result
	}
	result.AutoConfirmEligible = true
	return result
}

func hasConflictCandidate(candidates []CalibrationCandidate) bool {
	for _, candidate := range candidates {
		if candidate.Status == CalibrationStatusConflict || hasReason(candidate.ReasonCodes, ReasonExistingBindingConflict, ReasonMultipleExactMatches, ReasonMultipleNormalizedMatches) {
			return true
		}
	}
	return false
}

type LocalSKUCandidateProvider interface {
	ListLocalSKUCandidates(ctx context.Context, tenantID int64) ([]LocalSKUCandidate, error)
}

type GORMLocalSKUCandidateProvider struct {
	DB *gorm.DB
}

func NewGORMLocalSKUCandidateProvider(db *gorm.DB) GORMLocalSKUCandidateProvider {
	return GORMLocalSKUCandidateProvider{DB: db}
}

func (p GORMLocalSKUCandidateProvider) ListLocalSKUCandidates(ctx context.Context, tenantID int64) ([]LocalSKUCandidate, error) {
	if p.DB == nil {
		return nil, fmt.Errorf("local sku candidate provider: db is nil")
	}
	if validateTenantID(tenantID) != nil {
		return nil, ErrValidation
	}
	type row struct {
		TenantID  int64
		ProductID uuid.UUID
		SKUID     uuid.UUID
		SKUCode   string
	}
	var rows []row
	if err := p.DB.WithContext(ctx).Table("product_skus AS sk").
		Select("p.tenant_id AS tenant_id, p.id AS product_id, sk.id AS sku_id, sk.sku_code AS sku_code").
		Joins("JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL").
		Where("p.tenant_id = ?", tenantID).
		Order("sk.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	candidates := make([]LocalSKUCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, LocalSKUCandidate{TenantID: row.TenantID, LocalProductID: row.ProductID, LocalSKUID: row.SKUID, SKUCode: row.SKUCode})
	}
	return candidates, nil
}

type SKUBindingCalibrationService struct {
	DB                 *gorm.DB
	SnapshotRepository *InventorySnapshotRepository
	CandidateProvider  LocalSKUCandidateProvider
	Normalizer         SKUIdentifierNormalizer
	Matcher            ExactIdentifierMatcher
	Scorer             CandidateScoringService
	Policy             CalibrationThresholdPolicy
}

type CalibrationServiceResult struct {
	MatchStatus          string                  `json:"matchStatus"`
	Candidates           []CalibrationCandidate  `json:"candidates"`
	PolicyResult         CalibrationPolicyResult `json:"policyResult"`
	ManualBindingRequest *ManualBindingRequest   `json:"manualBindingRequest,omitempty"`
	InputFingerprint     string                  `json:"inputFingerprint"`
}

func NewSKUBindingCalibrationService(db *gorm.DB, provider LocalSKUCandidateProvider, policy CalibrationThresholdPolicy) *SKUBindingCalibrationService {
	normalizer := NewDefaultSKUIdentifierNormalizer()
	return &SKUBindingCalibrationService{
		DB:                 db,
		SnapshotRepository: NewInventorySnapshotRepository(db),
		CandidateProvider:  provider,
		Normalizer:         normalizer,
		Matcher:            NewExactIdentifierMatcher(normalizer),
		Scorer:             NewCandidateScoringService(),
		Policy:             policy,
	}
}

func (s *SKUBindingCalibrationService) CalibrateSnapshotItem(ctx context.Context, tenantID int64, runID uuid.UUID, snapshotID uuid.UUID) (*CalibrationServiceResult, error) {
	if s == nil || s.DB == nil || s.CandidateProvider == nil {
		return nil, fmt.Errorf("sku binding calibration service: dependencies are nil")
	}
	if validateTenantID(tenantID) != nil || runID == zeroUUID || snapshotID == zeroUUID {
		return nil, ErrValidation
	}
	var result *CalibrationServiceResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		calibrated, err := s.calibrateSnapshotItemWithDB(ctx, tx, tenantID, runID, snapshotID)
		if err != nil {
			return err
		}
		result = calibrated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RecalibrateSnapshotItem creates a new immutable calibration version while
// preserving all prior candidate rows. It is intentionally explicit so a
// caller cannot silently overwrite historical calibration evidence.
func (s *SKUBindingCalibrationService) RecalibrateSnapshotItem(ctx context.Context, tenantID int64, runID uuid.UUID, snapshotID uuid.UUID, expectedVersion int) (*CalibrationServiceResult, int, error) {
	if s == nil || s.DB == nil || s.CandidateProvider == nil {
		return nil, 0, fmt.Errorf("sku binding calibration service: dependencies are nil")
	}
	var result *CalibrationServiceResult
	version := 0
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, version, err = s.recalibrateSnapshotItemWithDB(ctx, tx, tenantID, runID, snapshotID, expectedVersion)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return result, version, nil
}

func (s *SKUBindingCalibrationService) recalibrateSnapshotItemWithDB(ctx context.Context, tx *gorm.DB, tenantID int64, runID uuid.UUID, snapshotID uuid.UUID, expectedVersion int) (*CalibrationServiceResult, int, error) {
	if s == nil || tx == nil || s.CandidateProvider == nil {
		return nil, 0, fmt.Errorf("sku binding calibration service: dependencies are nil")
	}
	if validateTenantID(tenantID) != nil || runID == zeroUUID || snapshotID == zeroUUID {
		return nil, 0, ErrValidation
	}
	var snapshot InventorySnapshotItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND inventory_sync_run_id = ? AND id = ?", tenantID, runID, snapshotID).First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, stableError(err, ErrStateConflict)
	}
	var current int
	if err := tx.Model(&SKUBindingCalibration{}).Where("tenant_id = ? AND inventory_snapshot_item_id = ?", tenantID, snapshotID).Select("COALESCE(MAX(calibration_version), 0)").Scan(&current).Error; err != nil {
		return nil, 0, stableError(err, ErrStateConflict)
	}
	if expectedVersion > 0 && current != expectedVersion {
		return nil, 0, ErrRevisionConflict
	}
	version := current + 1
	result, err := s.calibrateSnapshotWithDBVersion(ctx, tx, snapshot, version, true)
	if err != nil {
		return nil, 0, err
	}
	return result, version, nil
}

func (s *SKUBindingCalibrationService) calibrateSnapshotItemWithDB(ctx context.Context, tx *gorm.DB, tenantID int64, runID uuid.UUID, snapshotID uuid.UUID) (*CalibrationServiceResult, error) {
	if s == nil || tx == nil || s.CandidateProvider == nil {
		return nil, fmt.Errorf("sku binding calibration service: dependencies are nil")
	}
	var snapshot InventorySnapshotItem
	if err := tx.Where("tenant_id = ? AND inventory_sync_run_id = ? AND id = ?", tenantID, runID, snapshotID).First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return s.calibrateSnapshotWithDBVersion(ctx, tx, snapshot, 0, false)
}

func (s *SKUBindingCalibrationService) calibrateSnapshotWithDB(ctx context.Context, tx *gorm.DB, snapshot InventorySnapshotItem) (*CalibrationServiceResult, error) {
	return s.calibrateSnapshotWithDBVersion(ctx, tx, snapshot, 0, false)
}

func (s *SKUBindingCalibrationService) calibrateSnapshotWithDBVersion(ctx context.Context, tx *gorm.DB, snapshot InventorySnapshotItem, version int, force bool) (*CalibrationServiceResult, error) {
	candidates, err := s.CandidateProvider.ListLocalSKUCandidates(ctx, snapshot.TenantID)
	if err != nil {
		return nil, err
	}
	match := s.Matcher.Match(snapshot, candidates)
	scored, err := s.Scorer.Score(match)
	if err != nil {
		return nil, err
	}
	policyResult := s.Policy.Evaluate(match.Status, scored)
	fingerprint := inputFingerprint(snapshot, candidates)
	if force {
		fingerprint = hashString(fingerprint, version)
		for idx := range scored {
			scored[idx].CalibrationVersion = version
			scored[idx].InputFingerprint = fingerprint
		}
	} else if existing, err := listCalibrationsByFingerprint(ctx, tx, snapshot.TenantID, snapshot.ID, fingerprint); err == nil && len(existing) > 0 {
		result := &CalibrationServiceResult{MatchStatus: match.Status, Candidates: calibrationsToCandidates(existing), PolicyResult: policyResult, InputFingerprint: fingerprint}
		if req, err := getManualRequestByFingerprint(ctx, tx, snapshot.TenantID, snapshot.ID, fingerprint); err == nil {
			result.ManualBindingRequest = req
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return result, nil
	}
	if !force {
		if err := ensureNoCalibrationPayloadConflict(ctx, tx, snapshot.TenantID, snapshot.ID, fingerprint); err != nil {
			return nil, err
		}
	}
	if len(scored) > 0 {
		rows, err := candidatesToCalibrations(snapshot, scored)
		if err != nil {
			return nil, err
		}
		if err := createCalibrationBatchWithDB(ctx, tx, snapshot.TenantID, rows); err != nil {
			return nil, err
		}
	}
	result := &CalibrationServiceResult{MatchStatus: match.Status, Candidates: scored, PolicyResult: policyResult, InputFingerprint: fingerprint}
	if policyResult.ManualReviewRequired {
		req, err := createOrGetManualRequestWithDB(ctx, tx, snapshot, scored, policyResult, fingerprint)
		if err != nil {
			return nil, err
		}
		result.ManualBindingRequest = req
	}
	return result, nil
}

func candidatesToCalibrations(snapshot InventorySnapshotItem, candidates []CalibrationCandidate) ([]SKUBindingCalibration, error) {
	rows := make([]SKUBindingCalibration, 0, len(candidates))
	for _, candidate := range candidates {
		scoreBreakdown, err := json.Marshal(candidate.ScoreBreakdown)
		if err != nil {
			return nil, ErrValidation
		}
		reasonCodes, err := json.Marshal(candidate.ReasonCodes)
		if err != nil {
			return nil, ErrValidation
		}
		rows = append(rows, SKUBindingCalibration{
			TenantID:                snapshot.TenantID,
			InventorySyncRunID:      snapshot.InventorySyncRunID,
			InventorySnapshotItemID: snapshot.ID,
			ExternalSKUID:           snapshot.ExternalSKUID,
			CandidateLocalProductID: candidate.CandidateLocalProductID,
			CandidateLocalSKUID:     candidate.CandidateLocalSKUID,
			MatchStrategy:           candidate.MatchStrategy,
			Confidence:              candidate.Confidence,
			ScoreBreakdown:          datatypes.JSON(scoreBreakdown),
			ReasonCodes:             datatypes.JSON(reasonCodes),
			CalibrationVersion:      candidate.CalibrationVersion,
			Status:                  candidate.Status,
			InputFingerprint:        candidate.InputFingerprint,
		})
	}
	return rows, nil
}

func createOrGetManualRequestWithDB(ctx context.Context, tx *gorm.DB, snapshot InventorySnapshotItem, candidates []CalibrationCandidate, policy CalibrationPolicyResult, fingerprint string) (*ManualBindingRequest, error) {
	if existing, err := getManualRequestByFingerprint(ctx, tx, snapshot.TenantID, snapshot.ID, fingerprint); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	reason := ReasonManualReviewRequired
	if len(policy.ReasonCodes) > 0 {
		reason = policy.ReasonCodes[0]
	}
	var suggested *uuid.UUID
	if len(candidates) == 1 {
		suggested = &candidates[0].CandidateLocalSKUID
	}
	request := &ManualBindingRequest{
		TenantID:                snapshot.TenantID,
		InventorySyncRunID:      snapshot.InventorySyncRunID,
		InventorySnapshotItemID: snapshot.ID,
		ShopConnectionID:        snapshot.ShopConnectionID,
		ExternalSKUID:           snapshot.ExternalSKUID,
		Status:                  ManualBindingStatusPending,
		ReasonCode:              reason,
		CandidateCount:          len(candidates),
		SuggestedLocalSKUID:     suggested,
		RequestID:               "p9-manual-" + fingerprint[:24],
		IdempotencyKeyHash:      fingerprint,
		InputFingerprint:        fingerprint,
		Revision:                1,
	}
	if err := validateManualBindingRequest(request); err != nil {
		return nil, err
	}
	if err := tx.Create(request).Error; err != nil {
		if isUniqueViolation(err) {
			if existing, getErr := getPendingManualRequestByExternalSKU(ctx, tx, snapshot.TenantID, snapshot.ShopConnectionID, snapshot.ExternalSKUID); getErr == nil {
				return existing, nil
			}
			return nil, stableError(err, ErrManualBindingAlreadyPending)
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return request, nil
}

func listCalibrationsByFingerprint(ctx context.Context, db *gorm.DB, tenantID int64, snapshotID uuid.UUID, fingerprint string) ([]SKUBindingCalibration, error) {
	var rows []SKUBindingCalibration
	if err := db.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id = ? AND input_fingerprint = ?", tenantID, snapshotID, fingerprint).Order("confidence DESC, match_strategy ASC, candidate_local_sku_id ASC").Find(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return rows, nil
}

func ensureNoCalibrationPayloadConflict(ctx context.Context, db *gorm.DB, tenantID int64, snapshotID uuid.UUID, fingerprint string) error {
	var count int64
	if err := db.WithContext(ctx).Model(&SKUBindingCalibration{}).Where("tenant_id = ? AND inventory_snapshot_item_id = ? AND input_fingerprint <> ?", tenantID, snapshotID, fingerprint).Count(&count).Error; err != nil {
		return stableError(err, ErrStateConflict)
	}
	if count > 0 {
		return ErrIdempotencyPayloadConflict
	}
	return nil
}

func getManualRequestByFingerprint(ctx context.Context, db *gorm.DB, tenantID int64, snapshotID uuid.UUID, fingerprint string) (*ManualBindingRequest, error) {
	var request ManualBindingRequest
	if err := db.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id = ? AND input_fingerprint = ?", tenantID, snapshotID, fingerprint).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &request, nil
}

func getPendingManualRequestByExternalSKU(ctx context.Context, db *gorm.DB, tenantID int64, shopConnectionID uuid.UUID, externalSKUID string) (*ManualBindingRequest, error) {
	var request ManualBindingRequest
	if err := db.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id = ? AND status = ?", tenantID, shopConnectionID, normalizeString(externalSKUID), ManualBindingStatusPending).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &request, nil
}

func calibrationsToCandidates(rows []SKUBindingCalibration) []CalibrationCandidate {
	candidates := make([]CalibrationCandidate, 0, len(rows))
	for _, row := range rows {
		var breakdown []ScoreBreakdownItem
		_ = json.Unmarshal(row.ScoreBreakdown, &breakdown)
		var reasons []string
		_ = json.Unmarshal(row.ReasonCodes, &reasons)
		candidates = append(candidates, CalibrationCandidate{
			TenantID:                row.TenantID,
			CandidateLocalProductID: row.CandidateLocalProductID,
			CandidateLocalSKUID:     row.CandidateLocalSKUID,
			MatchStrategy:           row.MatchStrategy,
			Confidence:              row.Confidence,
			ScoreBreakdown:          breakdown,
			ReasonCodes:             reasons,
			CalibrationVersion:      row.CalibrationVersion,
			NormalizationVersion:    NormalizationVersionV1,
			InputFingerprint:        row.InputFingerprint,
			Status:                  row.Status,
		})
	}
	sortCalibrationCandidates(candidates)
	return candidates
}

func createCalibrationBatchWithDB(ctx context.Context, tx *gorm.DB, tenantID int64, calibrations []SKUBindingCalibration) error {
	repo := NewSKUBindingCalibrationRepository(tx)
	return repo.CreateBatch(ctx, tenantID, calibrations)
}

func inputFingerprint(snapshot InventorySnapshotItem, candidates []LocalSKUCandidate) string {
	type fingerprintCandidate struct {
		TenantID       int64  `json:"tenantId"`
		LocalProductID string `json:"localProductId"`
		LocalSKUID     string `json:"localSkuId"`
		SKUCode        string `json:"skuCode"`
		Barcode        string `json:"barcode"`
	}
	payload := struct {
		TenantID             int64                  `json:"tenantId"`
		InventorySyncRunID   string                 `json:"inventorySyncRunId"`
		InventorySnapshotID  string                 `json:"inventorySnapshotItemId"`
		ExternalSKUID        string                 `json:"externalSkuId"`
		ExternalSKUCode      string                 `json:"externalSkuCode"`
		Barcode              string                 `json:"barcode"`
		CalibrationVersion   int                    `json:"calibrationVersion"`
		NormalizationVersion string                 `json:"normalizationVersion"`
		Candidates           []fingerprintCandidate `json:"candidates"`
	}{
		TenantID:             snapshot.TenantID,
		InventorySyncRunID:   snapshot.InventorySyncRunID.String(),
		InventorySnapshotID:  snapshot.ID.String(),
		ExternalSKUID:        snapshot.ExternalSKUID,
		ExternalSKUCode:      snapshot.ExternalSKUCode,
		Barcode:              snapshot.Barcode,
		CalibrationVersion:   CalibrationVersionV1,
		NormalizationVersion: NormalizationVersionV1,
	}
	for _, candidate := range stableLocalCandidates(candidates) {
		payload.Candidates = append(payload.Candidates, fingerprintCandidate{
			TenantID:       candidate.TenantID,
			LocalProductID: candidate.LocalProductID.String(),
			LocalSKUID:     candidate.LocalSKUID.String(),
			SKUCode:        candidate.SKUCode,
			Barcode:        candidate.Barcode,
		})
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func sortCalibrationCandidates(candidates []CalibrationCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		if strategyPriority(candidates[i].MatchStrategy) != strategyPriority(candidates[j].MatchStrategy) {
			return strategyPriority(candidates[i].MatchStrategy) < strategyPriority(candidates[j].MatchStrategy)
		}
		return candidates[i].CandidateLocalSKUID.String() < candidates[j].CandidateLocalSKUID.String()
	})
}

func strategyPriority(strategy string) int {
	switch strategy {
	case MatchStrategyExactBarcode:
		return 1
	case MatchStrategyExactSKUCode:
		return 2
	case MatchStrategyNormalizedBarcode:
		return 3
	case MatchStrategyNormalizedSKUCode:
		return 4
	default:
		return 100
	}
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	out := make([]string, 0, len(values)+len(additions))
	for _, value := range append(values, additions...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasReason(values []string, reasons ...string) bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	for _, reason := range reasons {
		if set[reason] {
			return true
		}
	}
	return false
}
