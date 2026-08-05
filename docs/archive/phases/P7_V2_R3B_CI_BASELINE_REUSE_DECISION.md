# P7-V2-R3B-CI-RG Baseline Reuse Decision

Decision: **rebaseline_required**

The repair does not change backend API runtime, k6 request generation, metric collection, dataset, SLO, or credentials. It does change `scripts/p7-v2-*.mjs`. The existing `runtimeSourceTreeHash` contract includes those scripts, while the immutable recovery Baseline records the pre-repair hash.

Because final comparability requires exact `runtimeSourceTreeHash` equality and the frozen Baseline cannot be changed, pairing it with a post-repair Current would be non-comparable. Per the scoped rules, no Current run, artifact freeze, comparability, or Regression V2 may execute.
