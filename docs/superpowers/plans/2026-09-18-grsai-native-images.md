# grsai 原生生图实施计划

> **给执行型智能体：** 必须按任务逐项实施和复核。本文件中的复选框用于记录进度；每完成一个任务，先运行该任务的测试，再继续下一个任务。

**目标：** 在 `POST /v1/api/generate` 增加第一阶段 `platform=grsai` 原生生图网关，支持按模型保存价格快照，并以可持久化、幂等的方式结算。

**架构：** 专用 `GrsaiGatewayHandler` 只认证和调度 grsai 分组，再调用原生 grsai 客户端，不转换模型特有参数。提交上游之前落库结算记录；仅上游终态 `succeeded` 才经已有原子用量计费仓储收费。独立恢复运行时轮询已知运行中的任务，并重试待结算记录。

**技术栈：** Go、Gin、Ent、PostgreSQL SQL migration、Google Wire、Vue 3、TypeScript、Vitest、Go 单元/集成测试。

**设计依据：** `docs/superpowers/specs/2026-09-18-grsai-native-images-design.md`

## 全局约束

- 唯一新增公开路由为 `POST /v1/api/generate`；不增加根路径别名或公开任务查询路由。
- 仅接受 JSON `replyType`；缺省时归一为 `json`，本地拒绝 `stream=true` 和 `async=true`。
- 第一阶段只允许直接 `platform=grsai` 分组；composite 支持留在 Roadmap。
- 不改写模型特有请求字段，本阶段不建立按模型参数档案。
- 只有上游任务确认 `succeeded` 后收费；上游 HTTP 错误、`failed`、`violation`、畸形响应和初始 `running` 响应均不立即收费。
- 结算记录和日志不得保存上游 API Key、提示词、参考图字节、Base64 或未脱敏请求 JSON。
- 使用数据库扫描器恢复结算；不得依赖内存队列保证计费恢复。
- 依据 `AGENTS.md`，任何 migration 或生产部署改动前，必须定位并阅读 `docs/RELEASE_DEPLOYMENT.md`。当前工作区没有该文件；在仓库所有者恢复或确认其权威路径前，必须停止 migration 实施，不得创建 migration、生成 Ent 或启动服务。
- 不实现 Roadmap 项：composite 分组、公开任务查询、公开异步/流式模式、grsai 的 OpenAI 兼容生图路由、参数档案、模型同步、自动价格换算。

## 文件结构

| 路径 | 职责 |
| --- | --- |
| `backend/internal/domain/constants.go`、`backend/internal/service/domain_constants.go` | 与其他具体平台并列定义 `PlatformGrsai`。 |
| `backend/ent/schema/grsai_settlement.go` | 持久化结算实体与索引。 |
| `backend/migrations/239_grsai_native_images.sql` | 创建 `grsai_settlements` 和升级安全索引；实施时若 `239` 被占用，使用实际下一个编号。 |
| `backend/internal/repository/grsai_settlement_repo.go` | Ent 的创建、状态迁移、到期扫描、行锁与原子结算持久化。 |
| `backend/internal/service/grsai_native.go` | 原生请求校验、响应解析、URL 构造和 HTTP 客户端接口。 |
| `backend/internal/service/grsai_settlement.go` | 状态机、价格快照、用量日志构造与幂等计费编排。 |
| `backend/internal/service/grsai_settlement_recovery.go` | 数据库轮询、结果对账、退避重试和人工复核升级。 |
| `backend/internal/handler/grsai_gateway_handler.go` | 鉴权请求流、调度器集成、上游调用、响应透传和即时结算触发。 |
| `backend/internal/server/routes/gateway.go` | 注册并进行平台隔离的 `/v1/api/generate`。 |
| `frontend/src/types/index.ts`、`frontend/src/constants/platforms.ts`、`frontend/src/utils/platformColors.ts` | 管理端类型和展示层增加具体平台。 |
| `frontend/src/components/account/credentialsBuilder.ts`、账号/分组/渠道视图、语言文件 | 配置 grsai API Key/Base URL 凭据并展示/选择平台。 |

## 任务 1：确认发布与 Migration 前置条件

**文件：** `AGENTS.md`、`docs/RELEASE_DEPLOYMENT.md`、`backend/migrations/migrations.go`、当前最后一条 migration；前置满足后才创建 `backend/migrations/<实际编号>_grsai_native_images.sql`。

