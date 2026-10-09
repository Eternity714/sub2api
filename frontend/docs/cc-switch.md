# CC Switch

CC Switch 是用于管理 AI 客户端供应商配置的桌面工具。你可以在图形界面中保存 Gkotta 的地址、密钥和模型，然后为不同客户端启用对应配置。

本文按 [CC Switch v4.0.4 用户手册](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/README.md)说明操作；旧版可能使用顶部应用图标，而新版使用侧栏。工具提供的 MCP、Skills、路由和配置备份属于 CC Switch 自身功能。

## 下载与安装

从 [GitHub Releases](https://github.com/farion1231/cc-switch/releases)选择对应系统的软件包并安装。macOS 也可使用：

```bash
brew install --cask cc-switch
```

Windows 选择当前发布提供的安装程序或便携包；Linux 选择对应发行版包或 AppImage。软件包格式和系统要求以[官方安装指南](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/1-getting-started/1.2-installation.md)为准。

先安装你实际使用的客户端，例如 Claude Code、Codex 或 Gemini CLI。首次启动 CC Switch 时，已有客户端配置可能会被导入为 `default`；保留它，方便恢复。

## 准备接入信息

在[API 密钥](https://www.gkotta.bid/keys)创建密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认分组和模型。下面的地址取决于客户端协议：

| 工具 / 协议 | Base URL | 模型要求 |
| --- | --- | --- |
| Claude Code / Anthropic Messages | `https://www.gkotta.bid` | 当前分组可用的 Messages 模型 |
| Codex / OpenAI Responses | `https://www.gkotta.bid/v1` | 当前分组支持 Responses 的模型 |
| Gemini CLI / Gemini 原生 | `https://www.gkotta.bid` | 当前分组支持 Gemini 原生的模型 |
| OpenCode、Hermes、OpenClaw / OpenAI Chat Completions | `https://www.gkotta.bid/v1` | 当前分组支持 Chat Completions 的模型 |

不要把 `/docs` 或控制台页面地址填入供应商配置。每个客户端可以使用单独的密钥；选择一个分组不会自动开放其他分组的模型或协议。

## 添加 Gkotta 供应商

1. 在侧栏选择要管理的工具，例如 **Claude Code** 或 **Codex**。
2. 点击添加供应商，选择自定义配置。
3. 名称填写 `Gkotta`；接口格式按上表选择。
4. 输入对应 Base URL 与 API Key。
5. 填入当前分组实际可用的模型 ID。若界面提供获取模型列表，可先拉取再选择。
6. 保存配置，再在供应商列表点击启用、切换或添加。

Codex 要选 Responses，不能把 Chat Completions 配置直接当作它的配置。Claude Code 的根地址由客户端追加 `/v1/messages`；Gemini CLI 的根地址由客户端追加 API 版本。不要重复添加版本路径。

CC Switch 中的模型显示名称可自定义，实际请求模型 ID 必须与 Gkotta 支持的名称一致。不要沿用预设供应商中的模型 ID、套餐名称或价格查询脚本。

## 启用配置与重启

Claude Code、Codex 和 Gemini CLI 的供应商是切换式配置，启用的配置会写入对应客户端。OpenCode、OpenClaw 与 Hermes 使用共存式供应商，点击添加后还需要在客户端里选中 Gkotta 模型。

- **Codex / Gemini CLI**：保存后重启客户端；编辑器插件可能需要重开编辑器窗口。
- **Claude Code**：为确认配置生效，结束旧会话后重新启动，并查看 `/status`。
- **OpenCode / Hermes / OpenClaw**：在客户端选模型；若进程启动时已加载旧配置，重启该进程。
- **Claude Desktop**：使用专门的 [Claude Desktop](/claude-desktop)教程，切换后必须完全退出并重启应用。

不要同时让多个管理工具改写同一份配置。需要手动编辑时，先记录当前供应商，编辑后确认 CC Switch 没有重新写回旧配置。

## 验证接入

打开目标客户端，提出一个只要求文本回复的小请求，例如“只回复 Hello，不执行工具”。随后在[使用记录](https://www.gkotta.bid/usage)核对密钥和模型。

如果获取模型列表成功、实际对话失败，检查协议、模型能力、额度和分组。可用[OpenAI API](/openai-api)、[Anthropic API](/anthropic-api)或[Gemini API](/gemini-api)中的最小请求区分接口问题与客户端配置问题。

## 备份、恢复与路由

切换前可以使用 CC Switch 的数据备份或导出功能保留配置。备份可能含真实密钥，妥善保管。

需要恢复原来的服务时，切换回保留的 `default` 或官方供应商，再重启工具并按原服务的认证流程操作。CLI 的官方账号登录和 Gkotta API Key 是不同的认证方式。

本文的配置直接访问 Gkotta。CC Switch 还支持本地路由与协议转换；若主动选择该模式，需要保持 CC Switch 和本地路由服务运行。启用额外转换不等于目标分组获得了新模型能力。

## 常见问题

| 问题 | 处理方式 |
| --- | --- |
| 新模型没有出现 | 确认已保存且启用供应商，重启客户端，再核对当前分组的模型 |
| 获取模型失败 | 检查密钥、地址、空格和所选接口格式；部分工具需手工填写模型 |
| 请求仍发往旧服务 | 检查当前供应商、启动参数、环境变量和其他配置管理工具 |
| 返回 404 | 检查协议与路径，避免 `/v1/v1`，Codex 确认 Responses |
| 401 / 403 / 429 | 按[常见问题](/troubleshooting)检查凭证、权限与额度 |
| 路由模式连接失败 | 确认 CC Switch 仍运行，本地服务已启动且端口没有冲突 |

进阶操作见[添加供应商](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/2-providers/2.1-add.md)、[切换供应商](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/2-providers/2.2-switch.md)和[环境变量冲突](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/5-faq/5.4-env-conflict.md)。
