package operationtask_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const douyinFixturePayload = `{"title":"summer dress","description":"lightweight summer dress","category":"women_apparel","price":129.9,"inventory":20,"media":["asset:front"]}`

func draftAdapterInput(mode string) operationtask.DraftExecutionInput {
	return operationtask.DraftExecutionInput{
		TenantID:         101,
		OperationTaskID:  uuid.New(),
		PlatformDraftID:  uuid.New(),
		Platform:         operationtask.PlatformDouyin,
		AdapterMode:      mode,
		DraftVersion:     1,
		DraftPayloadHash: hash1,
		Payload:          datatypes.JSON([]byte(douyinFixturePayload)),
		RequestID:        "req-adapter-contract",
		IdempotencyKey:   "idem-adapter-contract",
		ActorID:          uuid.New(),
		AttemptNumber:    1,
	}
}

func localDraftAdapterInput() operationtask.DraftExecutionInput {
	input := draftAdapterInput(operationtask.ExecutionPortModeLocalDraftFixture)
	input.Platform = operationtask.PlatformLocal
	input.Payload = datatypes.JSON([]byte(`{"draft":{"title":"local safe draft"}}`))
	return input
}

func createApprovedDouyinExecutionTask(t *testing.T, db *gorm.DB) (operationtask.OperationTask, uuid.UUID) {
	t.Helper()
	actor := uuid.New()
	task := createOperationTask(t, db, operationtask.OperationTaskStatusSuggested)
	task = moveTaskToDraftPreparing(t, db, task, actor)
	draft, err := operationtask.NewDraftVersionService(db).CreateInitialDraft(context.Background(), operationtask.DraftVersionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		Payload:          datatypes.JSON([]byte(douyinFixturePayload)),
		ActorID:          &actor,
		RequestID:        uuid.NewString(),
		IdempotencyKey:   uuid.NewString(),
		ChangeReason:     "initial douyin fixture draft",
	})
	require.NoError(t, err)
	reviewer := uuid.New()
	latest, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	approval, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: latest.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "approved for douyin fixture execution",
		RequestID:        uuid.NewString(),
		IdempotencyKey:   uuid.NewString(),
	})
	require.NoError(t, err)
	approved, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.ApprovalDecisionApproved, approval.Decision)
	return *approved, actor
}

func requireDomainError(t *testing.T, err error, category string, code string) {
	t.Helper()
	var domain *operationtask.ExecutionDomainError
	require.ErrorAs(t, err, &domain)
	require.Equal(t, category, domain.Category)
	require.Equal(t, code, domain.Code)
}

func TestPlatformDraftAdapterContractCapabilitiesAndReferences(t *testing.T) {
	registry := operationtask.NewSafePlatformDraftAdapterRegistry()
	for _, tc := range []struct {
		name       string
		platform   string
		mode       string
		resultType string
		prefix     string
	}{
		{name: "local", platform: operationtask.PlatformLocal, mode: operationtask.ExecutionPortModeLocalDraftFixture, resultType: "local_draft", prefix: "local:"},
		{name: "douyin mock", platform: operationtask.PlatformDouyin, mode: operationtask.ExecutionPortModeMock, resultType: "mock_draft", prefix: "mock:douyin:"},
		{name: "douyin sandbox", platform: operationtask.PlatformDouyin, mode: operationtask.ExecutionPortModeSandboxFixture, resultType: "sandbox_fixture", prefix: "sandbox:douyin:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caps, ok := registry.Capabilities(tc.platform, tc.mode)
			require.True(t, ok)
			require.True(t, caps.DraftCreation)
			require.False(t, caps.Publish)
			require.False(t, caps.Listing)
			require.False(t, caps.NetworkAccess)
			require.False(t, caps.RealCredentials)
			require.False(t, caps.AutomaticExecution)

			input := draftAdapterInput(tc.mode)
			input.Platform = tc.platform
			if tc.platform == operationtask.PlatformLocal {
				input.Payload = datatypes.JSON([]byte(`{"draft":{"title":"local safe draft"}}`))
			}
			result, err := registry.ExecuteDraft(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, tc.resultType, result.ResultType)
			require.True(t, strings.HasPrefix(result.ExternalReference, tc.prefix))
			require.NotEmpty(t, result.SafeMetadata)
			require.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestPlatformDraftAdapterContractRejectsInvalidInputCredentialsAndContext(t *testing.T) {
	adapter := operationtask.NewLocalDraftAdapter()
	input := localDraftAdapterInput()
	badHash := input
	badHash.DraftPayloadHash = "not-a-sha"
	_, err := adapter.ExecuteDraft(context.Background(), badHash)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, operationtask.ErrCodeValidation)

	badJSON := input
	badJSON.Payload = datatypes.JSON([]byte(`{"draft":`))
	_, err = adapter.ExecuteDraft(context.Background(), badJSON)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, operationtask.ErrCodeValidation)

	secretPayload := input
	secretPayload.Payload = datatypes.JSON([]byte(`{"access_token":"forbidden"}`))
	_, err = adapter.ExecuteDraft(context.Background(), secretPayload)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryPermissionDenied, "real_credentials_forbidden")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = adapter.ExecuteDraft(ctx, input)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryProviderTimeout, "context_cancelled")
}

