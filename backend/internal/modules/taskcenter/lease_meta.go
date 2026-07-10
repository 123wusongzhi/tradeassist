package taskcenter

import "time"

type taskLeaseMeta struct {
	HeartbeatAt  *time.Time
	ExecutionID  *string
	LeaseVersion int
}

func applyLeaseMeta(dto *UnifiedTaskDTO, meta taskLeaseMeta) {
	if dto == nil {
		return
	}
	dto.HeartbeatAt = meta.HeartbeatAt
	dto.LeaseVersion = meta.LeaseVersion
	if meta.ExecutionID != nil {
		dto.ExecutionID = *meta.ExecutionID
	}
}

func applyUnknownResult(dto *UnifiedTaskDTO, errorCode string) {
	if dto == nil {
		return
	}
	switch errorCode {
	case "CUSTOMER_MESSAGE_UNKNOWN_RESULT", "INVENTORY_PUSH_UNKNOWN_RESULT", "PUBLISH_UNKNOWN_RESULT":
		dto.UnknownResult = true
		dto.ManualReviewRequired = true
		dto.SafeRetry = false
	}
}
