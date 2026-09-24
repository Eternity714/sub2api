# Task 3 实施报告：GRS.AI 余额冻结、释放与原子结算

## 状态与提交

- 状态：已实现并提交。
- Commit：中文主题 `实现 GRS.AI 余额冻结与原子结算`；最终 SHA 见任务回执。

## 实现内容

1. 新增 GRS.AI 专用余额 hold 契约与操作键：
   - `GrsaiBalanceHoldCommand` 包含 settlement ID、用户、API Key、金额和幂等键。
   - `GrsaiHoldBillingRepository` 暴露 Reserve、CaptureTx、ReleaseTx；未复用 Batch Image 的公开接口。
   - 操作键固定为 `grsai_hold:<id>`、`grsai_capture:<id>`、`grsai_release:<id>`。
2. Prepare 流程在发送上游前写入快照、冻结余额、标记 `hold_state=held`，再 claim submission；余额不足返回既有 `ErrInsufficientBalance` 语义的 GRS.AI 错误，不进入上游调用。
3. 成功结算在 settlement repository 的同一 `*sql.Tx` 内先 capture hold，再执行既有 `UsageBillingRepository.ApplyTx`，最后将任务标记 settled；重复结算继续由 settlement/billing 幂等键拦截。
4. failed/violation/永久轮询失败及 task identity 冲突路径支持原子 release + terminal transition；结算失败只进入 pending settlement，保留 held 金额，不释放成功上游结果的 hold。
5. SQL repository 新增专用冻结、消费、释放 SQL；Batch Image 的 Reserve/Capture/Release 实现未修改。

## TDD 与验证

先新增 `grsai_balance_hold_test.go` 的 hold 命令/操作键/错误语义测试，并在生产实现前运行目标测试；宿主机无 Go，使用项目已有 `golang:1.27.0` Docker 镜像执行。

最终聚焦命令：

```text
docker run --rm -v "${PWD}:/workspace" -w /workspace/backend golang:1.27.0 sh -c "gofmt -w internal/service/grsai_balance_hold.go internal/service/grsai_balance_hold_test.go internal/service/grsai_settlement.go internal/repository/usage_billing_repo.go internal/repository/grsai_settlement_repo.go && go test ./internal/service ./internal/repository -run 'TestGrsai' -count=1"
```

最终输出：

```text
ok  github.com/Wei-Shaw/sub2api/internal/service     0.049s
ok  github.com/Wei-Shaw/sub2api/internal/repository  0.024s
退出码：0
```

追加 GREEN 命令（含既有 Batch Image 回归）：

```text
docker run --rm -v "${PWD}:/workspace" -w /workspace/backend golang:1.27.0 sh -c "go test ./internal/service ./internal/repository -run '(TestGrsai|TestBatchImage)' -count=1"
```

输出：两个包均 `ok`，退出码 0。

## 关注事项

- 本轮未连接 PostgreSQL 执行带 `integration` 标签的真实数据库测试；SQL 逻辑沿用现有 repository 事务与 dedup 表。
- hold state 的扩展通过可选 repository 能力接口接入，旧的内存测试桩仍可运行；生产 repository 提供完整原子转换。
- 宿主机原生 `go`/`gofmt` 不可用，格式化与测试均在 Docker Go 环境完成。

## Review 修复

- Capture 现在只消费 frozen balance，传给 `ApplyTx` 的 `BalanceCost` 置零，因此 reserve + success 最终只扣一次 users.balance，同时保留 API Key、rate-limit、account quota 和 usage 记账金额。
- Prepare 在 `MarkHoldHeld` 或 submission claim 失败后调用幂等 release，覆盖 hold_state 为 none/held 的补偿路径。
- 新增 capture SQL 的余额断言与 claim 失败补偿测试；原有成功结算幂等、失败不记 usage 测试继续覆盖服务流程。
- 复审修复：capture SQL 仅减少 `frozen_balance`，不恢复 `balance`；成功结算仅在锁定副本上将 `BalanceCost` 置零；Prepare 补偿 release 错误通过 `errors.Join` 返回。定向 Docker Go 测试 `TestGrsai(PrepareReleases|PrepareReturns|CaptureGrsai|Hold|Settlement)` 已通过。
