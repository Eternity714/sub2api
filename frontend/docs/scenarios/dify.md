# Dify

Dify 是可视化 AI 应用开发平台。通过官方 **OpenAI-API-compatible** 模型供应商插件，可以把 Gkotta 接入聊天助手、Chatflow 或工作流中的 LLM 节点。

## 配置模型供应商

先按 [Dify 官方文档](https://docs.dify.ai/)部署或登录自己的工作区，在 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建专用密钥，到[可用渠道](https://www.gkotta.bid/available-channels)确认分组支持 Chat Completions 和所需模型。

1. 打开 Dify **设置 → 模型供应商**。
2. 如果没有 OpenAI-API-compatible，先从插件市场安装官方对应插件。
3. 在该供应商中点击添加模型，填写以下内容。

| 字段 | 设置 |
| --- | --- |
| 模型类型 | `LLM` |
| 模型名称 | `YOUR_MODEL_ID` |
| 显示名称 | 例如 `Gkotta 对话模型` |
| API Key | `sk-YOUR_API_KEY` |
| API Base URL / API endpoint URL | `https://www.gkotta.bid/v1` |
| API endpoint 中的模型名称 | `YOUR_MODEL_ID` |
| 对话类型 / Completion mode | `Chat` |

4. 上下文长度和最大输出量按当前模型及分组能力填写，不要直接沿用插件的示例值。
5. 函数调用、视觉、流式输出等选项只开启已经确认支持的能力，先保存并验证基础文本配置。

模型名称和 API endpoint 中的模型名称都应对应真实模型 ID。显示名称可以自定义，但「Gemini 2.5 Flash」这类展示文字不能替代实际接口 ID。API Base URL 不包含 `/chat/completions`，插件会追加请求路径。

## 创建一个最小聊天助手

1. 新建一个聊天助手，选择刚配置的 Gkotta 模型。
2. 系统提示词填写「你是一个中文助手，用两句话以内回答用户问题」。
3. 暂时不添加知识库、联网工具或附件，在预览里输入「解释什么是 API」。
4. 看到文本回复后，追问「给一个生活中的例子」，验证多轮对话。
5. 在 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对实际模型和费用，再发布 Dify 应用。

工作流可从**开始 → LLM → 结束**做起：开始节点添加文本输入 `question`，LLM 的用户消息引用该变量，结束节点输出 LLM 的文本。用同一个短问题运行一次，先确认变量传递和模型调用都成功，再加入分支或其他节点。

## 知识库与应用 API

知识库索引需要嵌入模型，重排节点需要重排模型。它们与 LLM 聊天模型是不同配置；不要因为 Chat Completions 已接通就假定同一密钥自动支持 embeddings 或 rerank。仅在已核实对应接口和分组时添加这些模型，或使用另外配置的供应商。

Dify 应用发布后有它自己的应用 API 地址和应用 API Key。调用 Dify 的 `/v1/chat-messages` 时使用 **Dify 应用密钥**，Gkotta Key 保存在模型供应商配置里，不能把两者互换或把上游密钥交给应用访问者。

## 常见问题

- **保存模型时报 404**：确认供应商插件是 OpenAI-API-compatible、Base URL 只有一次 `/v1`，对话类型选择 Chat。
- **模型验证失败**：确认实际模型 ID、密钥分组和余额；检查请求是否由 Dify 的插件服务发出，服务器代理或出站限制也会影响连接。
- **LLM 节点输出为空**：检查输入变量引用、输出变量选择和模型错误信息，先用固定短文本替代映射变量。
- **上下文或参数错误**：按模型能力修正长度、输出限制和采样参数，移除尚未确认支持的参数。
- **知识库创建失败**：排查嵌入模型、解析服务和向量存储，分别验证，避免把索引错误当成聊天地址错误。
- **工具节点报错**：先用无工具的 LLM 节点验证；启用工具后需要模型和上游完整支持工具调用。

## 官方资料

- [Dify 文档](https://docs.dify.ai/)
- [官方 OpenAI-compatible 插件配置字段](https://github.com/langgenius/dify-official-plugins/blob/main/models/openai_api_compatible/provider/openai_api_compatible.yaml)
- [Gkotta OpenAI API](/openai-api)
