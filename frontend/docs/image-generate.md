---
description: 使用 Gkotta 的 OpenAI 兼容与 Gemini 原生接口生成、编辑和保存图片。
---

# 图片生成与编辑

Gkotta 提供 OpenAI 兼容 Images 接口和 Gemini 原生图片接口。先确认密钥的分组、图片模型和协议，再选择下方对应的请求格式。

## 准备密钥与模型

1. 在[API 密钥](https://www.gkotta.bid/keys)创建或选择一个密钥。
2. 在[可用渠道](https://www.gkotta.bid/available-channels)确认分组开放了图片模型与对应接口。OpenAI Images 和 Grok 图片接口还要求分组开启图片生成权限。
3. 将 `YOUR_IMAGE_MODEL_ID` 替换为当前分组实际可用的图片模型 ID。文本模型不能直接代替图片模型。

macOS 或 Linux：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_IMAGE_MODEL="YOUR_IMAGE_MODEL_ID"
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_IMAGE_MODEL = "YOUR_IMAGE_MODEL_ID"
```

下面的 cURL 多行命令使用 Bash 写法，可在 macOS、Linux 或 Windows 的 Git Bash 中运行。

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

使用响应 `data[].id` 中与你的图片渠道对应的 ID。模型列表展示成功后，生成请求还会检查额度、分组权限及上游能力。Gemini 原生接口也可通过 `/v1beta/models` 查询模型，见 [Gemini API](/gemini-api)。

## 选择接口

| 接口 | 请求格式 | 支持条件 |
| --- | --- | --- |
| `/v1/images/generations` | OpenAI Images JSON | OpenAI 或 Grok 图片分组，或路由到相应平台的组合分组 |
| `/v1/images/edits` | OpenAI Images multipart 或受支持的 JSON | 分组和上游支持图片编辑 |
| `/v1beta/models/{model}:generateContent` | Gemini `contents` | Gemini 分组，或路由到 Gemini 的组合分组；所选模型支持图片输出 |

Gemini 图片模型也可以通过提供 OpenAI Images 协议的兼容上游调用 `/v1/images/*`。这种方式需要配置了该能力的 OpenAI API Key 渠道。持有 Gemini 原生分组的密钥时，应使用 `/v1beta` 教程中的格式。

## OpenAI 兼容图片生成

先用一张图片验证接入。下面的尺寸是示例值，支持范围由所选模型决定。

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/images/generations" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_IMAGE_MODEL\",\"prompt\":\"一只坐在窗边的橘猫，柔和的自然光\",\"n\":1,\"size\":\"1024x1024\"}" \
  -o image-response.json
```

请求字段：

| 字段 | 用途 |
| --- | --- |
| `model` | 当前分组开放的图片模型 ID |
| `prompt` | 描述主体、场景、风格和需要保留的细节 |
| `n` | 图片数量，先从 `1` 开始 |
| `size` | 图片尺寸；按模型支持范围设置 |

部分模型还支持 `quality`、`background`、`output_format` 等参数。先跑通最小请求，再按模型说明逐项添加；上游不支持的选项可能返回参数错误。

成功响应通常包含 `data` 数组，图片可能以 Base64 或 URL 返回，例如：

```json
{
  "data": [
    { "b64_json": "BASE64_IMAGE_DATA" }
  ]
}
```

### 保存图片

如果返回 `b64_json`，用 Python 标准库解码。将输出文件扩展名与实际图片格式保持一致；下面以 PNG 为例。

```python
import base64
import json
from pathlib import Path

response = json.loads(Path("image-response.json").read_text(encoding="utf-8"))
image = response["data"][0]
if "b64_json" not in image:
    raise SystemExit("响应使用 URL，请按下方 URL 下载方式保存。")
Path("output.png").write_bytes(base64.b64decode(image["b64_json"]))
```

如果返回 `data[0].url`，复制该 URL 后下载。签名链接可能有有效期，请及时保存文件。

```bash
curl --fail-with-body "YOUR_IMAGE_DOWNLOAD_URL" -o output.png
```

上面的下载命令用于响应提供的图片下载地址，不向第三方存储地址发送 Gkotta 密钥。

## 上传图片进行编辑

将待编辑文件放在当前目录，例如 `source.png`。用 multipart 上传时，让 cURL 自动生成 `Content-Type` 和 boundary。

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/images/edits" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -F "model=$GKOTTA_IMAGE_MODEL" \
  -F "prompt=保留猫的外观，将背景改为有绿植的明亮客厅" \
  -F "image=@source.png" \
  -o image-response.json
```

在 Windows PowerShell 中调用 cURL 时使用 `curl.exe`，避免某些版本的 `curl` 别名行为差异：

```powershell
curl.exe --fail-with-body "https://www.gkotta.bid/v1/images/edits" `
  -H "Authorization: Bearer $env:GKOTTA_API_KEY" `
  -F "model=$env:GKOTTA_IMAGE_MODEL" `
  -F "prompt=Keep the cat and change the background to a bright living room" `
  -F "image=@source.png" `
  -o image-response.json
```

编辑响应的保存方式与生成响应相同。多图输入、蒙版、文件大小和图片格式等限制由模型及渠道决定，先用单张图片验证，再扩展请求。

## Gemini 原生图片生成

切换到支持 Gemini 原生协议和图片输出的密钥与模型后，发送以下请求：

```bash
curl --fail-with-body "https://www.gkotta.bid/v1beta/models/${GKOTTA_IMAGE_MODEL}:generateContent" \
  -H "x-goog-api-key: $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data '{"contents":[{"role":"user","parts":[{"text":"生成一张窗边橘猫的图片，柔和自然光"}]}],"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}' \
  -o gemini-image-response.json
```

响应中查看 `candidates[].content.parts`：文本可能位于 `text`，图片位于 `inlineData.data`，格式由 `inlineData.mimeType` 指示。并非每次响应都会有图片；还应检查内容过滤或错误消息。

下面的脚本保存第一张内嵌图片：

```python
import base64
import json
from pathlib import Path

response = json.loads(Path("gemini-image-response.json").read_text(encoding="utf-8"))
for candidate in response.get("candidates", []):
    for part in candidate.get("content", {}).get("parts", []):
        image = part.get("inlineData") or part.get("inline_data")
        if image and image.get("data"):
            mime = image.get("mimeType") or image.get("mime_type", "image/png")
            suffix = {"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}.get(mime, ".bin")
            Path("gemini-output" + suffix).write_bytes(base64.b64decode(image["data"]))
            raise SystemExit(0)
raise SystemExit("响应没有图片，请检查模型、候选内容和错误信息。")
```

### Gemini 原生图片编辑

编辑时把图片字节编码成 Base64，放在同一个 `parts` 数组的 `inlineData` 中。下面的 Python 示例只使用标准库，会读取本地图片并发送请求：

```python
import base64
import json
import os
from pathlib import Path
from urllib.request import Request, urlopen

model = os.environ["GKOTTA_IMAGE_MODEL"]
body = {
    "contents": [{
        "role": "user",
        "parts": [
            {"text": "保留主体外观，将背景改为明亮客厅"},
            {"inlineData": {
                "mimeType": "image/png",
                "data": base64.b64encode(Path("source.png").read_bytes()).decode("ascii"),
            }},
        ],
    }],
    "generationConfig": {"responseModalities": ["TEXT", "IMAGE"]},
}
request = Request(
    f"https://www.gkotta.bid/v1beta/models/{model}:generateContent",
    data=json.dumps(body).encode("utf-8"),
    headers={"x-goog-api-key": os.environ["GKOTTA_API_KEY"], "Content-Type": "application/json"},
    method="POST",
)
with urlopen(request, timeout=300) as response:
    Path("gemini-image-response.json").write_bytes(response.read())
```

使用 JPEG 或 WebP 输入时，同时调整文件名和 `mimeType`。保存输出时复用前面的 Gemini 解码脚本。

## 可选：异步图片任务

如果当前部署已启用异步图片与图片存储，可以将 OpenAI Images 请求提交到 `/v1/images/generations/async`，或将编辑请求提交到 `/v1/images/edits/async`。该扩展要求 OpenAI 或 Grok 分组；组合分组请使用同步接口。

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/images/generations/async" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_IMAGE_MODEL\",\"prompt\":\"窗边的橘猫\",\"n\":1}" \
  -o image-task.json
```

收到 HTTP `202` 后，保存响应的 `task_id` 和 `poll_url`。使用创建时的同一密钥查询：

```bash
export GKOTTA_IMAGE_TASK="YOUR_RETURNED_TASK_ID"
curl --fail-with-body "https://www.gkotta.bid/v1/images/tasks/$GKOTTA_IMAGE_TASK" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

`processing` 表示仍在处理，按 `Retry-After` 提示间隔查询；`completed` 时读取 `image_url` 或 `result.data` 并保存图片；`failed` 时读取 `error`。任务和下载链接有有效期，及时保存结果。接口返回 `async image tasks are not enabled` 时，使用同步接口或联系管理员确认开放情况。异步请求不能设置 `stream: true`。

## 失败时检查

| 现象 | 检查方法 |
| --- | --- |
| 401 | 密钥是否有效，鉴权头是否完整 |
| 403 或图片权限未开启 | 核对所用接口的分组访问条件；OpenAI Images/Grok 接口还需图片生成权限。检查余额、订阅和密钥配额 |
| 404 或模型不可用 | 模型 ID、分组、所选协议和渠道能力是否一致 |
| 参数错误 | 从单张图片和最少参数开始，检查模型支持的尺寸、数量与编辑格式 |
| 413 或上传失败 | 缩小源图并检查文件类型、请求体大小限制 |
| 429、超时或上游错误 | 保存错误消息，按重试提示等待；超时后的生成结果可能已产生，不要连续重复提交 |

费用与调用结果可在[使用记录](https://www.gkotta.bid/usage)核对，额度说明见[计费与用量](/billing)。若需要让生成的图片运动起来，继续阅读[图片转视频](/image-to-video)。
