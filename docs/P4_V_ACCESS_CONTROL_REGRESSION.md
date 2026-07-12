# P4-V Access Control Regression

Automated insecure direct object reference (IDOR) and shop authorization scope regression results for Phase P4-V closure.

## Status Banner

**55 IDOR Cases PASS** · **21 Shop Scope Cases PASS** · **SQLite In-Memory** · **NOT Penetration Test**

---

## Execution Summary

| Suite | Cases | Package path | Command | Result | Exit code |
| --- | --- | --- | --- | --- | --- |
| IDOR | **55** | `backend/internal/securitytests/idor/` | `go test ./internal/securitytests/idor/... -count=1` | **PASS** | 0 |
| Shop Scope | **21** | `backend/internal/securitytests/shopscope/` | `go test ./internal/securitytests/shopscope/... -count=1` | **PASS** | 0 |

Test DB: in-memory SQLite per test. Tenants: `tenantA=1001`, `tenantB=2002` (IDOR); single tenant `42` with shops A/B (shop scope).

---

## IDOR Suite (55 cases)

### Test files

| File | Cases | Focus |
| --- | --- | --- |
| `idor_test.go` | 22 | Product, order, shop, file CRUD + context |
| `idor_repository_test.go` | 9 | Repository `FindByID` cross-tenant denial |
| `idor_tenant_list_test.go` | 7 | List endpoints exclude other tenants |
| `idor_p42_modules_test.go` | 12 | Export, op log, webhook, file scan, security rotation |
| `idor_taskcenter_test.go` | 5 | Task center alerts, failure marks, collect tasks |

### Product (6)

| Test | Assertion |
| --- | --- |
| `TestIDOR_ProductGetCrossTenant` | Tenant A cannot GET tenant B product |
| `TestIDOR_ProductUpdateCrossTenant` | Cross-tenant update denied |
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
| `TestIDOR_OrderUpdateCrossTenantNoMutation` | Update denied, no mutation |

### Shop (4)

| Test | Assertion |
| --- | --- |
| `TestIDOR_ShopGetCrossTenant` | Cross-tenant get denied |
| `TestIDOR_ShopUpdateCrossTenant` | Update denied |
| `TestIDOR_ShopListTenantScoped` | List scoped |
| `TestIDOR_ShopDeleteCrossTenant` | Delete denied |

### Files (7)

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

### Repository FindByID (8)

| Test | Assertion |
| --- | --- |
| `TestIDOR_InventoryTaskFindByIDCrossTenant` | Inventory sync task denied |
| `TestIDOR_OrderSyncTaskFindByIDCrossTenant` | Order sync task denied |
| `TestIDOR_ProductPublishTaskFindByIDCrossTenant` | Publish task denied |
| `TestIDOR_AITextBatchFindByIDCrossTenant` | AI text batch denied |
| `TestIDOR_AIImageBatchFindByIDCrossTenant` | AI image batch denied |
| `TestIDOR_CustomerConversationFindByIDCrossTenant` | Conversation denied |
| `TestIDOR_WebhookEventFindByIDCrossTenant` | Webhook event denied |
| `TestIDOR_ExportJobFindByIDCrossTenant` | Export job denied |
| `TestIDOR_SameTenantCanLoadOwnRow` | Positive control — same tenant succeeds |

### Tenant list scoping (7)

| Test | Assertion |
| --- | --- |
| `TestIDOR_InventoryListTasksExcludesOtherTenant` | Inventory list scoped |
| `TestIDOR_OrderSyncListExcludesOtherTenant` | Order sync list scoped |
| `TestIDOR_ProductPublishListExcludesOtherTenant` | Publish list scoped |
| `TestIDOR_AITextBatchListExcludesOtherTenant` | AI text batch list scoped |
| `TestIDOR_AIImageBatchListExcludesOtherTenant` | AI image batch list scoped |
| `TestIDOR_CustomerChatListExcludesOtherTenant` | Customer chat list scoped |
| `TestIDOR_ExportServiceListNoCrossTenantLeak` | Export list scoped |

### P4.2 extended modules (12)

| Test | Assertion |
| --- | --- |
| `TestIDOR_ExportGetCrossTenant` | Export get denied |
| `TestIDOR_ExportListTenantScoped` | Export list scoped |
| `TestIDOR_ExportCreateStampsTenant` | Export create stamps tenant |
| `TestIDOR_OpLogListTenantScoped` | Op log list scoped |
| `TestIDOR_OpLogCrossTenantNotInList` | Other tenant logs excluded |
| `TestIDOR_FileDeleteCrossTenantViaDelete` | File delete via service denied |
| `TestIDOR_WebhookEventListExcludesOtherTenant` | Webhook list scoped |
| `TestIDOR_FileScanCrossTenantLoadDenied` | Scan worker load denied |
| `TestIDOR_TaskTenantRequireZeroDenied` | `tenant_id=0` rejected |
| `TestIDOR_TaskTenantResourceMismatchDenied` | Resource/tenant mismatch denied |
| `TestIDOR_SecurityRotationFindByIDCrossTenant` | Rotation job cross-tenant denied |
| `TestIDOR_SecurityRotationNoLeakOnDeniedLoad` | No metadata leak on denial |

