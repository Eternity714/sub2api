# API 手册

::: warning 媒体 API 分组使用统一异步协议
如果密钥选择的是“媒体 API”分组，请先阅读[媒体 API 接入指南](/media-api)和[逐模型参数文档](/media-models/)。图片和视频创建均返回 `202` 任务，查询分别使用 `/v1/images/generations/{task_id}`、`/v1/videos/generations/{task_id}`；图片 `size` 为宽高比，分辨率放在 `metadata.resolution`。不要直接套用本页链接中的其它原生协议示例。
:::

Gkotta 提供多种模型协议。先选择密钥分组，再根据客户端使用的协议填写地址和请求体。所有示例中的密钥和模型 ID 都需要替换为自己的值。

## 地址与认证

| 协议 | 客户端常用 Base URL | 认证 |
| --- | --- | --- |
| OpenAI 兼容 | `https://www.gkotta.bid/v1` | `Authorization: Bearer sk-YOUR_API_KEY` |
| Anthropic Messages | `https://www.gkotta.bid` | `x-api-key` 或 Bearer，按客户端配置 |
| Gemini 原生 | 根地址或 `https://www.gkotta.bid/v1beta`，取决于客户端是否追加版本 | `x-goog-api-key` |
| 媒体 API | `https://www.gkotta.bid`，请求路径已包含 `/v1` | Bearer |
| Seedance 原生任务 | `https://www.gkotta.bid` | Bearer |

完整 HTTP 地址与客户端 Base URL 不同。客户端会追加路径时，不能把完整接口地址再当作 Base URL。具体填写规则见[使用场景](/scenarios/)。

## 文本与多模态接口

| 方法 | 路径 | 教程 |
| --- | --- | --- |
| GET | `/v1/models` | [OpenAI 模型列表](/openai-api) |
| POST | `/v1/chat/completions` | [Chat Completions](/openai-api) |
| POST | `/v1/responses` | [Responses](/openai-api)、[Codex](/codex) |
| POST | `/v1/messages` | [Anthropic API](/anthropic-api)、[Claude Code](/claude-code) |
| GET | `/v1beta/models` | [Gemini 模型列表](/gemini-api) |
| POST | `/v1beta/models/{model}:generateContent` | [Gemini 内容生成](/gemini-api) |
| POST | `/v1beta/models/{model}:streamGenerateContent?alt=sse` | [Gemini 流式生成](/gemini-api) |

视觉输入、工具调用、推理和上下文长度由具体模型决定。模型名称可见不代表它支持每种接口或参数；在[可用渠道](https://www.gkotta.bid/available-channels)确认密钥分组后，再选择对应教程。

## 图片接口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/v1/images/generations` | 媒体 API 分组：统一异步图片任务；其它支持分组：OpenAI 兼容图片生成 |
| GET | `/v1/images/generations/{task_id}` | 查询媒体 API 图片任务 |
| POST | `/v1/images/edits` | multipart 图片编辑 |
| POST | `/v1beta/models/{model}:generateContent` | Gemini 原生图片生成与编辑 |

媒体 API 请求格式见[统一接入指南](/media-api)及[模型目录](/media-models/)；其它图片协议、图片保存和分组条件见[图像生成 API](/image-generate)。需要支持的上游、图片模型及媒体生成权限，不能直接使用任意文本模型。

## 视频接口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/v1/videos/generations` | 媒体 API 分组：统一异步视频任务；Grok 分组：Grok 视频生成 |
| GET | `/v1/videos/generations/{task_id}` | 查询媒体 API 视频任务 |
| GET | `/v1/videos/{request_id}` | 查询 Grok 视频任务 |
| GET | `/v1/videos/{request_id}/content` | 下载 Grok 视频内容 |
| POST | `/v1/videos/edits` | Grok 视频编辑 |
| POST | `/v1/videos/extensions` | Grok 视频续写 |
| POST | `/api/v3/contents/generations/tasks` | 创建 Seedance 任务 |
| GET | `/api/v3/contents/generations/tasks/{id}` | 查询 Seedance 任务 |
| DELETE | `/api/v3/contents/generations/tasks/{id}` | 删除 Seedance 任务 |

媒体 API、Grok 和 Seedance 的任务 ID、查询路径、输入和完成状态不同。媒体 API 分组阅读[统一接入指南](/media-api)与[模型目录](/media-models/)；其它分组分别阅读[Grok 视频生成](/video-generation)、[Seedance API](/seedance-api)及[图片转视频](/image-to-video)。创建任务后保存 ID，使用创建时的同一 API Key 查询。

## 账户、费用和错误

密钥在[API 密钥](https://www.gkotta.bid/keys)管理，消费在[使用记录](https://www.gkotta.bid/usage)查询。计费方式见[计费与用量](/billing)，鉴权、路径、模型和额度错误见[排错指南](/troubleshooting)。

接入第三方工具时，只启用本页和对应教程列出的接口。向量、语音或其它工具功能需要各自支持的服务，不能因为兼容 OpenAI 就默认所有 API 都可用。
