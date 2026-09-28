# GRS.AI 视频生成增量扩展 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 `/v1/api/generate` 的 GRS.AI 持久任务链路中支持按秒计费的视频生成、MP4 本地 S3 交付和安全灰度，同时保持图片与历史任务行为。

**Architecture:** 沿用现有 API Key、分组、账号、渠道定价、三模式交付、余额冻结和 v2 持久任务。模型在当前分组解析出 `billing_mode=video` 时选择视频路径；新视频行用 `task_version=3` 隔离已部署的只处理 `task_version=2` 图片 worker，新 worker 同时领取两种版本，并按任务快照分别持久化和结算。

**Tech Stack:** Go 1.27、Gin、Ent、PostgreSQL、`database/sql`、S3 兼容对象存储、Vue 3、Vitest、Go `httptest` 与仓库已有集成测试。

**Spec:** `docs/superpowers/specs/2026-09-28-grsai-video-generation-spec.md`；复用 `docs/superpowers/specs/2026-09-24-grsai-durable-delivery-design.md` 和 `docs/superpowers/plans/2026-09-25-grsai-durable-delivery.md` 中已经实现的能力。

**Command directory:** `go` 命令在 `backend/` 执行；`pnpm` 命令在 `frontend/` 执行；Git 命令在仓库根目录执行。所有提交日志使用中文。计划中的代码片段是目标契约与测试示例，以仓库当前类型的精确签名为准；涉及的字段、版本号和状态不可任意替换。

## Global Constraints

- 唯一创建入口是 `POST /v1/api/generate`；下游仅 `replyType=json|stream|async`，默认 `json`；上游固定 `async`。查询及列表仍按 `user_id + api_key_id` 隔离。
- 使用现有分组→渠道定价优先级；仅明确配置来源的 `BillingModeVideo` 可进入视频路径，不能依据 `minimax-h3` 字符串推断媒体类型；禁止回落到 LiteLLM、Grok 专属视频价格或缺档位时的平价。
- 视频要求整数秒 `duration > 0`，有档位时须显式精确命中 `resolution`；`minimax-h3` 为 480p/768p/1080p、1–15 秒，1080p 不超过 10 秒。其他视频模型只校验通用计费字段与其已配置档位。
- `prompt`、`aspectRatio`、`images`、`audios`、`seed` 和未来非控制字段以 `json.RawMessage` 透传；本地控制字段和价格内部元数据不能透传。
- 基础费用 = 解析出的每秒单价 × 请求时长；再取现有用户/分组有效倍率或 `resolveVideoRateMultiplier` 的视频独立倍率，乘账号倍率，依现有 DECIMAL 精度存快照、冻结和结算。后续改价不追溯。
- 上游原始视频 URL、任务 ID、提示词与鉴权材料不得进入公开视图或错误日志；只在 MP4 转存、结果入库、冻结捕获同一终态事务完成后公开成功。
- 保留图片 v1/v2 的原有 SQL、结果 JSON、S3 图片下载上限及结算路径；新字段仅追加且有兼容默认值。视频用 `task_version=3`，旧编译版本的 v2 worker 只领取 `task_version=2`。
- 失败/违规释放冻结；有上游 ID 只轮询结果，无 ID 的不确定提交不重发；客户端断线不取消 worker。视频对象写入必须有界、可重试、幂等且防止内网地址访问。
- 共享数据库发布：先确保**稳定槽位**具备处理 v3 的兼容 worker，再由候选槽位创建视频任务；不能仅凭候选 0% 就建立旧稳定槽位无法恢复的冻结任务。只用 `AGENTS.md` 指定灰度脚本及 `sha-<commit>` 镜像，不运行 bootstrap/down，不删卷、不恢复数据库。

---

## 文件结构与责任

| 责任 | 目标文件 |
| --- | --- |
| 解析请求及视频定价快照 | `backend/internal/service/grsai_delivery.go`、`grsai_settlement.go`、`grsai_task_service.go`；新增 `grsai_video_pricing.go` 保存严格档位和模型约束 |
| 兼容迁移、版本化领取、冻结 | `backend/migrations/245_grsai_video_tasks.sql`、`backend/ent/schema/grsai_settlement.go`、`backend/internal/repository/grsai_settlement_repo.go`、`backend/internal/service/grsai_balance_hold.go` |
| 上游视频结果和有界媒体存储 | `backend/internal/service/grsai_native.go`、新增 `grsai_video_persistence.go`、`grsai_video_download.go`、`backend/internal/repository/image_storage_s3.go`；`image_storage_settings.go` 仅负责注入同一 S3 配置 |
| 任务终态、用量和对外视图 | `backend/internal/service/grsai_task_runtime.go`、`grsai_task_view.go`、`backend/internal/handler/grsai_delivery_handler.go` |
| 分组开关文字 | `frontend/src/i18n/locales/zh/admin/overview.ts`、`frontend/src/i18n/locales/en/admin/overview.ts`；`GroupsView.vue` 仅当既有键无法在 GRS.AI 范围内显示时调整 |

