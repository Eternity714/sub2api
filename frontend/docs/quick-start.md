# 快速开始

## 接入前准备

Gkotta 提供 OpenAI、Anthropic 和 Gemini 兼容接口。先准备账号、API 密钥，以及该密钥所属分组允许使用的模型。不同分组的协议、模型、额度和倍率可能不同。

1. 打开[注册页面](https://www.gkotta.bid/register)，按照页面要求注册并登录。若注册入口不可用，以站点当前展示的登录方式为准。
2. 在[可用渠道](https://www.gkotta.bid/available-channels)查看可使用的分组、模型和倍率；[模型广场](https://www.gkotta.bid/model-plaza)可帮助了解模型信息。
3. 在[API 密钥](https://www.gkotta.bid/keys)创建密钥，选择适合本次调用的分组并复制密钥。
4. 在[个人资料](https://www.gkotta.bid/profile)查看账户信息。使用订阅分组时，再到[我的订阅](https://www.gkotta.bid/subscriptions)确认该分组的订阅有效。

文档中的 `sk-YOUR_API_KEY` 和 `YOUR_MODEL_ID` 都是占位符，运行前必须换成你自己的密钥和可用模型 ID。

## 选择接入地址

| 使用方式 | 地址 | 进一步阅读 |
| --- | --- | --- |
| OpenAI SDK、Codex、OpenAI 兼容客户端 | `https://www.gkotta.bid/v1` | [OpenAI API](/openai-api) |
| Anthropic SDK、Claude Code | `https://www.gkotta.bid` | [Anthropic API](/anthropic-api) |
| Gemini 原生 HTTP 请求 | `https://www.gkotta.bid/v1beta` | [Gemini API](/gemini-api) |

Base URL 是接口地址，不是文档或控制台地址。不要填写 `/docs`、`/keys`，也不要在客户端自动追加 `/v1` 时重复填写该路径。

## 完成第一次调用

下面用 OpenAI 兼容接口测试。请使用支持该协议的分组和模型。

### macOS 或 Linux

在 Bash 或兼容终端中设置变量，然后查看此密钥可见的模型列表：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_MODEL="YOUR_MODEL_ID"

curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

发送一条对话请求：

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/chat/completions" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}"
```

成功响应中的 `choices[0].message.content` 通常包含模型回复。

### Windows PowerShell

使用 PowerShell 原生命令可以避免 `curl` 别名和引号的差异：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_MODEL = "YOUR_MODEL_ID"
$headers = @{ Authorization = "Bearer $env:GKOTTA_API_KEY" }

Invoke-RestMethod -Uri "https://www.gkotta.bid/v1/models" -Headers $headers

$body = @{
  model = $env:GKOTTA_MODEL
  messages = @(@{ role = "user"; content = "Hello!" })
} | ConvertTo-Json -Depth 8

$response = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/v1/chat/completions" `
  -Method Post -Headers $headers `
  -ContentType "application/json" -Body $body
$response.choices[0].message.content
```

## 确认调用和消耗

打开[使用记录](https://www.gkotta.bid/usage)，核对请求时间、密钥、模型、Token 用量和费用。模型列表返回成功只表示模型查询成功；生成请求还会检查分组权限、账户余额或订阅额度，以及密钥自身的配额和有效期。

若第一次请求失败，先看响应中的错误消息，再查看[常见问题](/troubleshooting)。不要反复提交同一条失败请求。

## 接下来做什么

- 使用编程工具：阅读 [Claude Code](/claude-code) 或 [Codex](/codex)。
- 使用桌面客户端或编辑器：阅读[常用客户端](/clients)。
- 管理不同项目的凭证：阅读[API 密钥管理](/api-keys)。
- 了解费用和限额：阅读[计费与用量](/billing)。

密钥仅应放在你控制的本地工具或服务端。避免把真实密钥写进网页前端、公开仓库、截图或分享的配置文件。
