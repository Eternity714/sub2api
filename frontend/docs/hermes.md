# Hermes Agent

Hermes Agent 是 Nous Research 的开源 Agent，提供终端会话、工具调用、记忆与消息网关。它支持自定义模型端点，可以通过 Gkotta 的兼容接口调用当前分组可用的模型。

## 安装

按照 [Hermes 官方安装文档](https://hermes-agent.nousresearch.com/docs/getting-started/installation)选择系统和安装方式。官方安装脚本为：

macOS、Linux 或 WSL2：

```bash
curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash
```

Windows PowerShell：

```powershell
iex (irm https://hermes-agent.nousresearch.com/install.ps1)
```

安装器会处理自身运行依赖。安装完成后重开终端，运行 `hermes --help` 确认命令可用。Windows 原生安装和 WSL 安装是两套不同环境，在哪套环境使用就在哪套环境配置。

## 准备 Gkotta 密钥

在[API 密钥](https://www.gkotta.bid/keys)创建密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认模型 ID、协议和额度。Agent 会多次调用模型及工具，应选择支持所需工具调用的模型，并为它单独设置合理的密钥额度。

下面默认采用 OpenAI Chat Completions，Base URL 为 `https://www.gkotta.bid/v1`。`sk-YOUR_API_KEY` 与 `YOUR_MODEL_ID` 都要换成真实值。

## 通过模型向导配置

在终端运行：

```bash
hermes model
```

选择 **Custom endpoint**，按向导依次输入：

| 项目 | 内容 |
| --- | --- |
| 供应商名称 | `Gkotta`，若当前版本询问名称 |
| API mode / transport | `chat_completions`，即 OpenAI Chat Completions |
| Base URL | `https://www.gkotta.bid/v1` |
| API Key | `sk-YOUR_API_KEY` |
| Model | `YOUR_MODEL_ID` |
| Context length | 按模型实际限制填写，或保持自动检测 |

向导的字段顺序可能随版本变化，核对地址、协议、密钥和模型后保存。若模型列表无法自动发现，可以输入模型 ID；查询列表失败的原因仍要排查，不能把密钥错误当成没有模型。

`hermes model` 是终端里的配置向导。在已运行的对话内输入 `/model` 主要用于切换已经配置的模型，不能代替初次配置或凭证录入。添加新端点前先退出当前会话，再运行配置向导。

## 命名供应商配置

需要为多个环境保留配置时，当前官方文档支持在 Hermes 的 `config.yaml` 中添加命名供应商。合并下面的字段，不覆盖已有设置：

```yaml
providers:
  gkotta:
    name: Gkotta
    api: https://www.gkotta.bid/v1
    key_env: GKOTTA_API_KEY
    transport: chat_completions
    default_model: YOUR_MODEL_ID
```

从启动 Hermes 的同一个终端设置密钥：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
hermes
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
hermes
```

在会话内切换到这个已配置的命名供应商：

```text
/model custom:gkotta:YOUR_MODEL_ID
```

完整配置和当前版本的文件位置见[官方 AI Providers 文档](https://hermes-agent.nousresearch.com/docs/integrations/providers)。现行版本以配置文件和模型向导为准，不要使用旧教程中的 `OPENAI_API_BASE`、`HERMES_MODEL` 或 `llm.*` 字段代替这套配置。

## 使用 Anthropic Messages

如果密钥分组支持 Anthropic Messages，可以在自定义端点向导中选 `anthropic_messages`，Base URL 填 `https://www.gkotta.bid`，再填写支持该协议的模型和密钥。

使用命名供应商时，对应改为：

```yaml
providers:
  gkotta-claude:
    name: Gkotta Claude
    api: https://www.gkotta.bid
    key_env: GKOTTA_API_KEY
    transport: anthropic_messages
    default_model: YOUR_MODEL_ID
```

不要把 OpenAI 的 `/v1` 地址、Messages 根地址和不同 transport 混用。缓存、推理及多模态功能的实际可用性取决于分组、模型与上游，不因选择某种 transport 自动获得。

## 启动与验证

运行 `hermes`，先提出“只回复 Hello，不执行工具”的请求。在[使用记录](https://www.gkotta.bid/usage)检查密钥、模型和用量，再尝试需要工具调用的任务。

配置消息渠道、定时任务或其他网关前，先确认终端会话正常。终端和后台网关可能使用不同的环境变量或 profile；后台进程需要自己的密钥配置。Hermes 的其他工具或辅助模型也可能独立产生调用费用。

## 排错

| 现象 | 检查方向 |
| --- | --- |
| 找不到 `hermes` | 重开终端，检查安装输出和命令路径 |
| 模型向导没有新供应商 | 在终端运行 `hermes model`，不要只在会话中输入 `/model` |
| 401 / 凭证未设置 | 检查 `key_env` 指向的变量是否存在于启动进程的环境 |
| 404 | 核对 `api` 与 `transport`，避免重复 `/v1` 或请求错协议 |
| 模型不可用 | 检查真实模型 ID 和该密钥的分组权限 |
| 终端正常，后台失败 | 检查后台服务的环境、profile 和实际读取的配置 |
| 工具调用失败 | 先验证文本，再确认模型支持工具及客户端的工具权限 |

使用 `hermes doctor` 查看本地配置诊断；额度、权限和服务状态问题见[常见问题](/troubleshooting)。官方资源：[项目仓库](https://github.com/NousResearch/hermes-agent)、[CLI 用法](https://hermes-agent.nousresearch.com/docs/user-guide/cli)、[配置说明](https://hermes-agent.nousresearch.com/docs/user-guide/configuration)。
