---
description: 统一媒体 API 的视频生成字段、图片与音频参考、任务创建和查询规范。
---

# 视频生成 API

统一媒体视频接口使用 `POST /v1/videos/generations` 创建任务，通过 `GET /v1/videos/generations/{id}` 查询。认证、状态、结果和计费规则与[媒体 API 接入总览](/media-api)一致。

::: info 当前开放状态
当前统一媒体 API 尚未开放可调用的视频模型。[当前模型目录](/media-models/#视频模型)会随实际配置更新。下方内容说明协议格式，模型占位符不能直接用来生成；模型开放后才能创建任务。
:::

Grok 和 Seedance 原生分组使用独立协议，见页面底部的[协议对照](#独立视频协议)。

## 请求参数

模型启用后，具体时长、分辨率、参考素材数量、输入模式和计费方式以该公开模型页为准。通用参数如下：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 必填，实际已开放的视频模型 ID |
| `prompt` | string | 主体动作、场景和镜头；必填性按模型规则 |
| `duration` | number / integer | 视频秒数，类型、取值和是否必填由模型决定 |
| `aspectRatio` | string | 比例，如 `16:9`；参考素材模式可能禁止显式填写 |
| `resolution` | string | 视频档位，如 `720p`；严格遵守大小写 |
| `images` | string[] | 模型支持的参考图片 URL，保持顺序 |
| `audios` | string[] | 音频参考 URL，仅在模型规则声明时接受 |
| `quality` | string | 可选质量参数，仅在模型规则声明时接受 |
| `replyType` | string | 目前仅 `async` |
| `metadata` | object | 仅限该模型声明的扩展字段，例如种子或模式选项 |

视频不接受顶层 `n`。图片的 `1K/2K` 不能直接当作任意视频模型的分辨率。旧的 `aspect_ratio`、`image_urls`、`metadata.resolution` 仍兼容，见[兼容映射](/media-api#旧参数兼容)。

## 文生视频请求格式

下面的模型 ID 及规格是**待替换的格式示例**。使用前必须在模型目录和可用渠道确认支持 `duration=5`、`16:9` 与 `720p`，不能根据接口名称推测支持范围。

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="YOUR_SUB2API_KEY"

curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "YOUR_ENABLED_VIDEO_MODEL_ID",
    "prompt": "海边日出，海浪轻柔拍岸，镜头缓慢前移",
    "duration": 5,
    "aspectRatio": "16:9",
    "resolution": "720p",
    "replyType": "async"
  }'
```

符合实际开放规则后，创建成功返回 HTTP 202 和 `id`。没有启用能力、有效路由或价格时请求会被拒绝，不会因为填写了一个上游型号就自动获得能力。

## 图片或音频参考

参考素材仍使用同一个创建接口。只有模型允许时，才在请求中添加：

```json
{
  "images": ["https://example.com/REPLACE_WITH_YOUR_FRAME.png"],
  "audios": ["https://example.com/REPLACE_WITH_YOUR_AUDIO.mp3"]
}
```

这只是可选字段片段，不是完整请求，也不表示一个模型同时支持图片与音频。将占位地址替换为可公开下载、在处理期间有效的素材 URL。

素材顺序、首尾帧、是否要求提示词、能否填写比例以及其他模式字段由模型规则确定。不要把第一张图自动当作首帧、把第二张图自动当作尾帧，也不要自行添加未声明的 `mode` 或 `metadata` 字段。当前统一接口没有通用视频 URL 输入字段或本地文件上传入口。

## 查询与获取结果

保存真实的创建响应 `id`，每隔 5–10 秒查询：

```bash
export TASK_ID="media_YOUR_VIDEO_TASK_ID"

curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

- `queued` / `in_progress`：继续查询。
- `completed`：停止轮询，检查 `result.data` 与 `error`。
- 成功文件在 `result.data` 字符串数组中，`result.type` 为 `video`。

生成结束后仍可能等待文件保存，完成生成和转存后才统一交付。结果结构、过期时间、失败处理及轮询程序见[任务查询与结果](/media-api#查询任务与下载结果)。

```bash
curl --fail-with-body "YOUR_RETURNED_VIDEO_URL" -o result.mp4
```

下载结果 URL 时不附加 Gkotta Key。`duration` 及按秒/按次计费方式由模型定义；失败部分释放预留，按创建时的价格结算。

## 独立视频协议

下表用于选择与 Key 分组匹配的教程，独立协议是否开放以可用渠道为准：

| 协议 | 创建 | 查询 | 任务 ID / 结果 |
| --- | --- | --- | --- |
| 统一媒体 API | `/v1/videos/generations` | `/v1/videos/generations/{id}` | `id`，`result.data[]` |
| [Grok 视频 API](/video-generation) | `/v1/videos/generations` | `/v1/videos/{request_id}` | `request_id`，按 Grok 响应读取 |
| [Seedance 原生任务](/seedance-api) | `/api/v3/contents/generations/tasks` | 同路径追加 `/{id}` | `id`，`content.video_url` |

相同的创建路径可能对应不同分组协议。请求字段、查询路径、完成状态和结果结构须使用同一套协议；原生图生视频步骤见[独立协议的图片转视频](/image-to-video)。
