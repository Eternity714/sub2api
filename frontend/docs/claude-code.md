# Claude Code

## 安装 Claude Code

Claude Code 是运行在终端中的编程工具。安装方式和系统依赖以 [Claude Code 官方安装文档](https://code.claude.com/docs/en/setup)为准。Windows 用户同时核对官方的 Git Bash 或 WSL 要求；Git 可从 [Git 官方网站](https://git-scm.com/downloads)获取。

如果使用 npm 安装方式，先安装当前工具支持的 [Node.js LTS](https://nodejs.org/en/download)，然后执行：

```bash
npm install -g @anthropic-ai/claude-code
claude --version
```

能看到版本号后，再配置 Gkotta 的密钥和地址。API 密钥的创建流程见[API 密钥管理](/api-keys)。

## 配置 Gkotta

Gkotta 的 Claude Code 接入地址是 `https://www.gkotta.bid`。这里不加 `/v1`；工具会按 Messages 协议拼接接口路径。

请选择支持 Anthropic Messages 协议的密钥分组，在[可用渠道](https://www.gkotta.bid/available-channels)确认模型，将下方 `YOUR_MODEL_ID` 替换成目标模型 ID。

### macOS 或 Linux

在将要启动 Claude Code 的终端中设置：

```bash
export ANTHROPIC_BASE_URL="https://www.gkotta.bid"
export ANTHROPIC_AUTH_TOKEN="sk-YOUR_API_KEY"
export ANTHROPIC_MODEL="YOUR_MODEL_ID"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1

claude
```

### Windows PowerShell

```powershell
$env:ANTHROPIC_BASE_URL = "https://www.gkotta.bid"
$env:ANTHROPIC_AUTH_TOKEN = "sk-YOUR_API_KEY"
$env:ANTHROPIC_MODEL = "YOUR_MODEL_ID"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC = "1"

claude
```

上面的环境变量作用于当前终端及它启动的进程。修改后需要从这个终端启动工具；已运行的进程不会自动更新环境变量。

## 保存到配置文件

希望以后启动时继续使用这些设置，可以编辑用户配置文件：

| 系统 | 文件位置 |
| --- | --- |
| macOS 或 Linux | `~/.claude/settings.json` |
| Windows | `%USERPROFILE%\.claude\settings.json` |

配置示例：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://www.gkotta.bid",
    "ANTHROPIC_AUTH_TOKEN": "sk-YOUR_API_KEY",
    "ANTHROPIC_MODEL": "YOUR_MODEL_ID",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}
```

文件已存在时，保留其他设置并合并 `env` 字段；不要直接覆盖整个文件。JSON 中不允许注释或末尾多余逗号。保存后关闭并重新启动 Claude Code。

此文件包含密钥，应保留在自己的用户目录中。项目级配置、终端变量及其他配置管理工具可能覆盖用户设置，出现地址不一致时按官方配置优先级逐一核对。

## 完成接入验证

1. 在一个你允许工具读取的项目目录中启动 `claude`。
2. 提出一条简单问题，例如“只回复 Hello，不修改文件”。
3. 确认模型返回回复，再到[使用记录](https://www.gkotta.bid/usage)核对模型、密钥和消耗。
4. 需要切换模型时，确认目标模型属于当前分组；工具的默认模型或子任务模型也可能需要按官方配置说明调整。

若工具启动失败，先使用[Anthropic API 请求样例](/anthropic-api)验证相同密钥和模型。直接请求成功而工具失败时，重点检查客户端版本、有效配置及它实际选择的模型。

## 常见接入问题

- **请求到了 `/v1/v1/messages`**：把 `ANTHROPIC_BASE_URL` 改为站点根地址，重启工具。
- **401 或提示密钥无效**：核对 `ANTHROPIC_AUTH_TOKEN` 的值、密钥状态，以及是否从设置变量的终端启动工具。
- **模型不可用**：到[可用渠道](https://www.gkotta.bid/available-channels)核对模型 ID 和密钥分组，检查默认模型与子任务模型设置。
- **403 或 429**：查看完整错误消息，检查密钥有效期、IP 限制、额度和分组订阅；参见[常见问题](/troubleshooting)。
- **修改配置没有生效**：关闭工具后重新启动，并核对项目配置、用户配置和终端变量是否互相覆盖。

客户端功能是否可用取决于当前分组、模型和上游能力。遇到特定工具调用不支持的错误时，先用最小文本请求排除基础接入问题。
