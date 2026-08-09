---
doc_type: archive
audience: maintainer
status: historical
owner: maintainers
---

# Archived Agent Rules v1

- **Frozen**: 2026-08-05
- **Original purpose**: Cursor alwaysApply / domain rules and workflow wrappers used before context-map routing.
- **Do not use** as active Agent policy.
- **Replacement**:
  - Global entry: `AGENTS.md`
  - Routing: `config/agent/context-map.json`
  - Active Cursor adapters: `.cursor/rules/*.mdc` (thin pointers only)
  - Domain procedures: `.agents/skills/*/SKILL.md`

These files remain for historical audit only.
