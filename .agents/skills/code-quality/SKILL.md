---
name: code-quality
description: TradeMind 全项目代码质量自动适用、轻量检查、深度审查、Baseline/Ratchet 和 CI 门禁的唯一完整主规范
---

# TradeMind 全项目 Code Quality 主规范

本 Skill 是 TradeMind 代码质量规则的唯一完整主规范。其他入口只能引用本文件，不得复制出第二套完整质量规则。

相关专项规范继续独立维护并由本规范引用：

- 全项目测试：`.agents/skills/project-testing/SKILL.md`
- Admin UI：`.agents/skills/frontend-design/SKILL.md`
- Admin E2E：`.agents/skills/admin-e2e-testing/SKILL.md`
- 前端单元：`.agents/skills/frontend-unit-testing/SKILL.md`
- 后端测试：`.agents/skills/backend-testing/SKILL.md`
- API 契约：`.agents/skills/api-contract-testing/SKILL.md`

## 1. 自动适用范围

任何代码新增、修改、重构或 Bug 修复均自动适用本规范。用户不需要显式说“检查代码质量”“使用 code-quality Skill”“做 Code Review”“优化可维护性”或“检查安全性”。

自动适用范围包括 Admin、Collector、Go 后端、shared packages、scripts、tests、migrations、API contracts、GitHub Actions、Docker 和环境配置、Redis、queue、worker、scheduler、第三方平台 adapter。

纯文档修改可以不运行完整代码质量检查，但仍必须检查链接和路径、命令准确性、规范是否冲突、是否包含敏感信息、是否形成重复规则体系。

## 2. 轻量检查和深度审查

### 2.1 轻量代码质量检查

任何代码修改都自动执行轻量检查，至少确认：

- 修改范围是否最小，是否存在无关格式化、无意义 import 排序或换行格式变化。
- 命名是否准确，类型是否清晰，是否新增无意义 `any`、`@ts-ignore`、宽泛 `eslint-disable`。
- 错误是否被吞掉，Promise 是否可能未处理，空值和异常状态是否完整。
- 是否有重复逻辑、死代码、debug 代码、`.only`、无理由 `.skip` 或遗留 `console.log`。
- 是否泄露敏感信息，是否缺少必要测试，是否运行受影响测试。
- 是否修改 API 或业务行为，是否引入额外请求或明显性能退化。

### 2.2 深度代码质量审查

以下情况自动触发深度审查：认证和权限、商品库存、发布和刊登、抖店等第三方平台、支付或资金、数据库事务、Redis 锁、队列和后台任务、并发、文件上传、鉴权 Token、新增第三方适配器、新增公共基础组件、新增公共 service、修改 API envelope、修改 shared type、修改 migration、跨三个以上业务模块、大型重构、单个文件职责明显继续膨胀、修复数据丢失/重复提交/重复任务问题。

深度审查在轻量检查基础上增加：事务边界、幂等性、重试边界、timeout、race condition、数据一致性、权限绕过、日志脱敏、第三方失败降级、回滚策略、缓存一致性、锁释放、goroutine 生命周期、数据库查询复杂度、模块依赖方向和测试完整性。

触发深度审查不等于允许大规模重构；除非任务本身要求，否则只报告风险并做最小必要修改。

## 3. 质量问题严重级别

### Critical

必须阻塞完成：生产凭据或私钥泄露、测试可能连接生产数据库、测试可能调用真实平台写接口、SQL 注入、命令注入、路径遍历、权限绕过、数据破坏、不受控并发写入、死锁、敏感日志泄露、明确的数据竞争、发布或库存重复执行。

### High

默认阻塞：新增 TypeScript 错误、新增 Go 编译错误、新增 lint error、未处理 Promise rejection、外部请求无 timeout、无边界 retry、事务部分提交、幂等性缺失、异常被完全吞掉、关键状态转换缺少校验、API contract 漂移、新功能完全无测试、Bug 修复无合理回归测试且无解释。

### Medium

需要修复或明确说明：大量重复代码、函数职责过多、非必要 `any`、不稳定 rowKey、React Hook 依赖问题、低效重复请求、N+1 查询风险、可读性差、错误上下文不足、测试过度依赖内部实现。

### Advisory

可后续处理：命名可进一步改进、局部抽象机会、历史文件过大、非阻塞性能优化、文档完善建议。

最终报告必须按严重等级分类，不能把所有建议都写成阻塞项。

## 4. Baseline/Ratchet 策略

