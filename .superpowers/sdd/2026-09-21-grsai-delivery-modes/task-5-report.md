# Task 5 报告：GRS.AI 异步任务恢复

## 状态

已完成任务服务、加密 Async 载荷、恢复运行时、claim-version fencing 和安全公开视图。未修改 Task 3 报告中的既有用户变更。

## 变更文件

- `backend/internal/service/grsai_task_service.go`
- `backend/internal/service/grsai_task_service_test.go`
- `backend/internal/service/grsai_task_runtime.go`
- `backend/internal/service/grsai_task_runtime_test.go`
- `backend/internal/service/grsai_settlement.go`
- `backend/internal/service/grsai_stream.go`
- `backend/internal/repository/grsai_settlement_repo.go`

## 验证命令与结果

1. `docker run --rm -v grsai-go-cache:/go -v "${PWD}/backend:/src" -w /src golang:1.27 gofmt -w internal/service/grsai_settlement.go internal/service/grsai_task_service.go internal/service/grsai_task_service_test.go internal/service/grsai_task_runtime.go internal/service/grsai_stream.go internal/repository/grsai_settlement_repo.go`：通过。
2. `docker run --rm -v grsai-go-cache:/go -v "${PWD}/backend:/src" -w /src golang:1.27 go test ./internal/service -run 'Test(AsyncTask|BoundDisconnect|QueuedTask|PublicTaskView)' -count=1`：通过。
3. `docker run --rm -v grsai-go-cache:/go -v "${PWD}/backend:/src" -w /src golang:1.27 go test ./internal/repository -run 'TestGrsai' -count=1`：通过。
4. `git diff --check`：通过；仅报告既有 Task 3 报告的换行转换提示。

## 设计决策

- Async 只保存 `OriginalBody` 的加密副本；运行时重新通过 Task 1 解析并使用新建的 `UpstreamBody`，上游请求固定 `replyType=stream`。
- 首个 SSE 事件被 Task 4 持久化并绑定上游 ID 后立即删除 payload；已绑定记录只允许结果轮询，不重新 POST。
- POST 前先持久化 `submitting` fence；恢复只把该状态视为 `upstream_unknown`，绝不再次 POST。首帧协议/持久化/回调错误 fail-closed 到 `manual_review`+release。
- `submitting` 超时由 Async runtime 转入 `manual_review`+release；已绑定首帧后的回调/协议错误转 `upstream_unknown` 并保留冻结余额进行轮询。
- pre-bind manual review 成功后，返回错误前删除加密 payload。
- Async runtime 使用 `ClaimDueForDeliveryMode(..., async)`，旧 JSON/Stream 记录不会被 async worker 租约或修改。
- 载荷删除 TTL 与公开结果 `ExpiresAt` 独立配置；公开视图保留 `manual_review`、`pending_settlement`、`upstream_unknown`。

## 关注事项

- 当前工作树在任务开始前已存在 `.superpowers/.../task-3-report.md` 的用户修改，本提交未包含该文件。
- 生产 Wire/config 生命周期接线留给 Task 7；HTTP POST/Result 路由留给 Task 6。