**产出：** 已记录 migration 编号、部署兼容规则，以及允许修改数据库 schema 的结论。

- [ ] 定位并完整阅读 `docs/RELEASE_DEPLOYMENT.md`。执行 `rg --files -g 'RELEASE_DEPLOYMENT.md' -g 'AGENTS.md' . docs`。预期是有可读取的权威文件；若仍缺失，停止实施并请仓库所有者恢复或提供权威路径，不得创建 migration、生成 Ent 或启动服务。
- [ ] 阅读发布兼容规则及当前 migration 顺序。执行 `Get-Content -Raw 'docs\RELEASE_DEPLOYMENT.md'` 并检查当前最后一条 SQL。记录必需的兼容性 trailer、事务限制、部署顺序和回滚步骤。
- [ ] 确定连续且未占用的 migration 前缀。执行 `Get-ChildItem 'backend\migrations' -Filter '*.sql' | Sort-Object Name | Select-Object -Last 10 Name`；`239` 仅为当前预期，实施时以检查结果为准。

## 任务 2：注册平台并完成管理端配置

**文件：** `backend/internal/domain/constants.go`、`backend/internal/service/domain_constants.go`、`backend/internal/service/account_service.go`、`backend/internal/handler/admin_group.go`、`backend/internal/service/scheduler_snapshot_service.go`、`backend/internal/service/composite_platform.go`、`backend/internal/model/error_passthrough_rule.go`；前端 `frontend/src/types/index.ts`、`frontend/src/constants/platforms.ts`、`frontend/src/utils/platformColors.ts`、`frontend/src/components/account/credentialsBuilder.ts` 及相应账号/分组/渠道视图与语言文件。

**产出：** 管理端可创建和编辑 `platform=grsai` 账号、分组和渠道，调度快照能识别它。

- [ ] 定义独立常量 `PlatformGrsai = "grsai"`，加入所有持久化校验、账号平台列表、分组平台列表和调度快照；错误、筛选项和标签遵循现有中文文案风格。
- [ ] 接入账号凭据与管理端展示，复用现有 Base URL/API Key 的构造和敏感字段处理模式；平台目录、颜色、类型、表单选择器、筛选器、新建和编辑流程均要包含 grsai。
- [ ] 维持传输层隔离：不得将 grsai 加入 OpenAI-compatible、OpenAI 特有图片路由或任何按 OpenAI 协议发送请求的 switch；在 Roadmap 完成前，composite 显式拒绝或排除 grsai。
- [ ] 补充平台注册回归测试：grsai 可保存和展示；不被 OpenAI transport 枚举选中；第一阶段 composite 不能将其作为成员。

## 任务 3：实现可持久化的结算存储

**文件：** 新建 `backend/ent/schema/grsai_settlement.go`、`backend/internal/repository/grsai_settlement_repo.go`、相关仓储集成测试及通过任务 1 后的 migration。

**产出：** 上游提交前可落库的结算记录，具有唯一幂等键、任务去重和可锁定的到期扫描。

- [ ] 定义最小脱敏 schema：账户、分组、令牌/用户归属的必要标识、模型、价格快照、计费幂等键、上游任务 ID、上游状态、内部状态、重试次数、下次尝试时间、最终错误摘要与审计时间戳。禁止保存密钥、prompt、原始图片和原始请求体。
- [ ] 新增数据库约束与 migration：`billing_idempotency_key` 必须唯一；对非空 `upstream_task_id` 增加 `(account_id, upstream_task_id)` 部分唯一索引；增加到期扫描和行锁领取索引；满足任务 1 的发布兼容约束。
- [ ] 实现 create、绑定上游任务、更新结果、领取到期记录、标记待结算/已结算/免收费关闭/人工复核等仓储操作。`Settle` 在事务内以行锁或等价条件更新，防止多个 worker 对同一记录并发收费。
- [ ] 编写集成测试，覆盖唯一计费键、同账号非空任务 ID 去重、空任务 ID 可共存、并发领取仅一方成功和终态记录不会再次领取。

## 任务 4：实现原生 grsai 协议客户端

**文件：** 新建 `backend/internal/service/grsai_native.go` 和 `backend/internal/service/grsai_native_test.go`。

**产出：** 可测试、不会改写模型特有参数的上游客户端。

- [ ] 定义以下接口及响应模型：

