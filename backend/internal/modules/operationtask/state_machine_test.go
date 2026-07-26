package operationtask_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
)

func TestTaskStateMachineCanonicalTransitions(t *testing.T) {
	sm := operationtask.NewTaskStateMachine()
	allowed := map[string][]string{
		operationtask.OperationTaskStatusSuggested: {
			operationtask.OperationTaskStatusDraftPreparing,
			operationtask.OperationTaskStatusCancelled,
		},
		operationtask.OperationTaskStatusDraftPreparing: {
			operationtask.OperationTaskStatusPendingReview,
			operationtask.OperationTaskStatusCancelled,
		},
		operationtask.OperationTaskStatusPendingReview: {
			operationtask.OperationTaskStatusApproved,
			operationtask.OperationTaskStatusRejected,
			operationtask.OperationTaskStatusCancelled,
		},
		operationtask.OperationTaskStatusApproved: {
			operationtask.OperationTaskStatusPendingReview,
			operationtask.OperationTaskStatusExecutionQueued,
			operationtask.OperationTaskStatusCancelled,
		},
		operationtask.OperationTaskStatusExecutionQueued: {
			operationtask.OperationTaskStatusExecuting,
			operationtask.OperationTaskStatusCancelled,
		},
		operationtask.OperationTaskStatusExecuting: {
			operationtask.OperationTaskStatusDraftWritten,
			operationtask.OperationTaskStatusExecutionFailed,
		},
		operationtask.OperationTaskStatusExecutionFailed: {
			operationtask.OperationTaskStatusExecutionQueued,
			operationtask.OperationTaskStatusCancelled,
		},
	}

	for from, tos := range allowed {
		for _, to := range tos {
			require.True(t, sm.CanTransition(from, to), "%s -> %s should be allowed", from, to)
			require.NoError(t, sm.ValidateTransition(from, to))
		}
	}
}

func TestTaskStateMachineRejectsUndeclaredAndSameStatusTransitions(t *testing.T) {
	sm := operationtask.NewTaskStateMachine()
	statuses := []string{
		operationtask.OperationTaskStatusSuggested,
		operationtask.OperationTaskStatusDraftPreparing,
		operationtask.OperationTaskStatusPendingReview,
		operationtask.OperationTaskStatusApproved,
		operationtask.OperationTaskStatusRejected,
		operationtask.OperationTaskStatusExecutionQueued,
		operationtask.OperationTaskStatusExecuting,
		operationtask.OperationTaskStatusDraftWritten,
		operationtask.OperationTaskStatusExecutionFailed,
		operationtask.OperationTaskStatusCancelled,
	}
	for _, from := range statuses {
		require.ErrorIs(t, sm.ValidateTransition(from, from), operationtask.ErrInvalidTransition)
		for _, to := range statuses {
			if from == to || sm.CanTransition(from, to) {
				continue
			}
			require.ErrorIs(t, sm.ValidateTransition(from, to), operationtask.ErrInvalidTransition, "%s -> %s", from, to)
		}
	}
	require.True(t, errors.Is(sm.ValidateTransition("missing", operationtask.OperationTaskStatusApproved), operationtask.ErrValidation))
}

func TestTaskStateMachineTerminalApprovalAndExecutionPredicates(t *testing.T) {
	sm := operationtask.NewTaskStateMachine()
	require.True(t, sm.IsTerminal(operationtask.OperationTaskStatusRejected))
	require.True(t, sm.IsTerminal(operationtask.OperationTaskStatusDraftWritten))
	require.True(t, sm.IsTerminal(operationtask.OperationTaskStatusCancelled))
	require.False(t, sm.IsTerminal(operationtask.OperationTaskStatusApproved))

	require.True(t, sm.RequiresApproval(operationtask.OperationTaskStatusPendingReview))
	require.False(t, sm.RequiresApproval(operationtask.OperationTaskStatusApproved))

	require.True(t, sm.CanExecute(operationtask.OperationTaskStatusApproved))
	require.False(t, sm.CanExecute(operationtask.OperationTaskStatusPendingReview))
	require.False(t, sm.CanExecute(operationtask.OperationTaskStatusRejected))
	require.False(t, sm.CanExecute(operationtask.OperationTaskStatusCancelled))
	require.False(t, sm.CanExecute(operationtask.OperationTaskStatusDraftWritten))
}
