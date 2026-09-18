# grsai 原生生图接入设计

**日期：** 2026-09-18
**状态：** 已完成设计确认，待书面规格评审
**范围：** 第一阶段仅接入 grsai 原生图片生成和可靠结算。

## 1. 目标与边界

为 Sub2API 增加一级平台 `grsai`，让管理员可以单独配置 grsai 账号、分组、渠道和按模型售价，并向终端用户暴露 grsai 原生图片生成协议。

第一阶段的目标是保留上游模型的差异化参数与任务状态，同时保证用户只会在任务最终成功时被扣费，且结算重试不会重复扣费。

以下内容不在本阶段：

- 不迁移现有 OpenAI 账号、分组、渠道或价格配置。
- 不修改 `/v1/images/generations`、`/v1/images/edits`、聊天接口或其响应契约。
- 不对每个 grsai 模型建立参数白名单、默认值或限制校验。
- 不公开 grsai 任务查询接口，不支持公开异步或流式生成。
- 不实现模型能力探测、自动同步模型、价格自动换算或后台运营界面改版。

## 2. 公开接口契约

新增且仅新增下列公开路由：

```text
POST /v1/api/generate
```

不提供无 `/v1` 前缀别名，也不提供公开 `GET /v1/api/result`。

### 2.1 认证与路由

- 使用现有 Sub2API 用户 API Key：`Authorization: Bearer <user-api-key>`。
- 上游 grsai Key 和 Base URL 只保存于 grsai 账号配置，绝不从用户请求读取或返回。
- API Key 所属分组必须是 `platform=grsai`。其他分组访问该路由返回本地 `404` 平台能力未开启错误，并记录现有运营限制原因。
- 第一阶段不允许 composite 分组通过该路由；这避免无模型参数的任务查询和平台解析混入首版调度语义。
- 完成认证、分组模型白名单、余额准入、并发控制和账号选择后，调度到 `platform=grsai` 的账号；不得落入 OpenAI、Grok 或其他平台账号池。

### 2.2 请求体

请求体为 JSON。至少要求 `model` 非空；其余 grsai 参数不做按模型语义校验，保留并转发，例如：

```json
{
  "model": "nano-banana-2",
  "prompt": "一座雨后的城市",
  "images": ["https://example.com/reference.png"],
  "aspectRatio": "1:1",
  "imageSize": "1K",
  "quality": "high",
  "background": "transparent",
  "replyType": "json"
}
```

规则：

- `replyType` 缺省时由网关写为 `json`；显式传入时只能是 `json`，否则本地返回 `400`。
- `stream=true` 或 `async=true` 本地返回 `400`，不会转发上游。类型不正确同样返回 `400`。
- `aspectRatio`、`imageSize`、`quality`、`background`、`images` 和未来上游新增字段原样透传。若上游静默替换不支持的参数，以其最终 `succeeded` 结果为准。
- 本地只负责请求大小、JSON、认证、必填 `model` 与上述协议限制；参数不合法的模型级错误由 grsai 决定并原样返回。

### 2.3 上游与响应

- 渠道向 grsai 账号配置的 Base URL 发起 `POST /v1/api/generate`，携带账号中的上游认证信息。
- 只要获得上游 HTTP 响应，尽可能保持其 HTTP 状态、JSON Body 和任务字段原样返回，包括 `id`、`status`、`progress`、`results` 和 `error`。
- 网络、超时、认证、请求解析和平台拦截等本地错误继续使用 Sub2API 本地错误格式；不得伪造上游任务成功或失败响应。
- 不记录上游密钥、完整提示词、参考图二进制或 Base64。运营日志仅可记录请求 ID、分组/账号/模型标识、任务 ID、状态、HTTP 状态和已脱敏错误摘要。

## 3. 管理面与平台模型

新增 `grsai` 作为具体账号平台和分组平台，并在管理端账号、分组、渠道的筛选与创建项中展示。管理员为 grsai 单独创建账号、分组和渠道；现有 OpenAI 配置保持不动。

渠道的模型映射和每模型价格继续使用现有渠道定价能力。请求被调度至某一渠道后，结算记录固化该渠道对应的模型、账号和价格快照；后续修改售价不会影响已经提交的任务。

账户凭据使用 grsai 的 API Key 和 Base URL。展示、审计和接口返回沿用现有敏感凭据脱敏规则。

## 4. 可靠结算设计

### 4.1 持久化记录

新增 Ent 实体和数据库表 `grsai_settlements`。每次通过原生路由的提交都先创建一条持久化记录；创建失败时不调用上游。

记录至少包含：

- 内部结算 ID、状态、提交开始时间、创建/更新时间和下次重试时间。
- 用户 ID、API Key ID、分组 ID、账号 ID、渠道模型和价格快照。
- 规范化请求的 SHA-256 指纹，不存储完整请求体。
- 上游任务 ID、上游状态、上游 HTTP 状态和有限长度的脱敏错误摘要。
- 结算幂等键、重试次数、最近一次结算错误和人工核查原因。

数据库约束：

- `billing_idempotency_key` 唯一，格式由内部结算 ID 派生。
- 同一账号的非空 `upstream_task_id` 唯一，避免一个上游任务被建立两条结算记录。
- 结算时锁定该记录，并在同一个数据库事务中写入用量账本和扣减余额，然后标记为 `settled`。重试者只能观察到已结算结果，不能再次扣款。

### 4.2 状态机

