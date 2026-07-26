package operationtask

import "strings"

type TaskStateMachine struct {
	transitions map[string]map[string]bool
}

func NewTaskStateMachine() *TaskStateMachine {
	return &TaskStateMachine{transitions: canonicalTaskTransitions()}
}

func canonicalTaskTransitions() map[string]map[string]bool {
	return map[string]map[string]bool{
		OperationTaskStatusSuggested: {
			OperationTaskStatusDraftPreparing: true,
			OperationTaskStatusCancelled:      true,
		},
		OperationTaskStatusDraftPreparing: {
			OperationTaskStatusPendingReview: true,
			OperationTaskStatusCancelled:     true,
		},
		OperationTaskStatusPendingReview: {
			OperationTaskStatusApproved:  true,
			OperationTaskStatusRejected:  true,
			OperationTaskStatusCancelled: true,
		},
		OperationTaskStatusApproved: {
			OperationTaskStatusPendingReview:   true,
			OperationTaskStatusExecutionQueued: true,
			OperationTaskStatusCancelled:       true,
		},
		OperationTaskStatusExecutionQueued: {
			OperationTaskStatusExecuting: true,
			OperationTaskStatusCancelled: true,
		},
		OperationTaskStatusExecuting: {
			OperationTaskStatusDraftWritten:    true,
			OperationTaskStatusExecutionFailed: true,
		},
		OperationTaskStatusExecutionFailed: {
			OperationTaskStatusExecutionQueued: true,
			OperationTaskStatusCancelled:       true,
		},
	}
}

func (m *TaskStateMachine) CanTransition(from, to string) bool {
	from = normalizeTaskStatusValue(from)
	to = normalizeTaskStatusValue(to)
	if from == "" || to == "" || from == to {
		return false
	}
	if m == nil || m.transitions == nil {
		m = NewTaskStateMachine()
	}
	return m.transitions[from][to]
}

func (m *TaskStateMachine) ValidateTransition(from, to string) error {
	from = normalizeTaskStatusValue(from)
	to = normalizeTaskStatusValue(to)
	if !allowedOperationTaskStatuses[from] || !allowedOperationTaskStatuses[to] {
		return ErrValidation
	}
	if !m.CanTransition(from, to) {
		return ErrInvalidTransition
	}
	return nil
}

func (m *TaskStateMachine) IsTerminal(status string) bool {
	switch normalizeTaskStatusValue(status) {
	case OperationTaskStatusRejected, OperationTaskStatusDraftWritten, OperationTaskStatusCancelled:
		return true
	default:
		return false
	}
}

func (m *TaskStateMachine) RequiresApproval(status string) bool {
	return normalizeTaskStatusValue(status) == OperationTaskStatusPendingReview
}

func (m *TaskStateMachine) CanExecute(status string) bool {
	return normalizeTaskStatusValue(status) == OperationTaskStatusApproved
}

func (m *TaskStateMachine) AllowedTransitions() map[string][]string {
	if m == nil || m.transitions == nil {
		m = NewTaskStateMachine()
	}
	out := make(map[string][]string, len(m.transitions))
	for from, tos := range m.transitions {
		for to := range tos {
			out[from] = append(out[from], to)
		}
	}
	return out
}

func normalizeTaskStatusValue(status string) string {
	return strings.TrimSpace(strings.ToLower(status))
}
