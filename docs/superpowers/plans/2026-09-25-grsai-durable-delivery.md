# GRS.AI 持久任务与三种下游交付 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 GRS.AI 原生图片生成实现本地持久任务、上游 Async 恢复、下游 JSON/Stream/Async 三种交付、公开或 presigned 图片链接、任务列表和灰度兼容发布。

**Architecture:** 三种下游模式先在数据库创建本地任务，再由唯一提交 claim 创建上游 Async 任务。上游任务 ID 只保存在内部；Worker 通过 `/result` 恢复，成功图片经现有 `ImageResultUploader` 写入 S3，结果与终态持久化后才交付成功。旧 GRS.AI 结算 Worker 继续处理旧状态，新任务使用 `v2_` 状态前缀，避免灰度期间新旧版本互相领取。

**Tech Stack:** Go、Gin、PostgreSQL、现有 Ent schema/SQL migration、database/sql 租约、现有 S3 兼容存储、Wire、Go tests、httptest、真实上游 Async live probe。

**Spec:** `docs/superpowers/specs/2026-09-24-grsai-durable-delivery-prd.md`；`docs/superpowers/specs/2026-09-24-grsai-durable-delivery-design.md`

**Command directory:** 下文 `go` 命令均在 `backend/` 目录执行；Git 和文档命令在仓库根目录执行。

## Global Constraints

- 下游只接受 `replyType=json|stream|async`，缺省 `json`；上游请求固定为 `replyType=async`。
- 本地任务 ID 必须先持久化；所有下游输出不得包含上游任务 ID、上游图片 URL、账号、价格或原始请求。
- 已绑定上游 ID 的恢复只能调用上游 `/result`，禁止再次 POST；无上游 ID 的不确定提交直接失败并释放冻结。
- 上游成功后必须先下载全部图片并保存到本地 S3，再写入结果和终态；转存失败不得报告成功。
- 图片 URL 复用 `ImageStorage.Save`：配置 `public_base_url` 返回公开 URL，否则返回 S3 presigned URL；不新增 Sub2API 图片代理接口。
- 任务列表和 `/result` 仍要求原 API Key，并按 `user_id + api_key_id` 过滤；图片 URL 不要求 API Key。
- presigned URL 默认有效期 24 小时，公开 URL 的 `link_expires_at` 为 null；过期不重新签发，S3 对象沿用既有清理策略。
- Async 单用户默认最多 20 个等待任务、3 个运行中任务；计数必须基于持久状态和租约，跨重启、跨槽位有效。
- 新任务使用 `v2_` 内部状态；旧 Worker 的旧状态查询不得接触新任务。新版本必须保留旧恢复运行时。
- 兼容发布期间保留旧 `GrsaiNativeClient.Generate` 和旧网关行为；新 Async 提交使用独立方法，启用开关只控制创建 v2 任务，不停止 v2 Worker 恢复。
- 数据库迁移只追加字段、索引和默认值；不能破坏仍在运行的旧槽位。提交日志使用中文。
- live probe 默认离线；真实运行必须显式指定环境和目标模型，不进入 CI，不记录密钥、提示词或图片 URL。
- 生产灰度只允许按 `AGENTS.md` 使用 `gray-status.sh`、`gray-deploy.sh`、`gray-set-traffic.sh`、`gray-promote.sh` 和不可变 `sha-<commit>` 镜像；不得初始化、销毁槽位或删除数据卷。

---

## Task 1: 固定协议并建立 v2 任务领域契约

**Files:**
- Create: `backend/internal/service/grsai_delivery.go`
- Create: `backend/internal/service/grsai_delivery_test.go`
- Modify: `backend/internal/service/grsai_native.go`
- Modify: `backend/internal/service/grsai_native_test.go`

**Interfaces:**
- `ParseGrsaiDeliveryRequest(raw []byte) (*GrsaiDeliveryRequest, error)`；输入字节保持不变。
- `GrsaiDeliveryMode`：`json`、`stream`、`async`。
- `GrsaiDeliveryRequest`：`Mode`、`OriginalBody`、`UpstreamBody`、`Model`、`ImageCount`、`ImageSize`。
- `PrepareGrsaiGenerateBody` 和既有 `Generate` 保留给旧链路；新交付链路使用 `GenerateAsync(ctx, account, UpstreamBody)`，不得改变旧请求行为。

