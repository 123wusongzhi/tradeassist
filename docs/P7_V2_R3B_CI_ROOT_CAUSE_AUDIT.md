# P7-V2-R3B-CI-RG Root Cause Audit

Root-cause classification: **process_identity_insufficient**

The blocked evidence recorded the same PID (`1100`) before and after the start path, but did not verify whether that PID represented the same Linux process. No start ticks, executable hash, port owner, old-process termination, or instance nonce was recorded. The previous `apiProcessChanged=false` result is therefore not a valid process-identity conclusion.

The worker is embedded in the API, Redis isolation uses `FLUSHALL`, and the provider path is an in-memory mock. Their evidence must be topology-specific rather than process-restart booleans.
