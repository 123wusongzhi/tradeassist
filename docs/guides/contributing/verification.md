---
doc_type: guide
audience: contributor
status: current
owner: maintainers
source_of_truth:
  - package.json
  - config/agent/context-map.json
review_cycle_days: 60
---

# 验证与检查

## 选择要跑什么

```bash
pnpm agent:context -- --files-from-git
# 或
pnpm agent:context -- --intent documentation-only
```

只运行 selector 输出的 `checks`。不要默认全量 E2E/集成。

## 常用稳定命令

完整列表见生成索引：`docs/reference/generated/commands.generated.md`。

| 目的 | 命令 |
| --- | --- |
| Agent 路由自检 | `pnpm agent:check` |
| 文档检查 | `pnpm docs:check` |
| 前端单测 | `pnpm test:frontend` |
| 采集单测 | `pnpm test:collector` |
| 契约 | `pnpm test:contracts` |
| 后端单元 | `pnpm test:backend` |
| 受影响质量 | `pnpm quality:affected` |
| 受影响测试 | `pnpm test:affected` |

## 文档影响

```bash
pnpm docs:impact -- --files-from-git
```

按输出更新真源或运行对应 `docs:generate*`。

## 禁止

- 生产 DB/Redis
- 未拦截真实第三方写请求
- 自动扩大 baseline
- 用 legacy/evidence 命令冒充日常必跑项