> `image_object_metadata` 列与旧图片调用方保持原名；视频对象元数据可按 `media_kind` 写入同列、在新 Go 领域对象中以中性名称表示。不要为了改名破坏旧 Ent 代码。`requested_image_count` 在视频行仍填 1 以满足旧 `NOT NULL` 约束，仅是内部兼容占位；视频金额严格乘 `video_duration_seconds`，视频用量的 `image_count` 不得记为 1。

## 依赖和交付顺序

`Task 1 请求/定价 → Task 2 迁移/仓储 → Task 3 冻结/结算 → Task 5 runtime`；`Task 4 上游/MP4` 在 Task 2 之后，与 Task 3 相互独立；`Task 6 接口/界面/回归` 依赖 Task 5；`Task 7 灰度` 依赖前六项。每项开发任务先写失败测试、确认失败、做最小改动、复测、中文提交；同一任务里的准备、测试和文档随该任务提交。实施时严格串行处理同一状态/账务链路。

### Task 1: 请求和现有渠道视频定价契约

**Files:**
- Modify: `backend/internal/service/grsai_delivery.go`、`grsai_delivery_test.go`、`grsai_settlement.go`、`grsai_settlement_test.go`、`grsai_task_service.go`、`grsai_task_service_test.go`
- Create: `backend/internal/service/grsai_video_pricing.go`、`grsai_video_pricing_test.go`

**Interfaces:**
- 保留 `GrsaiPricingResolver.GrsaiUnitPrice(ctx, model, group)` 给 v1 图片调用方；新增 `ResolveGrsaiTaskPrice(ctx context.Context, model string, group *Group, resolution string) (GrsaiTaskPrice, error)` 给任务创建，`GrsaiTaskPrice{Mode BillingMode; UnitPrice float64; Resolution string}`。
- `GrsaiDeliveryRequest` 新增 `DurationSeconds int`、`Resolution string`，视频字段解析错误由视频分支显式报错，图片历史默认行为不变。
- `ValidateGrsaiVideoRequest(model, resolution string, duration int) error`：`minimax-h3` 文档限制加通用正整数/配置档位规则；不建立全局模型分类表。

- [ ] **Step 1: 写失败测试。** 在 `grsai_video_pricing_test.go` 构造同一模型的 group/channel 视频档位 480p=0.10、768p=0.14、1080p=0.30，断言当前解析优先级、精确档位、平价无档位、显式 0 美元价格与缺价不同；对 tier 存在但无匹配时拒绝。`grsai_delivery_test.go` 检查 `duration` 的整数/负数/缺失、`resolution` 类型，并断言非控制 JSON 值完整透传。

```go
func TestGrsaiVideoPriceRequiresExactConfiguredTier(t *testing.T) {
    // 通过现有 ModelPricingResolver 的 group/channel 测试仓储提供 video 价格。
    got, err := resolver.ResolveGrsaiTaskPrice(ctx, "minimax-h3", group, "768p")
    require.NoError(t, err)
    require.Equal(t, BillingModeVideo, got.Mode)
    require.InDelta(t, 0.14, got.UnitPrice, 1e-10)
    _, err = resolver.ResolveGrsaiTaskPrice(ctx, "minimax-h3", group, "720p")
    require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
}
```

