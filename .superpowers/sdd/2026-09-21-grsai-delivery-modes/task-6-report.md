# Task 6 报告：三种下游交付与结果查询

## 状态

已接入 `replyType=json|stream|async` 分流、Task 5 任务服务、SSE 事件持久化后写出，以及 owner/API-key 约束的 `/v1/api/result` 路由。

## 变更文件

- `backend/internal/handler/grsai_gateway.go`
- `backend/internal/handler/grsai_gateway_test.go`
- `backend/internal/handler/wire.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/grsai_native.go`
- `backend/internal/service/wire.go`
- `backend/internal/repository/wire.go`
- `backend/cmd/server/wire_gen.go`

## 验证

Docker `golang:1.27`：

```text
go test ./internal/handler ./internal/server/routes ./cmd/server -run 'TestGrsai' -count=1
ok github.com/Wei-Shaw/sub2api/internal/handler
ok github.com/Wei-Shaw/sub2api/internal/server/routes [no tests to run]
ok github.com/Wei-Shaw/sub2api/cmd/server
```

空测试编译检查及 `gofmt`、`git diff --check` 通过。

复审修复后再次执行 Docker `golang:1.27` 定向回归：

```text
go test ./internal/service ./internal/handler ./internal/server/routes ./cmd/server -run 'Test(AsyncTask|BoundDisconnect|QueuedTask|SubmittingTask|RuntimeClaims|GrsaiGateway)' -count=1
ok github.com/Wei-Shaw/sub2api/internal/service
ok github.com/Wei-Shaw/sub2api/internal/handler
ok github.com/Wei-Shaw/sub2api/internal/server/routes
ok github.com/Wei-Shaw/sub2api/cmd/server
```

新增的 Handler 回归覆盖本地公开 ID 与上游 ID 查询、用户/API Key 双重归属的 404、空 ID 的 404，以及公共结果中不泄露账号 ID、价格或上游请求体。Runtime 回归断言即使批量上限大于 1，每次也只租约一条任务。

## 设计决策

- Async 只创建并返回 queued 任务，不在请求线程 POST 上游。
- JSON/Stream 使用 Task 4 的固定上游 SSE；Stream 在持久化回调后写 `data:` 并 Flush，客户端断开只停止输出，持久化运行继续。
- Result 只返回 `GrsaiTaskView`，空 ID、非归属 ID、非 GRS.AI 分组返回 404/未授权。
- Wire 通过独立 stream-client provider 注入 Task Service，避免破坏旧 native client 测试接口。

## 关注事项

- 复审修复：异步 Runtime 不再预先批量租约后串行处理，避免后续任务在真正发起 POST 前租约过期、被其他实例重新认领。SSE 客户端断线时，并发槽保持至后台流处理结束；JSON 路径不再累计未使用的事件切片。
- 当前仍未覆盖真实上游和完整 Handler 集成夹具；Task 8 负责真实契约探测，后续应补充带数据库/任务服务的 JSON、Stream、Async 集成测试。
