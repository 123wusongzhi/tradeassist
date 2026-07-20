# P7-V2-R3B Failed Formal Pair Binary Provenance Closeout

Status: **passed**

This closeout preserves the historical Recovery6 formal pair as audit evidence only.

| Field | Value |
| --- | --- |
| Baseline run ID | `p7v2-baseline-r3b-recovery6-20260718001301` |
| Current run ID | `p7v2-current-r3b-recovery6-20260718001301` |
| Runtime freeze ID | `2af8b39b123a132b56b983ab4de0d4181771b794d817ff1f406b512fd927217d` |
| Regression status | `failed` |
| Binary provenance passed | `false` |
| Baseline runtime commit | `notRecoverable` |
| Current runtime commit | `notRecoverable` |
| Baseline binary SHA256 | `notRecoverable` |
| Current binary SHA256 | `notRecoverable` |
| Primary root cause | `C_formal_binary_provenance_defect` |
| Confidence | `medium` |

The historical pair proves the formal harness executed and produced a regression failure, but it is missing complete frozen binary provenance. It is therefore valid for historical audit and invalid for performance comparison, repeatability matrix use, closure, soak, or reuse.

The failed regression report and frozen historical artifacts must not be deleted or rewritten. Any cleanup must use Cleanup Contract V2 with exact run IDs and must preserve historical frozen artifacts and evidence.
