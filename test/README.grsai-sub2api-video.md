# Sub2API GRS.AI 视频实测

脚本只通过 Sub2API 的 POST `/v1/api/generate` 提交一次付费请求，随后 GET `/v1/api/result?id=...` 轮询。成功后下载全部 MP4，验证文件签名并保存 SHA256，不按结果数量重复请求。

默认 `minimax-h3`、480p、1 秒；价格取 API key 所在分组的渠道配置。

针对 0% green 候选的入口先检查灰度槽位和镜像、再建立本机 SSH 隧道并检查 `/health`，通过后才提交一次生成请求。已经存在的本机隧道会复用；脚本新建的隧道在退出时关闭。默认用当前代码提交的 `sha-<7位提交>` 核对候选镜像；也可用 `-ExpectedTag` 显式指定。

```powershell
$env:SUB2API_KEY = '<grsai 测试分组的密钥>'
./test/grsai_green_video.ps1 -HealthOnly
./test/grsai_green_video.ps1
```

```powershell
$env:SUB2API_KEY = "<测试分组的密钥>"
python test/grsai_sub2api_video.py --base http://127.0.0.1:18082
# 或加载本地已有配置（不提交 .env）
python test/grsai_sub2api_video.py --env-file test/.env --base http://127.0.0.1:18082
# 超时/中断后用 output 中 task.json 的 id 续查，不会再次扣费提交
python test/grsai_sub2api_video.py --base http://127.0.0.1:18082 --task-id <id>
```

本地 HTTP 地址应通过 SSH 隧道连接当前无流量候选端口。密钥只在进程内使用，不发送给视频存储地址，不打印响应体或签名链接。
产物默认位于 `test/output/grsai-video/`（已忽略）：任务 ID、视频、脱敏摘要。`--duration`、`--resolution`、`--timeout` 可调整。发生提交网络错误不自动重发，需要先核对服务器任务以避免重复付费。
