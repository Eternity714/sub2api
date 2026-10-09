# Cherry Studio

Cherry Studio 支持创建自定义模型服务。接入 Gkotta 时先选择协议，再填写对应地址和模型，适合在同一客户端管理不同分组的对话服务。

## 选择协议

从 [Cherry Studio 官网](https://www.cherry-ai.com/)安装应用，在 [API 密钥](https://www.gkotta.bid/keys)创建密钥，并到[可用渠道](https://www.gkotta.bid/available-channels)核对分组支持的协议及模型。

| 渠道类型 | API 地址 | 最终请求路径 |
| --- | --- | --- |
| OpenAI 兼容 / Chat Completions | `https://www.gkotta.bid/v1` | `/v1/chat/completions` |
| Anthropic | `https://www.gkotta.bid` | `/v1/messages` |
| Google Gemini | `https://www.gkotta.bid` | `/v1beta/models/{model}:generateContent` |

Cherry Studio 会根据渠道类型补充协议路径，地址输入框下方的请求预览应与表格一致。不同版本可能分别显示「API 地址」「Base URL」或协议专用地址字段，以当前版本的预览为准。

三种协议不是跨模型通用转换开关。Anthropic 渠道要选择支持 Messages 的模型与分组，Gemini 渠道要选择支持 Gemini 原生接口的模型与分组；不要把任意模型加入渠道后就认为一定能调用。

## 配置 OpenAI 兼容渠道

1. 打开**设置 → 模型服务**，点击添加自定义提供商。
2. 名称填写 `Gkotta-OpenAI`，类型选择 **OpenAI/OpenAI 兼容**，聊天接口选择 Chat Completions。
3. API Key 填 `sk-YOUR_API_KEY`，API 地址填 `https://www.gkotta.bid/v1`。
4. 添加模型，模型 ID 填 `YOUR_MODEL_ID`，显示名称可以自定义。
5. 启用提供商，保存设置，再在助手的模型选择器中选择该模型。

如果模型列表查询没有返回所需模型，可以手动添加，但仍须确认密钥拥有权限。模型 ID 不应包含空格、显示名称或自行添加的 `-thinking` 等后缀。

## 添加 Anthropic 或 Gemini 渠道

需要原生协议时分别创建 `Gkotta-Anthropic` 或 `Gkotta-Gemini`，选择对应的渠道类型并填写表格中的根地址。密钥和模型替换成该协议对应分组的值，再启用提供商。

对于 Anthropic，检查预览是 `/v1/messages`，避免 `/v1/v1/messages`；对于 Gemini，检查预览中只有一次 `/v1beta`。新版若允许逐模型选择端点，模型的端点类型也要与所选渠道一致。

## 验证与日常使用

新建助手或会话，先关闭知识库、工具和图片输入，发送「用一句话介绍你能做什么」。收到文本后，再发送「把上一句话改写成英文」，确认多轮对话正常，并到[使用记录](https://www.gkotta.bid/usage)核对实际模型。

图片问答、文件解析、提示缓存、代码执行或图像生成需要分别确认模型、分组及客户端支持情况，先逐项验证，再用于正式任务。原生 Anthropic 的缓存是否命中及如何计费，以 Gkotta 使用记录为准。

## 常见问题

- **提供商配置好了但聊天列表没有模型**：启用提供商，确认模型已添加到对应服务，而不是另一个同名服务。
- **404**：对照请求预览排查重复版本前缀或错误端点类型；不要把完整 `/chat/completions` 路径填进普通 Base URL 字段。
- **模型不存在/无权限**：从当前分组确认真实模型 ID；OpenAI、Anthropic、Gemini 的模型权限需分别检查。
- **文本成功但图片/工具失败**：逐项确认模型能力，撤掉附加功能，用最小文本请求复测。
- **连接检查通过但生成失败**：检查余额、有效订阅和密钥限额，列表查询成功不能代替生成验证。

## 官方资料

- [Cherry Studio 文档](https://docs.cherry-ai.com/)
- [官方地址预览实现](https://github.com/CherryHQ/cherry-studio/blob/main/src/renderer/pages/settings/ProviderSettings/hooks/providerSetting/buildHostEndpointPreviews.ts)
- [OpenAI API](/openai-api) · [Anthropic API](/anthropic-api) · [Gemini API](/gemini-api)
