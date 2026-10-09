# Gemini API

## Gemini 原生接口

Gkotta 的 Gemini 原生接口使用 `/v1beta` 路径，适用于发送 Gemini 请求格式的 SDK、CLI 和脚本。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/v1beta/models` | 查询模型列表 |
| POST | `/v1beta/models/{model}:generateContent` | 生成内容 |
| POST | `/v1beta/models/{model}:streamGenerateContent?alt=sse` | SSE 流式生成 |

请选择支持 Gemini 原生协议的密钥分组，在[可用渠道](https://www.gkotta.bid/available-channels)确认模型。OpenAI 兼容工具应使用[OpenAI API](/openai-api)的格式和地址，不能仅替换 URL 后复用 Gemini 请求体。

## 设置变量与认证

推荐在 `x-goog-api-key` 请求头中携带 Gkotta 密钥。样例不把密钥放入 URL，避免网址、代理日志或历史记录暴露凭证。

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

将占位符换成[API 密钥](https://www.gkotta.bid/keys)中的密钥和该分组可用的模型 ID。若列表中的模型名称是 `models/某个模型ID`，下方变量只填写模型 ID 部分，避免 URL 出现 `/models/models/`。

## 查询模型列表

```bash
curl --fail-with-body "https://www.gkotta.bid/v1beta/models" \
  -H "x-goog-api-key: $GKOTTA_API_KEY"
```

结合响应中的模型名称和支持方法选择模型。列表查询成功后，生成请求还会检查账户、订阅、密钥配额及分组权限。

## 生成内容

### cURL

```bash
curl --fail-with-body "https://www.gkotta.bid/v1beta/models/${GKOTTA_MODEL}:generateContent" \
  -H "x-goog-api-key: $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data '{"contents":[{"role":"user","parts":[{"text":"Hello!"}]}]}'
```

文本结果通常位于 `candidates` 内的 `content.parts`。安全过滤、工具调用或其他内容类型可能产生不同结果，应先检查响应字段再读取文本。

### Windows PowerShell

```powershell
$headers = @{ "x-goog-api-key" = $env:GKOTTA_API_KEY }
$body = @{
  contents = @(@{
    role = "user"
    parts = @(@{ text = "Hello!" })
  })
} | ConvertTo-Json -Depth 8

$response = Invoke-RestMethod `
  -Uri "https://www.gkotta.bid/v1beta/models/$($env:GKOTTA_MODEL):generateContent" `
  -Method Post -Headers $headers `
  -ContentType "application/json" -Body $body
$response.candidates[0].content.parts | ForEach-Object { $_.text }
```

## 流式生成

```bash
curl --no-buffer --fail-with-body "https://www.gkotta.bid/v1beta/models/${GKOTTA_MODEL}:streamGenerateContent?alt=sse" \
  -H "x-goog-api-key: $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data '{"contents":[{"role":"user","parts":[{"text":"Hello!"}]}]}'
```

`alt=sse` 指定 SSE 输出方式。按 Gemini 的事件数据读取内容块，不要套用 OpenAI 的 `choices[0].delta`。应用需要处理断连、服务端错误和没有文本候选的情况。

## Gemini CLI 配置

安装和依赖要求见 [Gemini CLI 官方仓库](https://github.com/google-gemini/gemini-cli)。使用支持自定义地址的版本时，可按下面的环境变量方式接入。

macOS 或 Linux：

```bash
export GOOGLE_GEMINI_BASE_URL="https://www.gkotta.bid"
export GEMINI_API_KEY="sk-YOUR_API_KEY"
export GEMINI_MODEL="YOUR_MODEL_ID"
gemini
```

Windows PowerShell：

```powershell
$env:GOOGLE_GEMINI_BASE_URL = "https://www.gkotta.bid"
$env:GEMINI_API_KEY = "sk-YOUR_API_KEY"
$env:GEMINI_MODEL = "YOUR_MODEL_ID"
gemini
```

CLI 地址使用根路径，由客户端追加 API 版本。修改变量后从同一个终端重启工具；如果当前版本的配置项不同，按官方说明设置并确认最终请求指向 `/v1beta`。

## 接入排错

- **404 或请求路径不正确**：确认版本路径只出现一次，模型 ID 不包含重复 `models/` 前缀，并保留方法前的冒号。
- **认证失败**：核对 `x-goog-api-key`、密钥状态和实际发送请求的进程配置。
- **模型不可用**：确认密钥分组和模型支持 Gemini 原生协议及所请求的方法。
- **返回结构与程序预期不符**：检查 `candidates`、内容块和错误消息，避免只按固定文本字段读取。

更多额度、订阅和服务状态排查见[常见问题](/troubleshooting)，消耗记录见[使用记录](https://www.gkotta.bid/usage)。
