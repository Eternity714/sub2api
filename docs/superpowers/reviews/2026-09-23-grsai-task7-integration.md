# Task 7 复核：配置、Worker 生命周期和清理

## 状态

已实现 `grsai_delivery` 配置，默认关闭 Async 受理与 Worker；用户侧等待 20、进行中 3 的容量约束沿用 Task 5/6。启用时 Wire 启动 Worker，停机停止新领取并等待已开始的上游调用结束，再关闭数据库依赖。未部署，未执行真实上游请求。

## 范围

- 配置扫描周期、批次、密文 TTL 与结果保留期，并限制取值范围；TaskService 与 Runtime 使用同一配置。
- Async 关闭时返回 503，冻结和加密载荷创建前拒绝；JSON/Stream 路径不受影响。
- 密文清理每轮最多 100 条；未绑定且仍可能提交/回退的 `not_submitted` 和 `submitting` 任务保留，绑定任务或终态可提前删除密文。
- 只清理 `settled` 或 `closed_no_charge` 且冻结已捕获/释放或本来无需冻结的终态行；按 `closed_at` 加单条保留时长计算，不删除待结算、人工复核或冻结尚未释放的记录。
- 停止信号在领取后、POST 前再次检查；已开始的 POST 不因优雅停机主动取消。

## 验证与复审

- `go test ./internal/config ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server -count=1`：六包通过。
- `go test -tags integration ./internal/repository -run 'TestGrsaiDeliveryCleanup|TestGrsaiSettlementRepository_Async(WaitingLimit|RunningLimit)|TestGrsaiTaskPayloadRepository_QueuedTaskSurvivesPayloadTTL' -count=1`：PostgreSQL Testcontainers 通过，覆盖密文保护/绑定删除、任务终态与冻结状态、记录级保留期延长/缩短及每用户 20/3 容量。
- `go vet` 同六包、`gofmt -l`、`git diff --check` 通过。停止测试重复 10 次通过。
- 独立复审数轮后的最终结论：未发现 Task 7 新的 P0/P1。

## 仍需处理

- 停机排空上游请求最长约 30 分钟；实际部署前必须配置大于上游请求超时、HTTP 关闭及清理余量之和的终止宽限期。强制杀进程时仍需依赖持久化围栏和人工复核，不承诺优雅排空。
- 仍保留 Task 5/6 记录中的两个 P2；PRD AC-ID 机械追溯、Task 8 真实契约探测和发布门尚未完成。当前发布 **no-go**。
- 该隔离工作树含先前未提交的 Task 5/6 改动且与 Task 7 共用文件；本轮不拆分/提交混合补丁，不触碰主工作树。
