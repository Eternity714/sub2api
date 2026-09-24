# GRS.AI 多模式交付发布门（2026-09-24）

## 范围与结论

JSON、Async、Stream 共用上游 Stream 协议；Async 每用户等待 20、进行中 3。上游成功但缺结果 URL 时，Async 保留公开任务 ID 供恢复查询；JSON/Stream 不能向调用方交付结果，因此转人工复核并原子释放本地冻结，不再自动结算。已绑定任务 ID 的加密请求载荷在错误路径中清理。

本地验证通过不等于上线通过。用户已提供国际站测试凭据（不入库）。一次真实上游契约探测已执行，但返回 `transport_error`，结果不确定；不得自动重复 POST，也不得以离线模拟代替 live probe 或在该门未通过时切入生产流量。生产灰度槽位和镜像须在部署前重新以 `gray-status.sh` 核对。

2026-09-24 只读排查：探针进程继承本机 `HTTPS_PROXY=http://127.0.0.1:7890`；Python 对不存在任务的 GET 通过该代理超时，绕过代理的相同 GET 迅速返回 404，curl 两种路径均可达。代理超时是首轮失败的可能原因，不证明生成 POST 未到达上游。需先从上游后台核查任务/计费活动，再决定是否在只读直连检查通过后手动重试。探针已改为允许国际站 `https://grsaiapi.com`，并对 HTTP 状态码和传输超时做脱敏分类；首次运行的旧版本没有分类信息。不得记录密钥、完整任务 ID、原始响应或图片 URL。

生产核查：2026-09-24 稳定槽 blue、候选 green 0%，两槽健康。灰度 Compose 共用配置已加入 `stop_grace_period: 32m`，原文件备份为 `compose.yml.backup-20260924-grsai-grace`；`podman-compose config -q` 通过，展开配置中的 blue/green 均为 32 分钟。运行中的两容器尚未重建，实际 `stop_timeout` 仍为 10 秒。应用上游请求超时为 30 分钟，Worker 停机等待已开始请求完成。上线 Async 前，必须通过灰度脚本在 0% 候选槽位重建并核对实际停止超时；不能直接依赖配置文件或绕开灰度脚本切流。

## 本轮证据

- `go test -tags=unit ./...`：通过；新增 handler 502/不扣费及绑定任务载荷清理测试另行定向通过。
- `go test -tags=integration ./...`：独立重跑通过；最初与其他全量任务并行时 repository 包曾失败，单包和随后全量独立重跑均通过，首次失败原因未得到确定定位。
- `go vet ./...`、前端 lint/typecheck、Vitest 299 文件/2274 项：通过。
- `python test/grsai_live_contract.py --self-test`：9/9 通过，未访问真实上游。
- AC 追溯脚本红绿自检通过；对 PRD 及 6 个含 AC 引用的第一方测试文件扫描，11 条声明/11 条引用，无孤儿或幽灵 ID。仓库内的 `test/grsai_ac_traceability.py` 已接入 CI。脚本仅检查文本引用，不证明测试语义。
- `git diff --check`：无格式错误。

## 上线前硬门

1. 核查上游后台有无首次探测的任务及计费，确认后再手动决定是否重试 `python test/grsai_live_contract.py --live`；传输结果不确定时不能自动重复 POST。协议须重复验证，核查结果查询与 Stream 断线恢复。
2. 合并前完成 PR CI、整体对外契约复核，并确认数据库迁移和冻结/释放/幂等账务风险；不得以本地测试替代生产观察。
3. 已写入灰度槽位停止宽限期配置，但实际容器仍待验证。在 0% 候选槽位用不可变 `sha-<commit>` 部署后，先核对 `stop_timeout`、健康检查、Nginx、人工关键路径和账务，分阶段开启 JSON、Async、Stream，逐步切流并观察，最终晋升。异常时只用灰度脚本切回稳定槽位，保留现场。
