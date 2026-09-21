# GRS.AI 三种下游交付方式技术设计

## 文档信息

| 项目 | 内容 |
| --- | --- |
| 日期 | 2026-09-21 |
| 状态 | 已完成脑暴，待书面评审 |
| 关联 PRD | `2026-09-21-grsai-delivery-modes-prd.md` |
| 基线 | `2026-09-18-grsai-native-images-design.md` |

## 1. 决策摘要

1. GRS.AI 上游请求一律覆盖为 `replyType=stream`。
2. 下游只接受 `replyType=json|stream|async`，缺省为 `json`；不接受 `stream=true` 或
   `async=true`。
3. JSON、Stream、Async 共用同一条上游流解析、持久化状态和结算状态机。
4. 提交时创建价格快照并冻结额度；仅最终 `succeeded` 在幂等事务内结算一次。
5. 不提供用户取消；客户端断线不触发取消或上游重发。

## 2. 交付与状态流

```text
POST /v1/api/generate
  replyType=json   -> 上游 Stream -> 聚合终态 -> 200 JSON
  replyType=stream -> 上游 Stream -> 转发事件 -> SSE 终态
  replyType=async  -> 202 本地任务 -> Worker 上游 Stream -> GET 查询

所有路径：创建任务 -> 冻结额度 -> 绑定上游任务 ID -> 运行中/终态
  succeeded -> 幂等结算 -> settled
  failed|violation -> closed_no_charge + 释放冻结
  无任务 ID的不确定提交 -> manual_review + 释放冻结
```

任务状态扩展为 `queued`、`submitting`、`running`、`pending_settlement`、`settled`、
`closed_no_charge`、`upstream_unknown`、`manual_review`。已有第一阶段记录与状态迁移保持可读；
新状态只用于新增交付能力。

## 3. 协议边界

### 3.1 下游请求

`replyType` 必须是字符串枚举 `json`、`stream` 或 `async`，缺省为 `json`。解析完成后，输入请求体
保持只读；服务端解码单一 JSON 对象并新建上游对象，逐字段复制全部模型参数的原始 JSON 值，最后仅把
新对象的 `replyType` 写为 `stream`。随后重新编码该独立对象作为上游请求体，绝不在原始请求体上删除、
覆盖或复用可变字段。本地请求中的未知或冲突控制字段返回 400；模型特有参数仍不作本地语义改写。

测试必须同时断言：原始请求字节在解析后保持不变；独立上游请求体包含所有允许的原字段值，且唯一的
协议控制差异是 `replyType=stream`。这覆盖 JSON、Stream 与 Async 三条下游路径。

### 3.2 JSON

服务端读取上游 SSE，持续保存进度和任务 ID；收到成功、失败或违规终态后返回聚合的终态 JSON。
首个可见响应之前的本地错误采用项目标准 JSON 错误。客户端在任务 ID 已绑定后断线时，由恢复运行时
接手，而不是再次提交。

### 3.3 Stream

服务端仅在收到并验证首个上游事件后开始 SSE 响应。事件按上游顺序转发，且每个事件先更新可恢复任务
状态。连接断开后停止写入客户端，但已绑定任务仍由恢复运行时查询和结算；若未绑定 ID，转
`manual_review`。

### 3.4 Async 与结果查询

Async 在本地创建公开 ID 后返回 202；Worker 领取 `queued` 任务，使用相同上游 Stream 客户端完成提交。
`GET /v1/api/result?id=...` 可按本地公开 ID 或已绑定上游任务 ID 查找任务。查询条件必须同时匹配
`user_id` 与 `api_key_id`；不匹配与不存在统一为 404。

查询视图仅输出公开 ID、上游任务 ID（若已有）、状态、进度、模型、创建/更新时间、结果 URL 和受限
错误码/摘要。不得输出账号、分组内部 ID、上游凭据、价格快照、加密载荷或内部诊断。

## 4. 载荷与数据保留

Async 任务在开始上游提交前必须保留可恢复的请求载荷。载荷使用项目现有密钥管理能力加密保存，并与
任务归属绑定；原始提示词、参考图片和上游凭据不进入日志、公开 API 或测试产物。

Worker 成功开始处理后删除不再需要的载荷；未提交任务在过期或关闭时删除。终态任务只保留安全查询
视图 24 小时。实现前需确认大请求体的存储上限与清理作业。

## 5. 额度冻结与结算

任务创建时根据模型、图片数量、尺寸和有效用户倍率取得不可变价格快照，并冻结同等可用额度。冻结不是
扣费账本，也不代表上游已经收费。

终态为 `succeeded` 时，领取者在同一幂等结算事务内将冻结转换为一次扣费与用量记录。结算暂时失败时，
任务为 `pending_settlement` 并按既有退避恢复，冻结保持有效。失败、违规、未知提交、人工关闭均释放冻结
并不计费。任何恢复路径不得调用第二次生成。

异步任务另受 API Key 和分组的在途任务上限控制；具体默认值由上线前运营配置确定。

## 6. 不支持取消

不新增取消路由。客户端断线不改变任务状态；上游任务一旦可能已接收，即继续恢复和结算。没有安全绑定
上游任务 ID 的记录保持 `manual_review`，不尝试取消或重发。

## 7. 真实协议探测

新增 `test/grsai_live_contract.py`，以环境变量读取凭据且默认不执行。它必须显式要求
`--live --max-credits 10000`，并且每次运行最多创建一个 `nano-banana-2-lite` 任务。

探测只向上游发送 `replyType=stream`，逐帧验证 Content-Type、事件格式、稳定任务 ID、单调进度、终态和
`GET /v1/api/result` 一致性。调用前后读取 credits，只记录脱敏差额并验证不超过用户给定上限；由于上游
价格可能变更，运行前仍须由操作人确认当前单次价格。测试不进入 CI，也不得保存 Key、完整 prompt、图片
URL、完整原始响应。

## 8. 测试与发布门

| 层级 | 必测内容 |
| --- | --- |
| 单元 | 三态请求解析、上游强制 Stream、SSE 解析/聚合、状态迁移、冻结与释放 |
| 集成 | 双重归属、并发领取、重启恢复、幂等结算、断线后恢复、载荷清理 |
| Live | 单个上游 Stream、任务查询、credits 差额、脱敏输出 |
| 灰度 | JSON 后 Async，最后才开放 Stream；观察 5xx、冻结滞留、manual_review 与重复结算 |

Stream 对外开放门槛：live probe 连续通过、断线后任务可恢复查询、结算无重复、没有未处理的
`manual_review` 异常积压。若任一门槛不满足，保留 JSON/Async 并继续拒绝下游 Stream。

## 9. 非目标

不改造 OpenAI 图片接口；不支持 Composite；不实现上游 async 或非 Stream 请求；不支持任务取消；不以
上游 credits 退款结果替代 Sub2API 本地结算规则。
