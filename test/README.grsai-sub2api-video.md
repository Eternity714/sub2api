# Sub2API GRS.AI 视频测试

通过 Sub2API 的 `/v1/api/generate` 发起一次 `minimax-h3` 视频请求。

```powershell
$env:SUB2API_BASE = "https://灰度候选地址"
$env:SUB2API_KEY = "sk-..."
python test/grsai_sub2api_video.py
```

可选环境变量：`GRSAI_VIDEO_MODEL`、`GRSAI_VIDEO_DURATION`、`GRSAI_VIDEO_RESOLUTION`、`GRSAI_VIDEO_PROMPT`。
