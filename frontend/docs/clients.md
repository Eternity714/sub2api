# 客户端接入

客户端需要填写协议、Base URL、API 密钥和模型 ID。先在[API 密钥](https://www.gkotta.bid/keys)创建对应分组的密钥，再到[可用渠道](https://www.gkotta.bid/available-channels)确认模型和用途。

## 地址规则

| 协议 | 常用 Base URL | 对应教程 |
| --- | --- | --- |
| OpenAI Chat Completions | `https://www.gkotta.bid/v1` | [OpenAI API](/openai-api) |
| OpenAI Responses | `https://www.gkotta.bid/v1` | [Codex](/codex)、[OpenAI API](/openai-api) |
| Anthropic Messages | `https://www.gkotta.bid` | [Claude Code](/claude-code)、[Anthropic API](/anthropic-api) |
| Gemini 原生 | 根地址或 `/v1beta`，按客户端追加路径规则 | [Gemini CLI](/gemini-cli)、[Gemini API](/gemini-api) |

表格是常用规则，NextChat、Bob、Trae 等客户端有自己的地址拼接方式，安装配置以各篇教程为准。最终路径不能出现 `/v1/v1`、`/v1beta/v1beta`。

Claude Code 专属分组可能限制客户端用途，不能据此认为通用 Chat、Responses 或第三方 Messages 客户端也可用。选择与目标客户端匹配的普通或组合分组，并确认对应模型路由。

## 编程与 Agent 工具

| 工具 | 教程 |
| --- | --- |
| Claude Code CLI / Claude Desktop | [终端版](/claude-code)、[Desktop 3P](/claude-desktop) |
| Codex CLI / Codex++ | [Codex](/codex)、[Codex++](/codex-plus) |
| CC Switch | [管理多个工具的供应商配置](/cc-switch) |
| OpenCode / Gemini CLI | [OpenCode](/opencode)、[Gemini CLI](/gemini-cli) |
| Hermes / OpenClaw / FastClaw | [Hermes](/hermes)、[OpenClaw](/openclaw)、[FastClaw](/scenarios/fastclaw) |
| Cursor / Cline / Roo Code / Trae | [Cursor](/scenarios/cursor)、[Cline](/scenarios/cline)、[Roo Code](/scenarios/roo-code)、[Trae](/scenarios/trae) |

## 聊天、翻译与应用开发

| 类别 | 教程 |
| --- | --- |
| 桌面聊天 | [Cherry Studio](/scenarios/cherry-studio)、[Chatbox](/scenarios/chatbox) |
| 自部署聊天 | [NextChat](/scenarios/nextchat)、[Open WebUI](/scenarios/open-webui) |
| 翻译 | [沉浸式翻译](/scenarios/immersive-translate)、[Bob](/scenarios/bob) |
| 工作流与代码 | [Dify](/scenarios/dify)、[LangChain](/scenarios/langchain)、[Paper2Any](/scenarios/paper2any) |
| 自动化与图片 | [Make Gemini 图像理解](/scenarios/make-gemini-vision)、[飞书多维表格生图](/scenarios/lark-images) |

完整的按用途目录见[使用场景总览](/scenarios/)。

## 验证与排错

先使用[快速开始](/quick-start)中的 HTTP 请求验证密钥和模型，再配置客户端。发送一条简单消息后，到[使用记录](https://www.gkotta.bid/usage)确认实际模型、密钥和费用。

普通对话成功后，再测试工具调用、视觉、图片或 Agent 任务。这些能力需要模型与客户端共同支持。客户端自身的订阅、沙箱、插件或本地依赖按其官方要求设置；Gkotta API Key 用于本站模型请求。

401、403、404、429 或模型不可用的处理步骤见[常见问题与排错](/troubleshooting)。分享配置和日志前隐藏完整密钥。
