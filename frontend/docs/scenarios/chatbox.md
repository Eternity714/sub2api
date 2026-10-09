# Chatbox AI

Chatbox 是桌面及移动端 AI 对话客户端。本教程通过它的 OpenAI 兼容提供商接入 Gkotta，适合日常对话、提示词管理和支持视觉模型的图片问答。

## 准备密钥和模型

从 [Chatbox 官网](https://chatboxai.app/)安装应用，在 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建密钥。到[可用渠道](https://www.gkotta.bid/available-channels)确认该密钥分组支持 OpenAI Chat Completions，并记录实际模型 ID。

下文的 `sk-YOUR_API_KEY` 和 `YOUR_MODEL_ID` 都需要替换。客户端能添加模型，并不代表当前分组一定允许调用；图片、工具调用等能力还取决于模型和上游。

## 添加 Gkotta 提供商

1. 打开 Chatbox 的**设置 → 模型提供商/模型配置**。
2. 点击添加提供商，名称填写 `Gkotta`，API 类型选择 **OpenAI API Compatible**。
3. 填写以下配置并保存。

| 字段 | 值 |
| --- | --- |
| API Key | `sk-YOUR_API_KEY` |
| API Host / Base URL | `https://www.gkotta.bid/v1` |
| API Path | `/chat/completions` |

4. 在该提供商中添加模型，模型 ID 填 `YOUR_MODEL_ID`，显示名称可自行设置。
5. 回到聊天页面，选择刚创建的提供商和模型。

API Host 与 API Path 拼接后的请求应为 `https://www.gkotta.bid/v1/chat/completions`。不要同时在 Host 和 Path 里填写 `/v1`。若应用版本只显示一个 Base URL 字段，填写 `https://www.gkotta.bid/v1`，让客户端追加聊天路径。

## 完成第一次对话

新建会话，发送「请只回复：连接成功」。收到正常文本后，到 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对模型和消耗，再测试第二轮「把刚才的回复翻译成英文」，确认上下文能正常延续。

需要图片理解时，在模型设置中按实际能力启用图片支持，上传一张小图片并询问内容。文件上传、图片解析、导出和跨设备同步属于 Chatbox 的功能，具体支持范围以所安装版本为准；上传成功不等于模型支持所有文件类型。

参数先使用默认值。确认基础对话成功后，再按模型能力调整输出长度或采样参数。长文档建议分段发送，避免一次把无关历史全部带入请求。

## 连接失败时

- **404**：检查最终地址，常见错误是 `/v1/v1/chat/completions` 或缺少 `/v1`。
- **模型没有出现在列表中**：尝试手动添加实际模型 ID；刷新列表后仍需核对密钥分组，不要用显示名称代替 ID。
- **401/403**：重新检查密钥首尾空格、有效期、IP 限制和分组权限。
- **图片请求失败**：先发送纯文本，确认所选模型支持视觉，再检查图片格式和大小。
- **回复中断或变慢**：新建短会话排除上下文问题，检查网络及[使用记录](https://www.gkotta.bid/usage)，减少输出长度后重试。

## 官方资料

- [Chatbox 帮助中心](https://chatboxai.app/en/help-center)
- [Chatbox 开源仓库](https://github.com/chatboxai/chatbox)
- [官方自定义 OpenAI 地址处理源码](https://github.com/chatboxai/chatbox/blob/main/src/shared/utils/llm_utils.ts)
- [Gkotta OpenAI API](/openai-api)