### Task center (5)

| Test | Assertion |
| --- | --- |
| `TestIDOR_TaskCenterAlertFindByIDCrossTenant` | Alert get denied |
| `TestIDOR_TaskCenterFailureMarkFindByIDCrossTenant` | Failure mark get denied |
| `TestIDOR_TaskCenterCollectTaskScopedDenied` | Collect task cross-tenant denied |
| `TestIDOR_TaskCenterAlertScopedListExcludesOtherTenant` | Alert list scoped |
| `TestIDOR_TaskCenterFailureMarkScopedListExcludesOtherTenant` | Failure mark list scoped |

---

## Shop Scope Suite (21 cases)

### Test files

| File | Cases | Focus |
| --- | --- | --- |
| `shop_scope_test.go` | 5 | Core shop CRUD grants |
| `shopscope_extended_test.go` | 16 | Order, export, customer chat, product publish, op log |

### Core shop grants (5)

| Test | Role | Grants | Expected |
| --- | --- | --- | --- |
| `TestShopScope_OperatorCannotReadOtherShop` | operator | shop A only | Cannot `GetDetail` shop B |
| `TestShopScope_OperatorCannotUpdateOtherShop` | operator | shop A only | Cannot `Update` shop B |
| `TestShopScope_OperatorListOnlyOwnShops` | operator | shop A only | Can access shop A |
| `TestShopScope_ReadonlyCannotUpdateShop` | readonly | shop A | Update denied |
| `TestShopScope_AdminCanAccessAllShopsInTenant` | admin | all | Can access shop B |

### Extended module grants (16)

| Test | Module | Assertion |
| --- | --- | --- |
| `TestShopScope_ProductPublishListOnlyGrantedShops` | productpublish | Operator sees only granted shops |
| `TestShopScope_ExportCreateOtherShopDenied` | exportmod | Cannot create export for ungranted shop |
| `TestShopScope_ExportGetOtherShopDenied` | exportmod | Cannot get other shop's export |
| `TestShopScope_ExportListOnlyGrantedShops` | exportmod | List filtered by grants |
| `TestShopScope_CustomerChatGetOtherShopDenied` | customerchat | Get denied for other shop |
| `TestShopScope_CustomerChatListOnlyGrantedShops` | customerchat | List filtered |
| `TestShopScope_OrderGetOtherShopDenied` | order | Get denied |
| `TestShopScope_OrderListOnlyGrantedShops` | order | List filtered |
| `TestShopScope_OperatorMultiShopCanAccessBoth` | shop | Multi-grant operator OK |
| `TestShopScope_OperatorNoGrantsCannotGetShop` | shop | No grants → denied |
| `TestShopScope_ReadonlyCanGetCannotUpdate` | shop | Read OK, write denied |
| `TestShopScope_AdminCanAccessExportOtherShop` | exportmod | Admin sees all shops |
| `TestShopScope_OperatorCanCreateExportOwnShop` | exportmod | Own shop export OK |
| `TestShopScope_OpLogListStoreScoped` | operationlog | Op log filtered by store grant |
| `TestShopScope_AdminCanUpdateAnyShopInTenant` | shop | Admin update any shop |
| `TestShopScope_ExportAdminListSeesAllShops` | exportmod | Admin list unfiltered by grant |

### Enforcement layer

| Layer | Path | Mechanism |
| --- | --- | --- |
| Principal / grants | `backend/internal/pkg/adminperm/` | Store grants on Gin context |
| Shop service | `backend/internal/modules/shop/service.go` | `EnsureStoreVisible` / operate checks |
| Order service | `backend/internal/modules/order/service.go` | `ApplyStoreScope` on list paths |
| Export / customer / publish | Respective `service.go` / `service_queries.go` | Shop grant filters |

---

## P4-V Closure Targets vs P4.2 Baseline

| Metric | P4.2 baseline | P4-V target | P4-V actual |
| --- | --- | --- | --- |
| IDOR automated cases | 22 | 55 | **55 PASS** |
| Shop scope cases | 5 | 21 | **21 PASS** |
| Gate script threshold | warn < 40 | pass ≥ 55 / ≥ 21 | **pass** |

---

## Regression Commands

```bash
cd backend

# Full IDOR regression
go test ./internal/securitytests/idor/... -count=1 -v

# Full shop scope regression
go test ./internal/securitytests/shopscope/... -count=1 -v

# P4-V gate (counts cases statically)
node ../scripts/p4-v-security-closure-gate.mjs
```

---

## Known Limits

| Limit | Notes |
| --- | --- |
| Not a penetration test | SQLite in-memory; no network, no auth bypass fuzzing |
| No production data | Synthetic tenants and UUIDs only |
| Collect task HTTP paths | Partially covered via taskcenter; full HTTP matrix in `docs/P4_IDOR_TEST_MATRIX.md` |
| Settings list tenant filter | Not in IDOR suite; settings are admin-global per tenant design |

---

## Conclusion

**P4-V access control regression: PASS.**

All 55 IDOR and 21 shop scope automated cases pass. Coverage includes P4-V tenant-scope fixes for inventory, ordersync, productpublish, customerchat, and taskcenter. Results support P4-V security closure at the automated regression level.
