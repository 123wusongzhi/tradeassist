# P7-V2-R3B Runtime Freeze Identity Collision Closeout

Status: **blocked**

- Semantic gate passed: `false`
- Classification: `runtime_freeze_identity_collision`
- Old runtime freeze ID: `169a84ff16ecb11cfed96d434497ce8c52390d6e44035a8721d343f5515fbf43`
- Generated runtime freeze ID: `169a84ff16ecb11cfed96d434497ce8c52390d6e44035a8721d343f5515fbf43`
- Old runtime freeze ID reused: `true`
- Runtime freeze identity version: `1`
- Identity V1 missing plan binding: `true`

Root cause:

- `plan_checkpoint_not_bound`
- `planned_run_ids_not_bound`
- `plan_manifest_identity_not_bound`
- `canonical_identity_payload_incomplete`

The V1 identity payload included runtime content fingerprints but excluded formal plan identity. As a result, the old plan checkpoint and the fresh plan checkpoint could produce the same runtime freeze ID.

Plan binding hashes under Identity V2:

- Old plan binding hash: `b10a37286da24cc61e8c1fe39dac0c61c458b782af1124fe0ab0b0c27ed26024`
- New plan binding hash: `8efb6cd90a17748ba3996ea54223aaef514b30d649e73e4126a5a6ac40b2c666`

Formal resources created: `false`

- Baseline started: `false`
- Dataset executed: `false`
- k6 executed: `false`
- Artifact created: `false`
- Fresh Run IDs consumed: `false`
- New Run IDs required: `false`

This closeout preserves the collision failure evidence and does not authorize reuse of the old runtime freeze.