- [ ] **Step 2: 确认测试先失败。** `go test ./internal/service -run 'TestGrsai(VideoPrice|VideoRequest|TaskService)' -count=1`，预期新价格接口、时长和严格档位测试失败。
- [ ] **Step 3: 实现视频路径。** 先 `ModelPricingResolver.Resolve(PricingInput{Model:model, GroupID:&group.ID, Group:group})`；对 video 只接受 `PricingSourceGroup/Channel`；若 `len(RequestTiers)>0`，按 `TierLabel == resolution` 找非 nil `PerRequestPrice`，拒绝缺档与重复歧义档，不走默认价；若无 tier，则要求配置了 `channelPricing.PerRequestPrice` 并按每秒价使用。原图片路径继续拒绝 tier 和 video。`duration` 用严格 JSON 整数解码，不允许浮点、字符串或溢出；完整上游 body 只替换 `replyType`，不替换参考图/音频字段。
- [ ] **Step 4: 在 Create 入口先确定 Mode 后计算快照。** 视频用 `resolveVideoRateMultiplier(input.APIKey, effectiveGroupRate)`；图片保留 `resolveImageRateMultiplier`。通用视频只要求正时长和配置档位，`minimax-h3` 补 1–15 秒及 1080p≤10 秒限制；拒绝阶段必须早于 `CreateV2GrsaiTask`，无上游 POST、无冻结。不得把 Grok `NormalizeVideoBillingResolutionOrDefault` 用于 GRS.AI 768p。
- [ ] **Step 5: 复测并提交。** `go test ./internal/service -run 'Test(ParseGrsaiDelivery|GrsaiVideo|GrsaiTaskService|GrsaiSettlement)' -count=1`；预期通过，原图模型继续只按图片数收费。提交：`git add` 上述文件，`git commit -m "feat: 解析 GRS.AI 视频请求与渠道价格"`。

### Task 2: 兼容迁移与 v3 任务领取边界

**Files:**
- Create: `backend/migrations/245_grsai_video_tasks.sql`
- Modify: `backend/ent/schema/grsai_settlement.go`，生成 `backend/ent/`；`backend/internal/service/grsai_settlement.go`、`backend/internal/repository/grsai_settlement_repo.go`
- Test: `backend/internal/repository/grsai_settlement_repo_integration_test.go`、`grsai_gray_compatibility_test.go`

**Interfaces:**
- `CreateGrsaiSettlementParams` / `GrsaiSettlement` 新增 `MediaKind string`（旧行 `image`）、`VideoDurationSeconds int`（旧行 0）、`VideoResolution string`（旧行空）；`CreateV2GrsaiTask` 接受图片 task_version=2 和视频 task_version=3，仅新版本将视频写为 3。
- 保留 `ClaimDueV2` 名称和旧 v2 SQL 的源码语义作为回归基线；新实现可以让它同时领取 2/3，或增加 `ClaimDueVideoV3` 并在运行时合并，两者必须保证同一用户共享 20/3 配额、同一租约/claim 互斥。旧已部署二进制只查询 2，不能因新迁移改变行为。

- [ ] **Step 1: 写真实 PostgreSQL 失败测试。** 迁移后旧格式图片 INSERT 应填出 `media_kind='image'`、duration=0；创建 v3 视频，断言旧版本的 `task_version=2` 领取 SQL 返回 0 行，新版本领取 v2 图片与 v3 视频各一次；两 worker 并发只有一个胜者，负载约束跨版本共计。查询仍限制 `user_id + api_key_id`。

```sql
ALTER TABLE grsai_settlements
    ADD COLUMN IF NOT EXISTS media_kind VARCHAR(16) NOT NULL DEFAULT 'image',
    ADD COLUMN IF NOT EXISTS video_duration_seconds INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS video_resolution VARCHAR(32) NOT NULL DEFAULT '';
-- 仅 v3 视频写入新值；旧二进制的 task_version=2 谓词不变。
```

