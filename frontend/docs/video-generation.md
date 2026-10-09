---
description: 使用 Gkotta 的 Grok 视频接口创建任务、查询状态、下载结果并编辑或续写视频。
---

# 视频生成

Gkotta 的 Grok 视频接口采用异步任务方式：提交提示词，保存任务 ID，使用同一密钥查询状态，完成后下载视频。Seedance 使用另一套原生协议，接入方法见 [Seedance API](/seedance-api)。

## 确认视频渠道

在[可用渠道](https://www.gkotta.bid/available-channels)确认当前分组开放了 Grok 视频模型和媒体生成权限，再到[API 密钥](https://www.gkotta.bid/keys)选择对应分组的密钥。

- 文生视频需要支持文本输入的视频模型。
- 部分模型仅支持图片转视频，使用这类模型时按[图片转视频](/image-to-video)添加输入图片。
- Grok 视频创建和查询可以使用 Grok 分组，或路由到 Grok 的组合分组。
- 视频编辑与续写接口要求 Grok 渠道；组合分组须有匹配所选模型并指向 Grok 的路由。

将模型占位符替换为该密钥实际可用的视频模型 ID。模型列表里有名称还不代表它支持每一种视频操作，应结合渠道说明确认。

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_VIDEO_MODEL="YOUR_VIDEO_MODEL_ID"

curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

这里及后面的多行 cURL 示例使用 Bash。Windows 用户也可使用 Git Bash，或使用下方 PowerShell 示例。

## 视频接口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/v1/videos/generations` | 提交视频生成任务 |
| POST | `/v1/videos` | 视频生成的兼容入口 |
| GET | `/v1/videos/{request_id}` | 查询任务状态 |
| GET | `/v1/videos/{request_id}/content` | 下载完成的视频 |
| POST | `/v1/videos/edits` | 以原视频为输入进行编辑，要求 Grok 渠道或明确路由到 Grok 的组合分组 |
| POST | `/v1/videos/extensions` | 续写原视频，要求 Grok 渠道或明确路由到 Grok 的组合分组 |

全部请求使用 `Authorization: Bearer <Gkotta API Key>`。保存创建任务使用的密钥；改用同账户下另一个密钥查询，也可能得到任务不存在。

## 创建文生视频任务

先选择短时长进行验证。`duration` 的单位是秒，时长、分辨率和比例的允许范围由模型决定。

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/videos/generations" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_VIDEO_MODEL\",\"prompt\":\"海边日出，轻柔的海浪，镜头缓慢向前移动\",\"duration\":6,\"aspect_ratio\":\"16:9\",\"resolution\":\"720p\"}" \
  -o video-task.json
```

成功提交的响应示例：

```json
{
  "request_id": "YOUR_RETURNED_REQUEST_ID"
}
```

任务 ID 表示请求已接收，需要继续查询；创建响应中没有最终视频文件。保存这个 ID，不要把它当作模型名。

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_VIDEO_MODEL = "YOUR_VIDEO_MODEL_ID"
$headers = @{ Authorization = "Bearer $env:GKOTTA_API_KEY" }
$body = @{
  model = $env:GKOTTA_VIDEO_MODEL
  prompt = "海边日出，轻柔的海浪，镜头缓慢向前移动"
  duration = 6
  aspect_ratio = "16:9"
  resolution = "720p"
} | ConvertTo-Json

$task = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/v1/videos/generations" `
  -Method Post -Headers $headers -ContentType "application/json" `
  -Body ([Text.Encoding]::UTF8.GetBytes($body))
$task.request_id
```

## 查询状态

把创建响应的 `request_id` 填入变量，使用同一个密钥查询：

```bash
export GKOTTA_VIDEO_TASK="YOUR_RETURNED_REQUEST_ID"
curl --fail-with-body "https://www.gkotta.bid/v1/videos/$GKOTTA_VIDEO_TASK" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -o video-status.json
```

处理中通常返回 `status: "pending"`。完成响应示例：

```json
{
  "status": "done",
  "video": {
    "url": "https://www.gkotta.bid/v1/videos/YOUR_RETURNED_REQUEST_ID/content",
    "duration": 6
  }
}
```

上游可能返回更多字段。以实际响应为准；看到 `done` 且存在视频地址后再下载。若返回 `failed`、`expired` 或错误内容，应停止轮询并检查原因。

查询时留出几秒间隔，并为程序设置总等待时间。轮询状态接口即可，不需要重复提交生成请求。

PowerShell 查询：

```powershell
$status = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/v1/videos/$($task.request_id)" `
  -Headers $headers
$status.status
$status.video
```

## 下载视频

完成后通过 Gkotta 的内容接口下载，携带创建任务使用的密钥：

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/videos/$GKOTTA_VIDEO_TASK/content" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -o output.mp4
```

PowerShell：

```powershell
Invoke-WebRequest `
  -Uri "https://www.gkotta.bid/v1/videos/$($task.request_id)/content" `
  -Headers $headers -OutFile "output.mp4"
```

如果下载失败，先查看 HTTP 状态与错误消息，确认任务已完成。不要将 JSON 错误响应当作有效 MP4 使用。

## 编辑现有视频

选择支持视频编辑的模型和 Grok 渠道。使用组合分组时，模型路由必须匹配并指向 Grok；仅有组合分组本身不能保证编辑接口可用。提供该渠道能访问的视频 URL，需要登录、只在本机可访问或已过期的地址无法作为输入。

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/videos/edits" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_VIDEO_MODEL\",\"prompt\":\"保持镜头运动，将场景变为黄昏\",\"video\":{\"url\":\"https://YOUR_ASSET_HOST/source.mp4\"}}" \
  -o video-task.json
```

编辑也会创建异步任务，按返回的 `request_id` 使用前面的状态与下载接口处理结果。原视频长度、格式和可用编辑参数以所选模型要求为准。

## 续写视频

使用支持续写的模型和 Grok 渠道，组合分组同样需要明确路由到 Grok。描述后续镜头并提供输入视频：

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/videos/extensions" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_VIDEO_MODEL\",\"prompt\":\"镜头继续向海面推进，太阳缓缓升起\",\"video\":{\"url\":\"https://YOUR_ASSET_HOST/source.mp4\"}}" \
  -o video-task.json
```

同样保存 `request_id` 并查询、下载。编辑和续写开放情况由渠道决定；一个模型支持生成，并不保证它支持这两个操作。

## 失败与计费

| 现象 | 处理方式 |
| --- | --- |
| 401 | 检查 Bearer 密钥、有效期和实际使用的环境变量 |
| 403 | 核对分组媒体生成权限、额度和分组协议 |
| `No eligible Grok media accounts` | 当前渠道没有可用的视频上游，联系管理员确认或稍后再试 |
| 模型或参数错误 | 确认模型支持文生视频或图生视频，以及请求的时长、分辨率与比例 |
| 状态查询 404 | 使用原来的任务 ID、原用户及原密钥；确认任务记录仍有效 |
| `failed` 或 `expired` | 阅读实际错误原因，调整输入后再决定是否新建任务 |
| 提交后断连或超时 | 先保留响应和请求信息并确认任务是否已接收，避免盲目重复创建 |
| 下载出错 | 先查询任务状态，确认完成后使用 Gkotta 内容接口和原密钥下载 |

视频费用受实际模型、生成时长、分辨率及分组倍率影响。完成后的费用以[使用记录](https://www.gkotta.bid/usage)为准，详见[计费与用量](/billing)。
