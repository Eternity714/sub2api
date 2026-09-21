# Task 5 报告：GRS.AI 异步任务恢复

## 状态

已完成任务服务、加密 Async 载荷、恢复运行时、claim-version fencing 和安全公开视图。未修改 Task 3 报告中的既有用户变更。

## 变更文件

- `backend/internal/service/grsai_task_service.go`
- `backend/internal/service/grsai_task_service_test.go`
- `backend/internal/service/grsai_task_runtime.go`
- `backend/internal/service/grsai_task_runtime_test.go`
- `backend/internal/service/grsai_settlement.go`

## 验证命令与结果

1. `docker run --rm -v grsai-go-cache:/go -v "${PWD}/backend:/src" -w /src golang:1.27 gofmt -w internal/service/grsai_settlement.go internal/service/grsai_task_service.go internal/service/grsai_task_service_test.go internal/service/grsai_task_runtime.go internal/service/grsai_task_runtime_test.go`：通过。
2. `docker run --rm -v grsai-go-cache:/go -v "${PWD}/backend:/src" -w /src golang:1.27 go test ./internal/service -run 'Test(AsyncTask|BoundDisconnect|PublicTaskView|QueuedTask)' -count=1`：通过。
3. `docker run --rm -v grsai-go-cache:/go -v "${PWD}/backend:/src" -w /src golang:1.27 go test ./internal/service ./internal/repository -run 'Test(AsyncTask|BoundDisconnect|PublicTaskView|Grsai.*(Claim|Recover|Settle))' -count=1`：通过（service/repository 均 OK）。
4. `git diff --check`：通过；仅报告既有 Task 3 报告的换行转换提示。

## 设计决策

- Async 只保存 `OriginalBody` 的加密副本；运行时重新通过 Task 1 解析并使用新建的 `UpstreamBody`，上游请求固定 `replyType=stream`。
- 首个 SSE 事件被 Task 4 持久化并绑定上游 ID 后立即删除 payload；已绑定记录只允许结果轮询，不重新 POST。
- pre-bind 提交不确定、载荷缺失/过期、解析失败统一进入 `manual_review` 并释放冻结余额；不猜测上游 ID。
- 公开视图只包含约定的安全字段，错误码限定为 `upstream_failed`、`policy_violation`、`manual_review`，错误摘要去控制字符并限制长度。
- 旧 JSON/Stream 结算记录不由 Async runtime 处理，继续交给既有 settlement recovery runtime。

## 关注事项

- 当前工作树在任务开始前已存在 `.superpowers/.../task-3-report.md` 的用户修改，本提交未包含该文件。
- 生产 Wire/config 生命周期接线留给 Task 7；HTTP POST/Result 路由留给 Task 6。
