# P7-V2-R3B Recovery6 Consumed Plan Closeout

Status: **blocked**

- Old plan checkpoint: `c182977694a616ddd263c42e7089a88fb5093c9c`
- Old runtime freeze ID: `169a84ff16ecb11cfed96d434497ce8c52390d6e44035a8721d343f5515fbf43`
- Old plan status: `blocked`
- Old runtime freeze status: `superseded`
- Old run IDs consumed: `true`
- Valid for formal execution: `false`
- Valid for comparability: `false`
- Valid for regression: `false`
- Valid for closure: `false`

Root cause: `no_formal_same_run_retry_transition_for_prestart_database_residue`

The failed baseline prestart database residue was owned by the consumed plan and had no dataset, k6, raw artifact, or frozen artifact evidence. The only deleted database was:

`trademind_p7v2_p7v2_baseline_r3b_recovery6_20260716063639`

Cleanup result:

- Current failed plan database deleted count: `1`
- Historical database deleted count: `0`
- Unknown database deleted count: `0`
- Unknown connection terminated count: `0`
- Listener 18080 count: `0`

This closeout is historical evidence only. It is not a P7 pass, not Production Ready, and does not authorize reuse of the consumed Run IDs or runtime freeze.
