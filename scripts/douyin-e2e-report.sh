#!/usr/bin/env bash
# Aggregate Douyin E2E JSON reports into markdown (DOUYIN_E2E_REPORT_TEMPLATE fields).
set -euo pipefail

REPORT_DIR="${DOUYIN_E2E_REPORT_DIR:-./tmp/douyin-e2e}"
OUT_MD="${DOUYIN_E2E_REPORT_MD:-$REPORT_DIR/douyin-e2e-report.md}"
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

blocked="是"
release_status="Release Candidate"
gray_ok="否"
rollback_drill="environment_simulation_only"
ci_race="see .github/workflows/go.yml backend-race job"

if [ -d "$REPORT_DIR" ]; then
  if ls "$REPORT_DIR"/preflight-*.json >/dev/null 2>&1; then
    latest_preflight="$(ls -t "$REPORT_DIR"/preflight-*.json | head -1)"
    if grep -q '"blockedByRealCredentials"[[:space:]]*:[[:space:]]*false' "$latest_preflight" 2>/dev/null; then
      blocked="否"
    fi
  fi
fi

cat > "$OUT_MD" <<EOF
# 抖店 E2E 验收报告（脚本生成）

> 生成时间（UTC）：$TS  
> Git SHA：$GIT_SHA  
> 发布状态：$release_status  
> \`blocked_by_real_credentials\`：$blocked

## 发布结论（Phase 10.4）

| 字段 | 值 |
| --- | --- |
| release_candidate_status | $release_status |
| real_e2e_status | $([ "$blocked" = "是" ] && echo blocked_by_real_credentials || echo passed_or_partial) |
| gray_release_approved | $gray_ok |
| rollback_drill_status | $rollback_drill |
| ci_backend_race | $ci_race |
| production_available | 否 |

## 脚本工件

目录：\`$REPORT_DIR\`

| 脚本 | 用途 | 执行条件 |
| --- | --- | --- |
| \`scripts/douyin-e2e-preflight.sh\` | 健康检查 + 生产预检 + 运行状态 | 需 Admin 登录 |
| \`scripts/douyin-e2e-readonly.sh\` | 只读链路探针 | 凭证齐全时执行，否则 skipped |
| \`scripts/douyin-e2e-write.sh\` | 写链路探针 | 需 \`ALLOW_DOUYIN_WRITE_TEST=true\` |

## H1.3 前置检查报告字段

| 检查项 | 状态值 |
| --- | --- |
| 凭证 / App Key | passed / blocked_by_real_credentials |
| OAuth 授权 | passed / 未授权 / 待真实凭证 |
| Storage public_base | passed / blocked_by_environment |
| 真实写接口 | skipped（本阶段不自动调用） |
| 创建草稿 | skipped 或 manual |
| 直接上架 | 否 |

失败任务入口：\`/ops/task-center/failures?platform=douyin_shop\`

下一步建议：

- 配置 Storage \`public_base\` 并测试公网访问
- 配置抖店 App Key / Secret 并完成 OAuth
- 阅读 [\`docs/DOUYIN_E2E_PRECHECK_GUIDE.md\`](../docs/DOUYIN_E2E_PRECHECK_GUIDE.md)

## 下一步

- 配置真实抖店 App Key / Secret 并完成 OAuth 后重跑脚本
- 填写完整模板：[\`docs/DOUYIN_E2E_REPORT_TEMPLATE.md\`](../docs/DOUYIN_E2E_REPORT_TEMPLATE.md)
- 灰度门禁：[\`docs/DOUYIN_RELEASE_GATE.md\`](../docs/DOUYIN_RELEASE_GATE.md)

EOF

echo "report: $OUT_MD"
if [ "$blocked" = "是" ]; then
  echo "blocked_by_real_credentials" >&2
  exit 3
fi
echo "ok: report generated"
