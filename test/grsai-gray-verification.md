# GRSAI 视频 green 候选实测

日期：2026-09-29。候选代码 `6a747feaba4ad95430760288d5a13a2552e20d2e`，镜像 `ghcr.io/eternity714/sub2api:sha-6a747fe`。CI 和 GHCR 发布成功；`gray-deploy.sh green sha-6a747fe` 已在 0% 流量部署。稳定 blue 仍为 `sha-7a339bd`。

本机 SSH 隧道：`127.0.0.1:18082` → 服务器 `127.0.0.1:18082`（green）。`test/grsai_green_video.ps1` 的健康模式通过；green 容器健康、`/health` 返回 200，`nginx -t` 通过。

## 实际请求与结果

- 第一次测试任务 `grsai_f89d3236-ba56-48e9-8737-01c7f0dae07e`：`task_version=3` 的加密请求体查询误限版本 2，未调用上游即失败。修复并加入集成回归测试。结算 0，冻结已释放。
- 修复后任务 `grsai_ae28fffc-b21b-4899-9ac1-a309a105bf4d`：确已提交上游，上游返回 `aspectRatio must be portrait, landscape or square`；测试脚本补 `aspectRatio=landscape`。结算 0，冻结已释放。没有自动重发。
- 最终任务 `grsai_77e646cb-e3ac-4f2e-8fbe-1f810f51df79`：经本机 green 隧道 POST 一次，`minimax-h3`、480p、请求 `duration=1`、横屏、`replyType=async`。任务成功，返回 1 个结果 URL；脚本下载 MP4，789155 字节，SHA256 `3264b9f7f4be8c57f1472e234e53ae3c7f041e30c2215ecce556df8f48d23e81`。ffprobe 检出 H.264 864×480、AAC、媒体时长 1.625 秒。

账务：任务 `id=25` 的 `settled_amount=0.10`，冻结 `0.10` 已捕获；测试 Key `id=16` 的 `quota_used` 从 `0.88` 变为 `0.98`；对应的使用日志恰好 1 条，`billing_mode=video`、`video_count=1`、`video_duration_seconds=1`、`video_resolution=480p`、`total_cost=actual_cost=0.10`。按请求时长而不是 MP4 媒体时长计费。

验证结束时 `gray-status.sh`：stable=blue、candidate=green、candidate_percent=0，二者健康；Nginx 配置检查通过。尚未扩大流量或晋升，后续流量步骤需按项目灰度规则执行。

产物（Git 忽略）：`test/output/grsai-video/green-6a747fe-aspect/` 内的 `task.json`、`summary.json`、`video-0.mp4`。不存储凭据或签名 URL。