# 常用客户端

## 接入前检查

不同客户端的界面名称会随版本变化，但需要填写的信息相同：协议、Base URL、API 密钥和模型 ID。

先在[API 密钥](https://www.gkotta.bid/keys)创建对应分组的密钥，到[可用渠道](https://www.gkotta.bid/available-channels)确认模型。本文的 `sk-YOUR_API_KEY` 与 `YOUR_MODEL_ID` 需要替换成自己的值。

| 协议类型 | 常用配置地址 | 用途 |
| --- | --- | --- |
| OpenAI 兼容 | `https://www.gkotta.bid/v1` | 支持自定义 OpenAI 服务的聊天或编程工具 |
| Anthropic 兼容 | `https://www.gkotta.bid` | 支持自定义 Anthropic 服务的工具 |
| Gemini 原生 | 按客户端规则配置 `https://www.gkotta.bid` 或 `https://www.gkotta.bid/v1beta` | 客户端自行追加版本路径时用根地址 |

配置地址时，检查客户端是否会自动补充 `/v1` 或 `/v1beta`。最终请求应指向正确的模型接口，不能出现重复版本路径。

## Cherry Studio

从 [Cherry Studio 官方网站](https://www.cherry-ai.com/)下载适合系统的版本；具体界面和说明见[官方文档](https://docs.cherry-ai.com/)。

1. 打开客户端设置中的模型服务或提供商管理。
2. 添加支持自定义地址的 OpenAI 兼容服务，名称填写 `Gkotta`。
3. 将 API 地址设置为 `https://www.gkotta.bid/v1`，API Key 填入自己的 Gkotta 密钥。
4. 获取模型列表，或手工添加当前分组可用的模型 ID。
5. 启用服务，在聊天界面选择刚添加的模型并发送一条简单消息。

若当前版本会自动追加 `/v1`，按照它的地址说明调整填写方式，最终确认请求路径为 `/v1/chat/completions`。模型检测失败时可先使用[OpenAI API 的模型列表示例](/openai-api)验证密钥。

## Cursor

从 [Cursor 官方下载页](https://cursor.com/downloads)安装客户端；API 密钥支持范围以 [Cursor 官方文档](https://cursor.com/docs)为准。

1. 打开设置中的模型与 API Key 区域。
2. 在支持自定义 OpenAI Base URL 的配置中填入 `https://www.gkotta.bid/v1`。
3. 填入自己的 Gkotta API 密钥，启用对应的自定义地址选项。
4. 添加或选择当前分组支持的模型 ID，再使用客户端的验证入口或发送一条简单聊天请求。
5. 到[使用记录](https://www.gkotta.bid/usage)确认这条请求经过 Gkotta。

Cursor 不同版本、功能及模式对自定义密钥的支持可能不同。先测试明确支持自定义服务的聊天功能；某项编辑或补全功能未出现 Gkotta 使用记录时，核对该功能是否使用你配置的 provider。

## OpenCode

安装方法与配置规范见 [OpenCode 官方文档](https://opencode.ai/docs/)。若选择 npm 安装方式，先安装兼容的 [Node.js LTS](https://nodejs.org/en/download)：

```bash
npm install -g opencode-ai
opencode --version
```

下面示例使用 OpenAI 兼容 provider。把它合并到 OpenCode 当前使用的配置文件中，文件位置及现有格式以[官方配置说明](https://opencode.ai/docs/config/)为准。

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "gkotta": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Gkotta",
      "options": {
        "baseURL": "https://www.gkotta.bid/v1",
        "apiKey": "{env:GKOTTA_API_KEY}"
      },
      "models": {
        "YOUR_MODEL_ID": {
          "name": "Gkotta model"
        }
      }
    }
  },
  "model": "gkotta/YOUR_MODEL_ID"
}
```

在启动 OpenCode 的终端设置密钥：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
opencode
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
opencode
```

配置中的两个 `YOUR_MODEL_ID` 必须一起替换，`gkotta/` 前缀应与 provider 名称一致。该示例适用于支持 Chat Completions 的分组；需要 Responses 或 Anthropic 协议时，按照客户端官方 provider 说明配置并参照[OpenAI API](/openai-api)或[Anthropic API](/anthropic-api)。

## 用 CC Switch 管理编程工具配置

同时使用 Claude Code 和 Codex 时，可从 [CC Switch 官方仓库](https://github.com/farion1231/cc-switch)获取安装说明和发布包。

创建 Gkotta provider 后，分别按 [Claude Code](/claude-code) 与 [Codex](/codex)教程填写地址、认证信息和模型。切换 provider 后重新启动目标工具，避免它继续使用旧进程中的配置。

## 配置失败时

先使用[快速开始](/quick-start)中的 HTTP 请求确认密钥和模型可用，再回到客户端检查地址、协议、模型与认证字段。分享排错截图前隐藏完整密钥；常见 HTTP 错误见[常见问题](/troubleshooting)。
