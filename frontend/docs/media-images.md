---
description: 统一媒体 API 的文生图、参考图生图、请求参数和调用示例。
---

# 图片生成 API

通过 `POST /v1/images/generations` 创建图片任务，返回 HTTP 202；通过 `GET /v1/images/generations/{id}` 查询并获取结果 URL。使用媒体 API 分组的 Key，认证与完整响应见[接入总览](/media-api)。

## 先选择公开模型

在[当前模型目录](/media-models/#图片模型)查看已开放图片模型、参数和售价。使用公开 `model`，不需要填写上游平台或内部型号。

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="YOUR_SUB2API_KEY"

curl --fail-with-body "$SUB2API_BASE_URL/v1/models" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

当前图片模型的数量、档位和参考图限制以各模型页为准。同一个 `1K` 档位不保证各生成服务输出完全相同的像素。

## 请求参数

以下是统一协议字段；具体必填性、默认值、枚举和组合由公开模型规则决定。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 必填，当前分组开放的公开模型 ID |
| `prompt` | string | 描述主体、场景、风格，或对参考图的修改要求 |
| `images` | string[] | 可选参考图 HTTP(S) 直链，按数组顺序处理 |
| `aspectRatio` | string | 比例，如 `1:1`、`3:2`、`2:3`；允许范围见模型页 |
| `imageSize` | string | 仅在模型声明时可传；固定 `-1k` 型号不接受此字段及其别名 |
| `quality` | string | 质量，仅在模型页声明时传入，按模型枚举填写 |
| `n` | integer | 结果数；当前图片模型仅允许 `1` |
| `replyType` | string | 目前仅支持 `async` |
| `metadata` | object | 仅限模型页明确声明的扩展字段 |

省略字段时采用该模型声明的默认值。扩展字段不能任意添加；比例、分辨率、质量、张数不支持时会在创建前返回 400。

### 比例与像素

`gpt-image-2.6-1k` 和 `gpt-image-2.5-1k` 已固定 1K，使用 `aspectRatio` 选择比例，不接受 `imageSize`、`resolution`、`metadata.resolution` 或像素尺寸写法。`auto` 由生成服务决定比例，实际像素以结果为准。其他模型的档位参数见对应模型页。

`gpt-image-2.6-2k`、`gpt-image-2.5-2k` 固定 2K，同样不传分辨率字段。它们额外支持 `1:3`、`3:1`，`quality` 可选 `auto/low/medium/high`，四种质量均为 0.2 元/张。两个 2K 型号的 `aspectRatio:auto` 固定输出 2048×2048；1K 型号的 `auto` 仍由生成服务决定比例。

旧名字仅在模型声明了对应参数时兼容；别名不会增加可用参数。固定 `-1k` 型号不接受任何分辨率字段。映射见[旧参数兼容](/media-api#旧参数兼容)。

## 文生图

下面使用 `gpt-image-2.6-1k`。切换模型时先核对对应参数页。

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "gpt-image-2.6-1k",
    "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
    "aspectRatio": "3:2",
    "quality": "auto",
    "replyType": "async",
    "n": 1
  }'
```

创建响应中的 `id` 是本地媒体任务 ID。保存它，之后使用同一个 Key 查询，不要再 POST 一次。

## 参考图生图

使用相同接口，在请求中添加 `images`。将下面占位地址替换为实际可下载的图片直链：

```json
{
  "model": "gpt-image-2.6-1k",
  "prompt": "保留参考图的主体和构图，将陶瓷杯改为绿色，背景保持白色",
  "images": ["https://example.com/REPLACE_WITH_YOUR_REFERENCE.png"],
  "aspectRatio": "3:2",
  "quality": "auto",
  "replyType": "async",
  "n": 1
}
```

图片应满足以下条件：

- 是生成服务可直接读取的 HTTP(S) 图片文件，不能依赖网页登录。
- 带签名链接在任务处理期间仍有效；浏览器本地可见不代表生成服务能下载。
- 数量和顺序符合模型要求。目前不接受 Base64、`file://` 或本地文件路径。

统一媒体 API 的参考图生图使用 `images`，不需要改用 `/v1/images/edits`。蒙版、局部重绘等未列出的功能不由此示例承诺支持。

如果某个生成服务无法下载链接，系统可能转给其他支持本次请求的服务。继续查询原任务；若最终没有成功结果，按查询的 `error` 排查链接有效性、访问限制和格式。

## Windows PowerShell

```powershell
$env:SUB2API_KEY = "YOUR_SUB2API_KEY"
$headers = @{ Authorization = "Bearer $env:SUB2API_KEY" }
$body = @{
  model = "gpt-image-2.6-1k"
  prompt = "白色桌面上的蓝色陶瓷杯，柔和自然光"
  aspectRatio = "3:2"
  quality = "auto"
  replyType = "async"
  n = 1
} | ConvertTo-Json

$task = Invoke-RestMethod -Method Post `
  -Uri "https://www.gkotta.bid/v1/images/generations" `
  -Headers $headers -ContentType "application/json; charset=utf-8" `
  -Body ([System.Text.Encoding]::UTF8.GetBytes($body))

$task.id # 保存这个 id
Invoke-RestMethod -Method Get `
  -Uri "https://www.gkotta.bid/v1/images/generations/$($task.id)" `
  -Headers $headers
```

最后一次 GET 只查询一次，未完成时隔 5–10 秒继续 GET。完整轮询、结果判断、下载与计费见[任务查询与结果](/media-api#查询任务与下载结果)。
