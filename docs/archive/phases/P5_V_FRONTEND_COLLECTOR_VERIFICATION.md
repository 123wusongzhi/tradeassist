# P5-V Frontend and Collector Verification

Phase: P5-V
Status: passed

## Required Commands

```text
pnpm check:dev
pnpm check:ui-copy --strict
pnpm build:admin
pnpm build:collector
```

## Required Result

- `check:dev`: passed
- `check:ui-copy --strict`: passed
- `build:admin`: passed
- `build:collector`: passed

The operations UI must keep real telemetry backend verification distinct from Mock Collector verification. Mock Collector success must not be shown as production collector active.

## Code-Level UI Update

`/ops/observability` now reads `runtimeStatus.otlpExporter`, `runtimeStatus.otlpProtocol`, `runtimeStatus.mockCollectorVerification`, and `telemetry` counters from the overview API. The page displays standard protocol readiness, simulated receiver verification, real backend deferred, export degraded, disabled, and incomplete as distinct states.