- [ ] **Step 1: 写失败测试。** 覆盖缺省 JSON、三种模式、非法字符串、布尔 `stream/async`、重复控制字段、非对象 JSON、原始请求字节不变，以及上游 body 仅将 `replyType` 改为 `async`。
- [ ] **Step 2: 运行测试确认失败。**

Run: `go test ./internal/service -run 'TestParseGrsaiDelivery|TestGrsaiNativeClient' -count=1`

Expected: FAIL，因为新解析器和 Async body 构造尚不存在。

- [ ] **Step 3: 实现最小协议层。** 使用 `json.RawMessage` 深拷贝字段，拒绝旧布尔控制字段，独立重建上游对象；扩展 `GrsaiUpstreamResult` 解析结果状态、任务 ID、图片结果和有限错误。
- [ ] **Step 4: 增加并行 Async 方法。** `GenerateAsync` 使用新的 body，`Accept`/`Content-Type` 保持 JSON；`Result` 保留 `/v1/api/result?id=...`；httptest 断言新方法三种下游模式都只向上游发送 `replyType=async`，旧 `Generate` 仍发送原生 JSON。
- [ ] **Step 5: 运行 focused suite 并提交。**

Run: `go test ./internal/service -run 'Test(ParseGrsaiDelivery|GrsaiNativeClient)' -count=1`

Expected: PASS。

Commit: `git add backend/internal/service/grsai_delivery.go backend/internal/service/grsai_delivery_test.go backend/internal/service/grsai_native.go backend/internal/service/grsai_native_test.go; git commit -m "feat: 固定 GRS.AI 上游异步协议"`

## Task 2: 增加持久本地任务、载荷和 v2 状态仓储

**Files:**
- Create: `backend/migrations/243_grsai_durable_delivery.sql`
- Modify: `backend/ent/schema/grsai_settlement.go`
- Regenerate: `backend/ent/`
- Modify: `backend/internal/service/grsai_settlement.go`
- Modify: `backend/internal/repository/grsai_settlement_repo.go`
- Create: `backend/internal/repository/grsai_task_payload_repo.go`
- Create: `backend/internal/repository/grsai_task_payload_repo_test.go`
- Create/modify: `backend/internal/repository/grsai_settlement_repo_integration_test.go`

**Interfaces:**
- `GrsaiSettlement` 增加本地公开 ID、交付模式、公开状态/进度、结果 JSON、链接到期时间、对象元数据和 v2 claim 字段。
- `CreateV2GrsaiTask` 在本地任务 ID、API Key 归属和冻结准备完成后创建 `v2_queued` 记录。
- `ClaimDueV2`, `BindV2UpstreamTask`, `UpdateV2Progress`, `CompleteV2`, `FailV2`, `ListOwnedV2`, `GetOwnedV2`。
- `GrsaiTaskPayloadRepository`：`PutEncrypted`、`GetDecrypted`、`DeleteByTaskID`、`DeleteExpired`。

- [ ] **Step 1: 写真库失败测试。** 验证本地 ID 不可预测且唯一、`user_id + api_key_id` 精确查询、另一 Key 统一 not found、v2 状态不被旧 `ClaimDue` 返回、两个 Worker 只有一个 claim、载荷数据库中不出现明文。
- [ ] **Step 2: 运行相关集成测试确认失败。**

Run: `go test ./internal/repository -run 'TestGrsai(V2|TaskPayload|SettlementRepository)' -count=1`

Expected: FAIL，因为迁移字段和 v2 仓储不存在。

- [ ] **Step 3: 编写只追加迁移。** 在 `grsai_settlements` 增加本地任务 ID、交付模式、v2 状态、进度、结果 JSON、公开链接到期时间、S3 对象元数据和任务版本字段；增加 owner/list、v2 due、public ID 唯一索引；增加加密载荷表。所有新字段有兼容默认值，旧插入仍可执行。
- [ ] **Step 4: 更新 Ent 和扫描/写入代码。** 运行项目既有 Ent 生成命令，不手改生成文件；保留旧状态 SQL 语义，v2 查询只使用 `v2_` 前缀。
- [ ] **Step 5: 实现加密载荷仓储。** 使用现有 `SecretEncryptor`，加密后才入库，绑定上游 ID 或进入终态时删除；不在错误和日志中打印明文/密文。
- [ ] **Step 6: 运行集成回归并提交。**