Admin 当前存在历史 TypeScript 错误。不得一次性修复全部历史错误，不得直接将全部历史错误设为所有 PR 的硬门禁，不得让 code-quality 上线后阻塞全部后续开发。

Baseline/Ratchet 要求：

1. 执行真实 Admin typecheck：`pnpm quality:baseline:admin-ts`。
2. 收集历史错误，去除不稳定的行号和列号。
3. Baseline 只记录文件路径、TypeScript diagnostic code、规范化错误信息、同一错误出现次数。
4. Baseline 不记录绝对本地路径、用户目录、时间戳、临时文件或随机顺序。
5. 后续检查允许历史错误减少或消失。
6. 后续检查禁止新增新的错误签名、既有错误数量增加、新文件出现错误、修改文件引入新错误。
7. Baseline 不得自动扩大；更新 baseline 必须显式执行 `pnpm quality:baseline:admin-ts -- --update` 并说明原因。
8. 普通 CI 不得自动更新 baseline。

Baseline 文件：`tests/quality/baselines/admin-typescript.json`。

## 5. TypeScript 通用规范

- 不新增无意义 `any`，不滥用 `unknown as` 和非空断言。
- 不使用 `@ts-ignore` 掩盖真实错误；必须使用时优先 `@ts-expect-error` 并说明原因。
- 不使用宽泛 `eslint-disable`。
- 类型应描述真实领域含义，公共函数应明确输入和输出。
- API response 不直接当可信数据使用，外部数据必须经过校验或安全归一化。
- Promise 必须处理 reject，异步函数错误不能静默丢失。
- 避免隐式字符串枚举漂移，优先使用项目已有共享类型，不复制多个相同 DTO。
- 不为测试修改生产类型语义。

检查新增或修改文件中的 `any`、`@ts-ignore`、`@ts-expect-error`、`eslint-disable`、空 catch、未处理 Promise、无边界重试和非必要类型强制转换。脚本只负责发现候选，AI 必须结合上下文审查。

## 6. React/Umi/Ant Design 规范

Admin 修改必须同时考虑：不在 render 中产生副作用，Hook 依赖正确，避免 effect 重复请求，防止组件卸载后更新状态，防止快速点击重复提交，提交按钮必须有 loading/disabled，rowKey 稳定唯一，不使用会变化列表的数组 index 作为主 key。

不得依赖 Ant Design 内部私有 class 做逻辑，不使用宽泛全局 Ant 样式覆盖，不用 `!important` 掩盖结构问题，不创建嵌套 Form，Form instance 必须连接真实 Form，Modal/Drawer 关闭后状态清理明确，URL 状态和组件状态必须保持一致，深链和刷新必须恢复，readonly/disabled 语义准确，error/loading/empty 不得缺失。

优先复用 `TmPageContainer`、`SectionCard`、`MetricCard`、`OperationToolbar`、`TmProTable`、`EmptyState`、`AppDrawer`、`layoutTokens`。UI 细节由 `.agents/skills/frontend-design/SKILL.md` 定义，浏览器回归由 `.agents/skills/admin-e2e-testing/SKILL.md` 定义。

## 7. Node.js/Collector 规范

环境变量必须验证，不允许缺失配置时 fallback 到生产地址。网络请求必须有 timeout，retry 必须有次数和退避边界，外部响应必须校验，Promise rejection 必须处理，文件/流/句柄/连接必须释放。

转换逻辑尽量保持纯函数，相同输入应产生确定结果；金额和价格不得使用不安全浮点逻辑；时间和时区行为必须明确；不将完整第三方响应、Authorization、Cookie 或 Token 写日志；不使用无限队列或无限递归重试；测试不得访问真实外部站点或平台。

Collector 重点检查 env parsing、price normalize、quality score、抓取失败处理、超时、限流、重试、非法数据、重复商品、大批量数据内存使用和第三方字段缺失。

## 8. Go 后端规范

所有 error 必须处理，error wrap 必须保留上下文，不用无上下文错误字符串覆盖原始错误。`context.Context` 必须向下传递，外部 HTTP 请求必须设置 timeout，response body 必须关闭。

goroutine 生命周期必须可控，不启动无法停止的后台 goroutine；channel 必须明确关闭责任；shared state 必须同步保护；避免 data race。

transaction 必须明确 commit/rollback，rollback 错误需合理处理；查询避免 N+1；批量操作考虑分页和上限；Redis 锁必须有过期时间且释放验证 ownership；queue job 必须幂等；retry 必须有最大次数；第三方平台失败不得错误标记成功。

