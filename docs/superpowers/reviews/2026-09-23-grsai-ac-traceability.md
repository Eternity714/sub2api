# GRS.AI 交付方式 PRD AC-ID 追溯

## 现状

PRD 第 9 节 11 条验收标准已按原顺序固定为 `AC-001` 至 `AC-011`，未改动验收语义。编号发布后不得重排或复用。现有测试只在实际检查相关行为的用例上标注 `@covers`；没有为了通过脚本而给不相关测试贴标签。

| 验收项 | 主要证据 |
| --- | --- |
| AC-001 | `TestGrsaiGenerateMissingReplyTypeReturnsFinalJSON` 验证缺省 JSON 与强制上游 Stream。 |
| AC-002 | `TestGrsaiGenerateStreamWritesPersistedFrames` 验证有序帧与单个成功终态。 |
| AC-003 | `TestGrsaiGenerateAsyncReturns202WithoutPosting` 验证 202、零即时 POST 和任务查询。 |
| AC-004 | `TestGrsaiGatewayResultUsesOwnerAndAPIKeyScopedLocalOrUpstreamID` 验证跨用户与跨 API Key 404。 |
| AC-005 | `TestGrsaiSettlementService_ConcurrentSuccessChargesOnce` 验证真库并发只结算一次。 |
| AC-006 | `TestGrsaiSettlementService_RealFailuresNeverBill` 与 `TestGrsaiSettlementRecoveryManualReviewReleasesHeldBalance` 验证失败/违规不计费及未知提交人工复核释放。 |
| AC-007 | `TestGrsaiGenerateStreamPersistsBeforeOutputAndContinuesAfterDisconnect` 验证断线继续、公开查询终态、单次 POST。 |
| AC-008 | `ProbeTests.test_default_and_invalid_flags_never_open_network` 验证未显式传完整参数时零请求；CI 只执行离线 `--self-test`。 |
| AC-009 | `TestGrsaiGenerateQueueFullReturns429WithoutPost` 与 `TestGrsaiSettlementRepository_AsyncWaitingLimitAcrossConcurrentKeys` 验证 429/不建任务及跨 Key 并发 20 上限。 |
| AC-010 | `TestGrsaiSettlementRepository_AsyncRunningLimitAndLeaseRecovery` 验证第 4 项保留等待、终态释放名额。 |
| AC-011 | 两项 Async 真库并发测试验证跨数据库连接的受理/领取上限及租约恢复。 |

## 校验与边界

- `python <ai-product-dev-pack>/scripts/traceability_check.py --selftest` 红绿自检通过。
- 在隔离工作树根目录以 `docs/superpowers/specs` 为需求范围、仓库根目录为测试范围运行双向追溯：11 条声明、11 条引用，零孤儿和幽灵；新探测器位于 `test/`，原先的 `backend/internal` 范围不能覆盖它。扫描器只核对文本引用，不等于行为或真实上游已通过。
- 离线自测 9 项通过；无参数调用退出 2、零网络；六包 Go 回归、`go vet` 和 `git diff --check` 通过。CI 新增的只有 `--self-test`，不执行付费请求。
- 该脚本只扫描文本 ID，不识别 Go 断言、不执行测试，也不自动阻止合并。真库并发测试在 PostgreSQL Testcontainers 上通过，模拟独立连接与租约恢复，但不等同于生产多进程压测。
- 旧真库结算夹具曾绕过预冻结，导致并发结算用例稳定失败。夹具已按真实流程先冻结，再验证事务回滚、单次结算和失败释放；这不是产品代码改动。

## 下一步

Task 8 离线探测器已实现，且只将 `AC-008` 标在防误触测试上。2026-09-24 用户调整验收范围：上游 credits 和费用不作为发布门，真实探测只需显式 `--live`、每次最多一次 POST，并验证协议与结果查询；本地冻结、释放、幂等结算仍是硬门。下一步以真实上游验证、整体对外契约和灰度检查决定发布；不得仅据机械追溯结果放行。
