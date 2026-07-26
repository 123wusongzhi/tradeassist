package operationtask_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type denyRetryAuthorizer struct {
	allowExecutionAuthorizer
}

func (denyRetryAuthorizer) CanRetry(context.Context, int64, uuid.UUID, uuid.UUID) error {
	return operationtask.ErrPermissionDenied
}

type secretMetadataPort struct{}

func (secretMetadataPort) ExecuteDraft(context.Context, operationtask.DraftExecutionInput) (operationtask.DraftExecutionResult, error) {
	return operationtask.DraftExecutionResult{
		ResultType:        "sandbox_fixture",
		ExternalReference: "fixture-secret-metadata",
		SafeMetadata:      datatypes.JSON([]byte(`{"provider":"fixture","accessToken":"raw-token","nested":{"cookie":"raw-cookie"}}`)),
	}, nil
}

func createAdminUser(t *testing.T, db *gorm.DB, tenantID int64, role string, status string) uuid.UUID {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	id := uuid.New()
	user := admin.AdminUser{
		TenantID:     tenantID,
		Username:     strings.ReplaceAll(id.String(), "-", ""),
		Email:        id.String() + "@example.test",
		PasswordHash: "hash",
		Role:         role,
		Status:       status,
	}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

func TestRBACAuthorizerStrictRolesTenantAndActor(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	adminID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	reviewerID := createAdminUser(t, db, tenantID, "reviewer", admin.StatusActive)
	operatorID := createAdminUser(t, db, tenantID, "operator", admin.StatusActive)
	readonlyID := createAdminUser(t, db, tenantID, "readonly", admin.StatusActive)
	unknownID := createAdminUser(t, db, tenantID, "surprise", admin.StatusActive)
	otherTenantID := createAdminUser(t, db, 202, admin.RoleAdmin, admin.StatusActive)
	disabledID := createAdminUser(t, db, tenantID, admin.RoleAdmin, "disabled")

	authz := operationtask.NewRBACAuthorizer(db)
	taskID := uuid.New()
	require.NoError(t, authz.CanReview(context.Background(), tenantID, adminID))
	require.NoError(t, authz.CanReview(context.Background(), tenantID, reviewerID))
	require.ErrorIs(t, authz.CanReview(context.Background(), tenantID, operatorID), operationtask.ErrPermissionDenied)
	require.ErrorIs(t, authz.CanReview(context.Background(), tenantID, readonlyID), operationtask.ErrPermissionDenied)
	require.ErrorIs(t, authz.CanReview(context.Background(), tenantID, unknownID), operationtask.ErrPermissionDenied)
	require.ErrorIs(t, authz.CanReview(context.Background(), tenantID, otherTenantID), operationtask.ErrPermissionDenied)
	require.ErrorIs(t, authz.CanReview(context.Background(), tenantID, disabledID), operationtask.ErrPermissionDenied)

	require.NoError(t, authz.CanCreate(context.Background(), tenantID, adminID))
	require.NoError(t, authz.CanCreate(context.Background(), tenantID, operatorID))
	require.ErrorIs(t, authz.CanCreate(context.Background(), tenantID, reviewerID), operationtask.ErrPermissionDenied)
	require.ErrorIs(t, authz.CanCreate(context.Background(), tenantID, readonlyID), operationtask.ErrPermissionDenied)
	require.NoError(t, authz.CanEdit(context.Background(), tenantID, operatorID, taskID))

	require.NoError(t, authz.CanExecute(context.Background(), tenantID, adminID, taskID))
	require.NoError(t, authz.CanExecute(context.Background(), tenantID, reviewerID, taskID))
	require.ErrorIs(t, authz.CanExecute(context.Background(), tenantID, operatorID, taskID), operationtask.ErrPermissionDenied)
	require.ErrorIs(t, authz.CanExecute(context.Background(), tenantID, readonlyID, taskID), operationtask.ErrPermissionDenied)
	require.NoError(t, authz.CanRetry(context.Background(), tenantID, reviewerID, taskID))
	require.ErrorIs(t, authz.CanRetry(context.Background(), tenantID, operatorID, taskID), operationtask.ErrPermissionDenied)
}

func TestManualRetryAuthorizerDeniesBeforeRetryExecution(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	port := newRecordingExecutionPort()
	port.err = &operationtask.ExecutionDomainError{
		Category:    operationtask.ExecutionErrorCategoryProviderTimeout,
		Code:        "provider_timeout",
		SafeMessage: "Sandbox fixture timed out",
		Retryable:   true,
	}
	allowOrch := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port)
	_, err := allowOrch.Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-retry-authz-seed",
		IdempotencyKey:  "idem-retry-authz-seed",
	})
	require.Error(t, err)

	deniedOrch := operationtask.NewExecutionOrchestrator(db, denyRetryAuthorizer{}, port)
	_, err = operationtask.NewManualRetryService(db, deniedOrch, 3).Retry(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-retry-authz-denied",
		IdempotencyKey:  "idem-retry-authz-denied",
	})
	require.ErrorIs(t, err, operationtask.ErrPermissionDenied)
	require.Equal(t, 1, port.callCount())
}

func TestAuditEventsRecordReasonAndRedactMetadata(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	out, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, secretMetadataPort{}).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-redacted-success",
		IdempotencyKey:  "idem-redacted-success",
	})
	require.NoError(t, err)
	require.NotContains(t, string(out.Attempt.SafeMetadata), "raw-token")
	require.Contains(t, string(out.Attempt.SafeMetadata), "redactedSensitiveField")

	event, err := operationtask.NewOperationTaskEventRepository(db).GetLatestByTask(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskEventTypeDraftWritten, event.EventType)
	require.NotEmpty(t, event.BeforeState)
	require.Equal(t, operationtask.OperationTaskStatusDraftWritten, event.AfterState)
	require.NotEmpty(t, event.RequestID)
	require.NotContains(t, string(event.Metadata), "raw-token")
	require.Contains(t, string(event.Metadata), "redactedSensitiveField")
}

func TestExecutionFailureDetailsAreRedactedNotDropped(t *testing.T) {
	classifier := operationtask.NewExecutionFailureClassifier()
	failure := classifier.Classify(&operationtask.ExecutionDomainError{
		Category:    operationtask.ExecutionErrorCategoryProviderRejected,
		Code:        "provider_rejected",
		SafeMessage: "Provider rejected sandbox draft",
		Details:     datatypes.JSON([]byte(`{"provider":"fixture","accessToken":"raw-token","httpStatus":400}`)),
	})
	require.NotContains(t, string(failure.Details), "raw-token")
	require.Contains(t, string(failure.Details), "provider")
	require.Contains(t, string(failure.Details), "httpStatus")
	require.Contains(t, string(failure.Details), "redactedSensitiveField")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(failure.Details, &decoded))
	require.Equal(t, "fixture", decoded["provider"])
}
