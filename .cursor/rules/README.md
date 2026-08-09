# Cursor Rules

Cursor rules are thin adapters. They do not own project policy.

## Active model

| File | Role |
| --- | --- |
| `00-agent-router.mdc` | Only `alwaysApply: true` adapter; points to `AGENTS.md` + context selector |
| `admin-ui.mdc` | Admin UI → context `admin-ui` |
| `backend.mdc` | Backend → context `backend-development` |
| `collector.mdc` | Collector/extension → context `collector-development` |
| `docs.mdc` | Docs/agent config → context `docs-maintenance` |
| `14-ui-copywriting.mdc` | Scoped copywriting reminder |

## Source of truth

1. Global invariants: `AGENTS.md`
2. Task routing: `config/agent/context-map.json`
3. Doc impact: `config/agent/change-impact.json`
4. Domain procedures: `.agents/skills/*/SKILL.md`

## Rules for new adapters

- Prefer `alwaysApply: false` + narrow `globs`.
- At most one context/Skill pointer per file.
- No project snapshots, stage numbers, API inventories, or command matrices.
- Keep each adapter short (about 20 lines or fewer).

Archived historical rules live under `docs/archive/agent-rules-v1/` and must not be loaded as active policy.
