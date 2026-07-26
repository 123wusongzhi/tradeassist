package operationtask

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskTransitionService struct {
	DB           *gorm.DB
	StateMachine *TaskStateMachine
}

type TaskTransitionInput struct {
	TenantID         int64
	OperationTaskID  uuid.UUID
	ExpectedRevision int
	ToStatus         string
	ActorType        string
	ActorID          *uuid.UUID
	RequestID        string
	Reason           string
}

func NewTaskTransitionService(db *gorm.DB) *TaskTransitionService {
	return &TaskTransitionService{DB: db, StateMachine: NewTaskStateMachine()}
}

func (s *TaskTransitionService) Transition(ctx context.Context, in TaskTransitionInput) (*OperationTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("task transition service: db is nil")
	}
	if in.TenantID <= 0 || in.OperationTaskID == uuid.Nil || in.ExpectedRevision < 1 {
		return nil, ErrValidation
	}
	toStatus := normalizeTaskStatusValue(in.ToStatus)
	if !allowedOperationTaskStatuses[toStatus] {
		return nil, ErrValidation
	}
	sm := s.StateMachine
	if sm == nil {
		sm = NewTaskStateMachine()
	}
	var updated OperationTask
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if task.Revision != in.ExpectedRevision {
			return ErrRevisionConflict
		}
		if err := sm.ValidateTransition(task.Status, toStatus); err != nil {
			return err
		}
		patch := map[string]any{
			"status":     toStatus,
			"revision":   gorm.Expr("revision + 1"),
			"updated_at": utcNow(),
		}
		if in.ActorID != nil {
			patch["updated_by"] = in.ActorID
		}
		res := tx.Model(&OperationTask{}).
			Where("tenant_id = ? AND id = ? AND revision = ?", in.TenantID, in.OperationTaskID, in.ExpectedRevision).
			Updates(patch)
		if res.Error != nil {
			return stableError(res.Error, ErrConflict)
		}
		if res.RowsAffected == 0 {
			return ErrRevisionConflict
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       eventTypeForStatus(toStatus),
			ActorType:       normalizeActorType(in.ActorType),
			ActorID:         in.ActorID,
			BeforeState:     task.Status,
			AfterState:      toStatus,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          strings.TrimSpace(in.Reason),
			Metadata:        datatypes.JSON([]byte(`{}`)),
		}); err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND id = ?", in.TenantID, in.OperationTaskID).First(&updated).Error; err != nil {
			return stableError(err, ErrConflict)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

type DraftVersionService struct {
	DB           *gorm.DB
	StateMachine *TaskStateMachine
}

type DraftVersionInput struct {
	TenantID         int64
	OperationTaskID  uuid.UUID
	ExpectedRevision int
	Payload          datatypes.JSON
	ActorID          *uuid.UUID
	RequestID        string
	IdempotencyKey   string
	ChangeReason     string
}

func NewDraftVersionService(db *gorm.DB) *DraftVersionService {
	return &DraftVersionService{DB: db, StateMachine: NewTaskStateMachine()}
}

func (s *DraftVersionService) CreateInitialDraft(ctx context.Context, in DraftVersionInput) (*PlatformDraft, error) {
	return s.createDraftVersion(ctx, in, true)
}

func (s *DraftVersionService) CreateNextVersion(ctx context.Context, in DraftVersionInput) (*PlatformDraft, error) {
	return s.createDraftVersion(ctx, in, false)
}

func (s *DraftVersionService) EditDraft(ctx context.Context, in DraftVersionInput) (*PlatformDraft, error) {
	return s.CreateNextVersion(ctx, in)
}

func (s *DraftVersionService) GetLatestDraft(ctx context.Context, tenantID int64, taskID uuid.UUID) (*PlatformDraft, error) {
	return NewPlatformDraftRepository(s.DB).GetLatest(ctx, tenantID, taskID)
}

func (s *DraftVersionService) ComputePayloadHash(raw datatypes.JSON) (string, error) {
	return ComputePayloadHash([]byte(raw))
}

