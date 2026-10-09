# OpenClaw

OpenClaw 是可以运行在本机或服务器上的 Agent 工具，支持自定义模型供应商。本文通过 OpenAI Chat Completions 配置 Gkotta，完成一次文本对话后再配置消息渠道和其他工具。

## 安装与初始化

按 [OpenClaw 官方安装指南](https://docs.openclaw.ai/install)选择当前系统对应的方式。推荐使用官方安装器，它会检查运行依赖并引导初始化。

macOS、Linux 或 WSL2：

```bash
curl -fsSL https://openclaw.ai/install.sh | bash
```

Windows PowerShell：

```powershell
iwr -useb https://openclaw.ai/install.ps1 | iex
```

已经管理 Node.js 的用户也可按官方文档通过 npm 安装。依赖版本及 npm 的安装脚本审批参数会变化，以当前文档为准。遇到 PowerShell 执行策略限制时，按安装文档处理当前用户或当前进程，不需要长期关闭系统级限制。

安装后验证：

```bash
openclaw --version
openclaw onboard
```

若安装器已经完成 onboarding，无需重复初始化。先使用本地入口完成测试，再逐步配置常用渠道。

## 准备密钥与模型

在[API 密钥](https://www.gkotta.bid/keys)创建专用密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认支持 Chat Completions 和工具调用的模型。模型 ID 使用模型接口返回的标识，不用仅用于显示的名称替代。

将 `sk-YOUR_API_KEY` 与 `YOUR_MODEL_ID` 替换成当前密钥和模型。macOS / Linux 在启动进程的终端设置：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
```

如果使用守护服务，这些终端变量不会自动传给已启动的服务。需要在服务的启动环境或按 OpenClaw 支持的凭证方式设置密钥，然后重启对应服务。

## 配置自定义供应商

在 OpenClaw 当前使用的配置文件中合并下面的设置。默认配置通常位于 `~/.openclaw/openclaw.json`，设置了其他配置路径时以实际路径为准；保留已有渠道、工作区与工具配置。

```json
{
  "agents": {
    "defaults": {
      "model": {
        "primary": "gkotta/YOUR_MODEL_ID"
      }
    }
  },
  "models": {
    "mode": "merge",
    "providers": {
      "gkotta": {
        "baseUrl": "https://www.gkotta.bid/v1",
        "apiKey": "${GKOTTA_API_KEY}",
        "api": "openai-completions",
        "models": [
          {
            "id": "YOUR_MODEL_ID",
            "name": "Gkotta 模型"
          }
        ]
      }
    }
  }
}
```

这套字段来自[官方自定义供应商文档](https://docs.openclaw.ai/concepts/model-providers/custom-providers)：`baseUrl` 是协议的基础地址，`api` 选择请求格式，`models` 注册可选模型。`agents.defaults.model.primary` 中的供应商 ID 与模型 ID 必须与下方配置一致。

`openai-completions` 在 OpenClaw 中表示 OpenAI 兼容对话协议，目标请求为 `/v1/chat/completions`。如果准备使用其他协议，先查看其配置规范及[Gkotta API 文档](/openai-api)，不要仅修改地址而保留错误的请求格式。

## 通过 CC Switch 配置

也可以安装 [CC Switch](/cc-switch)，选择 OpenClaw，添加自定义供应商：

| 字段 | 内容 |
| --- | --- |
| 名称 | `Gkotta` |
| 接口格式 | OpenAI Compatible / Chat Completions |
| Base URL | `https://www.gkotta.bid/v1` |
| API Key | 当前 Gkotta 密钥 |
| 模型 ID | 当前分组可用的模型 |

保存后点击添加，让供应商写入 OpenClaw 配置，再在 OpenClaw 中选中对应模型。不要同时通过多个配置来源覆盖同一供应商。

## 启动并验证

配置保存后，在同一个运行环境中执行：

```bash
openclaw models list
openclaw models set gkotta/YOUR_MODEL_ID
openclaw dashboard
```

模型列表用于确认本地配置和模型选择；实际 API 是否可用仍需发送请求。在 dashboard 中输入“只回复 Hello，不执行工具”，到[使用记录](https://www.gkotta.bid/usage)核对时间、密钥和模型。

如果 dashboard 提示网关未启动，按 onboarding 和[官方入门文档](https://docs.openclaw.ai/start/getting-started)启动或修复网关，再验证对话。首次验证通过后，再接入你需要的渠道与工具。按任务限制工具权限和工作目录，避免让公开消息来源直接触发本机操作。

## 常见问题

| 问题 | 处理方式 |
| --- | --- |
| `openclaw` 命令不存在 | 重开终端，检查安装目录是否加入 PATH |
| 模型列表没有 Gkotta | 检查实际配置路径、JSON 格式和 provider ID |
| 配了供应商但仍用旧模型 | 显式选择 `gkotta/YOUR_MODEL_ID`，供应商添加不会自动更换默认模型 |
| 缺少密钥或 401 | 检查 `${GKOTTA_API_KEY}` 在当前网关进程中是否存在 |
| 返回 404 | 检查 Base URL、协议、真实模型 ID，避免重复 `/v1` |
| 文本正常，工具失败 | 确认模型支持工具调用，再检查 OpenClaw 工具权限与执行环境 |
| 403 / 429 | 核对密钥分组、IP 限制、额度和并发，见[常见问题](/troubleshooting) |

完整管理方式见[项目仓库](https://github.com/openclaw/openclaw)、[模型供应商说明](https://docs.openclaw.ai/concepts/model-providers)和[配置参考](https://docs.openclaw.ai/gateway/configuration)。
