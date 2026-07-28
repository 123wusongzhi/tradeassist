package inventorysyncp9

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const (
	BindingResolutionMatchedExisting      = "matched_existing"
	BindingResolutionMatchedAutomatic     = "matched_automatic"
	BindingResolutionManualReviewRequired = "manual_review_required"
	BindingResolutionUnmatched            = "unmatched"
	BindingResolutionConflict             = "conflict"
	BindingResolutionFailed               = "failed"
)

type BindingResolutionPipeline struct {
	DB                 *gorm.DB
	CalibrationService *SKUBindingCalibrationService
}

type BindingResolutionPageResult struct {
	TotalRecordCount          int
	MatchedRecordCount        int
	UnmatchedRecordCount      int
	ConflictRecordCount       int
	FailedRecordCount         int
	CalibrationCount          int
	ManualBindingRequestCount int
	ConfirmedBindingCount     int
	Results                   []BindingResolutionItemResult
}

type BindingResolutionItemResult struct {
	SnapshotID                string
	ExternalSKUID             string
	Resolution                string
	ManualBindingRequestID    string
	CalibrationCandidateCount int
	SafeReasonCodes           []string
}

func NewBindingResolutionPipeline(db *gorm.DB, calibrationService *SKUBindingCalibrationService) *BindingResolutionPipeline {
	return &BindingResolutionPipeline{DB: db, CalibrationService: calibrationService}
}

func (p *BindingResolutionPipeline) ResolvePageWithDB(ctx context.Context, tx *gorm.DB, tenantID int64, runID any, snapshots []InventorySnapshotItem) (BindingResolutionPageResult, error) {
	if p == nil || tx == nil || p.CalibrationService == nil {
		return BindingResolutionPageResult{}, fmt.Errorf("binding resolution pipeline: dependencies are nil")
	}
	result := BindingResolutionPageResult{TotalRecordCount: len(snapshots), Results: make([]BindingResolutionItemResult, 0, len(snapshots))}
	for _, snapshot := range snapshots {
		if err := ctx.Err(); err != nil {
			return result, ErrSyncCancelled
		}
		itemResult := BindingResolutionItemResult{SnapshotID: snapshot.ID.String(), ExternalSKUID: snapshot.ExternalSKUID}
		confirmed, err := getCurrentConfirmedWithDB(ctx, tx, tenantID, snapshot.ShopConnectionID, snapshot.ExternalSKUID)
		if err == nil {
			if verifyLocalSKU(ctx, tx, tenantID, confirmed.LocalProductID, confirmed.LocalSKUID) == nil {
				result.MatchedRecordCount++
				result.ConfirmedBindingCount++
				itemResult.Resolution = BindingResolutionMatchedExisting
				itemResult.SafeReasonCodes = []string{ReasonExistingConfirmedBinding}
				result.Results = append(result.Results, itemResult)
				continue
			}
		} else if !errors.Is(err, ErrNotFound) {
			return result, err
		}
		calibrated, err := p.CalibrationService.calibrateSnapshotWithDB(ctx, tx, snapshot)
		if err != nil {
			result.FailedRecordCount++
			itemResult.Resolution = BindingResolutionFailed
			result.Results = append(result.Results, itemResult)
			return result, err
		}
		result.CalibrationCount += len(calibrated.Candidates)
		itemResult.CalibrationCandidateCount = len(calibrated.Candidates)
		itemResult.SafeReasonCodes = append([]string(nil), calibrated.PolicyResult.ReasonCodes...)
		if calibrated.ManualBindingRequest != nil {
			result.ManualBindingRequestCount++
			itemResult.ManualBindingRequestID = calibrated.ManualBindingRequest.ID.String()
		}
		switch {
		case calibrated.PolicyResult.ConflictDetected || calibrated.MatchStatus == MatchResultConflict:
			result.ConflictRecordCount++
			itemResult.Resolution = BindingResolutionConflict
		case calibrated.PolicyResult.AutoConfirmEligible:
			result.MatchedRecordCount++
			itemResult.Resolution = BindingResolutionMatchedAutomatic
		case calibrated.PolicyResult.NoCandidate || calibrated.MatchStatus == MatchResultNoCandidate:
			result.UnmatchedRecordCount++
			itemResult.Resolution = BindingResolutionUnmatched
		case calibrated.PolicyResult.ManualReviewRequired:
			result.UnmatchedRecordCount++
			itemResult.Resolution = BindingResolutionManualReviewRequired
		default:
			result.UnmatchedRecordCount++
			itemResult.Resolution = BindingResolutionUnmatched
		}
		result.Results = append(result.Results, itemResult)
	}
	if result.TotalRecordCount != result.MatchedRecordCount+result.UnmatchedRecordCount+result.ConflictRecordCount+result.FailedRecordCount {
		return result, ErrStateConflict
	}
	return result, nil
}
