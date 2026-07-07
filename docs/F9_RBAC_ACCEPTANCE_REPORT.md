# F9.1 多角色权限走查报告

> **日期**：2026-07-07  
> **结论**：**passed**

## 账号

| 角色 | 账号 |
| --- | --- |
| admin | demo_admin@trademind.local |
| operator | demo_operator@trademind.local |
| readonly | demo_readonly@trademind.local |

## 检查项

| 检查 | 结果 |
| --- | --- |
| admin 可见全部菜单 | ✅ |
| operator 只见授权范围 | ✅ |
| readonly 只读查看 | ✅ |
| readonly 写 API 403 | ✅ |
| readonly 写 UI disabled | ✅ |
| operator 跨店铺深链拒绝 | ✅ |
| 用户管理仅 admin | ✅ |
| 配置状态中心权限 | ✅ |

- API smoke：[`demo-rbac-smoke.auto.json`](demo-rbac-smoke.auto.json)
- 机器可读：[`f9-rbac-acceptance.json`](f9-rbac-acceptance.json)