func (s *DraftVersionService) createDraftVersion(ctx context.Context, in DraftVersionInput, initial bool) (*PlatformDraft, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("draft version service: db is nil")
	}
	if in.TenantID <= 0 || in.OperationTaskID == uuid.Nil || in.ExpectedRevision < 1 || strings.TrimSpace(in.IdempotencyKey) == "" {
		return nil, ErrValidation
	}
	if !isValidJSON(in.Payload) || payloadHasSecret(in.Payload) {
		return nil, ErrValidation
	}
	hash, err := ComputePayloadHash([]byte(in.Payload))
	if err != nil {
		return nil, err
	}
	sm := s.StateMachine
	if sm == nil {
		sm = NewTaskStateMachine()
	}
	var out PlatformDraft
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if sm.IsTerminal(task.Status) {
			return ErrStateConflict
		}
		if existing, err := findDraftByIdempotency(tx, in.TenantID, in.OperationTaskID, in.IdempotencyKey); err == nil {
			if existing.PayloadHash != hash {
				return ErrDuplicateRequest
			}
			out = *existing
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if task.Revision != in.ExpectedRevision {
			return ErrRevisionConflict
		}
		latest, latestErr := findLatestDraftTx(tx, in.TenantID, in.OperationTaskID)
		if initial {
			if latestErr == nil {
				return ErrDuplicateDraftVersion
			}
			if !errors.Is(latestErr, ErrNotFound) {
				return latestErr
			}
			if err := sm.ValidateTransition(task.Status, OperationTaskStatusPendingReview); err != nil {
				return err
			}
			out = newDraftFromInput(task, in, 1, hash)
			out.Status = PlatformDraftStatusPendingReview
			if err := tx.Create(&out).Error; err != nil {
				return handleDraftCreateError(err, tx, in, hash, &out)
			}
			if err := updateTaskStatusRevisionTx(tx, task, OperationTaskStatusPendingReview, in.ActorID); err != nil {
				return err
			}
			return appendDraftEventsTx(tx, task, out, in, []draftEventSpec{
				{eventType: OperationTaskEventTypeDraftGenerated, before: task.Status, after: OperationTaskStatusPendingReview},
				{eventType: OperationTaskEventTypeReviewRequested, before: OperationTaskStatusPendingReview, after: OperationTaskStatusPendingReview},
			})
		}
		if errors.Is(latestErr, ErrNotFound) {
			return ErrNotFound
		}
		if latestErr != nil {
			return latestErr
		}
		nextStatus := task.Status
		events := []draftEventSpec{{eventType: OperationTaskEventTypeDraftUpdated, before: task.Status, after: task.Status}}
		if task.Status == OperationTaskStatusApproved {
			if err := sm.ValidateTransition(task.Status, OperationTaskStatusPendingReview); err != nil {
				return err
			}
			nextStatus = OperationTaskStatusPendingReview
			events[0].after = nextStatus
			events = append(events, draftEventSpec{eventType: OperationTaskEventTypeReviewRequested, before: nextStatus, after: nextStatus})
		}
		out = newDraftFromInput(task, in, latest.DraftVersion+1, hash)
		out.Status = PlatformDraftStatusPendingReview
		if err := tx.Create(&out).Error; err != nil {
			return handleDraftCreateError(err, tx, in, hash, &out)
		}
		if err := updateTaskStatusRevisionTx(tx, task, nextStatus, in.ActorID); err != nil {
			return err
		}
		return appendDraftEventsTx(tx, task, out, in, events)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type ApprovalAuthorizer interface {
	CanReview(ctx context.Context, tenantID int64, actorID uuid.UUID) error
}

type ApprovalService struct {
	DB           *gorm.DB
	StateMachine *TaskStateMachine
	Authorizer   ApprovalAuthorizer
}

type ApprovalInput struct {
	TenantID         int64
	OperationTaskID  uuid.UUID
	ExpectedRevision int
	DraftVersion     int
	DraftPayloadHash string
	ReviewerID       uuid.UUID
	ReviewerRole     string
	Reason           string
	Comment          string
	RequestID        string
	IdempotencyKey   string
}

func NewApprovalService(db *gorm.DB, authorizer ApprovalAuthorizer) *ApprovalService {
	return &ApprovalService{DB: db, StateMachine: NewTaskStateMachine(), Authorizer: authorizer}
}

func (s *ApprovalService) Approve(ctx context.Context, in ApprovalInput) (*ApprovalRecord, error) {
	return s.decide(ctx, in, ApprovalDecisionApproved, OperationTaskStatusApproved, OperationTaskEventTypeApproved)
}

func (s *ApprovalService) Reject(ctx context.Context, in ApprovalInput) (*ApprovalRecord, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrValidation
	}
	return s.decide(ctx, in, ApprovalDecisionRejected, OperationTaskStatusRejected, OperationTaskEventTypeRejected)
}

