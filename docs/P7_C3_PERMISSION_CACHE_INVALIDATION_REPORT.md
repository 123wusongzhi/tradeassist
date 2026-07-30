# P7-C3 Permission Cache Invalidation Report

Status: Partial.

Implemented:

- Added a local versioned principal cache in `adminperm`.
- Cache key includes tenant, user, token/security version, status, and role.
- Each request reads current user version/status before using the cached principal.
- Disabled users become a no-permission principal and do not keep old read/write permissions.
- Added `InvalidateUserPermissionCache(userID)` for post-commit write paths.

Gaps:

- Existing role/grant write paths still need explicit post-commit invalidation calls.
- Multi-instance invalidation transport is not yet implemented.

