package taskcenter

// sourcePriority returns a fixed deterministic ordering when sort times tie.
// Lower values sort earlier in DESC merge (higher precedence).
func sourcePriority(taskType string) int {
	switch taskType {
	case TaskTypeCollect:
		return 1
	case TaskTypeImage:
		return 2
	case TaskTypeOrderSync:
		return 3
	case TaskTypeCustomerMessageSync:
		return 4
	case TaskTypeProductPublish:
		return 5
	case TaskTypeInventorySync:
		return 6
	case TaskTypeAIText:
		return 7
	case TaskTypeAIImage:
		return 8
	case TaskTypeCustomerFailure:
		return 9
	default:
		return 99
	}
}

// allowedSourceIDs is the fixed whitelist for merge cursor sources.
var allowedSourceIDs = map[string]struct{}{
	TaskTypeCollect:             {},
	TaskTypeImage:               {},
	TaskTypeOrderSync:           {},
	TaskTypeCustomerMessageSync: {},
	TaskTypeProductPublish:      {},
	TaskTypeInventorySync:       {},
	TaskTypeAIText:              {},
	TaskTypeAIImage:             {},
	TaskTypeCustomerFailure:     {},
}
