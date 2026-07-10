package tasklease

import "errors"

const ErrCodeLeaseLost = "TASK_LEASE_LOST"

var ErrLeaseLost = errors.New(ErrCodeLeaseLost)
