# GRS.AI 多模式交付发布门（2026-09-24）

## 范围与结论

JSON、Async、Stream 共用上游 Stream 协议；Async 每用户等待 20、进行中 3。上游成功但缺结果 URL 时，Async 保留公开任务 ID 供恢复查询；JSON/Stream 不能向调用方交付结果，因此转人工复核并原子释放本地冻结，不再自动结算。已绑定任务 ID 的加密请求载荷在错误路径中清理。

本地验证通过不等于上线通过。真实上游契约探测仍未执行：本机没有配置专用 `GRSAI_BASE` 和 `GRSAI_KEY`。不得以离线模拟代替 live probe，也不得在该门未通过时切入生产流量。生产灰度槽位和镜像须在部署前重新以 `gray-status.sh` 核对。

## 本轮证据

- `go test -tags=unit ./...`：通过；新增 handler 502/不扣费及绑定任务载荷清理测试另行定向通过。
- `go test -tags=integration ./...`：独立重跑通过；最初与其他全量任务并行时 repository 包曾失败，单包和随后全量独立重跑均通过，首次失败原因未得到确定定位。
- `go vet ./...`、前端 lint/typecheck、Vitest 299 文件/2274 项：通过。
- `python test/grsai_live_contract.py --self-test`：9/9 通过，未访问真实上游。
- AC 追溯脚本红绿自检通过；对 PRD 及 6 个含 AC 引用的第一方测试文件扫描，11 条声明/11 条引用，无孤儿或幽灵 ID。脚本仅检查文本引用，不证明测试语义。
- `git diff --check`：无格式错误。

## 上线前硬门

1. 取得专用上游凭据，显式执行一次 `python test/grsai_live_contract.py --live`；运输层结果不确定时先核查上游，不能自动重复 POST。协议须重复验证，核查结果查询与 Stream 断线恢复。
2. 合并前完成 PR CI、整体对外契约复核，并确认数据库迁移和冻结/释放/幂等账务风险；不得以本地测试替代生产观察。
3. 先在 0% 候选槽位用不可变 `sha-<commit>` 部署，验证健康检查、Nginx、人工关键路径和账务，再分阶段开启 JSON、Async、Stream，逐步切流并观察，最终晋升。异常时只用灰度脚本切回稳定槽位，保留现场。
