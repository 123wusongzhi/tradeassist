# Moved

模块影响图改为生成文档：[reference/generated/module-map.generated.md](reference/generated/module-map.generated.md)。

路由真源：`config/agent/change-impact.json`。运行：

```bash
pnpm docs:generate:module-map
pnpm docs:impact -- --files-from-git
```
