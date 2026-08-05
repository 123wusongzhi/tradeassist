# P7-V2 R3B Pre-Formal Cleanup V2 Repair

Status: passed.

| Field | Value |
| --- | --- |
| Cleanup contract | 2 |
| Old listener PID | 9837 |
| Old current run | `p7v2-current-r3b-recovery6-20260717051755` |
| Old current DB | `trademind_p7v2_p7v2_current_r3b_recovery6_20260717051755_restar` |
| Stale process ownership verified | true |
| Old current DB ownership verified | true |
| Unknown DBs before | 4 |
| Unknown DBs after | 0 |
| Diagnostic DBs classified | 1 |
| Diagnostic DBs retained | 1 |
| Diagnostic DBs deleted | 0 |
| Previous unclassified delete incident recorded | true |
| Action history preserved | true |
| Check does not overwrite execute | true |
| Current formal residuals | 0 |
| Unknown processes | 0 |
| Unknown connections | 0 |
| Listener 18080 count | 0 |

Cleanup V2 now writes each attempt to `artifacts/p7-v2/cleanup-attempts/<cleanupAttemptId>.json`. The canonical cleanup report is only an index and summary of the latest attempt, so later check-only runs do not erase prior execute evidence.