Run: `go generate ./ent; go test ./ent/... ./internal/repository -run 'TestGrsai' -count=1`

Expected: PASS，且既有 GRS.AI settlement integration tests 不回归。

Commit: `git add backend/migrations/243_grsai_durable_delivery.sql backend/ent backend/internal/service/grsai_settlement.go backend/internal/repository/grsai_settlement_repo.go backend/internal/repository/grsai_task_payload_repo.go backend/internal/repository/grsai_task_payload_repo_test.go backend/internal/repository/grsai_settlement_repo_integration_test.go; git commit -m "feat: 持久化 GRS.AI 本地任务"`

## Task 3: 补齐 S3 URL 元数据和结果持久化边界

**Files:**
- Modify: `backend/internal/service/image_storage.go`
- Modify: `backend/internal/repository/image_storage_s3.go`
- Modify: `backend/internal/service/image_storage_settings.go`
- Modify: `backend/internal/service/image_storage_test.go`
- Create: `backend/internal/repository/image_storage_s3_test.go`
- Create: `backend/internal/service/grsai_result_persistence.go`
- Create: `backend/internal/service/grsai_result_persistence_test.go`

**Interfaces:**
- 保留现有 `ImageStorage.Save` 兼容所有调用者；增加内部可选 `ImageStorageSaveMetadata`，返回 URL、`link_expires_at`、content type 和对象 key。
- `PersistGrsaiImages(ctx, localTaskID, upstreamResult) (storedResult, error)` 必须保存全部图片后才返回成功。

- [ ] **Step 1: 写失败测试。** 覆盖 `public_base_url` 返回公开 URL 且到期时间为 null；无公开 URL 返回 presigned URL 且记录配置时长；配置变化不改变已生成链接；单张图片失败时不返回部分成功结果；重试使用稳定 task/index key。
- [ ] **Step 2: 运行 S3/uploader 测试确认失败。**

Run: `go test ./internal/service ./internal/repository -run 'Test(ImageStorage|GrsaiResultPersistence|ImageResultUploader)' -count=1`

Expected: FAIL，因当前存储接口没有链接元数据和任务结果编排。

- [ ] **Step 3: 扩展 S3 实现。** 公开 URL 分支返回 null expiry；presigned 分支使用 `presign_expiry_hours`，记录签名生成时间和过期时间；不得生成 Sub2API 图片代理地址。
- [ ] **Step 4: 实现结果持久化。** 复用现有下载、base64 解码、S3 PutObject 和 MIME 检测；将所有 URL、expiry 和必要的对象元数据写入本地任务结果，禁止保存上游 URL。
- [ ] **Step 5: 运行回归并提交。**

Run: `go test ./internal/service ./internal/repository -run 'Test(ImageStorage|GrsaiResultPersistence|ImageResultUploader)' -count=1`

Expected: PASS。

Commit: `git add backend/internal/service/image_storage.go backend/internal/repository/image_storage_s3.go backend/internal/service/image_storage_settings.go backend/internal/service/grsai_result_persistence.go backend/internal/service/grsai_result_persistence_test.go; git commit -m "feat: 保存 GRS.AI 图片链接元数据"`

## Task 4: 实现 v2 Worker、冻结计数和重启恢复

**Files:**
- Create: `backend/internal/service/grsai_task_service.go`
- Create: `backend/internal/service/grsai_task_runtime.go`
- Create: `backend/internal/service/grsai_task_runtime_test.go`
- Create: `backend/internal/service/grsai_task_limits.go`
- Create: `backend/internal/service/grsai_balance_hold.go`
- Create: `backend/internal/service/grsai_balance_hold_test.go`
- Modify: `backend/internal/service/grsai_settlement.go`
- Modify: `backend/internal/service/grsai_settlement_recovery.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/repository/usage_billing_repo_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/cmd/server/wire.go`
- Regenerate: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- `GrsaiTaskService.Create(ctx, input)`, `SubmitOnce(ctx, claim)`, `GetOwned(ctx, userID, apiKeyID, localID)`, `ListOwned(ctx, userID, apiKeyID, page)`。
- `GrsaiTaskRuntime.Start()`, `Stop()`, `RunOnce(ctx)`。
- `GrsaiTaskLimits`：`MaxWaitingPerAPIKey=20`、`MaxRunningPerAPIKey=3`，均配置化。
- `ReserveGrsaiBalance`、`CaptureGrsaiBalanceTx`、`ReleaseGrsaiBalanceTx`：以本地任务 ID 为幂等键，在结算状态转换的同一事务中捕获或释放冻结；不可复用 Batch Image 的任务身份。

