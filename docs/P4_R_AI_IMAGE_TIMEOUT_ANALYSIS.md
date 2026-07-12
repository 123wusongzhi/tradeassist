# Phase P4-R AI Image Timeout Analysis

## Finding

The previous AI image trial reported a 600 second timeout without enough evidence to determine whether it was provider timeout, worker stuck, lease loss, storage/file scan blocking, polling bug, or state transition bug.

## Fix

`scripts/ai-image-trial-run.ps1` now records:

- `finalStatus`
- `reasonCode`
- `lastCompletedStage`
- `providerStatus`
- batch/item status
- sanitized `lastErrorCode`

## Categories

| Category | Evidence |
| --- | --- |
| `environment_blocked` | Provider not configured, key missing, settings missing |
| `provider_timeout` | timeout/deadline evidence |
| `provider_rate_limited` | 429, quota, rate limit evidence |
| `provider_failed` | provider returned failed result |
| `worker_not_running` | task remains pending/running with no progress |
| `worker_stuck` | batch has running/pending items at timeout |
| `task_lease_lost` | lease error evidence |
| `file_scan_blocked` | scan/security evidence |
| `storage_blocked` | upload/storage/object evidence |
| `polling_bug` | polling evidence |
| `state_transition_bug` | terminal task but batch/item not advanced |
| `unknown_result` | provider result cannot be confirmed |

## Trial Status Model

Trial final status is one of:

- `passed`
- `passed_with_warning`
- `environment_blocked`
- `code_failed`
- `manual_required`

Timeouts are not automatically classified as environment blocked.