- [ ] **Step 2: 运行红灯。** `CI=1 go test -tags integration ./internal/repository -run 'TestGrsai(Video|Gray)' -count=1`，预期 schema/领取测试失败；若 Docker 不可用，先恢复测试环境，不得把跳过当成通过。
- [ ] **Step 3: 实现只追加迁移、Ent 字段和 SQL。** 保留 `requested_image_count` NOT NULL，视频写 1 占位；扫描、INSERT/RETURNING 都读写三个新字段。校验视频 `task_version==3`、`media_kind==video`、duration>0，图片 `task_version==2` 和 image；旧 v1 写入走原默认。新 worker 的 Claim/Bind/Progress/Complete/Fail/GetOwned/ListOwned 的版本限制改为 2/3，并依靠行锁中的 `TaskVersion`、`ClaimVersion` 校验。旧 worker 已编译的 `task_version=2` SQL 仍只能处理图片。
- [ ] **Step 4: 用同一 capacity advisory lock 跨版本统计排队/运行任务。** 旧编译二进制只统计 v2；兼容 worker 成为稳定槽位后才开放 v3 创建，避免在过渡期由两个版本分别计数造成超限。新代码中的 owner 查询返回两种版本及历史图片；不能放宽到旧 v1 私有记录。
- [ ] **Step 5: 验证并提交。** `go generate ./ent`、`CI=1 go test -tags integration ./internal/repository -run 'TestGrsai' -count=1`、`go test ./internal/service ./internal/repository -run 'TestGrsai' -count=1`，再 `git diff --exit-code -- backend/ent`（在生成文件已加入暂存后核对）。预期旧图/v2/v3 状态互不误领。提交：`git add backend/migrations/245_grsai_video_tasks.sql backend/ent backend/internal/service/grsai_settlement.go backend/internal/repository/grsai_settlement_repo.go backend/internal/repository/grsai_settlement_repo_integration_test.go backend/internal/repository/grsai_gray_compatibility_test.go; git commit -m "feat: 持久化视频任务并隔离旧版 worker"`。

### Task 3: 时长金额、冻结捕获和用量快照

**Files:**
- Modify: `backend/internal/service/grsai_balance_hold.go`、`grsai_balance_hold_test.go`、`grsai_task_service.go`、`grsai_task_service_test.go`、`backend/internal/repository/usage_billing_repo.go`、`grsai_gray_compatibility_test.go`

**Interfaces:**
- `GrsaiTaskHoldAmount(*GrsaiSettlement) (float64,error)`：图片 multiplier=`RequestedImageCount`，视频 multiplier=`VideoDurationSeconds`，取行内 `BillableUnitPrice`；复用同一 `local_task_id` 的 hold/capture/release 与 `GrsaiTaskUsageCommand`。
- 视频 `BaseUnitPrice` 和 `BillableUnitPrice` 都表示“每秒价”；`grsai_balance_holds.amount` / `settled_amount` 表示总金额（保存 DECIMAL(20,8/10) 当前舍入边界）。

- [ ] **Step 1: 写失败测试。** 768p×5 秒×分组视频倍率 1.5×账号倍率 1.2 = 1.26；图 2 张×0.25 = 0.50；价格变更后旧任务仍按快照。覆盖显式零价、无价格拒绝、冻结不足、失败释放、成功捕获一次、重复 Complete 和并发 claim 只一次、用户余额与 API Key quota 一致。

```go
func TestGrsaiVideoHoldUsesDuration(t *testing.T) {
    id := "grsai_test"
    record := &GrsaiSettlement{LocalTaskID:&id, UserID:7, TaskVersion:3,
        MediaKind:"video", VideoDurationSeconds:5, BillableUnitPrice:0.252}
    amount, err := GrsaiTaskHoldAmount(record)
    require.NoError(t, err)
    require.InDelta(t, 1.26, amount, 1e-8)
}
```

- [ ] **Step 2: 红灯。** `go test ./internal/service -run 'TestGrsai(VideoHold|BalanceHold|TaskService)' -count=1`，预期视频金额错误。
- [ ] **Step 3: 按媒体快照计算金额。** 校验 `TaskVersion` / `MediaKind` 配对，价格与倍率必须有限且非负。创建事务中先写 v3 行和加密载荷、冻结总金额，再提交；冻结失败整事务回滚。捕获和成功终态仍通过 `CompleteV2(..., CaptureGrsaiBalanceTx)` 在一个事务内完成；释放和失败终态仍通过 `FailV2(..., ReleaseGrsaiBalanceTx)` 一起提交。旧任务不因新字段默认值而改变倍数。
- [ ] **Step 4: 复测并提交。** `go test ./internal/service -run 'TestGrsai(Task|Balance|Settlement)' -count=1`，`CI=1 go test -tags integration ./internal/repository -run 'TestGrsai' -count=1`；预期余额、hold 状态与终态一致。提交：`git add` 本任务所改文件，`git commit -m "feat: 按视频时长冻结并幂等结算"`。

### Task 4: 上游 MP4 结果与有界流式 S3 保存

**Files:**
- Modify: `backend/internal/service/grsai_native.go`、`grsai_native_test.go`、`image_storage.go`、`image_storage_settings.go`、`backend/internal/repository/image_storage_s3.go`、`image_storage_s3_test.go`
- Create: `backend/internal/service/grsai_video_download.go`、`grsai_video_download_test.go`、`grsai_video_persistence.go`、`grsai_video_persistence_test.go`