- [ ] **Step 1: 写失败恢复/冻结测试。** 覆盖 queued → submitting → bound running、上游 Async POST 只调用一次、重启后 bound 任务只调用 Result、running/succeeded/failure 状态转换、S3 失败重试、无上游 ID 失败解冻、数据库写入失败不报告成功；冻结、捕获、释放均通过同一幂等任务键测试。
- [ ] **Step 2: 写 20/3 限额测试。** 20 个等待任务允许，第 21 个返回容量错误；3 个运行任务后不再领取第 4 个；任务完成或失败后释放计数；新 Worker 重启后从数据库重新计算。
- [ ] **Step 3: 运行测试确认失败。**

Run: `go test ./internal/service -run 'TestGrsai(Task|BalanceHold)' -count=1`

Expected: FAIL，因为 v2 服务和 Worker 不存在。

- [ ] **Step 4: 实现状态机、冻结和唯一提交 claim。** 创建任务时价格快照与冻结必须成功，才允许任务进入可提交状态；Async 载荷加密保存；Worker 领取 queued 后只允许一次 POST；响应先绑定上游 ID，再删除载荷；没有 ID 的不确定提交以事务 FailV2 + release。冻结失败不能上游 POST，释放失败不能宣称已解冻。
- [ ] **Step 5: 实现 Result 轮询和结果落库顺序。** 只 GET 上游 `/result`；成功先执行图片转存，再保存结果/终态，最后执行幂等 capture；失败/违规 release；结算异常进入可恢复 pending 状态。
- [ ] **Step 6: 接入配置和生命周期。** 增加 `grsai_delivery.enabled`、扫描间隔、batch、payload TTL、结果保留期、waiting/running 上限；保留旧 `GrsaiSettlementRecoveryRuntime`，新 runtime 单独启动/停止。
- [ ] **Step 7: 运行 service/repository 回归并提交。**

Run: `go test ./internal/service ./internal/repository ./internal/config ./cmd/server -run 'TestGrsai|TestWire' -count=1`

Expected: PASS，且旧 settlement recovery tests 全部通过。

Commit: `git add backend/internal/service/grsai_task_service.go backend/internal/service/grsai_task_runtime.go backend/internal/service/grsai_task_limits.go backend/internal/service/grsai_balance_hold.go backend/internal/service/grsai_balance_hold_test.go backend/internal/service/grsai_settlement.go backend/internal/service/grsai_settlement_recovery.go backend/internal/repository/usage_billing_repo.go backend/internal/repository/usage_billing_repo_test.go backend/internal/config/config.go backend/internal/service/wire.go backend/internal/repository/wire.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go; git commit -m "feat: 增加 GRS.AI 持久任务恢复 Worker"`

## Task 5: 提供三种下游交付、查询和任务列表

**Files:**
- Modify: `backend/internal/handler/grsai_gateway.go`
- Modify: `backend/internal/handler/grsai_gateway_test.go`
- Create: `backend/internal/handler/grsai_delivery_handler_test.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/server/routes/gateway_test.go`
- Modify: `backend/internal/handler/wire.go`

**Interfaces:**
- `POST /v1/api/generate` 调用 `GrsaiTaskService`，不直接把上游响应透传给下游。
- `GET /v1/api/result?id=<local-id>` 返回 owner-scoped `GrsaiTaskView`。
- `GET /v1/api/tasks` 返回当前 API Key 的分页任务列表。

