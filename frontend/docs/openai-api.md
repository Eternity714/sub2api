# OpenAI 兼容 API

## 地址与协议

Gkotta 的 OpenAI 兼容 Base URL 是 `https://www.gkotta.bid/v1`。使用 SDK 时通常填写这个地址；直接发送 HTTP 请求时，需补上具体接口路径。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/v1/models` | 查询当前密钥可见的模型 |
| POST | `/v1/responses` | Responses 协议生成内容，适用于 Codex 等工具 |
| POST | `/v1/chat/completions` | Chat Completions 协议对话 |

接口是否能调用、模型是否可用取决于密钥分组和上游能力。先到[可用渠道](https://www.gkotta.bid/available-channels)确认分组与模型，再运行请求。

## 设置密钥和模型

下列样例使用占位符。将其替换为[API 密钥](https://www.gkotta.bid/keys)中的有效密钥和当前分组允许使用的模型 ID。

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

请求使用 `Authorization: Bearer` 鉴权。密钥放在请求头中，不要放在 URL 查询参数。

## 查询模型

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

使用响应 `data` 中的模型 `id`。页面上展示的名称不一定就是接口 ID。模型查询成功后，实际生成请求仍会检查余额、订阅、密钥配额与权限。

## Responses 请求

### cURL

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/responses" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"input\":\"Hello!\",\"store\":false}"
```

Responses 返回内容保存在 `output` 中，可包含消息、文本或其他内容项。不要使用 Chat Completions 的 `choices` 路径解析这个响应。

### Python

安装 [OpenAI 官方 Python SDK](https://github.com/openai/openai-python)：

```bash
python -m pip install openai
```

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["GKOTTA_API_KEY"],
    base_url="https://www.gkotta.bid/v1",
)

response = client.responses.create(
    model=os.environ["GKOTTA_MODEL"],
    input="Hello!",
    store=False,
)
print(response.output_text)
```

### Node.js

安装 [OpenAI 官方 Node.js SDK](https://github.com/openai/openai-node)，下面示例可以保存为 `.mjs` 文件运行：

```bash
npm install openai
```

```javascript
import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: process.env.GKOTTA_API_KEY,
  baseURL: 'https://www.gkotta.bid/v1',
})

const response = await client.responses.create({
  model: process.env.GKOTTA_MODEL,
  input: 'Hello!',
  store: false,
})
console.log(response.output_text)
```

运行前，在启动程序的同一个终端中设置密钥和模型变量。SDK 版本需支持 Responses 接口，详细说明见[官方 API 参考](https://platform.openai.com/docs/api-reference/responses)。

## Chat Completions 请求

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/chat/completions" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}"
```

该协议通常从 `choices[0].message.content` 读取回复。Python SDK 中对应 `client.chat.completions.create`，参数使用 `messages`，不要与 Responses 的 `input` 混用。

Windows PowerShell 原生请求示例：

```powershell
$headers = @{ Authorization = "Bearer $env:GKOTTA_API_KEY" }
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

## 接收流式回复

在请求中设置 `stream: true`。cURL 的 `--no-buffer` 能减少客户端输出缓冲：

```bash
curl --no-buffer --fail-with-body "https://www.gkotta.bid/v1/chat/completions" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data "{\"model\":\"$GKOTTA_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}],\"stream\":true}"
```

Chat Completions 的 SSE 数据通常从 `choices[0].delta` 读取，并以 `data: [DONE]` 结束。Responses 使用它自己的事件类型，需按 Responses 协议处理，不能用同一套字段和结束标记硬解析。

## 请求失败时

保留 HTTP 状态和错误消息，再检查地址、密钥、分组及模型。不要将密钥或 Authorization 请求头放入公开排错日志。

参数支持范围由模型决定，`temperature`、输出长度、工具调用等选项并非对所有模型通用。先验证最小文本请求，再逐项添加参数。

费用以[使用记录](https://www.gkotta.bid/usage)为准；权限、额度和重试建议见[常见问题](/troubleshooting)。
