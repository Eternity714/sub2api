# Task 4 实施报告

## 变更文件

- `backend/internal/service/grsai_native.go`：新增严格的 `OpenGenerateStream`，强制上游 `Accept: text/event-stream`，校验 2xx 和 SSE Content-Type，错误响应限长读取、脱敏并关闭 Body。
- `backend/internal/service/grsai_stream.go`：新增 SSE 帧解析、稳定任务 ID、单调进度、终态和结果 URL 校验；新增持久化优先的 `ConsumeGrsaiSSE`/`RecordStreamEvent` 协作边界。
- `backend/internal/service/grsai_stream_test.go`：覆盖成功流、ID 变化、进度回退、无结果成功、非 SSE 关闭 Body、回调顺序和中断错误。
- `backend/internal/repository/grsai_settlement_repo.go`：新增带 claim fence 的 `RecordStreamEvent` 持久化更新。

## 验证

主机未安装 Go；使用项目 Docker Go 镜像执行：

```text
docker run --rm -v "${PWD}:/workspace" -w /workspace/backend golang:1.27.0 sh -c "gofmt -w internal/service/grsai_native.go internal/service/grsai_stream.go internal/service/grsai_stream_test.go internal/repository/grsai_settlement_repo.go && go test ./internal/service -run 'Test(ParseGrsaiSSE|OpenGenerateStream|Grsai)' -count=1"
ok   github.com/Wei-Shaw/sub2api/internal/service 0.067s

docker run --rm -v "${PWD}:/workspace" -w /workspace/backend golang:1.27.0 sh -c "go test ./internal/repository -run 'TestGrsai' -count=1"
ok   github.com/Wei-Shaw/sub2api/internal/repository 0.023s
```

`git diff --check` 通过。

## 设计决定

- 只对完整 `data:` JSON 帧调用回调；注释、事件名、retry 和空 keepalive 被忽略。
- 首帧必须有任务 ID；后续帧必须携带同一 ID。进度限制在 0..100 且不可回退。
- `succeeded` 必须至少包含一个 URL；`failed`/`violation`/`succeeded` 是唯一终态，流在终态后不得继续发送数据。
- SSE 事件的调用方通过 `ConsumeGrsaiSSE` 先记录帧，再执行下游回调；首次事件使用围栏绑定上游 ID。
- 读流错误在绑定前进入人工复核并释放冻结；绑定后记录 unknown/pending-upstream，保留冻结等待 `/v1/api/result` 恢复查询，不重新 POST。

## 关注事项

- 当前任务事件持久化复用结算表的 `progress`/`result_urls` 字段，没有新增逐帧审计表；若未来需要逐帧审计，应单独追加 migration 和事件表。
- `RecordStreamEvent` 需要生产 repository 实现；不实现该扩展的测试 double 在消费流时会得到显式 `ErrGrsaiStreamPersistenceUnavailable`，不会静默丢帧。
