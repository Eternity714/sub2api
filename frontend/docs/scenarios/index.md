# 使用场景总览

按使用目的选择工具。每篇教程包含 Gkotta 地址、模型选择、实际配置、验证和常见错误；先完成[快速开始](/quick-start)和[密钥分组](/api-keys)，再配置客户端。

## 聊天与翻译

| 场景 | 推荐阅读 |
| --- | --- |
| 多模型桌面聊天 | [Cherry Studio](/scenarios/cherry-studio)、[Chatbox](/scenarios/chatbox) |
| 自部署网页聊天 | [NextChat](/scenarios/nextchat)、[Open WebUI](/scenarios/open-webui) |
| 网页翻译 | [沉浸式翻译](/scenarios/immersive-translate) |
| macOS 划词和截图翻译 | [Bob](/scenarios/bob) |
| GLM 与 Qwen | [国内模型接入](/domestic-models) |

## 编程与 Agent

| 场景 | 推荐阅读 |
| --- | --- |
| 终端编程 | [Claude Code](/claude-code)、[Codex](/codex)、[OpenCode](/opencode)、[Gemini CLI](/gemini-cli) |
| 桌面编程工具 | [Claude Desktop](/claude-desktop)、[Codex++](/codex-plus) |
| 管理供应商配置 | [CC Switch](/cc-switch) |
| 编辑器集成 | [Cursor](/scenarios/cursor)、[Cline](/scenarios/cline)、[Roo Code](/scenarios/roo-code)、[Trae](/scenarios/trae) |
| 通用 Agent | [Hermes](/hermes)、[OpenClaw](/openclaw)、[FastClaw](/scenarios/fastclaw) |

## 工作流与自动化

| 场景 | 推荐阅读 |
| --- | --- |
| 搭建 AI 应用 | [Dify](/scenarios/dify) |
| Python / JavaScript 集成 | [LangChain](/scenarios/langchain) |
| 论文与演示文稿文本工作流 | [Paper2Any](/scenarios/paper2any) |
| 自动理解图片 | [Make.com + Gemini](/scenarios/make-gemini-vision) |
| 表格批量生成图片 | [飞书多维表格集成流程](/scenarios/lark-images) |

## 图片与视频 API

需要直接开发应用时，阅读[API 手册](/api-manual)。图片与视频教程包括[图片生成和编辑](/image-generate)、[Grok 视频生成](/video-generation)、[Seedance API](/seedance-api)与[图片转视频](/image-to-video)。

媒体模型需要对应渠道与权限，客户端能聊天不代表能生成图片或视频。异步视频任务需要保存创建返回的 ID，再用同一密钥查询并下载结果。

## 配置原则

模型 ID 使用当前分组的实际值，价格和可用性在控制台查询。第三方客户端的功能、界面和依赖会随版本变化；本文给出兼容接入方式，客户端功能按对应官方文档配置。

配置失败时先核对[协议与地址](/clients)，再查[排错指南](/troubleshooting)。
