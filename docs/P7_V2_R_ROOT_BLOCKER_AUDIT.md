# P7-V2-R Root Blocker Audit

## Root Blocker

```text
rootBlocker=k6_unavailable
```

## Dependent Failures

```text
dependentFailures=[
  load,
  baseline,
  current,
  regression,
  soak,
  demo,
  final_gates
]
```

## Prior State

P7-V2 harness was ready (host guard, medium dataset, runtime cleanup, Linux race) but formal load verification was blocked because k6 was unavailable.

Prior failed provisioning attempts:

- GitHub release download via wget/curl (network timeout)
- `apt install k6` (not in sources)
- `docker pull grafana/k6:0.57.0` (registry timeout)

## Resolution (P7-V2-R)

k6 v0.57.0 was provisioned via `go install go.k6.io/k6@v0.57.0` using `GOPROXY=https://goproxy.cn,direct`, then atomically installed to `tools/k6/k6`.

Discovery order now enforced by `scripts/p7-v2-k6-discovery.mjs`:

1. `P7_K6_BIN`
2. `tools/k6/k6`
3. PATH `k6`
4. Local Docker image `grafana/k6:0.57.0`

See `docs/p7-v2-r-k6-provisioning-report.json` for path, version, and SHA256.
