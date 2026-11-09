---
description: GPT Image 2.5 图片生成的统一参数与异步任务调用示例。
---

# GPT Image 2.5

公开模型 ID 为 `gpt-image-2.5`，使用媒体 API 分组的 Key 调用。当前售价为 0.066/张，以[可用渠道](https://www.gkotta.bid/available-channels)实时显示为准；按成功转存并交付的图片数量结算。

| 字段 | 当前支持范围 |
| --- | --- |
| `model` | 必填，`gpt-image-2.5` |
| `prompt` | 必填，1–32000 个字符 |
| `images` | 可选，1–16 个可公开读取的 HTTP(S) 参考图 URL；保持顺序 |
| `aspectRatio` | `1:1`、`3:2`、`2:3`，默认 `1:1` |
| `imageSize` | `1K`，默认 `1K`；也接受 `resolution` 别名 |
| `quality` | `auto`，默认 `auto` |
| `replyType` | `async`，默认 `async` |
| `n` | 当前仅 `1` |

优先使用比例和分辨率档位。像素写法只会选择能保留该尺寸的生成渠道，无法支持时在创建任务前返回参数错误；不会把 2K/4K 请求降为 1K。参考图暂不接受 Base64。

```bash
curl --fail-with-body 'https://www.gkotta.bid/v1/images/generations' \
  -H 'Authorization: Bearer YOUR_SUB2API_KEY' \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "gpt-image-2.5",
    "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
    "aspectRatio": "3:2",
    "imageSize": "1K",
    "quality": "auto",
    "replyType": "async",
    "n": 1
  }'
```

创建返回 HTTP 202。保存响应中的 `id`，每隔 5–10 秒使用同一个 Key 查询 `GET /v1/images/generations/{id}`。当 `status` 为 `completed` 时，读取 `result.data` 中的文件 URL，并检查 `error`。

旧字段 `size`、`image_urls`、`metadata.quality` 和 `metadata.resolution` 继续兼容。新旧别名同时传入时必须一致。当前仅提供异步任务，`replyType:json/stream` 会返回参数错误。

完整响应、状态和错误处理见[媒体 API 接入指南](/media-api)。
