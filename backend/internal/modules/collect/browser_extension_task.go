package collect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// Browser-extension interactive collect task: created directly by the TradeMind
// browser extension, completed by its device token, never queued to Redis and
// never executed by collect workers.

const (
	browserExtensionEngine     = "browser_extension"
	browserExtensionLockPrefix = "browser-extension:"
	browserExtensionLockTTL    = 2 * time.Minute
)

var (
	// ErrBrowserExtensionTaskBusy means another device request currently owns the task.
	ErrBrowserExtensionTaskBusy = errors.New("browser extension task is being processed")
	// ErrBrowserExtensionTaskInvalid means the task is not bound to this device.
	ErrBrowserExtensionTaskInvalid = errors.New("browser extension task does not belong to this device")
	// ErrBrowserExtensionTaskNotActive means the task is no longer accepting results.
	ErrBrowserExtensionTaskNotActive = errors.New("browser extension task is not active")
)

// BrowserExtensionTaskInput creates an interactive task owned by one device.
type BrowserExtensionTaskInput struct {
	TenantID int64
	AdminID  uuid.UUID
	DeviceID uuid.UUID
	Source   string
	URL      string
}

// BrowserExtensionResultInput completes an interactive task with normalized product JSON.
type BrowserExtensionResultInput struct {
	TenantID    int64
	AdminID     uuid.UUID
	DeviceID    uuid.UUID
	TaskID      uuid.UUID
	ProductJSON json.RawMessage
}

// BrowserExtensionFailureInput fails an interactive task with an error code/message.
type BrowserExtensionFailureInput struct {
	TenantID int64
	AdminID  uuid.UUID
	DeviceID uuid.UUID
	TaskID   uuid.UUID
	Code     string
	Message  string
}

// BrowserExtensionTaskResult mirrors the extension CollectTask shape.
type BrowserExtensionTaskResult struct {
	ID              uuid.UUID  `json:"id"`
	Source          string     `json:"source"`
	SourceURL       string     `json:"sourceUrl"`
	Status          string     `json:"status"`
	ResultProductID *uuid.UUID `json:"resultProductId,omitempty"`
}