- [ ] **Step 1: 写失败 handler 测试。** 覆盖 Async 202 只返回本地 ID、JSON 等待最终结果、Stream 顺序为本地 ID/5/running/100/succeeded、错误不带上游 ID/URL、列表分页。
- [ ] **Step 2: 写鉴权测试。** Key A 可查自己的任务，Key B/另一用户/无 Key 不能查；图片 URL 字段本身是公开或 presigned URL，不要求增加下载路由。
- [ ] **Step 3: 写失败路由测试并实现路由。** 注册 `/api/result` 和 `/api/tasks`，保留既有认证中间件；空 ID、未知 ID、非 owner 统一 404。
- [ ] **Step 4: 实现 JSON/Stream/Async 行为。** JSON 连接只在本地成功结果入库后返回；Stream 只发送合成进度，不透传上游帧；HTTP 断开停止写响应但不取消 Worker；Async 立即返回本地 ID。
- [ ] **Step 5: 限制公开视图。** 仅返回本地 ID、状态、进度、模型、时间、错误摘要、图片 URL、link expiry；公开 URL expiry 为 null，presigned expiry 使用落库时间；不返回内部对象 key 和上游字段。
- [ ] **Step 6: 运行 API 回归并提交。**

Run: `go test ./internal/handler ./internal/server/routes -run 'TestGrsai' -count=1`

Expected: PASS，且现有 `/v1/images/*` 路由测试不回归。

Commit: `git add backend/internal/handler/grsai_gateway.go backend/internal/handler/grsai_gateway_test.go backend/internal/handler/grsai_delivery_handler_test.go backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_test.go backend/internal/handler/wire.go; git commit -m "feat: 提供 GRS.AI 三种下游交付"`

## Task 6: 完成 AC-ID 追溯和灰度兼容验证

**Files:**
- Create: `backend/internal/repository/grsai_gray_compatibility_test.go`
- Create: `backend/internal/service/grsai_gray_compatibility_test.go`
- Create: `test/grsai_live_contract.py`
- Create: `test/README.grsai-live-contract.md`
- Modify: `docs/superpowers/specs/2026-09-24-grsai-durable-delivery-design.md` (用真实测试文件替换计划追溯项)

**Interfaces:**
- 离线 probe 自测默认禁止网络；显式 live 模式只创建一个 Async 任务并轮询 `/result`。
- 灰度兼容测试验证旧状态只被旧 recovery 查询，v2 状态只被 v2 Worker 查询。

- [ ] **Step 1: 写 AC-01..15 追溯检查。** 每条 AC 至少关联一个实际 Go/Python 测试；加入脚本检查需求 ID 不孤儿、测试不幽灵。
- [ ] **Step 2: 写离线 live probe 自测。** 无 `--live` 不发网络请求；脱敏输出不含 key、prompt、完整响应和图片 URL；命令参数限定目标模型和显式额度上限。
- [ ] **Step 3: 写灰度演练测试。** 在共享测试数据库中创建旧任务和 v2 任务，启动旧 recovery 与 v2 runtime，断言互不领取；模拟候选停止、稳定槽位接手、切流、回滚、晋升，断言 POST、Result、S3、冻结和结算各只发生预期次数。
- [ ] **Step 4: 运行离线和全量测试。**

Run: 仓库根目录执行 `python test/grsai_live_contract.py --self-test`；`backend/` 目录依次执行 `go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./internal/config ./cmd/server -count=1`、`go vet ./internal/service ./internal/repository ./internal/handler ./internal/server/routes`。

Expected: PASS；无 flags 的 probe 不联网；在 `backend/` 执行 `go generate ./ent`，再于仓库根目录执行 `git diff --exit-code -- backend/ent`。

- [ ] **Step 5: 真实验证。** 经明确授权后运行 live probe，验证 Async POST、running、succeeded、`/result` 和真实 S3 URL；保存脱敏结果，不保存任何密钥或图片 URL。
- [ ] **Step 6: 更新追溯文档并提交。**

Commit: `git add backend/internal/repository/grsai_gray_compatibility_test.go backend/internal/service/grsai_gray_compatibility_test.go test/grsai_live_contract.py test/README.grsai-live-contract.md docs/superpowers/specs/2026-09-24-grsai-durable-delivery-design.md; git commit -m "test: 增加 GRS.AI 灰度兼容验收"`