```go
type GrsaiNativeClient interface {
    Generate(ctx context.Context, account *Account, body []byte) (*GrsaiUpstreamResult, error)
    Result(ctx context.Context, account *Account, taskID string) (*GrsaiUpstreamResult, error)
}
```

响应模型保留原始响应 body（仅回传和受控解析）及标准化任务 ID、状态、错误码、错误信息；不得在日志记录 prompt 或 API Key。

- [ ] 以账号 Base URL 可靠拼接 `/v1/api/generate`，内部轮询拼接 `/v1/api/result?id=...`；按 grsai 认证规范发送密钥，配置超时并保留可诊断但不泄密的 HTTP 错误上下文。
- [ ] 仅做协议边界校验：拒绝非 JSON body、`stream=true`、`async=true` 和非法或非 `json` 的 `replyType`；缺省 `replyType` 写入 `json`。其余 `aspectRatio`、`imageSize`、`quality`、`background`、`images` 及未来字段原样发送。
- [ ] 使用 `httptest` 覆盖路径、鉴权头、正文原样透传、replyType 默认值、stream/async 拒绝、终态/运行中/失败/违规解析、畸形 body 和网络错误。

## 任务 5：实现价格快照与结算服务

**文件：** 新建 `backend/internal/service/grsai_settlement.go` 和测试；参考 `backend/internal/service/openai_gateway_usage.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/batch_image_settlement.go`。

**产出：** 仅为 `succeeded` 结算、按提交时价格收费的幂等状态机。

- [ ] 定义并限制状态迁移：`submission_pending`、`awaiting_result`、`settlement_pending`、`settled`、`closed_no_charge`、`upstream_unknown`、`manual_review`。重复执行不得改变已结算金额。
- [ ] 在调用上游前用当前账号/分组/模型定价规则计算并保存价格快照。恢复任务永久使用此快照，管理员之后改价不得改变历史扣费。
- [ ] 复用既有 `UsageBillingRepository.Apply` 或 `applyUsageBilling` 写入余额、额度和用量日志，稳定内部请求 ID 为 `grsai_settlement:<settlement-id>`；不得实现第二套余额扣减逻辑。
- [ ] 若上游成功而即时计费临时失败，仍返回上游成功，记录保持 `settlement_pending` 并由恢复机制处理；不得将成功伪装为失败，也不得自动重发生成请求。
- [ ] 测试 succeeded 仅收费一次、failed/violation/畸形响应不收费、价格快照不受改价影响、重复 worker 幂等、计费失败可重试和已结算记录不再次扣款。

## 任务 6：实现恢复运行时

**文件：** 新建 `backend/internal/service/grsai_settlement_recovery.go` 和测试；修改配置类型、默认值及 Wire/runtime 生命周期装配；参考 `backend/internal/service/batch_image_worker_runtime.go`。

**产出：** 可重启、可重试、不会重复提交上游的数据库驱动恢复机制。

- [ ] 增加 `grsai_settlement` 配置：`enabled`、扫描间隔、单批上限、无任务 ID 的提交未知超时；采用项目既有配置载入和默认值模式。
- [ ] 对有任务 ID 的 `awaiting_result` 记录调用内部结果接口：`running` 按退避继续轮询，`succeeded` 转入结算，`failed`/`violation` 免收费关闭，畸形结果保留可重试诊断。
- [ ] 请求可能已发送但未取得可解析响应或任务 ID 时绝不自动重新提交；超过配置超时后标记 `manual_review`，记录高优先级运维错误且不收费。
- [ ] 对 `settlement_pending` 按 1 分钟、5 分钟、15 分钟、1 小时、6 小时重试，超过次数转 `manual_review`；调度完全依赖数据库 `next_attempt_at`，重启不得丢失。
- [ ] 按现有 runtime 的启动/停止模式通过 Wire 接入。测试扫描、行锁竞争、运行中轮询、成功结算、失败关闭、退避序列、重启恢复和未知提交不重发。

## 任务 7：实现 Handler 和公开路由

**文件：** 新建 `backend/internal/handler/grsai_gateway_handler.go` 和测试；修改 `backend/internal/handler/handler.go`、`backend/internal/handler/wire.go`、`backend/internal/service/wire.go`、`backend/cmd/server/wire.go`、生成的 `backend/cmd/server/wire_gen.go`、`backend/internal/server/routes/gateway.go`。

