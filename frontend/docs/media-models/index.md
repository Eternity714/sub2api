---
description: 统一媒体 API 当前开放的图片与视频模型、参数和调用示例。
---

# 当前媒体模型

核对日期：**2026-11-10**。当前开放 **6 个图片模型、0 个视频模型**。

此处只列出当前已开放且提供价格的型号。模型是否对你的 Key 开放以及实时价格，以[可用渠道](https://www.gkotta.bid/available-channels)和 `GET /v1/models` 为准。

图片使用[图片生成 API](/media-images)，视频使用[视频生成 API](/media-videos)。两者均为异步任务，[查询、结果和计费规则](/media-api#查询任务与下载结果)共用。

## 图片模型

| 模型与参数 | 分辨率 | 每次结果数 | 标价 |
| --- | --- | --- | --- |
| [gpt-image-2.5-flare](/media-models/gpt-image-2.5-flare) | `1K`、`2K`、`4K` | 1 | 1K：0.12 元/张；2K：0.2 元/张；4K：0.35 元/张 |
| [gpt-image-2.5-sunburst](/media-models/gpt-image-2.5-sunburst) | `1K`、`2K`、`4K` | 1 | 1K：0.12 元/张；2K：0.2 元/张；4K：0.35 元/张 |
| [gpt-image-2.6-flare](/media-models/gpt-image-2.6-flare) | `1K`、`2K`、`4K` | 1 | 1K：0.12 元/张；2K：0.2 元/张；4K：0.35 元/张 |
| [gpt-image-2.6-sunburst](/media-models/gpt-image-2.6-sunburst) | `1K`、`2K`、`4K` | 1 | 1K：0.12 元/张；2K：0.2 元/张；4K：0.35 元/张 |
| [nano-banana-2.1](/media-models/nano-banana-2.1) | `1K`、`2K`、`4K` | 1 | 0.16 元/张 |
| [nano-banana-pro](/media-models/nano-banana-pro) | `1K`、`2K`、`4K` | 1 | 0.25 元/张 |

## 视频模型

当前统一媒体 API 尚未开放可调用的视频模型。接口说明用于了解协议，模型开放后才能创建任务。

## 旧型号链接

旧目录中的型号不代表当前可调用能力。原链接保留未开放提示，当前清单之外的型号不再展示旧参数与创建示例。Grok、Seedance 等独立协议是否可用，应按其对应分组和可用渠道确认。