func (s *ApprovalService) decide(ctx context.Context, in ApprovalInput, decision string, toStatus string, eventType string) (*ApprovalRecord, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("approval service: db is nil")
	}
	if in.TenantID <= 0 || in.OperationTaskID == uuid.Nil || in.ExpectedRevision < 1 ||
		in.DraftVersion < 1 || in.ReviewerID == uuid.Nil || strings.TrimSpace(in.IdempotencyKey) == "" ||
		strings.TrimSpace(in.RequestID) == "" {
		return nil, ErrValidation
	}
	in.DraftPayloadHash = strings.TrimSpace(strings.ToLower(in.DraftPayloadHash))
	if !sha256LowerHex.MatchString(in.DraftPayloadHash) {
		return nil, ErrValidation
	}
	if s.Authorizer == nil {
		return nil, ErrPermissionDenied
	}
	if err := s.Authorizer.CanReview(ctx, in.TenantID, in.ReviewerID); err != nil {
		return nil, ErrPermissionDenied
	}
	sm := s.StateMachine
	if sm == nil {
		sm = NewTaskStateMachine()
	}
	var out ApprovalRecord
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := findApprovalByIdempotency(tx, in.TenantID, in.OperationTaskID, in.IdempotencyKey); err == nil {
			if existing.Decision != decision || existing.DraftVersion != in.DraftVersion || existing.DraftPayloadHash != in.DraftPayloadHash {
				return ErrApprovalIdemConflict
			}
			out = *existing
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if task.Revision != in.ExpectedRevision {
			return ErrRevisionConflict
		}
		if task.Status != OperationTaskStatusPendingReview {
			return ErrStateConflict
		}
		if err := sm.ValidateTransition(task.Status, toStatus); err != nil {
			return err
		}
		latest, err := findLatestDraftTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if latest.DraftVersion != in.DraftVersion {
			return ErrDraftNotLatest
		}
		if latest.PayloadHash != in.DraftPayloadHash {
			return ErrDraftHashMismatch
		}
		key := strings.TrimSpace(in.IdempotencyKey)
		out = ApprovalRecord{
			TenantID:         in.TenantID,
			OperationTaskID:  in.OperationTaskID,
			PlatformDraftID:  latest.ID,
			Decision:         decision,
			DraftVersion:     latest.DraftVersion,
			DraftPayloadHash: latest.PayloadHash,
			ReviewerID:       in.ReviewerID,
			ReviewerRole:     strings.TrimSpace(strings.ToLower(in.ReviewerRole)),
			Reason:           strings.TrimSpace(in.Reason),
			Comment:          strings.TrimSpace(in.Comment),
			RequestID:        strings.TrimSpace(in.RequestID),
			IdempotencyKey:   &key,
		}
		if err := validateApprovalRecord(&out); err != nil {
			return err
		}
		if err := tx.Create(&out).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateRequest
			}
			return stableError(err, ErrConflict)
		}
		if err := updateTaskStatusRevisionTx(tx, task, toStatus, &in.ReviewerID); err != nil {
			return err
		}
		return appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       eventType,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &in.ReviewerID,
			BeforeState:     task.Status,
			AfterState:      toStatus,
			PlatformDraftID: &latest.ID,
			DraftVersion:    latest.DraftVersion,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          strings.TrimSpace(in.Reason),
			Metadata:        datatypes.JSON([]byte(`{}`)),
		})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type draftEventSpec struct {
	eventType string
	before    string
	after     string
}

func newDraftFromInput(task *OperationTask, in DraftVersionInput, version int, hash string) PlatformDraft {
	key := strings.TrimSpace(in.IdempotencyKey)
	return PlatformDraft{
		TenantID:        in.TenantID,
		OperationTaskID: in.OperationTaskID,
		Platform:        task.Platform,
		AdapterMode:     adapterModeForPlatform(task.Platform),
		DraftVersion:    version,
		Payload:         in.Payload,
		PayloadHash:     hash,
		Status:          PlatformDraftStatusPendingReview,
		ChangeReason:    strings.TrimSpace(in.ChangeReason),
		IdempotencyKey:  &key,
		CreatedBy:       in.ActorID,
		UpdatedBy:       in.ActorID,
	}
}

func lockTask(tx *gorm.DB, tenantID int64, taskID uuid.UUID) (*OperationTask, error) {
	var task OperationTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &task, nil
}