DTO 校验位于正确边界；handler 不承载大量领域逻辑；repository 不决定业务状态机；service 不依赖 HTTP 细节；日志不得泄露凭据；不使用 panic 处理可恢复业务错误；不忽略 JSON 编解码错误；时间处理统一使用项目标准。

自动质量检查优先复用 `gofmt`、`go test`、`go vet`。仅当仓库已使用时复用 staticcheck 或 golangci-lint；本项目当前不新增大型 Go lint 工具。

## 9. HTTP/API 规范

API 变更必须检查 method、URL、path params、query、payload、response envelope、error envelope、enum、nullable、pagination、permission、readonly、idempotency 和兼容性。

重点保护多平台草稿、抖店平台草稿、传统 `publishProduct`、readiness 阻断、publication refresh、SKU binding、inventory sync。不得以重构名义修改 API method、payload、路由、权限、readonly、状态机或 reload 语义。

第三方平台适配器必须有 timeout、有边界 retry、区分 HTTP 错误和业务错误、校验响应结构、正确处理 token 失效和 rate limit、不把失败误判成功、不记录完整 Token。测试使用 fake adapter 或 mock server，不访问真实平台。API 变更自动读取 `.agents/skills/api-contract-testing/SKILL.md`。

## 10. 数据库和事务规范

检查 migration 是否可重复执行、是否依赖手工数据、事务边界是否完整、部分失败是否回滚、unique/foreign key 是否与业务规则一致、nullable 是否准确、状态字段是否表达真实状态、查询是否 N+1、批量更新是否有限制、分页和排序是否稳定、是否需要乐观锁或并发控制、重复请求是否可能重复写入、JSON/JSONB 是否缺少结构校验、删除行为是否产生孤儿数据、repository 是否泄露 ORM 细节到领域层。

任何 migration 修改自动触发 migration test、database integration、repository tests、API contract 和深度质量审查。测试不得 fallback 到开发或生产数据库。

## 11. Redis/锁/队列规范

检查 Redis key 命名空间、TTL、缓存失效、缓存击穿风险、分布式锁过期、锁释放 token/owner 校验、任务幂等、重复消息安全、retry 上限、dead-letter 可观测性、失败任务不得标记成功、running 状态是否永久卡住、worker 停止是否释放资源、scheduler 是否重复注册、时间逻辑是否可测试、日志是否包含 taskId/traceId 且不记录敏感 payload。

Redis/queue 变更自动触发 backend tests、Redis integration、queue tests、idempotency tests、retry tests 和深度质量审查。

## 12. 并发和异步规范

异步入口必须有生命周期、取消、超时和错误传播策略。并发写入必须有同步或幂等边界。后台任务必须能停止，重复启动必须安全。前端异步请求必须避免过期响应覆盖新状态，后端 goroutine 必须能随 context 或进程生命周期退出。

## 13. 错误处理规范

禁止空 catch、捕获后完全忽略、只输出 `console.log(error)`、只返回“失败”而丢失上下文、同一错误多层重复日志、混淆用户可见错误和内部错误、向前端暴露 stack trace。

错误至少包含适当上下文：operation、entity ID、task ID、trace ID、provider/platform、retry count。不得包含密码、Token、Cookie、私钥、支付敏感数据或不必要隐私。

## 14. 日志和可观测性规范

日志级别必须合理：debug 用于调试细节，info 用于正常业务状态，warn 用于可恢复异常，error 用于需要处理的失败。不得把正常业务校验失败全部记录为 error。不得记录完整第三方响应、Authorization、Cookie、Token 或敏感用户数据。

## 15. 安全和敏感信息规范

优先复用 GitHub secret scanning 和现有平台能力。本项目本地使用轻量 changed-diff 高置信扫描：私钥头、GitHub classic token、AWS access key、OpenAI 风格高置信 secret、明显 JWT、带用户名密码的数据库 URL、硬编码 Authorization Bearer、硬编码 Cookie、生产环境密码变量赋值。

扫描只检查新增或修改内容，输出文件和行号，secret 必须脱敏，不输出完整 secret，不上传扫描结果产物。示例值必须明显是假值；不得把普通 UUID 当 secret。发现高置信敏感信息必须非零退出。

## 16. 性能规范

不得引入明显 N+1、无界分页、无界队列、无限 retry、重复请求风暴或大对象全量日志。批量处理要有上限和分页，外部请求要有 timeout，缓存要有一致性和失效策略。性能优化建议按严重级别分类，不把非阻塞优化写成必须阻塞。

## 17. 测试代码质量规范

