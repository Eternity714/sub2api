# 常见问题与排错

## 先完成四项检查

1. **地址**：OpenAI SDK 通常用 `https://www.gkotta.bid/v1`；Claude Code 和 Anthropic SDK 用根地址；Gemini 原生请求使用 `/v1beta`。
2. **密钥**：在[API 密钥](https://www.gkotta.bid/keys)确认它没有被停用、过期或达到额度上限。
3. **分组和模型**：在[可用渠道](https://www.gkotta.bid/available-channels)核对密钥分组、协议与模型 ID。
4. **费用和状态**：检查账户、[我的订阅](https://www.gkotta.bid/subscriptions)、[使用记录](https://www.gkotta.bid/usage)和[渠道状态](https://www.gkotta.bid/monitor)。

先用[快速开始](/quick-start)或对应协议教程中的最小请求测试。最小请求成功后，再加历史上下文、工具、图片或其他参数。

## 按 HTTP 状态排查

状态码表示错误类别，具体原因要看响应消息。不同协议或上游服务的字段和状态可能不同，下表列出项目中常见的情况。

| 状态 | 常见原因 | 优先操作 |
| --- | --- | --- |
| 400 | JSON 格式、模型或参数不正确，使用了不支持的密钥查询参数 | 核对请求体、协议和鉴权头，退回最小请求 |
| 401 | 密钥未填写、无效、停用，或账户不可用 | 检查完整密钥、请求头与密钥状态 |
| 403 | IP 限制、密钥过期、余额不足、无有效订阅或分组无权限 | 按错误消息分别检查对应设置 |
| 404 | 路径、模型或该分组支持能力不匹配 | 检查 Base URL、模型 ID 与请求方法 |
| 429 | 密钥额度、订阅周期额度、并发或频率限制 | 区分额度耗尽与临时限流，再决定调整还是等待 |
| 500、502、503、504 | 服务端、上游容量或超时问题 | 查询渠道状态，保留错误和请求时间后适度重试 |

## 401：密钥未被正确读取

OpenAI 请求使用：

```http
Authorization: Bearer sk-YOUR_API_KEY
```

Anthropic 请求可使用 `x-api-key`；Gemini 原生请求推荐使用 `x-goog-api-key`。不要保留教程占位符，也不要复制出多余引号、换行或缺失字符。

若终端请求成功而客户端失败，检查客户端实际生效的 provider、认证字段及配置目录。环境变量只传给从该终端启动的进程，修改后应重新启动应用。

## 403：分开检查权限和额度

根据错误消息查找原因：

- **`ACCESS_DENIED`**：核对密钥 IP 限制和当前公网出口 IP，尤其是网络或代理切换之后。
- **`API_KEY_EXPIRED`**：核对密钥到期时间，调整有效期或换用有效密钥。
- **`INSUFFICIENT_BALANCE`**：余额计费请求需要足够账户余额，到[购买与充值](https://www.gkotta.bid/purchase)处理。
- **`SUBSCRIPTION_NOT_FOUND` 或订阅无效**：到[我的订阅](https://www.gkotta.bid/subscriptions)确认当前密钥分组的订阅，而不是只看其他分组的套餐。
- **分组停用、删除或不再允许使用**：在[可用渠道](https://www.gkotta.bid/available-channels)核对可用分组，选择仍有权限的分组和模型。

不要只凭 `403` 修改客户端认证模式或重复充值。

## 429：额度用尽还是临时限流

### 密钥配额用尽

错误可能表现为 `API_KEY_QUOTA_EXHAUSTED` 或协议对应的额度错误。在[API 密钥](https://www.gkotta.bid/keys)核对已用额度和上限。账户还有余额也不能绕过单个密钥的配额。

### 订阅周期额度用尽

错误可能表现为 `USAGE_LIMIT_EXCEEDED`。在[我的订阅](https://www.gkotta.bid/subscriptions)检查日、周或月额度与页面显示的重置时间。更换同一分组的密钥不会重置分组订阅用量。

### 并发或频率限制

减少并行任务，为暂时性限流设置有上限的指数退避。若响应包含 `Retry-After`，优先遵循它。不要对明确的配额耗尽进行无限重试。

反复使用错误密钥也可能触发认证限流。先修正凭证，再等待限制解除。

## 404：路径或模型不匹配

| 使用方式 | 正确的配置或请求路径 |
| --- | --- |
| OpenAI SDK / Codex | Base URL 为 `https://www.gkotta.bid/v1` |
| Claude Code / Anthropic SDK | Base URL 为 `https://www.gkotta.bid` |
| Anthropic HTTP | `/v1/messages` |
| Gemini HTTP | `/v1beta/models/YOUR_MODEL_ID:generateContent` |

检查是否出现 `/v1/v1`、`/models/models/`，或把文档地址 `/docs` 当作 API 地址。Gemini 方法前的冒号也必须保留。

模型 ID 以当前分组或模型接口返回的值为准，不能仅按显示名称猜测。接口存在不代表每个分组都支持所有功能。

## 超时或流式内容无法显示

流式响应使用 SSE，需要客户端持续读取事件。cURL 可添加 `--no-buffer`；程序应使用对应 SDK 的流式迭代方式，不要把整段 SSE 当作一个 JSON 对象解析。

OpenAI Responses、Chat Completions、Anthropic 和 Gemini 的事件及字段不同。采用错误的解析器时，服务器可能已经返回内容，界面却显示为空。

网络或代理可能中断连接。检查[渠道状态](https://www.gkotta.bid/monitor)，使用短文本请求对比验证，再按任务耗时调整超时配置。客户端超时或取消不代表上游没有生成内容，重试前核对[使用记录](https://www.gkotta.bid/usage)。

## Windows 命令问题

PowerShell 的 `curl` 可能指向 `Invoke-WebRequest`，Bash 的换行符和引号写法也不能直接复制到 PowerShell。优先使用教程中的 `Invoke-RestMethod` 样例；需要真正的 cURL 时使用 `curl.exe` 并按 Windows 引号规则调整参数。

Python 或 Node.js 程序没有读到环境变量时，确认变量与程序在同一个终端中设置和启动。不要把错误消息中的整个请求头复制到公开聊天。

## 提供有效的排错信息

先记录发生时间、客户端和版本、协议、模型 ID、HTTP 状态、脱敏后的错误消息，以及对应的使用记录。若响应提供请求标识，也一并保留。

隐藏 API 密钥、Authorization 请求头、个人信息和敏感提示内容。说明最小请求是否成功，通常能更快区分账户问题、客户端配置问题和某项模型能力问题。

重新配置工具可参考 [Claude Code](/claude-code)、[Codex](/codex)和[常用客户端](/clients)；额度概念见[计费与用量](/billing)。