func browserExtensionRequestOptions(deviceID uuid.UUID) (datatypes.JSON, error) {
	if deviceID == uuid.Nil {
		return nil, fmt.Errorf("browser extension device id missing")
	}
	b, err := json.Marshal(map[string]any{
		"engine":   browserExtensionEngine,
		"deviceId": deviceID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("browser extension request options: %w", err)
	}
	return datatypes.JSON(b), nil
}

func browserExtensionTaskDeviceID(raw []byte) string {
	var snapshot struct {
		DeviceID string `json:"deviceId"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &snapshot)
	}
	return strings.TrimSpace(snapshot.DeviceID)
}

func browserExtensionTaskBoundTo(task *CollectTask, deviceID uuid.UUID) bool {
	if task == nil || deviceID == uuid.Nil {
		return false
	}
	return strings.EqualFold(browserExtensionTaskDeviceID(task.RequestOptions), deviceID.String())
}

func browserExtensionTaskResultOf(task *CollectTask) BrowserExtensionTaskResult {
	if task == nil {
		return BrowserExtensionTaskResult{}
	}
	return BrowserExtensionTaskResult{
		ID:              task.ID,
		Source:          task.Source,
		SourceURL:       task.SourceURL,
		Status:          task.Status,
		ResultProductID: task.ResultProductID,
	}
}

func (s *Service) releaseBrowserExtensionTaskLock(ctx context.Context, taskID uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{"locked_by": nil, "locked_until": nil, "updated_at": time.Now().UTC()}).Error
}

// claimBrowserExtensionTask loads the task, verifies tenant/device binding and
// atomically takes a short lease so concurrent submits cannot double-complete.
func (s *Service) claimBrowserExtensionTask(
	ctx context.Context,
	taskID uuid.UUID,
	tenantID int64,
	deviceID uuid.UUID,
) (*CollectTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var task CollectTask
	if err := s.DB.WithContext(ctx).
		First(&task, "id = ? AND tenant_id = ?", taskID, tenantID).Error; err != nil {
		return nil, err
	}
	if !browserExtensionTaskBoundTo(&task, deviceID) {
		return nil, ErrBrowserExtensionTaskInvalid
	}
	if task.Status != StatusRunning {
		return nil, ErrBrowserExtensionTaskNotActive
	}
	now := time.Now().UTC()
	lockID := browserExtensionLockPrefix + deviceID.String()
	until := now.Add(browserExtensionLockTTL)
	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where(
			"id = ? AND status = ? AND (locked_by IS NULL OR locked_until IS NULL OR locked_until < ?)",
			taskID, StatusRunning, now,
		).
		Updates(map[string]any{
			"locked_by":    lockID,
			"locked_until": until,
			"updated_at":   now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		var current CollectTask
		if err := s.DB.WithContext(ctx).First(&current, "id = ?", taskID).Error; err == nil {
			if current.Status != StatusRunning {
				return nil, ErrBrowserExtensionTaskNotActive
			}
			if current.LockedBy != nil &&
				*current.LockedBy != lockID &&
				current.LockedUntil != nil &&
				current.LockedUntil.After(now) {
				return nil, ErrBrowserExtensionTaskBusy
			}
		}
		return nil, ErrBrowserExtensionTaskBusy
	}
	task.LockedBy = &lockID
	task.LockedUntil = &until
	return &task, nil
}

// CreateBrowserExtensionTask persists a running interactive task bound to the
// device. It is never enqueued, so collect workers will not pick it up.
func (s *Service) CreateBrowserExtensionTask(
	ctx context.Context,
	in BrowserExtensionTaskInput,
) (BrowserExtensionTaskResult, error) {
	zero := BrowserExtensionTaskResult{}
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	source, err := canonicalBrowserExtensionSource(in.Source)
	if err != nil {
		return zero, err
	}
	url := strings.TrimSpace(in.URL)
	if err := validateBrowserExtensionSourceAndURL(source, url); err != nil {
		return zero, err
	}
	if in.DeviceID == uuid.Nil || in.AdminID == uuid.Nil || in.TenantID < 0 {
		return zero, fmt.Errorf("browser extension task identity invalid")
	}
	reqOpts, err := browserExtensionRequestOptions(in.DeviceID)
	if err != nil {
		return zero, err
	}
	now := time.Now().UTC()
	createdBy := in.AdminID
	task := &CollectTask{
		TenantID:       in.TenantID,
		Source:         source,
		SourceURL:      url,
		Status:         StatusRunning,
		MaxRetries:     0,
		CreatedBy:      &createdBy,
		RequestOptions: reqOpts,
		StartedAt:      &now,
	}
	if err := s.DB.WithContext(ctx).Create(task).Error; err != nil {
		return zero, err
	}
	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType: EventTaskCreated,
		ToStatus:  StatusRunning,
		Message:   "browser extension interactive task created",
		PayloadMap: map[string]any{
			"engine":   browserExtensionEngine,
			"deviceId": in.DeviceID.String(),
		},
	})
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.browser_extension.task.created",
			Resource:    "collect_task",
			ResourceID:  task.ID.String(),
			Status:      "success",
			Message:     "browser extension task created",
		})
	}
	return browserExtensionTaskResultOf(task), nil
}

// CompleteBrowserExtensionTask validates the submitted normalized product,
// imports it as a product draft and marks the task successful.
func (s *Service) CompleteBrowserExtensionTask(
	ctx context.Context,
	in BrowserExtensionResultInput,
) (BrowserExtensionTaskResult, error) {
	zero := BrowserExtensionTaskResult{}
	if s == nil || s.DB == nil || s.Products == nil {
		return zero, fmt.Errorf("browser extension collect unavailable")
	}
	if len(in.ProductJSON) == 0 {
		return zero, fmt.Errorf("browser extension product result is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// 产品导入依赖上下文中的租户（handler 的 gin TenantContext 不会自动进入
	// c.Request.Context()），这里显式注入任务租户。
	ctx = security.WithTenantContext(ctx, security.WorkerTenantContext(in.TenantID, in.AdminID))
	task, err := s.claimBrowserExtensionTask(ctx, in.TaskID, in.TenantID, in.DeviceID)
	if err != nil {
		return zero, err
	}

	norm, err := parseNormalized(in.ProductJSON)
	if err != nil {
		s.releaseBrowserExtensionTaskLock(ctx, task.ID)
		return zero, fmt.Errorf("parse normalized product: %w", err)
	}
	if strings.TrimSpace(norm.Title) == "" {
		s.releaseBrowserExtensionTaskLock(ctx, task.ID)
		return zero, fmt.Errorf("browser extension product title is empty")
	}
	if strings.TrimSpace(norm.Source) == "" {
		s.releaseBrowserExtensionTaskLock(ctx, task.ID)
		return zero, fmt.Errorf("browser extension product source is empty")
	}
	taskSource, taskSourceErr := canonicalBrowserExtensionSource(task.Source)
	productSource, productSourceErr := canonicalBrowserExtensionSource(norm.Source)
	if taskSourceErr != nil || productSourceErr != nil || productSource != taskSource {
		s.releaseBrowserExtensionTaskLock(ctx, task.ID)
		return zero, fmt.Errorf("browser extension product source %q does not match task source %q",
			norm.Source, task.Source)
	}

	params := norm.importParams(in.ProductJSON)
	storedJSON := in.ProductJSON
	switch {
	case isTaobaoTmallCollectSource(taskSource):
		params, storedJSON = normalizeTaobaoTmallImport(taskSource, norm, in.ProductJSON)
	case isBrowserExtension1688Source(taskSource):
		// Align with worker path: 1688 drafts require at least one main image.
		if len(norm.MainImages) == 0 {
			s.releaseBrowserExtensionTaskLock(ctx, task.ID)
			return zero, fmt.Errorf("missing main images")
		}
		// Keep generic importParams; full tiers/MOQ live in FullNormalizedJSON raw.
	default:
		s.releaseBrowserExtensionTaskLock(ctx, task.ID)
		return zero, fmt.Errorf("browser extension unsupported source %q", task.Source)
	}
	// The already-validated task identity is authoritative for persisted drafts.
	params.Source = taskSource
	params.SourceURL = task.SourceURL
	created, err := s.Products.ImportDraftWithContext(ctx, task.CreatedBy, params)
	if err != nil {
		s.releaseBrowserExtensionTaskLock(ctx, task.ID)
		return zero, err
	}

	fin := time.Now().UTC()
	pid := created.ID
	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ? AND status = ?", task.ID, StatusRunning).
		Updates(map[string]any{
			"status":            StatusSuccess,
			"result_product_id": pid,
			"raw_result":        datatypes.JSON(storedJSON),
			"error_message":     "",
			"finished_at":       &fin,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"retry_count":       0,
			"locked_by":         nil,
			"locked_until":      nil,
			"updated_at":        fin,
		})
	if res.Error != nil {
		return zero, res.Error
	}
	if res.RowsAffected != 1 {
		return zero, ErrBrowserExtensionTaskNotActive
	}

	var refreshed CollectTask
	if err := s.DB.WithContext(ctx).First(&refreshed, "id = ?", task.ID).Error; err != nil {
		return zero, err
	}
	s.RecordTaskEvent(ctx, &refreshed, TaskEventInput{
		EventType:  EventTaskSuccess,
		FromStatus: StatusRunning,
		ToStatus:   StatusSuccess,
		Message:    "browser extension submitted product",
		PayloadMap: map[string]any{"productId": pid.String()},
	})
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: refreshed.CreatedBy,
			Action:      "collect.browser_extension.task.success",
			Resource:    "collect_task",
			ResourceID:  refreshed.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("product_id=%s", pid.String()),
		})
	}
	return browserExtensionTaskResultOf(&refreshed), nil
}

// FailBrowserExtensionTask marks the interactive task failed with the
// device-reported error code/message.
func (s *Service) FailBrowserExtensionTask(
	ctx context.Context,
	in BrowserExtensionFailureInput,
) (BrowserExtensionTaskResult, error) {
	zero := BrowserExtensionTaskResult{}
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	task, err := s.claimBrowserExtensionTask(ctx, in.TaskID, in.TenantID, in.DeviceID)
	if err != nil {
		return zero, err
	}
	code := strings.TrimSpace(in.Code)
	msg := strings.TrimSpace(in.Message)
	if code == "" {
		code = "BROWSER_EXTENSION_FAILED"
	}
	errorMessage := truncateRunes(fmt.Sprintf("%s:%s", code, msg), 8000)
	fin := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ? AND status = ?", task.ID, StatusRunning).
		Updates(map[string]any{
			"status":            StatusFailed,
			"error_message":     errorMessage,
			"finished_at":       &fin,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"locked_by":         nil,
			"locked_until":      nil,
			"updated_at":        fin,
		})
	if res.Error != nil {
		return zero, res.Error
	}
	if res.RowsAffected != 1 {
		return zero, ErrBrowserExtensionTaskNotActive
	}

	var refreshed CollectTask
	if err := s.DB.WithContext(ctx).First(&refreshed, "id = ?", task.ID).Error; err != nil {
		return zero, err
	}
	s.RecordTaskEvent(ctx, &refreshed, TaskEventInput{
		EventType:    EventTaskFailed,
		FromStatus:   StatusRunning,
		ToStatus:     StatusFailed,
		Message:      "browser extension reported failure",
		ErrorMessage: errorMessage,
	})
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: refreshed.CreatedBy,
			Action:      "collect.browser_extension.task.failed",
			Resource:    "collect_task",
			ResourceID:  refreshed.ID.String(),
			Status:      "failed",
			Message:     truncateRunes(errorMessage, 2000),
		})
	}
	return browserExtensionTaskResultOf(&refreshed), nil
}