func TestLocalDraftAdapterIdempotencyAndModeSafety(t *testing.T) {
	adapter := operationtask.NewLocalDraftAdapter()
	input := localDraftAdapterInput()
	first, err := adapter.ExecuteDraft(context.Background(), input)
	require.NoError(t, err)
	second, err := adapter.ExecuteDraft(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first.ExternalReference, second.ExternalReference)

	conflict := input
	conflict.DraftPayloadHash = hash2
	_, err = adapter.ExecuteDraft(context.Background(), conflict)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryIdempotencyConflict, operationtask.ErrCodeIdemPayloadConflict)

	wrongMode := input
	wrongMode.AdapterMode = operationtask.ExecutionPortModeMock
	_, err = adapter.ExecuteDraft(context.Background(), wrongMode)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, "unsupported_adapter_mode")
}

func TestDouyinDraftFixtureAdapterScenariosAndValidation(t *testing.T) {
	mock := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeMock, operationtask.DraftFixtureScenarioSuccess)
	mockResult, err := mock.ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeMock))
	require.NoError(t, err)
	require.Equal(t, "mock_draft", mockResult.ResultType)
	require.True(t, strings.HasPrefix(mockResult.ExternalReference, "mock:douyin:"))

	sandbox := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeSandboxFixture, operationtask.DraftFixtureScenarioSuccess)
	sandboxResult, err := sandbox.ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeSandboxFixture))
	require.NoError(t, err)
	require.Equal(t, "sandbox_fixture", sandboxResult.ResultType)
	require.True(t, strings.HasPrefix(sandboxResult.ExternalReference, "sandbox:douyin:"))

	missingRequired := draftAdapterInput(operationtask.ExecutionPortModeMock)
	missingRequired.Payload = datatypes.JSON([]byte(`{"title":"missing fields"}`))
	_, err = mock.ExecuteDraft(context.Background(), missingRequired)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, operationtask.ErrCodeValidation)

	for _, tc := range []struct {
		scenario string
		category string
		code     string
	}{
		{operationtask.DraftFixtureScenarioValidationRejected, operationtask.ExecutionErrorCategoryValidation, "validation_rejected"},
		{operationtask.DraftFixtureScenarioAdapterUnavailable, operationtask.ExecutionErrorCategoryAdapterUnavailable, "adapter_unavailable"},
		{operationtask.DraftFixtureScenarioProviderTimeout, operationtask.ExecutionErrorCategoryProviderTimeout, "provider_timeout"},
		{operationtask.DraftFixtureScenarioProviderRejected, operationtask.ExecutionErrorCategoryProviderRejected, "provider_rejected"},
		{operationtask.DraftFixtureScenarioContextCancelled, operationtask.ExecutionErrorCategoryProviderTimeout, "context_cancelled"},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			adapter := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeMock, tc.scenario)
			_, err := adapter.ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeMock))
			requireDomainError(t, err, tc.category, tc.code)
		})
	}
}