```text
submission_pending
  -> awaiting_result          上游返回 running，等待内部查询
  -> settlement_pending       上游返回 succeeded，等待或执行结算
  -> closed_no_charge         上游 HTTP 错误、failed、violation 或无效终态
  -> upstream_unknown         提交结果不确定

awaiting_result
  -> settlement_pending       内部查询得到 succeeded
  -> closed_no_charge         内部查询得到 failed 或 violation
  -> manual_review            超出查询时限或无法确认

settlement_pending
  -> settled                  幂等扣款与用量记录事务成功
  -> manual_review            重试耗尽

upstream_unknown
  -> awaiting_result          已持久化上游任务 ID，可内部查询
  -> manual_review            未取得任务 ID，不能安全重发
```

`succeeded` 是唯一可进入 `settlement_pending` 的上游任务状态。HTTP 4xx/5xx、`failed`、`violation`、无法解析的响应，以及初次返回的 `running` 均不立即扣费。

若 `running` 随后被内部查询为 `succeeded`，该任务进入结算；若未成功则关闭且不收费。内部查询只用于结算恢复，不向用户公开任务查询能力。

### 4.3 提交、响应与扣费顺序

1. 完成现有余额准入、模型映射、价格解析和账号选择，创建 `submission_pending` 记录并固化价格快照。
2. 调用 grsai 原生生成接口。
3. 收到上游响应后，先持久化任务 ID 与上游状态：`succeeded` 转为 `settlement_pending`，`running` 转为 `awaiting_result`，其他结果转为 `closed_no_charge`。
4. 对 `succeeded`，立刻尝试同事务、幂等结算；无论本次结算成功还是暂时失败，均返回已持久化的原始上游成功响应。暂时失败时记录高优先级运营错误，由持久化重试器继续结算；不得把已成功生成伪装成请求失败，诱导用户重复生成。
5. 对 `running`，返回原始响应但不收费，后台使用上游 `GET /v1/api/result?id=<task-id>` 查询最终状态。
6. 对其他结果，关闭为 `closed_no_charge` 后返回上游响应或本地错误。

结算重试由数据库扫描器驱动，不依赖仅内存的 worker queue。默认退避为 1 分钟、5 分钟、15 分钟、1 小时、6 小时；最后一次失败后转 `manual_review` 并产生高优先级运营告警。运维人员可以对已确认成功的记录重新触发同一幂等结算，不能重发生成请求。

### 4.4 不确定提交窗口

上游未声明可由客户端提供的生成幂等键，也不会在 HTTP 请求发送前分配任务 ID。因此，进程恰好在“请求已被上游接收”和“响应中的任务 ID 已持久化”之间中断时，系统无法凭空查询该任务。

恢复扫描发现过期 `submission_pending` 记录时：

- 已有上游任务 ID 的记录进入内部查询流程；
- 没有上游任务 ID 的记录标记为 `upstream_unknown` 后进入 `manual_review` 并告警；
- 绝不自动重发该生成请求，以免重复生成和重复产生上游费用。

这个策略优先避免用户与上游的重复损失。它可能造成极少量已生成但需人工依据上游账单或日志确认的任务；在 grsai 提供提交幂等键或按客户端请求 ID 查询能力前，无法完全自动消除此分布式不确定窗口。

## 5. 失败处理与可观测性

- 任意本地校验失败或持久化首建失败都不发起上游请求。
- 上游 HTTP 错误和明确失败终态不扣费；响应可用时原样返回。
- 结算失败不改变已经获得的上游成功响应。记录包含可关联的内部请求 ID、结算 ID、上游任务 ID 和账号 ID，供运营定位。
- 告警条件：`manual_review`、结算重试耗尽、状态转换非法、上游成功但任务 ID 缺失、用量账本/余额事务失败。
- 不把 grsai 账户可用性错误混入其他平台的临时熔断、账号切换或指标维度。

## 6. 验收与测试

必须覆盖下列场景：

1. grsai 分组发起有效原生请求，得到 `succeeded` 原始 JSON，并准确生成一笔使用记录和一次余额扣减。
2. grsai 特有参数透传，且本地只拒绝 `replyType`、`stream`、`async` 和基础 JSON/`model` 错误；上游可静默替换的参数不在本地拦截。
3. 上游 HTTP 4xx/5xx、`failed`、`violation`、不可解析响应均不扣费。
4. 初始 `running` 不立即扣费；内部查询转 `succeeded` 时恰好扣一次，转失败时不扣费。
5. 对同一结算记录并发执行、进程重启后的重试和人工重放，最终最多扣一次。
6. 结算首次失败但随后重试成功时，客户端仍收到成功的上游响应，账本最终只有一次扣款。
7. `upstream_unknown` 的已知任务 ID 可被内部查询恢复；未知任务 ID 不自动重发，转人工核查。
8. OpenAI、Grok 和其他非 grsai 分组访问新路由被隔离；grsai 账号不会被既有图片或聊天路由选中。
9. 既有 `/v1/images/generations`、`/v1/images/edits`、`/v1/chat/completions` 与 OpenAI 图片计费回归测试保持通过。
10. 管理端可创建和筛选 grsai 账号/分组/渠道，模型售价在提交时被正确快照。

## 7. 实施约束

- 新增平台必须同时更新后端平台常量、账号/分组校验、调度分流、管理端类型与平台选项，避免“可保存但不可路由”或“可路由但不可配置”。
- 结算实体采用现有 Ent schema、生成代码、仓储和事务模式；不以内存队列替代持久化重试。
- 数据库 migration 的实现开始前，必须先定位并阅读仓库规则要求的 `docs/RELEASE_DEPLOYMENT.md`。当前工作区未发现该文件，实施计划须把恢复或确认其权威位置列为前置条件。
- 新增 migration 必须保持升级兼容性、可回滚说明与既有发布流程一致；不在本设计阶段执行 migration 或部署。
- 测试使用假账号、假任务 ID 和假图片 URL，不提交任何真实 API Key、上游账单数据或用户提示词。
