---
description: 媒体 API 已开放模型目录，每个模型的参数、输入模式、输出规格和调用示例。
---

# 媒体模型目录

本目录包含 62 个当前开放的模型能力（25 个图片、37 个视频），参数核对日期：2026-10-16。点击模型查看完整参数和可复制示例。

请先阅读[媒体 API 接入指南](/media-api)，确认 API Key 使用媒体 API 分组。模型 ID 区分大小写，可用模型与实际价格以[可用渠道](https://www.gkotta.bid/available-channels)为准。

图片统一使用 `POST /v1/images/generations`，视频统一使用 `POST /v1/videos/generations`；随后通过对应的 `GET .../generations/{task_id}` 查询结果。所有模型均为异步任务。

每个模型只接受其页面列出的字段。不同模型的分辨率、时长、参考图及音频限制各不相同，不要直接复用其它模型的扩展参数。未开放型号不在本目录中列出。

## 图片模型

| 模型 ID 与文档 | 分辨率档位 | 参考图 |
| --- | --- | --- |
| [doubao-seedream-4-0](/media-models/doubao-seedream-4-0) | `1K`、`2K`、`4K` | 支持，最多 10 张，以模式限制为准 |
| [doubao-seedream-4-5](/media-models/doubao-seedream-4-5) | `2K`、`4K` | 支持，最多 10 张，以模式限制为准 |
| [doubao-seedream-5-0](/media-models/doubao-seedream-5-0) | `2K`、`3K` | 支持，最多 10 张，以模式限制为准 |
| [doubao-seedream-5-0-pro](/media-models/doubao-seedream-5-0-pro) | `1K`、`2K` | 必填，最多 1 张，以模式限制为准 |
| [flux-2-flex](/media-models/flux-2-flex) | `1K`、`2K` | 支持，最多 8 张，以模式限制为准 |
| [flux-2-pro](/media-models/flux-2-pro) | `1K`、`2K` | 支持，最多 8 张，以模式限制为准 |
| [flux-kontext-max](/media-models/flux-kontext-max) | 见模型参数 | 支持，最多 1 张，以模式限制为准 |
| [flux-kontext-pro](/media-models/flux-kontext-pro) | 见模型参数 | 支持，最多 1 张，以模式限制为准 |
| [gemini-2.5-flash-image-official](/media-models/gemini-2.5-flash-image-official) | 见模型参数 | 支持，最多 3 张，以模式限制为准 |
| [gemini-2.5-flash-image-preview](/media-models/gemini-2.5-flash-image-preview) | `1K` | 支持，以模式限制为准 |
| [gemini-3-pro-image-preview](/media-models/gemini-3-pro-image-preview) | `1K`、`2K`、`4K` | 支持，最多 6 张，以模式限制为准 |
| [gemini-3-pro-image-preview-vip](/media-models/gemini-3-pro-image-preview-vip) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [gemini-3.1-flash-image-preview](/media-models/gemini-3.1-flash-image-preview) | `1K`、`2K`、`4K` | 支持，最多 6 张，以模式限制为准 |
| [gemini-3.1-flash-image-preview-vip](/media-models/gemini-3.1-flash-image-preview-vip) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [gemini-nano-banana-2.1](/media-models/gemini-nano-banana-2.1) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [gemini-nano-banana-2.1-vip](/media-models/gemini-nano-banana-2.1-vip) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [gpt-image-2](/media-models/gpt-image-2) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [gpt-image-2-vip](/media-models/gpt-image-2-vip) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [gpt-image-2.5-flare](/media-models/gpt-image-2.5-flare) | `1K`、`2K`、`4K` | 支持，以模式限制为准 |
| [gpt-image-2.5-sunburst](/media-models/gpt-image-2.5-sunburst) | `1K`、`2K`、`4K` | 支持，以模式限制为准 |
| [nano-banana-2.1](/media-models/nano-banana-2.1) | `1K`、`2K`、`4K` | 支持，最多 14 张，以模式限制为准 |
| [vidu-image](/media-models/vidu-image) | `1K`、`2K`、`4K` | 支持，最多 7 张，以模式限制为准 |
| [viduq2-fast](/media-models/viduq2-fast) | `1K` | 支持，最多 7 张，以模式限制为准 |
| [viduq2-pro](/media-models/viduq2-pro) | `1K`、`2K`、`4K` | 支持，最多 7 张，以模式限制为准 |
| [viduq3-fast](/media-models/viduq3-fast) | `1K`、`2K`、`4K` | 支持，最多 7 张，以模式限制为准 |

## 视频模型

| 模型 ID 与文档 | 分辨率档位 | 参考图 |
| --- | --- | --- |
| [doubao-seedance-1-0-pro-fast](/media-models/doubao-seedance-1-0-pro-fast) | `480p`、`720p`、`1080p` | 支持，最多 1 张，以模式限制为准 |
| [doubao-seedance-1-0-pro-quality](/media-models/doubao-seedance-1-0-pro-quality) | `480p`、`720p`、`1080p` | 支持，最多 1 张，以模式限制为准 |
| [doubao-seedance-1-5-pro](/media-models/doubao-seedance-1-5-pro) | `480p`、`720p`、`1080p` | 支持，最多 2 张，以模式限制为准 |
| [gemini-omni-1.1-flash-preview](/media-models/gemini-omni-1.1-flash-preview) | `720p` | 支持，最多 5 张，以模式限制为准 |
| [gemini-omni-1.1-flash-preview-official](/media-models/gemini-omni-1.1-flash-preview-official) | `360p`、`720p`、`1080p`、`4K` | 支持，最多 10 张，以模式限制为准 |
| [gemini-omni-1.1-flash-preview-vip](/media-models/gemini-omni-1.1-flash-preview-vip) | `360p`、`720p`、`1080p`、`4k` | 支持，最多 1 张，以模式限制为准 |
| [gemini-omni-flash-preview-official](/media-models/gemini-omni-flash-preview-official) | `720p` | 支持，最多 10 张，以模式限制为准 |
| [grok-video-1.0](/media-models/grok-video-1.0) | `480p`、`720p` | 支持，最多 8 张，以模式限制为准 |
| [grok-video-1.5](/media-models/grok-video-1.5) | `480p`、`720p` | 必填，最多 1 张，以模式限制为准 |
| [happyhorse-1.1](/media-models/happyhorse-1.1) | `720P`、`1080P` | 支持，最多 1 张，以模式限制为准 |
| [kling-3.0-turbo](/media-models/kling-3.0-turbo) | `720p`、`1080p` | 支持，最多 1 张，以模式限制为准 |
| [kling-v2-6](/media-models/kling-v2-6) | 见模型参数 | 支持，以模式限制为准 |
| [kling-v3](/media-models/kling-v3) | 见模型参数 | 支持，以模式限制为准 |
| [kling-v3-omni](/media-models/kling-v3-omni) | 见模型参数 | 未开放 |
| [kling-video-o1](/media-models/kling-video-o1) | 见模型参数 | 未开放 |
| [MiniMax-H3](/media-models/minimax-h3-uppercase) | `2K`、`768p` | 支持，最多 5 张，以模式限制为准 |
| [minimax-h3](/media-models/minimax-h3) | `768p` | 支持，最多 5 张，以模式限制为准 |
| [MiniMax-H3-Max](/media-models/minimax-h3-max) | `480P`、`768P` | 支持，最多 2 张，以模式限制为准 |
| [MiniMax-H3-Max-Turbo](/media-models/minimax-h3-max-turbo) | `480P`、`768P` | 支持，最多 2 张，以模式限制为准 |
| [MiniMax-Hailuo-02](/media-models/minimax-hailuo-02) | `768P` | 支持，最多 1 张，以模式限制为准 |
| [MiniMax-Hailuo-2.3](/media-models/minimax-hailuo-2.3) | `768P` | 支持，最多 1 张，以模式限制为准 |
| [MiniMax-Hailuo-2.3-Fast](/media-models/minimax-hailuo-2.3-fast) | `768P` | 必填，最多 1 张，以模式限制为准 |
| [seedance-2-fast-ext](/media-models/seedance-2-fast-ext) | `480p`、`720p`、`1080p` | 支持，最多 9 张，以模式限制为准 |
| [sora-2-official](/media-models/sora-2-official) | 见模型参数 | 支持，最多 1 张，以模式限制为准 |
| [veo3.1-fast](/media-models/veo3.1-fast) | `720p`、`1080p` | 必填，最多 2 张，以模式限制为准 |
| [Veo3.1-fast-official](/media-models/veo3.1-fast-official) | `720p`、`1080p`、`4k` | 支持，最多 1 张，以模式限制为准 |
| [veo3.1-lite](/media-models/veo3.1-lite) | `720p` | 支持，最多 1 张，以模式限制为准 |
| [Veo3.1-lite-official](/media-models/veo3.1-lite-official) | `720p` | 支持，最多 1 张，以模式限制为准 |
| [veo3.1-quality](/media-models/veo3.1-quality) | `720p`、`1080p` | 支持，最多 1 张，以模式限制为准 |
| [Veo3.1-quality-official](/media-models/veo3.1-quality-official) | `720p`、`1080p`、`4k` | 支持，最多 1 张，以模式限制为准 |
| [viduq3](/media-models/viduq3) | `720p`、`1080p` | 必填，最多 7 张，以模式限制为准 |
| [viduq3-pro](/media-models/viduq3-pro) | `540p`、`720p`、`1080p` | 支持，最多 2 张，以模式限制为准 |
| [viduq3-turbo](/media-models/viduq3-turbo) | `540p`、`720p`、`1080p` | 支持，最多 2 张，以模式限制为准 |
| [viduq4-preview](/media-models/viduq4-preview) | `540p`、`720p`、`1080p`、`2K`、`4K` | 必填，最多 1 张，以模式限制为准 |
| [wan2.6](/media-models/wan2.6) | `720p`、`1080p` | 支持，最多 1 张，以模式限制为准 |
| [wan2.6-flash](/media-models/wan2.6-flash) | `720p`、`1080p` | 必填，最多 1 张，以模式限制为准 |
| [wan3.0-video](/media-models/wan3.0-video) | `480p`、`720p`、`1080p` | 未开放 |

