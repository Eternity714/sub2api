# Gemini CLI

Gemini CLI 是 Google 的终端 Agent。使用 API Key 认证及自定义根地址，可以通过 Gkotta 的 Gemini 原生接口接入当前分组支持的模型。

## 安装

先准备兼容的 Node.js；当前[官方安装文档](https://www.geminicli.com/docs/get-started/installation)要求 Node.js 20.0.0 或更高版本。安装和验证：

```bash
node --version
npm install -g @google/gemini-cli
gemini --version
```

macOS / Linux 也可按官方文档使用 Homebrew。Windows 安装 Node.js 后若命令不存在，先重开终端再检查 PATH。

## 选择密钥与模型

在[API 密钥](https://www.gkotta.bid/keys)创建密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认它的分组支持 Gemini 原生协议及目标模型。

文档中的 `YOUR_MODEL_ID` 填写模型 ID 部分，不带重复的 `models/`。需要验证模型列表和 HTTP 请求时，阅读[Gemini API](/gemini-api)。OpenAI 兼容分组不一定同时开放 Gemini 原生接口。

## 配置环境变量

macOS / Linux：

```bash
export GOOGLE_GEMINI_BASE_URL="https://www.gkotta.bid"
export GEMINI_API_KEY="sk-YOUR_API_KEY"
export GEMINI_MODEL="YOUR_MODEL_ID"
export GOOGLE_GENAI_API_VERSION="v1beta"

gemini
```

Windows PowerShell：

```powershell
$env:GOOGLE_GEMINI_BASE_URL = "https://www.gkotta.bid"
$env:GEMINI_API_KEY = "sk-YOUR_API_KEY"
$env:GEMINI_MODEL = "YOUR_MODEL_ID"
$env:GOOGLE_GENAI_API_VERSION = "v1beta"

gemini
```

将密钥与模型占位符替换后，从同一个终端启动。`GOOGLE_GEMINI_BASE_URL` 使用根地址，CLI / SDK 追加 API 版本与方法，最终请求应进入 `/v1beta/models/...`。不要把 `/v1`、`/v1beta`、`/docs` 或 `/gemini` 追加到这个变量中。

`GOOGLE_GENAI_API_VERSION` 显式选择本站已有的 `v1beta` 路径，避免其他配置覆盖客户端默认版本。变量用途见[官方配置参考](https://www.geminicli.com/docs/reference/configuration)。

## 选择 API Key 认证

首次启动时，选择 **Gemini API Key** 认证。已经用 Google 账号或 Vertex AI 登录的会话，可输入 `/auth` 切换认证方式，再选择 Gemini API Key。

自定义 `GOOGLE_GEMINI_BASE_URL` 作用于 Gemini API Key 路径。使用 Google 账号登录或 Vertex AI 时，不会因为填了 Gkotta 密钥就自动切换成这套请求。Gkotta 的 API 凭证与官方账号订阅是不同来源。

Gemini 原生请求可用 `x-goog-api-key` 认证；CLI 会根据 API Key 认证方式传递凭证，通常不需要手工添加请求头。手写请求可参考[Gemini API](/gemini-api)。

## 保留长期配置

上述变量只影响当前终端及它启动的进程。需要保留配置时，可以按官方文档在用户级 `.gemini/.env` 中设置：

```dotenv
GOOGLE_GEMINI_BASE_URL=https://www.gkotta.bid
GEMINI_API_KEY=sk-YOUR_API_KEY
GEMINI_MODEL=YOUR_MODEL_ID
GOOGLE_GENAI_API_VERSION=v1beta
```

环境文件包含真实密钥，放在自己的用户目录并限制访问，不提交到项目仓库。项目、工作区或 shell 中已有的设置可能覆盖这些值；更改后重启 Gemini CLI。

如果使用 [CC Switch](/cc-switch)，选择 Gemini CLI，按根地址、密钥与模型保存并启用供应商。不要再用另一个旧环境文件覆盖它生成的配置。

## 验证与使用

先在会话里发送：

```text
只回复 Hello，不修改文件或执行命令。
```

收到回复后，到[使用记录](https://www.gkotta.bid/usage)核对密钥、模型与消耗。也可以从终端进行一次非交互验证：

```bash
gemini -m YOUR_MODEL_ID -p "只回复 Hello，不执行工具。"
```

编码任务可先要求解释指定文件或提出修改方案，再授权编辑。Agent 会话可能产生多轮调用；具体工具、联网检索或多模态功能还取决于客户端、模型和当前分组，不应仅凭文本请求成功判断所有功能都可用。

## 排错

| 现象 | 检查方向 |
| --- | --- |
| 命令找不到 | Node.js 与 npm 是否安装完成，全局命令路径是否可用 |
| 仍走 Google 账号或 Vertex AI | 用 `/auth` 切换到 Gemini API Key |
| 401 / 密钥无效 | 检查实际进程中的 `GEMINI_API_KEY` 和密钥状态 |
| 404 | Base URL 保持根地址，版本为 `v1beta`，模型 ID 没有重复 `models/` |
| 模型不可用 | 检查分组支持的 Gemini 模型，不沿用客户端默认模型 |
| 配置改了无变化 | 检查 shell、`.env` 与 settings 覆盖，结束旧进程再启动 |
| 403 / 429 / 超时 | 按[常见问题](/troubleshooting)区分权限、额度、并发与服务状态 |

官方资源：[项目仓库](https://github.com/google-gemini/gemini-cli)、[认证说明](https://www.geminicli.com/docs/get-started/authentication)、[配置参考](https://www.geminicli.com/docs/reference/configuration)。
