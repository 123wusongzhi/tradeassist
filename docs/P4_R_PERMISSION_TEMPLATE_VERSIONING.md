# Phase P4-R Permission Template Versioning

## Version

`DEMO_PERMISSION_TEMPLATE_VERSION=p4-r-v1`

## Template

The demo permission template covers:

- `demo_admin@trademind.local` with `admin`
- `demo_operator@trademind.local` with `operator`
- `demo_readonly@trademind.local` with `readonly`
- explicit shop grants for operator/read-only demo users

## Repeat Semantics

Existing users and grants are `unchanged`, not failures. The script only supplements missing demo grants and does not reset custom roles.
