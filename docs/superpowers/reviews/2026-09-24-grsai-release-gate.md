# GRS.AI 多模式交付发布门（2026-09-24）

## 范围与结论

JSON、Async、Stream 共用上游 Stream 协议；Async 每用户等待 20、进行中 3。上游成功但缺结果 URL 时，Async 保留公开任务 ID 供恢复查询；JSON/Stream 不能向调用方交付结果，因此转人工复核并原子释放本地冻结，不再自动结算。已绑定任务 ID 的加密请求载荷在错误路径中清理。

本地验证通过不等于上线通过。用户已提供国际站测试凭据（不入库）。用户提供的 12:29:12 成功调用详情与探针的模型、`replyType=stream`、提示词一致；详情 JSON 显示 `succeeded`、progress 100 和 1 个结果，确认该次生成成功。15:21 对该任务 ID 做只读结果查询时，上游返回 `result not exist, valid for 2 hours`；该任务当时已超过两小时结果保留期，不视作协议失败。为补充原始流验证，15:23 通过 `NO_PROXY` 直连执行一次新 live probe，客户端返回 HTTP 404，但现有探针输出没有标明错误发生在生成 POST 还是结果 GET；不得再次重试，先核查上游约 15:23 的请求记录。此 live 门仍未通过，不能切入生产流量。生产灰度槽位和镜像须在部署前重新以 `gray-status.sh` 核对。

后续核查（2026-09-24）：上游后台确认 15:22:37 的第二个 `replyType=stream` 生成成功，耗时 6 秒、有 1 个结果；15:26 对该任务 ID 只读查询 `/v1/api/result` 返回 HTTP 404，仍在两小时窗口内，不能解释为过期。一次受控 `replyType=async` 对照只提交了一个 POST，返回 HTTP 200、任务 ID 和 `running`；对同一 ID 的只读 GET 依次得到 `running`、`running`、`succeeded`，最后包含一个结果。证据表明这两个实测任务的结果查询能力不同，尚不能断言所有 Stream ID 都不可查询，但已足以否定“任何 Stream ID 都能由 `/result` 恢复”的发布前提。禁止为排查该 404 重复生成 POST。当前仍为 **no-go**，不合并、不切生产流量。

2026-09-24 只读排查：探针进程继承本机 `HTTPS_PROXY=http://127.0.0.1:7890`；Python 对不存在任务的 GET 通过该代理超时，绕过代理的相同 GET 迅速返回 404，curl 两种路径均可达。用户随后提供的成功记录证明 12:29 请求已生成成功；15:21 查询其结果得到 `result not exist, valid for 2 hours`，与已过期相符。15:23 的新 live probe 通过 `NO_PROXY` 直连后得到 HTTP 404，当前错误输出不能区分生成接口与结果接口；禁止自动再次 POST，必须先核查上游该时刻的请求活动。探针已改为允许国际站 `https://grsaiapi.com`，并对 HTTP 状态码和传输超时做脱敏分类。不得记录密钥、完整任务 ID、原始响应或图片 URL。

生产核查：2026-09-24 稳定槽 blue、候选 green 0%，两槽健康。灰度 Compose 共用配置已加入 `stop_grace_period: 32m`，原文件备份为 `compose.yml.backup-20260924-grsai-grace`；`podman-compose config -q` 通过，展开配置中的 blue/green 均为 32 分钟。运行中的两容器尚未重建，实际 `stop_timeout` 仍为 10 秒。应用上游请求超时为 30 分钟，Worker 停机等待已开始请求完成。上线 Async 前，必须通过灰度脚本在 0% 候选槽位重建并核对实际停止超时；不能直接依赖配置文件或绕开灰度脚本切流。

## 本轮证据

- `go test -tags=unit ./...`：通过；新增 handler 502/不扣费及绑定任务载荷清理测试另行定向通过。
- `go test -tags=integration ./...`：独立重跑通过；最初与其他全量任务并行时 repository 包曾失败，单包和随后全量独立重跑均通过，首次失败原因未得到确定定位。
- `go vet ./...`、前端 lint/typecheck、Vitest 299 文件/2274 项：通过。
- `python test/grsai_live_contract.py --self-test`：当前 13/13 通过，未访问真实上游（本轮增加 GET 404 阶段回归）；此前已记录的 Go 与前端全量结果未因本轮文档/探针改动重跑。
- 探针现已在脱敏失败输出中区分 `generate_post`、`stream_read`、`result_get` 和 `result_parse` 阶段；离线回归覆盖“Stream 成功、GET 404”，不会误报为生成 POST 404。此改动不能替代真实协议验收。
- AC 追溯脚本红绿自检通过；对 PRD 及 6 个含 AC 引用的第一方测试文件扫描，11 条声明/11 条引用，无孤儿或幽灵 ID。仓库内的 `test/grsai_ac_traceability.py` 已接入 CI。脚本仅检查文本引用，不证明测试语义。
- `git diff --check`：无格式错误。

## 上线前硬门

1. 上游后台已确认 15:22:37 的 Stream 生成成功，其 ID 在窗口内的结果 GET 为 404；Async 对照的结果 GET 已通过。需向供应商确认 Stream ID 的 `/result` 支持范围和一致性，不以更多重复 POST 或放宽探针掩盖差异。评审产品契约与技术路线：例如 Async 上游改用原生 Async、JSON/Stream 保持 Stream 并为断线和进程重启提供可验证的可靠恢复，或显式收缩承诺并重新审阅 PRD/AC-007；未经确认不得擅自削弱可恢复性。任何方案都须补对应真协议和断线/重启/账务回归。
2. 合并前完成 PR CI、整体对外契约复核，并确认数据库迁移和冻结/释放/幂等账务风险；不得以本地测试替代生产观察。
3. 已写入灰度槽位停止宽限期配置，但实际容器仍待验证。在 0% 候选槽位用不可变 `sha-<commit>` 部署后，先核对 `stop_timeout`、健康检查、Nginx、人工关键路径和账务，分阶段开启 JSON、Async、Stream，逐步切流并观察，最终晋升。异常时只用灰度脚本切回稳定槽位，保留现场。