## Task 7: 执行生产灰度发布门

**Files:**
- Read only: `AGENTS.md`
- Read only on server: `/opt/sub2api/gray-status.sh` output and the existing gray scripts
- Evidence: candidate health, Nginx config check, logs, task IDs, S3 URLs, billing records

- [ ] **Step 1: 发布前确认。** 运行 `ssh tenxunyun.guigu /opt/sub2api/gray-status.sh`，确认稳定/候选槽位、镜像和健康状态；确认数据库 migration 已在兼容版本完成，旧 recovery 和 v2 Worker 配置正确。
- [ ] **Step 2: 部署兼容版本到 0%。** 使用 `gray-deploy.sh <候选槽位> sha-<commit>`；候选版本只识别/恢复 v2 状态，不开放新任务创建；验证 `/health`、Nginx、旧任务恢复和无异常 5xx。
- [ ] **Step 3: 晋升兼容版本。** 在 0% 验证通过后按仓库脚本完成兼容版本晋升；稳定槽位必须具备 v2 Worker 后才允许创建 v2 任务。
- [ ] **Step 4: 部署功能版本到 0%。** 创建 JSON、Stream、Async 任务，验证本地 ID、上游 Async、S3 URL、任务列表、链接期限、账务和 20/3 限额；模拟候选停止后由稳定槽位恢复。
- [ ] **Step 5: 逐步切流。** 只用 `gray-set-traffic.sh` 扩大流量；每个阶段检查运行中连接、等待任务、重复 POST/结算、S3 访问、鉴权和最近错误日志；流量切换不搬迁已有连接。
- [ ] **Step 6: 回滚演练和观察。** 模拟候选异常，先切回稳定槽位，保留候选容器/日志/备份；确认兼容稳定槽位继续处理 v2 任务，不执行 `compose down`、不删卷、不恢复数据库备份。
- [ ] **Step 7: 达到 100% 后观察并晋升。** 观察任务恢复、冻结滞留、manual review、S3 转存失败、5xx/超时和重复账务；通过后运行 `gray-promote.sh`，再用 `gray-status.sh` 核对槽位、镜像和健康状态。

## Final Verification Checklist

- [ ] PRD 的 GRSAI-DM-AC-01..15 均有测试映射和执行证据。
- [ ] 上游只走 Async；下游 JSON/Stream/Async 都先创建本地任务 ID。
- [ ] 旧状态与 v2 状态在 SQL 领取条件上隔离；旧 Worker 不会领取 v2 任务。
- [ ] 进程重启、客户端断开、候选停止和灰度回滚都不会重复 POST 或重复结算。
- [ ] 图片先落本地 S3，再保存结果/终态；公开 URL/presigned URL 直接访问，不新增图片代理。
- [ ] 任务列表和 `/result` 仍按 API Key 归属鉴权；图片 URL 不受 Sub2API API Key 鉴权。
- [ ] public URL 的 expiry 为 null；presigned URL 默认 24 小时且过期不重签。
- [ ] 单用户等待 20、运行 3 的限额跨重启和跨槽位正确。
- [ ] `go test`、`go vet`、Ent 生成一致性和 live probe 离线自测通过。
- [ ] 生产只使用仓库灰度脚本和 `sha-<commit>`，无未验证放量或破坏性回滚。

## Self-Review

| 设计要求 | 计划覆盖 |
| --- | --- |
| 三种下游模式、上游 Async、先本地 ID | Tasks 1, 4, 5 |
| v2 持久状态、owner 查询、加密载荷 | Task 2 |
| 图片本地 S3、公开/presigned URL、链接期限 | Task 3 |
| 重启恢复、无 ID 失败解冻、一次结算 | Task 4 |
| 任务列表、/result、Stream 合成进度 | Task 5 |
| AC-ID 追溯、live、灰度兼容 | Task 6 |
| 新旧槽位发布、回滚、运行中/等待中任务 | Task 7 |

自检：计划不使用未决占位符；每个开发任务列明文件、接口、失败测试、执行命令、预期结果和提交点。执行生产灰度前必须在服务器以 `gray-status.sh` 确认实际脚本路径、稳定槽位和候选槽位；本计划未执行任何生产操作。
