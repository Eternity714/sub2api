---
description: 使用 Seedance 首帧图片或 Grok 图片输入，将静态图片生成视频。
---

# 图片转视频

图片转视频以一张输入图片作为画面参考，再通过提示词描述主体、镜头和场景如何运动。Gkotta 已提供 Seedance 首帧图片请求和 Grok 图片输入请求，选择与你的密钥分组对应的协议即可。

## 准备输入图片

可以使用已有图片，也可以先按[图片生成与编辑](/image-generate)创建一张图片。选择主体清楚、尺寸符合模型要求的输入，先用短视频验证效果。

将图片放在视频渠道能够直接访问的 HTTPS 地址。`C:\Pictures\source.png`、`file://` 路径、需要网页登录的地址以及只能在本机访问的 URL 都不能直接填进示例中的图片地址字段。

如果使用带签名的图片 URL，确保任务处理期间地址仍然有效。提示词建议描述动作和镜头，例如“猫轻轻抬头，镜头缓慢推进”；需要保留的外观、构图和背景也可以明确写出。

## 选择渠道和模型

在[可用渠道](https://www.gkotta.bid/available-channels)确认图片转视频模型，再在[API 密钥](https://www.gkotta.bid/keys)选择对应分组的密钥。

| 协议 | 分组条件 | 图片字段 | 创建返回 |
| --- | --- | --- | --- |
| Seedance 原生 | 开放 Seedance 的 OpenAI/组合分组 | `content[].image_url`，首帧角色为 `first_frame` | `id` |
| Grok 视频 | Grok 分组或路由到 Grok 的组合分组 | `image.url` | `request_id` |

模型和媒体生成权限必须由分组实际开放。下方模型占位符需替换为 `/v1/models` 或渠道说明中给出的真实 ID；一个模型支持文生视频，不代表它支持图片输入。

下面的多行 cURL 示例使用 Bash，可在 macOS、Linux 或 Git Bash 运行。Windows PowerShell 示例放在 Seedance 步骤后。

## Seedance：使用图片作为首帧

设置密钥、模型和图片地址：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_SEEDANCE_MODEL="YOUR_SEEDANCE_MODEL_ID"
export GKOTTA_INPUT_IMAGE="https://YOUR_ASSET_HOST/source.png"

curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

创建请求的 `content` 包含文字提示词与图片项，图片项的 `role` 使用 `first_frame`：

```bash
curl --fail-with-body "https://www.gkotta.bid/api/v3/contents/generations/tasks" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_SEEDANCE_MODEL\",\"content\":[{\"type\":\"text\",\"text\":\"保持猫的外观和背景，猫缓慢抬头，镜头轻轻推进\"},{\"type\":\"image_url\",\"image_url\":{\"url\":\"$GKOTTA_INPUT_IMAGE\"},\"role\":\"first_frame\"}]}" \
  -o seedance-task.json
```

保存响应中的 `id`，使用创建时的同一密钥查询：

```bash
export GKOTTA_SEEDANCE_TASK="YOUR_RETURNED_TASK_ID"
curl --fail-with-body "https://www.gkotta.bid/api/v3/contents/generations/tasks/$GKOTTA_SEEDANCE_TASK" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -o seedance-status.json
```

`queued` 或 `running` 时等待后继续查询；`succeeded` 时读取 `content.video_url` 并下载：

```bash
curl --fail-with-body "YOUR_RETURNED_CONTENT_VIDEO_URL" -o image-to-video.mp4
```

下载渠道返回的结果 URL 时不附加 Gkotta 密钥。`failed`、`cancelled`、`expired` 为需要停止轮询并检查原因的状态。参数扩展、完整状态和取消操作见 [Seedance API](/seedance-api)。

### Windows PowerShell

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_SEEDANCE_MODEL = "YOUR_SEEDANCE_MODEL_ID"
$imageUrl = "https://YOUR_ASSET_HOST/source.png"
$headers = @{ Authorization = "Bearer $env:GKOTTA_API_KEY" }
$body = @{
  model = $env:GKOTTA_SEEDANCE_MODEL
  content = @(
    @{ type = "text"; text = "保持主体外观，缓慢抬头，镜头轻轻推进" }
    @{ type = "image_url"; image_url = @{ url = $imageUrl }; role = "first_frame" }
  )
} | ConvertTo-Json -Depth 8

$task = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/api/v3/contents/generations/tasks" `
  -Method Post -Headers $headers -ContentType "application/json" `
  -Body ([Text.Encoding]::UTF8.GetBytes($body))
$task.id
```

隔几秒后用同一终端和密钥查询，直到任务进入最终状态：

```powershell
$status = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/api/v3/contents/generations/tasks/$($task.id)" `
  -Headers $headers
$status.status

if ($status.status -eq "succeeded" -and $status.content.video_url) {
  Invoke-WebRequest -Uri $status.content.video_url -OutFile "image-to-video.mp4"
}
```

## Grok：使用图片输入生成视频

切换到开放 Grok 图片转视频能力的密钥和模型。Grok 请求使用 `image` 对象，任务结果通过 `request_id` 查询。

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_VIDEO_MODEL="YOUR_VIDEO_MODEL_ID"
export GKOTTA_INPUT_IMAGE="https://YOUR_ASSET_HOST/source.png"

curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/videos/generations" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_VIDEO_MODEL\",\"prompt\":\"保持猫的外观，猫缓慢抬头，镜头轻轻推进\",\"image\":{\"url\":\"$GKOTTA_INPUT_IMAGE\"},\"duration\":6}" \
  -o video-task.json
```

`duration` 是示例时长，按所选模型支持的范围调整。使用创建响应的 `request_id` 查询：

```bash
export GKOTTA_VIDEO_TASK="YOUR_RETURNED_REQUEST_ID"
curl --fail-with-body "https://www.gkotta.bid/v1/videos/$GKOTTA_VIDEO_TASK" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -o video-status.json
```

状态为 `done` 且存在 `video.url` 后，使用相同密钥从 Gkotta 内容接口下载：

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/videos/$GKOTTA_VIDEO_TASK/content" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -o grok-image-to-video.mp4
```

`pending` 时继续按间隔查询；`failed` 或 `expired` 时读取错误内容。完整查询、编辑与续写步骤见[视频生成](/video-generation)，其中视频编辑和续写要求 Grok 渠道；组合分组必须有匹配并指向 Grok 的模型路由。

## 改进效果与排错

- **输入图片无法读取**：检查图片 URL 能否直接下载、格式是否支持、签名是否过期，以及渠道是否可访问该地址。
- **模型拒绝图片输入**：换用当前分组明确支持图片转视频的模型，确认使用了对应协议的图片字段。
- **主体变化较大**：使用主体更清楚的图片，降低复杂动作描述，并明确需要保留的外观和构图；具体效果取决于模型。
- **创建成功但查不到任务**：核对 `id` 与 `request_id` 的区别，并继续使用创建时的原用户和原 API Key。
- **视频处理失败**：读取最终状态和错误信息，检查图片、提示词及模型参数后再新建任务。
- **请求超时**：保留已经返回的任务 ID，先查询原任务，避免重复创建产生额外消耗。

完成后及时保存视频，在[使用记录](https://www.gkotta.bid/usage)核对结果和费用；更多接入问题见[常见问题](/troubleshooting)。