**产出：** 完整、平台隔离的 `POST /v1/api/generate` 请求链路。

- [ ] 复用鉴权、模型白名单、图片权限、内容审核、并发控制、计费资格校验和调度器。选择账号后必须验证 `getGroupPlatform(c) == service.PlatformGrsai`，否则按既有网关错误格式拒绝。
- [ ] 在上游提交前读取价格并创建 `submission_pending` 记录，生成稳定计费幂等键；仅记录成功持久化后才发出上游 HTTP 请求。
- [ ] 获取上游响应后，先持久化任务 ID 与上游状态，再回传原始 JSON：`succeeded` 触发即时结算；`running` 交由内部轮询；`failed`/`violation` 免收费关闭；HTTP 错误和畸形响应不收费。响应后不得 failover 或重新提交。
- [ ] 只在 `/v1` 网关组注册 `gateway.POST("/api/generate", handlers.GrsaiGateway.Generate)`；不注册 `/api/result`、根路径别名，也不修改 `/v1/images/generations` 协议语义。
- [ ] 测试鉴权、仅 grsai 分组、参数透传、replyType 归一、stream/async 拒绝、succeeded 单次收费、running 不立即收费、失败/违规/HTTP 错误不收费、即时结算失败仍成功回传及未知提交不重发。

## 任务 8：完成管理端体验与隔离回归

**文件：** 修改任务 2 列出的账号、分组、渠道组件与语言文件；新增/修改对应 `*.spec.ts`；修改后端平台隔离回归测试。

**产出：** 管理员可正确配置 grsai，其他平台的图片能力不会误选 grsai。

- [ ] 验证新建、编辑、筛选、详情和模型定价界面均可展示和保存 grsai，Base URL/API Key 字段沿用既有敏感数据处理方式。
- [ ] 断言 composite 分组被拒绝；OpenAI 图片路由永远不会选中 grsai 账号；grsai 原生路由不能选中非 grsai 直接分组。
- [ ] 在前端运行 `npm run test -- --run <相关 spec 文件>`，确保类型、平台选项、凭据构造和表单回归测试通过。

## 任务 9：完成全量验证与交付准备

**文件：** 全部上述实现与测试文件。

**产出：** 有证据支持的完成结论，不包含未授权发布。

- [ ] 仅当任务 1 的发布文档已可用时，在 `backend` 执行 `go generate ./ent`、`go generate ./cmd/server`、`go test ./internal/service/... ./internal/repository/... ./internal/handler/...`、`go test -race ./internal/service/... ./internal/repository/... ./internal/handler/...`。如包边界不同，采用 `backend/Makefile` 定义的等价命令并记录差异。
- [ ] 在 `frontend` 执行 `npm run test -- --run` 和 `npm run build`。
- [ ] 只用一次性测试账号和密钥做人工冒烟，密钥不得写入测试文件。验证同步 JSON、`running` 后轮询结算、上游失败、本地拒绝 stream/async、改价后旧任务仍按价格快照结算。
- [ ] 执行 `git diff --check` 与 `git status --short`。不得触碰或提交用户既有的未跟踪 `.agents/`、`.playwright-cli/`、`test/`；`docs/superpowers` 受忽略规则影响时使用 `git add -f`。
- [ ] 在宣称实现完成前请求代码审查并处理阻断问题。缺少 `RELEASE_DEPLOYMENT.md` 时不得进行 migration、生成、部署或发布；文件可用后，严格遵循其中的兼容性 trailer、人工本机验收、Blue-Green 和回滚流程。不得创建包含无关改动的最终汇总提交。

## 完成标准

- `platform=grsai` 是独立平台，不是 OpenAI 兼容通道别名。
- 第一阶段唯一公开入口为 `POST /v1/api/generate`，且仅服务直接 grsai 分组。
- 模型特有字段完整透传；`replyType` 缺省为 `json`；明确拒绝 `stream` 和 `async`。
- 只有确认成功的任务才收费，任意重试最多扣款一次。
- 上游成功但暂时无法结算时仍返回上游成功，随后由可靠恢复机制处理。
- 发送后未知的任务绝不自动重复提交，需要人工复核。
- 管理端可配置 grsai，且 OpenAI 图片路径与 grsai 保持隔离。
- 所有聚焦测试、生成、构建和人工冒烟均有记录；部署仅在发布文档恢复后按其规则执行。