**Interfaces:**
- `GrsaiUpstreamResult` 新增 `VideoURLs []string`，图片 `ImageURLs` 路径与错误处理保持不变；已知视频结果仅接受上游契约中的 `results[].url`，通过 `media_kind` 解释 URL，避免不加校验地把图片当视频。
- `VideoStorageWithMetadata.SaveVideoReader(ctx context.Context, key string, body io.Reader, size int64) (ImageStorageSaveMetadata,error)`：使用已有 S3 客户端与公开/presign 链接策略；`PersistGrsaiVideo(ctx, localTaskID, result)` 返回 `GrsaiStoredResult` 的 `{"results":[{"url":"<本地MP4>"}]}` 与对象元数据。

- [ ] **Step 1: 写假上游和假 S3 红灯测试。** 解析成功、失败、空 URL、混合/重复结果；抓取 1 MiB+ 流并断言不会走 `fetchImageBytes`；无 S3 配置拒绝，下载、截断、超额、错误 MIME、重定向到 loopback/私网、上传和 presign 失败不得返回本地成功 URL。重试相同 `localTaskID` 得相同 `grsai/<id>/video-0.mp4` key。

```go
func TestGrsaiVideoResultIsLocalMP4Only(t *testing.T) {
    // fakeDownloader 提供 video/mp4 流，fakeStorage 记录对象 key、size 和 URL。
    stored, err := persister.PersistGrsaiVideo(ctx, "grsai_test", upstream)
    require.NoError(t, err)
    require.JSONEq(t, `{"results":[{"url":"https://media.example/grsai_test.mp4"}]}`, string(stored.ResultJSON))
    require.NotContains(t, string(stored.ResultJSON), "upstream.example")
}
```

- [ ] **Step 2: 红灯。** `go test ./internal/service ./internal/repository -run 'TestGrsai(VideoResult|VideoDownload|VideoPersistence|VideoStorage)' -count=1`；预期新媒体存储接口或校验测试失败。
- [ ] **Step 3: 实现受约束下载。** URL 仅 HTTPS（本地 `httptest` 用注入 transport 验证），按上游允许的媒体主机/域名策略校验 host；DNS 解析和每次重定向都拒绝 loopback、私网、link-local、非全局单播，拨号地址与验证 IP 一致以防重绑定。要求 2xx、`video/mp4`（或经固定 MP4 签名确认的 `application/octet-stream`）、正长度，按 `io.LimitReader(max+1)` 验证实际字节数。采用独立配置视频超时与大小上限，例如 5 分钟和 512 MiB，默认值、配置验证与生产值一起测试；不使用图片 32 MiB 路径。
- [ ] **Step 4: 从 HTTP 响应流写有界临时文件，再调用 S3 PutObject。** 使用 `os.CreateTemp`，限制磁盘占用与目录、`defer os.Remove`，用文件大小设 `ContentLength`、`ContentType:video/mp4`，避免整片视频驻留内存；沿用 `S3ImageStorage.SaveWithMetadata` 的 public/presign 行为提取链接元数据，不重新发明链接。未知长度也按实际写入量验证。固定对象 key 覆盖失败重试，只有 `PutObject` 和签名成功才返回 URL；存储配置变化导致的签名期限仍与图片契约一致。
- [ ] **Step 5: 复测并提交。** `go test ./internal/service ./internal/repository -run 'Test(Grsai|ImageStorage)' -count=1`；预期旧图片数据 URL/base64/下载行为不变。提交：`git add` 本任务所改文件，`git commit -m "feat: 安全转存 GRS.AI 视频至 S3"`。

### Task 5: worker 分派、结果终态与视频用量

**Files:**
- Modify: `backend/internal/service/grsai_task_runtime.go`、`grsai_task_runtime_test.go`、`grsai_task_view.go`、`grsai_task_view_test.go`、`backend/internal/service/wire.go`、`backend/cmd/server/wire_gen.go`
- Test: `backend/internal/repository/grsai_gray_compatibility_test.go`

**Interfaces:**
- `GrsaiTaskRuntime` 在已持久化行中用 `MediaKind` 分派：`image` 继续 `PersistGrsaiImages`，`video` 调 `PersistGrsaiVideo`；单一 `CompleteV2` 事务处理两类任务。不能用模型名或上游 URL 扩展名猜类别。
- 视频 `UsageLog` 写 `BillingModeVideo`、`VideoCount=1`、`VideoResolution`、`VideoDurationSeconds`、按秒基础金额和实际账单；图片既有字段、request type、API Key 归属保留。

