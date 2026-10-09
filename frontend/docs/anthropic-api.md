# Anthropic API

## 接入地址

Anthropic SDK 的 Base URL 填写 `https://www.gkotta.bid`。直接 HTTP 调用使用以下完整路径：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/v1/messages` | 发送 Messages 请求 |
| POST | `/v1/messages/count_tokens` | 查询或估算输入 Token 数量 |

请选择支持 Messages 协议的密钥分组。模型、工具调用和 Token 统计的支持程度受分组及上游能力约束，先在[可用渠道](https://www.gkotta.bid/available-channels)核对。

## 准备认证信息

本文使用 `x-api-key` 请求头携带 Gkotta 密钥，并提供协议版本头 `anthropic-version`。不要把真实密钥放在 URL 中。

macOS 或 Linux：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_MODEL="YOUR_MODEL_ID"
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_MODEL = "YOUR_MODEL_ID"
```

将占位符替换成[API 密钥](https://www.gkotta.bid/keys)中的密钥和对应分组可用的模型 ID。

## 发送 Messages 请求

### cURL

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/messages" \
  -H "x-api-key: $GKOTTA_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"max_tokens\":256,\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}"
```

`max_tokens` 指本次回复的最大输出 Token 数。返回的 `content` 是内容块数组，文本通常位于 `type` 为 `text` 的块中；工具调用等内容块需要单独处理。

### Windows PowerShell

```powershell
$headers = @{
  "x-api-key" = $env:GKOTTA_API_KEY
  "anthropic-version" = "2023-06-01"
}
$body = @{
  model = $env:GKOTTA_MODEL
  max_tokens = 256
  messages = @(@{ role = "user"; content = "Hello!" })
} | ConvertTo-Json -Depth 8

$response = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/v1/messages" `
  -Method Post -Headers $headers `
  -ContentType "application/json" -Body $body
$response.content | Where-Object { $_.type -eq "text" } | ForEach-Object { $_.text }
```

## 使用 Python SDK

安装 [Anthropic 官方 SDK](https://github.com/anthropics/anthropic-sdk-python)：

```bash
python -m pip install anthropic
```

```python
import os
from anthropic import Anthropic

client = Anthropic(
    api_key=os.environ["GKOTTA_API_KEY"],
    base_url="https://www.gkotta.bid",
)

message = client.messages.create(
    model=os.environ["GKOTTA_MODEL"],
    max_tokens=256,
    messages=[{"role": "user", "content": "Hello!"}],
)
for block in message.content:
    if block.type == "text":
        print(block.text)
```

SDK 会按协议追加 `/v1/messages`。Base URL 不应重复包含 `/v1`。

## 查询输入 Token 数量

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/messages/count_tokens" \
  -H "x-api-key: $GKOTTA_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}"
```

可用时，响应中的 `input_tokens` 用于了解输入规模。不同平台可能采用不同统计方式，这个结果不能代替生成请求最终记录的用量和费用。实际费用到[使用记录](https://www.gkotta.bid/usage)核对。

## 流式响应

发送 Messages 请求时增加 `stream: true`：

```bash
curl --no-buffer --fail-with-body "https://www.gkotta.bid/v1/messages" \
  -H "x-api-key: $GKOTTA_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"max_tokens\":256,\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}],\"stream\":true}"
```

按 Anthropic SSE 的消息和内容块事件处理输出。不要把该协议当作 OpenAI Chat Completions 的 `choices` 数据解析。

## 常见问题

- **路径出现 `/v1/v1/messages`**：SDK Base URL 改为 `https://www.gkotta.bid`。
- **模型或参数不支持**：检查当前分组、模型 ID 和该模型支持的内容类型，先测试最小文本请求。
- **没有有效订阅**：密钥可能绑定订阅分组，到[我的订阅](https://www.gkotta.bid/subscriptions)确认该分组状态。
- **401、403、429**：结合完整响应区分密钥无效、有效期、IP 限制、余额、配额和周期限额，参见[常见问题](/troubleshooting)。

使用 Claude Code 的终端及文件配置见 [Claude Code 教程](/claude-code)。
