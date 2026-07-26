# P8 Task Batch 5 Platform Draft Adapters

Status: **completed**

```text
batchId=P8-TASK-BATCH-5
baseBranch=dev
phase=P8
phaseStatus=In Progress
productionReady=false
changesCommitted=false
checkpointCreated=false
```

## Scope

Completed tasks:

- `P8-301` Platform Draft Interface
- `P8-302` Local Draft Adapter
- `P8-303` Douyin Mock/Sandbox Adapter
- `P8-304` Unsupported Platform Guard
- `P8-305` Automatic Publish Guard

Not implemented in this batch:

- Real Douyin API client or OAuth
- Real access tokens, refresh tokens, shop binding, or platform credentials
- Real platform draft writes, publish, listing, or live product updates
- HTTP handlers, routers, REST APIs, Admin UI, frontend requests, or background workers
- P8 permission/audit final integration tasks and all P8 API/Admin UI tasks

## Adapter Boundary

Batch 5 reuses `DraftExecutionPort` from `backend/internal/modules/operationtask/execution_services.go`. No parallel `PlatformDraftAdapter` interface is introduced.

Implemented in `backend/internal/modules/operationtask/platform_draft_adapters.go`:

- `DraftAdapterCapabilities`
- `PlatformDraftAdapterRegistry`
- `LocalDraftAdapter`
- `DouyinDraftFixtureAdapter`
- `UnsupportedPlatformGuard`
- `AutomaticPublishGuard`
- `CredentialAbsenceGuard`

Allowed adapter outcomes remain non-production only:

```text
local_draft
mock_draft
sandbox_fixture
```

Reference prefixes are explicit and non-production:

```text
local:
mock:douyin:
sandbox:douyin:
```

## Capabilities

All Batch 5 adapters declare:

```text
DraftCreation=true
Publish=false
Listing=false
NetworkAccess=false
RealCredentials=false
AutomaticExecution=false
```

Unknown, omitted, or unsafe capabilities are not allowed. Registry registration rejects production, real-write, auto-publish, auto-listing, duplicate, nil, and unsafe-capability registrations.

## Guard Order

Before adapter delegation, registry execution applies:

1. Platform validation
2. Adapter mode validation
3. Capability validation
4. Credential absence validation
5. Automatic publish/listing guard
6. Registry resolution
7. Payload validation

Dangerous runtime configuration is blocked with `production_capability_forbidden`:

```text
AUTO_PUBLISH=true
AUTO_LISTING=true
REAL_PLATFORM_WRITE=true
DOUYIN_REAL_WRITE=true
PRODUCTION_ADAPTER=true
```

Credential-like payload fields are blocked with `real_credentials_forbidden`:

```text
access_token
refresh_token
authorization
cookie
password
secret
client_secret
app_secret
```

```text
credentialPresenceGuardImplemented=true
fullSecretRedactionDeferredTo=P8-404
```

## Douyin Mock/Sandbox

The Douyin fixture adapter is offline only. It validates the minimal draft payload shape for:

```text
title
description
category
price
inventory
media
```

Supported deterministic fixture scenarios:

```text
success
validation_rejected
adapter_unavailable
provider_timeout
provider_rejected
context_cancelled
```

These scenarios are configured by tests/fixture adapter construction, not by arbitrary user payload fields.

## Success State Semantics

`draft_written` means only that a local draft or mock/sandbox fixture draft result was generated and finalized by the P8 execution orchestrator.

```text
draft_written != published
draft_written != listed
draft_written != live
draft_written != production write
```

No Batch 5 code treats `draft_written` as publish/listing/live state.

## Validation Evidence

```text
adapterContractTestsPassed=true
localAdapterTestsPassed=true
douyinMockSandboxTestsPassed=true
unsupportedPlatformGuardTestsPassed=true
automaticPublishGuardTestsPassed=true
networkIsolationTestsPassed=true
idempotencyTestsPassed=true
concurrencyTestsPassed=true
orchestratorIntegrationTestsPassed=true
racePassed=true
dataRaces=0
```

Network isolation is covered by construction and source-level regression checks:

```text
networkClientDependencyPresent=false
httpClientNotRequired=true
dnsResolutionNotAttempted=true
networkCallCount=0
```

## Boundary

```text
apiImplemented=false
adminUiImplemented=false
backgroundWorkerImplemented=false
productionPlatformAdapterImplemented=false
realDouyinApiImplemented=false
oauthImplemented=false
networkAccessEnabled=false
realCredentialsEnabled=false
realPlatformWriteImplemented=false
automaticPublishImplemented=false
automaticListingImplemented=false
humanConfirmationRequired=true
productionReady=false
```

P8 remains **In Progress**. P7 deferred performance and P10 production boundary remain preserved.