- [ ] **Step 1: 写 worker 红灯测试。** 视频成功 → S3 → 入库/捕获 → `succeeded`；每种交付模式只公开本地 ID/URL；上游失败释放，轮询异常、MP4 失败、数据库提交失败按原有限次退避到 `manual_review`；上游 ID 绑定后重启只 GET，不重复 POST；重复/并发 RunOnce 只有一笔结算。图片 v2 同跑仍走图片 persister。

```go
switch claim.MediaKind {
case "image":
    stored, err = r.images.PersistGrsaiImages(ctx, *claim.LocalTaskID, result)
case "video":
    stored, err = r.videos.PersistGrsaiVideo(ctx, *claim.LocalTaskID, result)
default:
    return r.manualReview(ctx, claim, "unsupported_media_kind")
}
```

- [ ] **Step 2: 红灯。** `go test ./internal/service -run 'TestGrsai(TaskRuntime|TaskView|VideoUsage)' -count=1`；预期视频成功/计费测试失败。
- [ ] **Step 3: 实现分派、用量和依赖注入。** 复用原有 `GrsaiTaskRuntime` 扫描与 Async 提交；视频明确使用 `VideoDurationSeconds` 和 `VideoResolution`，`ImageCount=0`、`ImageSize=nil`，`BillingMode="video"`。外显状态仍经 `NewGrsaiTaskView`，结果只有已保存本地 URL，未完成/人工核查时不带上游链接；后台日志只保存固定错误代码，不记上游 URL。
- [ ] **Step 4: 验证并提交。** 在 `backend/` 执行 `go generate ./ent ./cmd/server`（与 `backend/Makefile` 的生成目标一致），`go test ./internal/service ./internal/repository ./cmd/server -run 'TestGrsai' -count=1`，`go vet ./internal/service ./internal/repository ./cmd/server`；预期通过。提交：`git add` 本任务所改文件，`git commit -m "feat: 恢复视频任务并记录视频用量"`。

### Task 6: 对外契约、界面文案、追溯脚本与完整离线回归

**Files:**
- Modify: `backend/internal/handler/grsai_gateway.go`、`grsai_delivery_handler.go`、`grsai_gateway_test.go`、`grsai_delivery_handler_test.go`、`backend/internal/server/routes/gateway_test.go`、`frontend/src/i18n/locales/zh/admin/overview.ts`、`frontend/src/i18n/locales/en/admin/overview.ts`、`test/grsai_live_contract.py`
- Create: `frontend/src/views/admin/__tests__/GroupsView.spec.ts`、`test/grsai_video_traceability.py`
**Interfaces:**
- `POST /v1/api/generate` 和 `GET /v1/api/result?id=...` / `GET /v1/api/tasks` 均保持原路由、原 API Key 鉴权；只复用 `allow_image_generation` 布尔配置。
- GRS.AI 分组中文显示“允许图片/视频生成”，英文显示“Allow image/video generation”；图片专属倍率和 Grok 定价说明不误改。

- [ ] **Step 1: 写端到端 handler 红灯测试。** 用假 GRS.AI Async HTTP 和假 S3 验证 480p×1 秒的视频 `json/stream/async`；`results[0].url` 只指向本地 MP4，Key B 查询详情/列表均 404 或不可见；组开关 false 提交前拒绝、true 允许；缺档/缺时长无上游 POST、无 hold；原图 JSON/Stream/Async 和历史 owner 查询回归。
- [ ] **Step 2: 红灯。** `go test ./internal/handler ./internal/server/routes -run 'TestGrsai' -count=1`；预期新视频断言失败。
- [ ] **Step 3: 只补缺失契约与文案。** 将 `imagePricingI18nKey(platform,"allowImageGeneration")` 的 GRS.AI 专属显示键改为图片/视频；其他平台继续使用图片文案。同步检查并更新 GRS.AI 独立倍率区域的标题和说明，明确视频使用现有视频独立倍率或有效分组倍率。保持字段 `allow_image_generation`、Grok `video_model_prices` 和前端模型分类不变。
- [ ] **Step 4: 全量离线验证。** `go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./internal/config ./cmd/server -count=1`；`go vet ./internal/service ./internal/repository ./internal/handler ./internal/server/routes`；`CI=1 go test -tags integration ./internal/repository -run 'TestGrsai' -count=1`；`pnpm test:run`、`pnpm build`。`test/grsai_live_contract.py --self-test` 无网络无密钥；`python test/grsai_video_traceability.py` 检查 `GRSAI-VIDEO-01..08` 每条至少关联一个实际测试且无未知 ID；每条验收均有失败和成功证据。
- [ ] **Step 5: 提交。** `git add` 本任务所改测试、接口、i18n 和脚本，`git commit -m "test: 验证 GRS.AI 视频契约与界面文案"`。