测试代码同样适用 code-quality。测试名描述真实行为，不依赖执行顺序，不共享可变全局数据，不使用长时间 sleep，不使用随机不确定数据，不吞测试异常，不使用 `.only`，不随意 `.skip`，不降低断言强度，不使用宽泛 console allowlist，不连接真实服务，fixture 最小合法，mock 与真实契约一致，失败信息可读，helper 职责单一，不创建巨大万能测试文件。

## 18. Diff Hygiene

任务完成前检查是否修改无关文件、产生全文件格式化、改变换行格式、产生无意义 import 排序、引入生成文件/日志/测试产物、误改 lockfile、删除用户已有修改、留下 debug 代码/console.log/TODO 无上下文/`.only`/`.skip`/宽泛白名单。

禁止 `git add .`、全仓无关格式化、为通过质量检查重写无关模块、以“顺便优化”为由扩大范围。

## 19. 依赖和配置质量

新增依赖必须有必要性说明，不得为了质量门禁盲目安装大型平台或第二套 lint/typecheck。配置修改必须避免重复 CI 和冲突规则，不得引入真实凭据、开发机绝对路径或平台私有缓存。

## 20. 新功能质量要求

新功能必须有清晰边界、类型、错误处理、测试和受影响质量检查。高风险新功能必须有幂等、权限、事务、重试、timeout、日志脱敏和回滚/失败语义审查。

## 21. Bug 修复质量要求

Bug 修复优先补回归测试，修复必须最小化且针对首个真实根因。不得顺手重构无关模块，不得通过 skip、ignore、宽泛 allowlist 或降低断言掩盖失败。

## 22. 高风险模块质量要求

认证权限、发布、库存、第三方平台、数据库、Redis、队列、worker、scheduler、文件上传、Token、API envelope、shared type、migration 和跨模块重构必须做深度审查，并按 Critical/High/Medium/Advisory 输出发现。

## 23. 与 project-testing 的联动

任何代码修改必须根据 `.agents/skills/project-testing/SKILL.md` 选择受影响测试。`quality:affected` 负责质量检查选择，`test:affected` 负责测试选择；两者互补，不能互相替代。

## 24. 与 frontend-design 的联动

Admin UI 变更必须同时遵循 `.agents/skills/frontend-design/SKILL.md`。code-quality 负责类型、错误、异步、安全、diff hygiene 和测试质量；UI 视觉、布局、响应式和共享组件细节由 frontend-design 定义。

## 25. 与 admin-e2e-testing 的联动

涉及 Admin 页面、交互、写请求、响应式、路由、状态或 E2E 文件时，必须遵循 `.agents/skills/admin-e2e-testing/SKILL.md`。code-quality 不复制完整 E2E 规范，只要求必要浏览器回归和写请求安全不得跳过。

## 26. 模块化边界

code-quality 负责发现模块化风险：文件职责过多，页面/API/状态机混在同一文件，handler 承载领域逻辑，repository 决定业务规则，UI 直接依赖低层 API 实现，跨层循环依赖，多处重复 DTO/平台判断/状态映射，公共模块反向依赖业务模块。

不要仅按行数强制拆分。只有新增业务领域、新增平台适配器、跨三个以上模块、大型组件持续膨胀、循环依赖、共享层职责不清或新增 worker/queue/scheduler 体系时，才建议后续触发模块化专项任务。小修复不得被迫大规模模块化。

## 27. 禁止项

禁止真实凭据、生产 DB/Redis、真实平台写接口、SQL/命令注入、路径遍历、权限绕过、敏感日志、无界 retry/队列/并发、空 catch、宽泛 ignore/skip/allowlist、无关重构、重复 CI、自动扩大 baseline、未经用户要求 commit/push。

## 28. 测试失败或检查失败处理

失败时定位首个真实根因，区分生产缺陷、测试缺陷、环境错误、历史 baseline、工具配置错误和 flaky。不得直接绕过，不得自动扩大 baseline。无法运行的检查必须说明命令、阻塞原因和风险；不能声称跳过的检查已通过。

## 29. 完成报告格式

最终报告列出：当前分支、开始工作区状态、审计结论、Admin 历史 TypeScript 错误数量、Collector/Go 状态、新增/修改文件、自动触发范围、轻量/深度条件、Critical/High 规则、baseline 路径和规范化方式、baseline 更新命令、新错误阻塞方式、quality scripts、CI 触发、是否新增依赖、是否修改生产代码、实际运行命令结果、跳过原因、Critical/High/Medium/Advisory 发现、diff stat、当前未提交文件、是否存在敏感文件/测试产物、是否适合签收/commit/push。