func TestDouyinDraftFixtureAdapterIdempotencyConflictAndProductionMode(t *testing.T) {
	adapter := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeMock, operationtask.DraftFixtureScenarioSuccess)
	input := draftAdapterInput(operationtask.ExecutionPortModeMock)
	_, err := adapter.ExecuteDraft(context.Background(), input)
	require.NoError(t, err)
	conflict := input
	conflict.DraftPayloadHash = hash2
	_, err = adapter.ExecuteDraft(context.Background(), conflict)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryIdempotencyConflict, operationtask.ErrCodeIdemPayloadConflict)

	production := input
	production.AdapterMode = "production"
	_, err = adapter.ExecuteDraft(context.Background(), production)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, "unsupported_adapter_mode")
}

func TestUnsupportedPlatformGuardAndRegistryResolution(t *testing.T) {
	registry := operationtask.NewSafePlatformDraftAdapterRegistry()
	_, err := registry.ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeMock))
	require.NoError(t, err)
	_, err = registry.ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeSandboxFixture))
	require.NoError(t, err)

	knownLocal := localDraftAdapterInput()
	knownLocal.Platform = "amazon"
	result, err := registry.ExecuteDraft(context.Background(), knownLocal)
	require.NoError(t, err)
	require.Equal(t, "local_draft", result.ResultType)
	require.Contains(t, string(result.SafeMetadata), "local_draft_only_fallback")

	unknown := knownLocal
	unknown.Platform = "unknown-platform"
	_, err = registry.ExecuteDraft(context.Background(), unknown)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryAdapterUnavailable, "unsupported_platform")

	unsupportedMode := knownLocal
	unsupportedMode.AdapterMode = operationtask.ExecutionPortModeMock
	_, err = registry.ExecuteDraft(context.Background(), unsupportedMode)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, "unsupported_adapter_mode")
}

func TestAutomaticPublishGuardBlocksDangerousConfigAndPayloadBeforeAdapter(t *testing.T) {
	input := localDraftAdapterInput()
	input.Payload = datatypes.JSON([]byte(`{"draft":{"title":"local"},"auto_publish":true}`))
	err := operationtask.AutomaticPublishGuard(input)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryPermissionDenied, "production_capability_forbidden")

	for _, key := range []string{"AUTO_PUBLISH", "AUTO_LISTING", "REAL_PLATFORM_WRITE", "DOUYIN_REAL_WRITE", "PRODUCTION_ADAPTER"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "true")
			err := operationtask.AutomaticPublishGuard(localDraftAdapterInput())
			requireDomainError(t, err, operationtask.ExecutionErrorCategoryPermissionDenied, "production_capability_forbidden")
		})
	}

	for _, key := range []string{"AUTO_PUBLISH", "AUTO_LISTING", "REAL_PLATFORM_WRITE", "DOUYIN_REAL_WRITE", "PRODUCTION_ADAPTER"} {
		require.Empty(t, os.Getenv(key))
	}
}

func TestPlatformDraftAdapterRegistryRejectsUnsafeRegistration(t *testing.T) {
	registry := operationtask.NewSafePlatformDraftAdapterRegistry()
	unsafe := operationtask.SafeDraftCreationCapabilities()
	unsafe.Publish = true
	err := registry.Register("custom", operationtask.ExecutionPortModeLocalDraftFixture, operationtask.NewLocalDraftAdapter(), unsafe)
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryPermissionDenied, "production_capability_forbidden")

	err = registry.Register("custom", "production", operationtask.NewLocalDraftAdapter(), operationtask.SafeDraftCreationCapabilities())
	requireDomainError(t, err, operationtask.ExecutionErrorCategoryValidation, "unsupported_adapter_mode")
}

func TestPlatformDraftAdapterRegistryConcurrentCallsAreStable(t *testing.T) {
	registry := operationtask.NewSafePlatformDraftAdapterRegistry()
	input := draftAdapterInput(operationtask.ExecutionPortModeMock)
	var wg sync.WaitGroup
	results := make(chan string, 32)
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := registry.ExecuteDraft(context.Background(), input)
			if err != nil {
				errs <- err
				return
			}
			results <- result.ExternalReference
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	require.Empty(t, errs)
	var first string
	for ref := range results {
		if first == "" {
			first = ref
		}
		require.Equal(t, first, ref)
	}
}

