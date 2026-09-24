# Task 5/6 修复整合复核（2026-09-23）

## 范围与状态

- 工作树：`C:\Users\Administrator\.codex\worktrees\grsai-delivery-modes\sub2api`，detached HEAD `5406ef2af`；未修改 `main`，未部署，未调用真实/付费上游。
- 保留进入本轮时已存在的 Task 3/6 报告、Handler、Runtime 和测试改动；较早的 `D:\work\个人\sub2api-task5-fix` 未改动。
- Task 5/6 关键修复已整合到目标工作树，但仍是未提交状态；发布门为 **no-go**。

## 逐项结论

| 检查项 | 结论 |
| --- | --- |
| Async 加密原始载荷、重新解析后固定上游 Stream、首帧绑定后删除载荷 | 原实现和定向测试覆盖，保留。 |
| 提交前围栏、丢失 claim、重复 POST | 新增条件更新 `MarkSubmitting`；仅活动 claim、未绑定 ID、`not_submitted` 状态可过栅栏。零行/失效 claim 不 POST；写故障原始错误保留，尝试人工复核且不能静默跳过冻结释放。 |
| Async 与旧恢复领取互斥 | 通用 `ClaimDue` 排除 Async，Async worker 仅领取 Async 且逐条领取；SQL mock 和运行时回归覆盖。 |
| 已绑定任务恢复 | 仅结果查询，不再 POST；断线后消费与结算持续，用户/账号并发槽延至任务处理结束；保留既有修复。 |
| 公开查询 | 双重归属与空 ID 404 已有 Handler 测试；公开状态对齐计划中的 `submitting`、`upstream_unknown`、`settled`、`closed_no_charge`；终态后保留期到期返回 404，`manual_review` 仍可查。 |
| 三种交付 | 路由和 Wire 注入已核对；新增完整 `Generate` Handler 夹具覆盖 Async 202/零 POST、429、JSON 验证终态、SSE 帧和断线续跑。 |

## 验证

- `go test ./internal/repository -run 'TestGrsaiSettlementRepository(AsyncClaim|MarkSubmitting|LegacyClaim)' -count=1`：通过。
- `go test ./internal/service -run 'TestPublicTaskView|TestAsyncTask|TestSubmittingTask|TestRuntimeClaims' -count=1`：通过。
- `go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server -count=1`：五包通过（service 138.902s、repository 4.386s、handler 43.560s、routes 14.445s、server 0.137s）。
- `gofmt -w` 本轮 Go 文件；`git diff --check`：通过，仅已有 Markdown 换行转换警告。
- `traceability_check.py docs/superpowers/specs backend/internal`：未通过；spec 无稳定 AC-ID，机械追溯不可用。

## 后续门槛

### Async 用户容量补充（2026-09-23）

用户确定跨 API Key 每用户等待中 20、进行中 3。PRD、设计和计划已回填；追加迁移 `244_grsai_async_capacity.sql` 的首次领取标记。受理事务按用户序列化计数，满额返回 429 且不执行冻结；领取事务按用户序列化，已启动任务的租约恢复不占新名额，终态/仅结算重试不再占进行中名额。仓储定向测试和真库并发测试通过（25 并发受理 20 成功/5 拒绝，4 并发领取 3 成功、租约恢复及终态补位）。此补充不代表 Task 6/7 或发布门已经完成。

1. 补完整 Handler 夹具，验证 Async 202 且零 POST、JSON 终态、SSE 头/帧持久化顺序、断线后继续且无二次 POST；随后补真库并发领取与重启恢复测试。
2. Task 7 清理必须按终态时间计算结果保留期；当前 Async 的 `expires_at` 在创建时写入，不能直接作为终态行删除条件，否则长任务可能提前被清理。JSON/Stream 的终态清理也需落实。
3. 为 PRD 验收项建立稳定 AC-ID 并在测试中引用，重跑可追溯门；Task 8 再执行显式真实上游探测和发布验证。

## 独立 Review 与修复（2026-09-23）

独立评审指出 4 个 P1、未确认 P0：Async 缺生产 Worker 接线（Task 7 尚未执行），排队载荷在 15 分钟后失效，JSON 上游成功但本地结算故障时错误返回 502，以及旧恢复器人工核查跳过冻结释放。后 3 项已按 RED→GREEN 修复：仍未提交上游的等待/刚领取 Async 密文可越过名义 TTL；仅完整验证的终态 JSON 不被暂时结算错误替换为 502，流消费错误不返回部分终态；旧恢复器改用事务性冻结释放入口。Async Worker 接线保留为 Task 7 入口门槛。

- `go test ./internal/handler -run 'TestGrsaiGenerate' -count=1`：通过，含三模式、429、SSE 断线持久化及单次 POST。
- `go test -tags integration ./internal/repository -run 'TestGrsaiTaskPayloadRepository_QueuedTaskSurvivesPayloadTTL|TestGrsaiSettlementRepository_Async(WaitingLimit|RunningLimit)' -count=1`：通过。
- `go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server -count=1`：五包通过（service 135.167s、repository 5.026s、handler 49.718s、routes 8.812s、server 6.323s）。
- 第二轮独立复审尚未返回，Task 6 不标完成，Task 7 尚未开始，发布仍 **no-go**。

## 第二轮复审与修复（2026-09-23）

第二轮复审未发现 P0，指出两个 P1：暂时性密文读取故障被当作永久丢失，以及余额冻结与 `hold_state=held` 分事务提交。前者已改为仅在明确缺失/损坏时转人工，短暂读取错误保留密文和冻结并调度重试；后者以 `ReserveHold` 事务统一锁记录、账务去重、冻结余额和状态标记。首次领取失败、冻结提交结果不明时，事务性终结路径依据持久化状态释放；正金额任务在 `held` 未持久化时禁止上游 POST。相应服务、SQL mock 和真库失败窗口测试均已补充。

- `go test -tags integration ./internal/repository -run 'TestGrsaiSettlementRepository_ReservationAndHoldStateAreAtomic|TestGrsaiSettlementRepository_Async(WaitingLimit|RunningLimit)|TestGrsaiTaskPayloadRepository_QueuedTaskSurvivesPayloadTTL' -count=1`：通过，含事务回滚、未领取记录释放、20/3 容量。
- `go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server -count=1`：五包通过（service 135.988s、repository 5.187s、handler 38.007s、routes 14.737s、server 6.086s）。
- `go vet` 同五包：通过；`git diff --check`：通过（仅已有 CRLF 转换警告）。
- 第三轮独立复审进行中；Task 7 尚未开始，发布仍 **no-go**。原“第二轮尚未返回”状态由本节更新。

## 第三至六轮复审结论（2026-09-23）

后续四轮针对暂时性账户读取、受理后读取失败、提交/领取/首帧绑定结果不明等故障窗口继续补测并修复。正金额任务在 `hold_state=held` 未持久化前不得向上游 POST。第六轮独立复审未发现 Task 5/6 新的 P0/P1，允许按既定顺序进入 Task 7；本结论不意味着可部署或开放 Async 流量。

- 最近一次五包全量 `go test ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server -count=1`、`go vet`、真库定向测试与 `git diff --check` 均通过；完整命令及输出需在 Task 7 收尾重新核验。
- 未解决的 P2：入队写前失败可能等待约 31 分钟租约；跨用户 `SKIP LOCKED` 竞争可能推迟一个扫描周期。PRD AC-ID 追溯与 Task 8 均未完成，发布维持 **no-go**。
