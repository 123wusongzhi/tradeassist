---
doc_type: adr
audience: maintainer
status: current
owner: maintainers
source_of_truth:
  - skills-lock.json
  - NOTICE
  - .agents/skills/frontend-design/SKILL.md
  - .agents/skills/trademind-admin-ui/SKILL.md
review_cycle_days: 180
---

# ADR 0001: Vendor `frontend-design` as a TradeMind project fork

## Status

Accepted — 2026-08-05

## Context

`skills-lock.json` recorded `frontend-design` as an unmodified import from
`anthropics/skills`, but the on-disk `SKILL.md` hash no longer matched the
locked `computedHash`. The file had already been rewritten into a TradeMind-
specific Admin UI specification.

Leaving the lock as “pure upstream” while shipping local content is misleading
for upgrades, audits, and license provenance.

## Decision

1. Formally **vendor** `.agents/skills/frontend-design/` as a **project fork**.
2. Update `skills-lock.json` to:
   - `sourceType: project-vendor`
   - current content `computedHash`
   - retain `upstream` provenance (original source path and original hash)
3. Document attribution in root `NOTICE` and keep Apache-2.0 terms in
   `.agents/skills/frontend-design/LICENSE.txt`.
4. Treat the skill as **optional deep reference** (`frontend-design-deep`).
   Default Admin work uses the project overlay `trademind-admin-ui`.

## Consequences

- Hash checks must use the project hash, not the original upstream hash.
- Syncing from anthropics/skills is a deliberate merge, not an automatic
  overwrite.
- Future edits to the vendored file must update `skills-lock.json` `computedHash`
  in the same change.
- Routine UI tasks should not auto-load this large skill.
