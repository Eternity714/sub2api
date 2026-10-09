---
description: 通过 Gkotta 的 Seedance 原生任务协议创建、查询、下载和删除视频任务。
---

# Seedance API

Seedance 使用 Ark 原生异步任务协议。通过 `/api/v3/contents/generations/tasks` 创建视频任务，保存返回的 `id`，用同一密钥查询完成状态，再从 `content.video_url` 下载结果。

## 使用条件与模型

在[可用渠道](https://www.gkotta.bid/available-channels)确认已有 Seedance 渠道后，到[API 密钥](https://www.gkotta.bid/keys)选择支持它的 OpenAI 或组合分组。该渠道需要管理员配置支持 Seedance 的上游和模型映射。

`YOUR_SEEDANCE_MODEL_ID` 是 Gkotta 向当前分组开放的模型 ID。它可能是一个公开别名；请使用渠道或 `/v1/models` 返回的 ID，不需要自行填写上游凭据或猜测接入点名称。

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_SEEDANCE_MODEL="YOUR_SEEDANCE_MODEL_ID"

curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

模型可见后，还需确认分组开放了媒体生成权限，并有足够的余额、订阅或密钥额度。普通 OpenAI 分组不会自动获得 Seedance 模型。

这里的多行 cURL 示例使用 Bash；Windows 用户可使用 Git Bash，或使用下方 PowerShell 示例。

## 与 Grok 视频协议的区别

| 项目 | Seedance | Grok 视频 |
| --- | --- | --- |
| 创建地址 | `/api/v3/contents/generations/tasks` | `/v1/videos/generations` |
| 输入内容 | `content` 数组 | `prompt` 等字段 |
| 创建返回 | `id` | `request_id` |
| 完成状态 | `succeeded` | `done` |
| 结果地址 | `content.video_url` | `video.url` 或 Gkotta 内容下载接口 |

按所用渠道选择完整的一套协议。Grok 的教程见[视频生成](/video-generation)。

## 创建文生视频任务

先提交最小请求：

```bash
curl --fail-with-body "https://www.gkotta.bid/api/v3/contents/generations/tasks" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_SEEDANCE_MODEL\",\"content\":[{\"type\":\"text\",\"text\":\"海边日出，轻柔的海浪，镜头缓慢推进\"}]}" \
  -o seedance-task.json
```

成功响应示例：

```json
{
  "id": "YOUR_RETURNED_TASK_ID"
}
```

保存 `id`，接收任务之后继续查询。模型支持时，可以在创建请求中添加 `duration`、`generate_audio` 等参数；时长与音频能力由实际模型决定。

```json
{
  "model": "YOUR_SEEDANCE_MODEL_ID",
  "content": [
    { "type": "text", "text": "海边日出，轻柔的海浪，镜头缓慢推进" }
  ],
  "duration": 5,
  "generate_audio": true
}
```

需要首帧图片时，在 `content` 中添加图片项，完整步骤见[图片转视频](/image-to-video)。

Windows PowerShell 创建示例：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_SEEDANCE_MODEL = "YOUR_SEEDANCE_MODEL_ID"
$headers = @{ Authorization = "Bearer $env:GKOTTA_API_KEY" }
$body = @{
  model = $env:GKOTTA_SEEDANCE_MODEL
  content = @(@{ type = "text"; text = "海边日出，轻柔的海浪，镜头缓慢推进" })
} | ConvertTo-Json -Depth 8

$task = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/api/v3/contents/generations/tasks" `
  -Method Post -Headers $headers -ContentType "application/json" `
  -Body ([Text.Encoding]::UTF8.GetBytes($body))
$task.id
```

## 查询任务

把返回的 `id` 填入变量，使用创建时的同一用户和同一 API Key：

```bash
export GKOTTA_SEEDANCE_TASK="YOUR_RETURNED_TASK_ID"
curl --fail-with-body "https://www.gkotta.bid/api/v3/contents/generations/tasks/$GKOTTA_SEEDANCE_TASK" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -o seedance-status.json
```

| 状态 | 含义与处理 |
| --- | --- |
| `queued` | 等待执行，间隔几秒后查询 |
| `running` | 正在生成，继续查询 |
| `succeeded` | 已完成，读取 `content.video_url` |
| `failed` | 失败，检查错误内容和输入参数 |
| `cancelled` | 已取消，停止查询 |
| `expired` | 已过期，停止查询并检查任务有效期 |

完成响应示例：

```json
{
  "id": "YOUR_RETURNED_TASK_ID",
  "status": "succeeded",
  "content": {
    "video_url": "https://YOUR_RESULT_HOST/output.mp4"
  },
  "usage": {
    "completion_tokens": 12345
  }
}
```

其他元数据和错误结构以实际响应为准。不要只根据 HTTP `200` 判断视频生成成功，还要检查任务 `status`。

PowerShell 查询：

```powershell
$status = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/api/v3/contents/generations/tasks/$($task.id)" `
  -Headers $headers
$status.status
$status.content.video_url
```

程序轮询应设置间隔和总等待时间；任务仍在排队或执行时，继续查询原任务即可。

## 下载生成结果

当状态为 `succeeded`，复制 `content.video_url` 并及时下载。这个地址由视频渠道返回，可能有有效期。

```bash
export GKOTTA_VIDEO_URL="YOUR_RETURNED_CONTENT_VIDEO_URL"
curl --fail-with-body "$GKOTTA_VIDEO_URL" -o seedance-output.mp4
```

PowerShell：

```powershell
if ($status.status -eq "succeeded" -and $status.content.video_url) {
  Invoke-WebRequest -Uri $status.content.video_url -OutFile "seedance-output.mp4"
}
```

下载返回的视频文件地址时，无需附加 Gkotta API Key。Seedance 的结果地址与 Grok 的 `/v1/videos/{request_id}/content` 下载接口不同。

## 删除或取消任务

需要删除任务时，向原任务地址发送 `DELETE`：

```bash
curl --fail-with-body -X DELETE \
  "https://www.gkotta.bid/api/v3/contents/generations/tasks/$GKOTTA_SEEDANCE_TASK" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

支持删除的状态及其对执行中任务的影响由上游决定。成功时可能返回 HTTP `204` 且没有正文；仍需保留返回状态。删除操作不能作为费用退还的依据。

## 常见错误

| 现象 | 处理方式 |
| --- | --- |
| 401 | 核对 Bearer 密钥及有效期 |
| 403 或分组错误 | 确认密钥属于开放 Seedance 的 OpenAI/组合分组，且有媒体生成权限 |
| `No eligible Seedance accounts` | 当前渠道未配置或没有可用的 Seedance 上游，联系管理员确认 |
| `model is required` 或 `content` 错误 | 提供有效模型 ID 和非空 `content` 数组 |
| 415 | 创建请求使用 `Content-Type: application/json` |
| 模型/参数不支持 | 按该模型要求调整时长、图片、音频和生成参数 |
| 查询或删除 404 | 核对任务 ID、原用户和原 API Key，并检查任务是否已过期 |
| 创建请求超时 | 保留已有任务 ID或错误响应，先确认任务是否已接收，再决定是否重新创建 |

Seedance 使用上游实际返回的完成用量计费，费用受模型定价和分组倍率影响。请在[使用记录](https://www.gkotta.bid/usage)核对消耗，阅读[计费与用量](/billing)了解账户额度。