func TestPlatformDraftAdaptersSourceHasNoNetworkClientDependency(t *testing.T) {
	data, err := os.ReadFile("platform_draft_adapters.go")
	require.NoError(t, err)
	source := strings.ToLower(string(data))
	for _, forbidden := range []string{"net/http", "http.client", "http.newrequest", "oauth", "grpc", "websocket"} {
		require.NotContains(t, source, forbidden)
	}
}

func TestExecutionOrchestratorWithSafeRegistryWritesDraftAndRejectsGuardFailure(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, actor := createApprovedDouyinExecutionTask(t, db)
	registry := operationtask.NewSafePlatformDraftAdapterRegistry()
	out, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, registry).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-registry-success",
		IdempotencyKey:  "idem-registry-success",
	})
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusSucceeded, out.Status)
	require.Equal(t, operationtask.OperationTaskStatusDraftWritten, mustTaskStatus(t, db, task.TenantID, task.ID))
	require.Equal(t, "sandbox_fixture", out.Attempt.ResultType)
	require.NotContains(t, out.Attempt.ResultType, "published")
	require.NotContains(t, out.Attempt.ResultType, "listed")
	require.NotContains(t, out.Attempt.ResultType, "live")

	blockedTask, _, _, blockedActor := createApprovedExecutionTask(t, db)
	t.Setenv("AUTO_PUBLISH", "true")
	blockedOut, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, registry).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        blockedTask.TenantID,
		OperationTaskID: blockedTask.ID,
		ActorID:         blockedActor,
		RequestID:       "req-registry-blocked",
		IdempotencyKey:  "idem-registry-blocked",
	})
	require.Error(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusFailedFinal, blockedOut.Status)
	require.Equal(t, operationtask.OperationTaskStatusExecutionFailed, mustTaskStatus(t, db, blockedTask.TenantID, blockedTask.ID))
	latestErr, latestErrLoad := operationtask.NewExecutionErrorRepository(db).GetLatestByAttempt(context.Background(), blockedTask.TenantID, blockedOut.Attempt.ID)
	require.NoError(t, latestErrLoad)
	require.Equal(t, "production_capability_forbidden", latestErr.Code)
}

func TestExecutionOrchestratorIdempotentReplayDoesNotRecallAdapter(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	port := newRecordingExecutionPort()
	_, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-replay",
		IdempotencyKey:  "idem-replay",
	})
	require.NoError(t, err)
	out, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-replay-again",
		IdempotencyKey:  "idem-replay",
	})
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusSucceeded, out.Status)
	require.Equal(t, 1, port.callCount())
}

func TestExecutionClassifierMapsDraftAdapterErrors(t *testing.T) {
	classifier := operationtask.NewExecutionFailureClassifier()
	_, timeoutErr := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeMock, operationtask.DraftFixtureScenarioProviderTimeout).ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeMock))
	failure := classifier.Classify(timeoutErr)
	require.Equal(t, operationtask.ExecutionErrorCategoryProviderTimeout, failure.Category)
	require.True(t, failure.Retryable)

	_, rejectedErr := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeMock, operationtask.DraftFixtureScenarioProviderRejected).ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeMock))
	failure = classifier.Classify(rejectedErr)
	require.Equal(t, operationtask.ExecutionErrorCategoryProviderRejected, failure.Category)
	require.False(t, failure.Retryable)

	_, unavailableErr := operationtask.NewDouyinDraftFixtureAdapter(operationtask.ExecutionPortModeMock, operationtask.DraftFixtureScenarioAdapterUnavailable).ExecuteDraft(context.Background(), draftAdapterInput(operationtask.ExecutionPortModeMock))
	failure = classifier.Classify(unavailableErr)
	require.Equal(t, operationtask.ExecutionErrorCategoryAdapterUnavailable, failure.Category)
}

func mustTaskStatus(t *testing.T, db *gorm.DB, tenantID int64, taskID uuid.UUID) string {
	t.Helper()
	task, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), tenantID, taskID)
	require.NoError(t, err)
	return task.Status
}