func updateTaskStatusRevisionTx(tx *gorm.DB, task *OperationTask, status string, actorID *uuid.UUID) error {
	updates := map[string]any{
		"status":     status,
		"revision":   gorm.Expr("revision + 1"),
		"updated_at": utcNow(),
	}
	if actorID != nil {
		updates["updated_by"] = actorID
	}
	res := tx.Model(&OperationTask{}).
		Where("tenant_id = ? AND id = ? AND revision = ?", task.TenantID, task.ID, task.Revision).
		Updates(updates)
	if res.Error != nil {
		return stableError(res.Error, ErrConflict)
	}
	if res.RowsAffected == 0 {
		return ErrRevisionConflict
	}
	return nil
}

func appendDraftEventsTx(tx *gorm.DB, task *OperationTask, draft PlatformDraft, in DraftVersionInput, events []draftEventSpec) error {
	for _, event := range events {
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       event.eventType,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         in.ActorID,
			BeforeState:     event.before,
			AfterState:      event.after,
			PlatformDraftID: &draft.ID,
			DraftVersion:    draft.DraftVersion,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          strings.TrimSpace(in.ChangeReason),
			Metadata:        datatypes.JSON([]byte(`{}`)),
		}); err != nil {
			return err
		}
		task.Status = event.after
	}
	return nil
}

func appendAuditEventTx(tx *gorm.DB, event OperationTaskEvent) error {
	event.Metadata = redactSafeJSON(event.Metadata)
	latest := OperationTaskEvent{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND operation_task_id = ?", event.TenantID, event.OperationTaskID).
		Order("sequence DESC, id DESC").
		First(&latest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return stableError(err, ErrConflict)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		event.Sequence = 1
	} else {
		event.Sequence = latest.Sequence + 1
	}
	if err := validateOperationTaskEvent(&event); err != nil {
		return err
	}
	if err := tx.Create(&event).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateEventSequence
		}
		return stableError(err, ErrConflict)
	}
	return nil
}

func findLatestDraftTx(tx *gorm.DB, tenantID int64, taskID uuid.UUID) (*PlatformDraft, error) {
	var draft PlatformDraft
	err := tx.Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("draft_version DESC, id DESC").
		First(&draft).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func findDraftByIdempotency(tx *gorm.DB, tenantID int64, taskID uuid.UUID, key string) (*PlatformDraft, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrNotFound
	}
	var draft PlatformDraft
	err := tx.Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", tenantID, taskID, key).
		First(&draft).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func findApprovalByIdempotency(tx *gorm.DB, tenantID int64, taskID uuid.UUID, key string) (*ApprovalRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrNotFound
	}
	var record ApprovalRecord
	err := tx.Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", tenantID, taskID, key).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &record, nil
}

func handleDraftCreateError(err error, tx *gorm.DB, in DraftVersionInput, hash string, out *PlatformDraft) error {
	if !isUniqueViolation(err) {
		return stableError(err, ErrConflict)
	}
	if existing, lookupErr := findDraftByIdempotency(tx, in.TenantID, in.OperationTaskID, in.IdempotencyKey); lookupErr == nil {
		if existing.PayloadHash != hash {
			return ErrDuplicateRequest
		}
		*out = *existing
		return nil
	}
	return ErrDuplicateDraftVersion
}

func adapterModeForPlatform(platform string) string {
	switch strings.TrimSpace(strings.ToLower(platform)) {
	case PlatformDouyin:
		return AdapterModeSandbox
	default:
		return AdapterModeLocalDraftOnly
	}
}

func eventTypeForStatus(status string) string {
	switch normalizeTaskStatusValue(status) {
	case OperationTaskStatusPendingReview:
		return OperationTaskEventTypeReviewRequested
	case OperationTaskStatusApproved:
		return OperationTaskEventTypeApproved
	case OperationTaskStatusRejected:
		return OperationTaskEventTypeRejected
	case OperationTaskStatusExecutionQueued:
		return OperationTaskEventTypeExecutionQueued
	case OperationTaskStatusExecuting:
		return OperationTaskEventTypeExecutionStarted
	case OperationTaskStatusDraftWritten:
		return OperationTaskEventTypeDraftWritten
	case OperationTaskStatusExecutionFailed:
		return OperationTaskEventTypeExecutionFailed
	case OperationTaskStatusCancelled:
		return OperationTaskEventTypeCancelled
	default:
		return OperationTaskEventTypeDraftGenerated
	}
}

func normalizeActorType(actorType string) string {
	actorType = strings.TrimSpace(strings.ToLower(actorType))
	if actorType == "" {
		return OperationTaskEventActorSystem
	}
	return actorType
}
