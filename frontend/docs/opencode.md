# OpenCode

OpenCode 提供终端、桌面和编辑器入口，支持通过自定义 provider 接入 OpenAI 兼容服务。下面为 Gkotta 添加独立供应商，不覆盖客户端已有的官方供应商。

## 下载与安装

桌面版从 [OpenCode 官网下载](https://opencode.ai/zh/download)，选择对应系统的软件包。CLI 可以按[官方安装文档](https://opencode.ai/docs/)安装，例如：

```bash
npm install -g opencode-ai
opencode --version
```

macOS / Linux 也可使用官方安装脚本：

```bash
curl -fsSL https://opencode.ai/install | bash
```

选择一种方式安装即可。依赖版本和 Windows 安装方式以当前官方说明为准。

## 准备密钥与模型

在[API 密钥](https://www.gkotta.bid/keys)创建专用密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认当前分组支持的协议、模型与用量限制。

下面用 `YOUR_MODEL_ID` 表示真实模型 ID，`sk-YOUR_API_KEY` 表示你的密钥。Agent 任务还需要模型支持工具调用；先验证文本，再测试文件或工具操作。

## 方法一：配置文件与环境变量

在用户级配置 `~/.config/opencode/opencode.json` 或项目根目录 `opencode.json` 中合并下面的字段。Windows 和设置了自定义配置目录的用户按[配置文档](https://opencode.ai/docs/config/)确认实际目录。

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
          "name": "Gkotta 模型"
        }
      }
    }
  },
  "model": "gkotta/YOUR_MODEL_ID"
}
```

这套配置使用 `@ai-sdk/openai-compatible`，请求 OpenAI Chat Completions。`provider.gkotta` 是供应商 ID，`models` 的键是实际接口模型 ID；`name` 只是显示名称。

在启动 OpenCode 的终端设置密钥：

macOS / Linux：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
opencode
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
opencode
```

桌面版从系统菜单启动时不会继承另一个终端临时设置的变量。可以使用下方 `/connect` 凭证方式，或按官方配置机制提供应用实际可读取的环境变量。

## 方法二：使用 /connect 保存凭证

1. 启动 OpenCode，输入 `/connect`。
2. 选择 **Other**，provider ID 输入 `gkotta`。
3. 输入 Gkotta API Key，保存凭证。
4. 保留上方 `provider.gkotta` 的协议、Base URL 和模型配置，移除 `options.apiKey` 环境变量字段。
5. 输入 `/models`，选择 Gkotta 下的模型。

`/connect` 只保存凭证，不会自动补齐自定义 Base URL 或模型。provider ID 必须与配置文件中的 `gkotta` 完全相同；不要同时保留失效的环境变量配置而期待它自动读取另一套凭证。

当前官方文档说明凭证保存在 `~/.local/share/opencode/auth.json`。文件可能包含真实密钥，不应提交或分享。

## 协议选择

| 分组支持的接口 | OpenCode SDK 包 | Base URL |
| --- | --- | --- |
| OpenAI Chat Completions | `@ai-sdk/openai-compatible` | `https://www.gkotta.bid/v1` |
| OpenAI Responses | `@ai-sdk/openai` | `https://www.gkotta.bid/v1` |

如果当前模型仅支持 Responses，将配置中的 `npm` 改为 `@ai-sdk/openai`，并确认 OpenCode 版本及 provider 设置按[官方自定义供应商文档](https://opencode.ai/docs/providers/#custom-provider)使用 Responses。协议与该分组不匹配时，改模型显示名称没有作用。

没有确认上下文或输出长度时，不从其他模型复制 `limit.context`、`limit.output` 或多模态能力设置。需要填写这些字段时，使用目标模型真实限制。

## 通过 CC Switch 配置

也可在 [CC Switch](/cc-switch)中选择 OpenCode，添加自定义供应商，填写对应协议、`https://www.gkotta.bid/v1`、密钥与模型。保存并添加后，在 OpenCode 的模型列表中选择 Gkotta。

手工配置、项目配置和 CC Switch 都可能影响最终配置。排错时先确认当前使用哪一套，不要并行覆盖相同字段。

## 使用与验证

启动 `opencode`，输入 `/models` 确认模型，再提出“只回复 Hello，不修改文件”的请求。到[使用记录](https://www.gkotta.bid/usage)确认密钥和模型。

在编码任务中先描述目标、文件范围和限制。使用只读规划模式分析后，再允许编辑或命令执行。编辑完成后查看差异，并运行项目自己的验证流程。客户端的会话管理、MCP 和权限功能见官方文档。

## 常见问题

| 现象 | 检查方向 |
| --- | --- |
| `/models` 找不到供应商 | 是否添加 `provider.gkotta` 与 `models`，是否读取了正确配置 |
| `/connect` 成功但无法聊天 | 是否仍缺 Base URL / 模型，provider ID 是否一致 |
| 环境变量为空 | 从设置变量的同一个终端启动，桌面应用确认凭证来源 |
| 404 / 不支持接口 | 检查 SDK 包、Base URL 和当前分组的接口协议 |
| 配置改了无效 | 检查项目、全局及 `OPENCODE_CONFIG` 等配置的覆盖关系，重启客户端 |
| 文本可用但工具失败 | 检查目标模型工具能力及工具权限，不要先扩大所有权限 |
| 401 / 403 / 429 | 按[常见问题](/troubleshooting)核对密钥、分组和额度 |

官方资源：[项目仓库](https://github.com/anomalyco/opencode)、[供应商配置](https://opencode.ai/docs/providers/)、[配置文件](https://opencode.ai/docs/config/)。
