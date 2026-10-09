# Trae

Trae 可添加自定义模型，通过 OpenAI Chat Completions 或 Anthropic Messages 协议接入 Gkotta。不同地区及版本的设置界面会有差异，配置时需要辨别「基础地址」与「完整请求地址」。

## 准备密钥与模型

从 [Trae 国际版](https://www.trae.ai/)或[国内版](https://www.trae.cn/)安装所需版本。在 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建密钥，到[可用渠道](https://www.gkotta.bid/available-channels)核对分组支持的协议、模型以及工具调用能力。

只支持文本对话的模型不一定能完成 Builder/Agent 的多步工具流程。需要同时配置两种协议时，应分别使用匹配分组的密钥和模型，不假定同一把 Key 同时具有两种协议权限。

## 添加 OpenAI 兼容模型

1. 打开 **Settings → Models → Add Model**。
2. 选择 **Custom Model**，API Format 选择 **OpenAI Chat Completions**；旧版可能显示为 OpenAI 服务商下的自定义模型。
3. Model ID 填 `YOUR_MODEL_ID`，显示名称填写 `Gkotta-OpenAI`，API Key 填 `sk-YOUR_API_KEY`。
4. 按 Full URL 开关选择下面的一种地址写法。

| 界面设置 | Custom Request URL |
| --- | --- |
| Full URL 开启，直接发送完整地址 | `https://www.gkotta.bid/v1/chat/completions` |
| Full URL 关闭，由客户端追加路径 | `https://www.gkotta.bid/v1` |

5. 点击 Add Model，查看验证结果，回到聊天的模型选择器中启用该模型。

当前官方文档支持两种地址方式。旧版若没有 Full URL 开关而要求完整请求地址，应填写完整端点；若字段明确要求 Base URL 并自动追加路径，则使用基础地址。不要单凭一个版本号判断所有地区版本都采用同一规则，错误响应中的实际请求路径才是排查依据。

## 添加 Anthropic 模型

另外创建一个自定义模型，API Format 选择 **Anthropic Messages**，填该协议可用的 `YOUR_MODEL_ID` 与 `sk-YOUR_API_KEY`，名称可用 `Gkotta-Anthropic`。

| 界面设置 | Custom Request URL |
| --- | --- |
| Full URL 开启 | `https://www.gkotta.bid/v1/messages` |
| Full URL 关闭 | `https://www.gkotta.bid` |

保存后验证。两种协议的请求体、鉴权和返回格式不同，不能把 Anthropic 模型条目的 URL 改成 Chat Completions 后保留原来的协议设置。

## 最小验证工作流

先在普通 Chat 中选择刚添加的模型，发送「请只回复连接成功，不操作文件」。然后提供一个短代码片段，请它解释用途，确认文本与上下文正常；到 [Gkotta 使用记录](https://www.gkotta.bid/usage)检查实际模型。

需要 Agent/Builder 时，打开一个小型项目，只请求读取单个文件并解释内容，再验证一处小改动。检查每次工具操作和代码差异，并按项目规则运行验证。若工具失败而文本正常，应检查模型与上游的工具协议，避免反复提交整个复杂任务。

## 常见问题

- **404**：核对 Full URL 开关与输入地址是否对应，OpenAI 最终必须是 `/v1/chat/completions`，Anthropic 必须是 `/v1/messages`。
- **地址出现 `/messages/v1/messages` 等重复路径**：客户端在完整 URL 后又追加了端点，改用匹配的 Full URL 设置或基础地址。
- **找不到自定义地址入口**：查阅所安装地区版本的官方说明，使用包含自定义模型功能的版本；不要把 Gkotta Key 填到无法覆盖地址的官方预设里。
- **验证失败**：核对模型 ID、协议及密钥分组，保存界面返回的错误消息，检查余额、订阅和限额。
- **Chat 正常而 Builder 失败**：检查工具调用、上下文和所选模型能力，用单文件小任务复测。
- **切换协议后仍使用旧模型**：核对当前模型选择器以及两个自定义条目的独立配置。

## 官方资料

- [Trae 内置与自定义模型说明](https://docs.trae.ai/ide/models)
- [Trae 国内版文档](https://docs.trae.cn/)
- [Gkotta OpenAI API](/openai-api) · [Anthropic API](/anthropic-api)