### Task 7: 双阶段兼容发布和 0% 候选实测

**Files:**
- Read: `AGENTS.md`、生产 `/opt/sub2api/gray-status.sh` 输出；每次操作以当前槽位为准
- Evidence: 候选容器健康、`/health`、Nginx 校验、脱敏任务 ID、S3 HEAD/GET、价格/余额/用量一致性、最近错误日志

- [ ] **Step 1: 确认发布门。** 运行 `ssh tenxunyun.guigu /opt/sub2api/gray-status.sh`；核对数据库迁移、候选/稳定镜像、创建开关是每槽位独立还是共库。采用独立版本或只控制**创建**的槽位级开关，确保兼容 worker 能恢复视频而老稳定槽位不会发出视频任务。
- [ ] **Step 2: 发布兼容 worker 版本（创建视频关闭）。** 用 `gray-deploy.sh <候选槽位> sha-<兼容提交>` 部署 0%，验证旧图片和 v3 恢复测试，检查 `/health`、Nginx、容器和日志；按灰度脚本缓升并经观察后 `gray-promote.sh`，再 `gray-status.sh` 确认稳定槽位已具备 v3 worker。没有此状态不得创建视频任务。
- [ ] **Step 3: 功能版到 0% 候选。** `gray-deploy.sh <候选槽位> sha-<功能提交>`；直连/定向候选但不扩大公网投入流量，以显式授权的有效 API Key 提交一个 `minimax-h3`、480p、1 秒、`replyType=async`；轮询本地 ID，检查只有本地 MP4、可访问链接、一个捕获 hold 和符合当前渠道单价/倍率的一条用量。检查另一 Key 查询被隔离、图片功能、Nginx、日志、健康状态。
- [ ] **Step 4: 回滚与逐步切流。** 有任一不健康/5xx/超时/鉴权/账务异常，先用 `gray-set-traffic.sh 0` 切回稳定，保留候选与现场，报告并等待进一步指示；兼容稳定槽位继续恢复 v3。通过后才逐步提高比例，每次查状态、错误日志、重复 POST/扣费、S3 链接和冻结滞留。
- [ ] **Step 5: 100% 后观察、晋升、核对。** 经观察期后 `gray-promote.sh`，随后 `gray-status.sh` 核对稳定/候选标记、不可变镜像和健康；不启动旧容器、不执行 bootstrap、compose down、删卷或数据库恢复。

## 自检和验收映射

| 验收 | 任务与证据 |
| --- | --- |
| GRSAI-VIDEO-01 | Task 1 价格模式判定；Task 6 原开关与授权/账号 handler |
| GRSAI-VIDEO-02 | Task 1 严格档位/时长、Task 3 冻结与倍率真库测试 |
| GRSAI-VIDEO-03 | Task 1 `json.RawMessage` 透传与假上游断言 |
| GRSAI-VIDEO-04 | Task 4 本地 MP4、Task 5 事务终态、Task 6 三模式 |
| GRSAI-VIDEO-05 | Task 3/5 幂等和失败恢复、Task 4 S3 异常/安全边界 |
| GRSAI-VIDEO-06 | Task 5 用量快照、Task 6 两 API Key 隔离 |
| GRSAI-VIDEO-07 | Task 2 真库新旧 worker 领取、Task 5 图片 v2 回归 |
| GRSAI-VIDEO-08 | Task 7 兼容稳定槽位、候选 0% 实测及灰度门 |

实施前特别核对：当前旧 `ClaimDueV2`、`BindV2UpstreamTask`、`CompleteV2`、owner 查询全带 `task_version=2`，仅改领取 SQL 不能完成 v3；图片 v2 的 `requested_image_count` 和 `image_size` 均为真实图片计费字段，不能直接复用为视频时长/分辨率；旧图片下载为 32 MiB 内存路径。以上三处构成本计划最容易遗漏的接缝。
