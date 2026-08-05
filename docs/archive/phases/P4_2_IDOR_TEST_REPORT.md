# P4.2 IDOR Test Report

Automated insecure direct object reference regression tests (Phase P4.2).

## Status Banner

**22 Automated Cases Passing (SQLite)** · **Closure Target 40+ Not Yet Met** · **NOT Penetration Test Passed**

---

## Test package

- Path: `backend/internal/securitytests/idor/idor_test.go`
- Run: `go test ./internal/securitytests/idor/... -count=1`
- DB: in-memory SQLite per test
- Tenants: `tenantA=1001`, `tenantB=2002`

---

## Implemented cases (22)

### Product (6)

| Test | Assertion |
| --- | --- |
| `TestIDOR_ProductGetCrossTenant` | Tenant A cannot GET tenant B product |
| `TestIDOR_ProductUpdateCrossTenant` | Cross-tenant update denied, no mutation |
| `TestIDOR_ProductDeleteCrossTenant` | Cross-tenant delete denied |
| `TestIDOR_ProductListExcludesOtherTenant` | List returns only own tenant |
| `TestIDOR_ProductGetSameIDDifferentTenant` | Reverse direction denied |
| `TestIDOR_ProductCreateStampsTenant` | Body `tenantId` ignored; JWT tenant stamped |

### Order (4)

| Test | Assertion |
| --- | --- |
| `TestIDOR_OrderGetCrossTenant` | Cross-tenant get denied |
| `TestIDOR_OrderPeekCrossTenant` | Peek denied |
| `TestIDOR_OrderListTenantScoped` | List scoped to tenant |
| `TestIDOR_OrderUpdateCrossTenantNoMutation` | Update denied |

### Shop (4)

| Test | Assertion |
| --- | --- |
| `TestIDOR_ShopGetCrossTenant` | Cross-tenant get denied |
| `TestIDOR_ShopUpdateCrossTenant` | Update denied |
| `TestIDOR_ShopListTenantScoped` | List scoped |
| `TestIDOR_ShopDeleteCrossTenant` | Delete denied |

### Files (6)

| Test | Assertion |
| --- | --- |
| `TestIDOR_FileDeleteCrossTenant` | Delete denied |
| `TestIDOR_FileListTenantScoped` | List scoped |
| `TestIDOR_FileAccessCrossTenant` | Signed URL denied |
| `TestIDOR_FileDownloadLoadCrossTenant` | Load denied |
| `TestIDOR_FilePendingNotAccessible` | `pending_scan` blocked |
| `TestIDOR_FileQuarantinedNotAccessible` | `quarantined` blocked |
| `TestIDOR_CrossTenantObjectKeyPrefix` | assetId scope beats object key guess |

### Context (1)

| Test | Assertion |
| --- | --- |
| `TestIDOR_MissingTenantContext` | Missing tenant returns error |

---

## Closure gap (40+ target)

Manual matrix: `docs/P4_IDOR_TEST_MATRIX.md` (~35 planned cases).

**Not yet automated (priority backlog):**

- Collect tasks (`COL-01`, `COL-02`)
- Settings list tenant filter (`SET-02`)
- AI product sub-paths (`PRD-05`)
- Auth session cross-user revoke (partially covered in matrix, not in idor_test.go)
- Export jobs, taskcenter failure list, operation log cross-tenant

---

## Result

| Metric | Value |
| --- | --- |
| Automated cases | **22** |
| Closure target | **40+** |
| Static scan status | **warning** (below target, above minimum 20) |
| Last run environment | Local `go test` (recommended before closure PR) |

**Real environment IDOR penetration deferred to P10.**
