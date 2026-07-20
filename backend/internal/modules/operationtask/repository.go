package operationtask

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OperationTaskRepository struct {
	DB *gorm.DB
}

func NewOperationTaskRepository(db *gorm.DB) *OperationTaskRepository {
	return &OperationTaskRepository{DB: db}
}

func (r *OperationTaskRepository) Create(ctx context.Context, task *OperationTask) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("operation task repository: db is nil")
	}
	if err := validateOperationTask(task); err != nil {
		return err
	}
	if err := r.DB.WithContext(ctx).Create(task).Error; err != nil {
		if isUniqueViolation(err) {
			return stableError(err, ErrDuplicateIdempotencyKey)
		}
		return stableError(err, ErrConflict)
	}
	return nil
}

func (r *OperationTaskRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*OperationTask, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task repository: db is nil")
	}
	var task OperationTask
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &task, nil
}

func (r *OperationTaskRepository) GetByIdempotencyKey(ctx context.Context, tenantID int64, key string) (*OperationTask, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task repository: db is nil")
	}
	key = strings.TrimSpace(key)
	if tenantID <= 0 || key == "" {
		return nil, ErrValidation
	}
	var task OperationTask
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).
		First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &task, nil
}

func (r *OperationTaskRepository) List(ctx context.Context, p OperationTaskListParams) (OperationTaskListResult, error) {
	var zero OperationTaskListResult
	if r == nil || r.DB == nil {
		return zero, fmt.Errorf("operation task repository: db is nil")
	}
	if p.TenantID <= 0 {
		return zero, ErrValidation
	}
	limit := p.Limit
	if limit <= 0 {
		limit = pagination.DefaultLimit
	}
	if limit > pagination.MaxLimit {
		limit = pagination.MaxLimit
	}

	scopeHash := operationTaskListScopeHash(p)
	var cur pagination.CursorPayload
	if strings.TrimSpace(p.Cursor) != "" {
		decoded, err := pagination.DecodeCursor(p.Cursor, p.TenantID, "", scopeHash)
		if err != nil {
			return zero, err
		}
		cur = decoded
	}

	q := r.DB.WithContext(ctx).Model(&OperationTask{}).Where("tenant_id = ?", p.TenantID)
	if p.Status = strings.TrimSpace(strings.ToLower(p.Status)); p.Status != "" {
		q = q.Where("status = ?", p.Status)
	}
	if p.Platform = strings.TrimSpace(strings.ToLower(p.Platform)); p.Platform != "" {
		q = q.Where("platform = ?", p.Platform)
	}
	if p.TaskType = strings.TrimSpace(strings.ToLower(p.TaskType)); p.TaskType != "" {
		q = q.Where("task_type = ?", p.TaskType)
	}
	q, err := pagination.ApplyDescKeyset(q, "updated_at", "id", cur)
	if err != nil {
		return zero, err
	}

	var rows []OperationTask
	if err := q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return zero, stableError(err, ErrConflict)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	next := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		next, err = pagination.BuildNextCursor(true, p.TenantID, "", scopeHash, "updated_at", last.UpdatedAt, last.ID.String())
		if err != nil {
			return zero, err
		}
	}
	return OperationTaskListResult{Items: rows, Limit: limit, HasMore: hasMore, NextCursor: next}, nil
}

func (r *OperationTaskRepository) UpdateRevision(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int, patch OperationTaskPatch) (*OperationTask, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task repository: db is nil")
	}
	if tenantID <= 0 || expectedRevision < 1 {
		return nil, ErrValidation
	}
	now := utcNow()
	updates := map[string]any{
		"revision":   gorm.Expr("revision + 1"),
		"updated_at": now,
	}
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return nil, ErrValidation
		}
		updates["title"] = title
	}
	if patch.Summary != nil {
		updates["summary"] = strings.TrimSpace(*patch.Summary)
	}
	if patch.Payload != nil {
		if !isValidJSON(*patch.Payload) || payloadHasSecret(*patch.Payload) {
			return nil, ErrValidation
		}
		updates["payload"] = *patch.Payload
	}
	if patch.Status != nil {
		status := strings.TrimSpace(strings.ToLower(*patch.Status))
		if !allowedOperationTaskStatuses[status] {
			return nil, ErrValidation
		}
		updates["status"] = status
	}
	if patch.Priority != nil {
		priority := strings.TrimSpace(strings.ToLower(*patch.Priority))
		if !allowedPriorities[priority] {
			return nil, ErrValidation
		}
		updates["priority"] = priority
	}
	if patch.UpdatedBy != nil {
		updates["updated_by"] = patch.UpdatedBy
	}

	res := r.DB.WithContext(ctx).Model(&OperationTask{}).
		Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, expectedRevision).
		Updates(updates)
	if res.Error != nil {
		return nil, stableError(res.Error, ErrConflict)
	}
	if res.RowsAffected == 0 {
		var exists int64
		if err := r.DB.WithContext(ctx).Model(&OperationTask{}).Where("tenant_id = ? AND id = ?", tenantID, id).Count(&exists).Error; err != nil {
			return nil, stableError(err, ErrConflict)
		}
		if exists == 0 {
			return nil, ErrNotFound
		}
		return nil, ErrRevisionConflict
	}
	return r.GetByID(ctx, tenantID, id)
}

func operationTaskListScopeHash(p OperationTaskListParams) string {
	return pagination.Fingerprint(map[string]any{
		"tenantId": p.TenantID,
		"status":   p.Status,
		"platform": p.Platform,
		"taskType": p.TaskType,
		"sort":     "updated_at_desc_id_desc",
	})
}

type PlatformDraftRepository struct {
	DB *gorm.DB
}

func NewPlatformDraftRepository(db *gorm.DB) *PlatformDraftRepository {
	return &PlatformDraftRepository{DB: db}
}

func (r *PlatformDraftRepository) CreateVersion(ctx context.Context, draft *PlatformDraft) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("platform draft repository: db is nil")
	}
	if err := validatePlatformDraft(draft); err != nil {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task OperationTask
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", draft.OperationTaskID).
			First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return stableError(err, ErrConflict)
		}
		if task.TenantID != draft.TenantID {
			return ErrTenantMismatch
		}
		if !strings.EqualFold(task.Platform, draft.Platform) {
			return ErrValidation
		}
		if err := tx.Create(draft).Error; err != nil {
			if isUniqueViolation(err) {
				return stableError(err, ErrDuplicateDraftVersion)
			}
			return stableError(err, ErrConflict)
		}
		return nil
	})
}

func (r *PlatformDraftRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var draft PlatformDraft
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&draft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func (r *PlatformDraftRepository) GetVersion(ctx context.Context, tenantID int64, taskID uuid.UUID, version int) (*PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var draft PlatformDraft
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND draft_version = ?", tenantID, taskID, version).
		First(&draft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func (r *PlatformDraftRepository) GetLatest(ctx context.Context, tenantID int64, taskID uuid.UUID) (*PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var draft PlatformDraft
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("draft_version DESC").
		First(&draft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func (r *PlatformDraftRepository) ListVersions(ctx context.Context, tenantID int64, taskID uuid.UUID) ([]PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var drafts []PlatformDraft
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("draft_version DESC, id DESC").
		Find(&drafts).Error; err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return drafts, nil
}
