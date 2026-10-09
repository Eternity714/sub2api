# Roo Code（VS Code）

Roo Code 可以按 Ask、Code、Architect、Debug 等模式处理编程任务，并使用不同配置档切换模型。通过 OpenAI Compatible 提供商可以接入 Gkotta。

## 确认模型能力

安装 [Roo Code 扩展](https://marketplace.visualstudio.com/items?itemName=RooVeterinaryInc.roo-cline)，在 [API 密钥](https://www.gkotta.bid/keys)创建专用密钥，再到[可用渠道](https://www.gkotta.bid/available-channels)确认模型与分组。

当前 Roo Code 官方文档要求 OpenAI Compatible 模型支持**原生工具调用**。除了 `/v1/chat/completions` 能生成文本，还需模型与上游完整支持 OpenAI 的工具调用格式；仅支持普通聊天的模型不能作为该模式的完整替代。

## 创建 API 配置档

1. 打开 Roo Code 面板，点击设置图标。
2. 新建 API Configuration Profile，名称填写 `Gkotta`。
3. API Provider 选择 **OpenAI Compatible**，填写以下内容。

| 字段 | 值 |
| --- | --- |
| Base URL | `https://www.gkotta.bid/v1` |
| API Key | `sk-YOUR_API_KEY` |
| Model ID | `YOUR_MODEL_ID` |

4. 按模型实际能力配置上下文长度、最大输出和图片支持，保存配置档。
5. 回到任务面板，选择该配置档及要使用的模式。

不要将密钥写入示例 JSON、工作区设置或公开配置档。多个模式需要不同模型时，分别建立配置档并按 Roo Code 的模式配置方式关联；切换模式后再确认当前实际模型。

## 最小验证工作流

打开一个小项目，先用 Ask 模式发送「请只回复连接成功，不修改文件」。收到回复后，再明确指定一个小文件，要求「读取这个文件并解释入口函数，不修改代码」，验证原生工具调用和工具结果能正常返回。

然后切到 Code 模式，只请求一处容易审查的改动，例如补充函数注释。保持自动批准关闭，检查读取、编辑和命令请求，确认每处差异再批准。最后按照自己的项目规则验证改动，并到 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对模型与费用。

如果第一步文本回复成功、第二步读取工具失败，应优先检查工具协议支持，不能把这个结果当作整个 Agent 工作流已接通。

## 按模式组织任务

- **Ask**：解释代码和讨论问题，先明确是否允许读取文件。
- **Architect**：制定实现方案，限制上下文为当前功能的相关文件。
- **Code**：执行已明确的改动，逐项审查文件差异。
- **Debug**：提供具体错误、复现步骤和相关日志，避免上传完整无关日志。

模式名称和可用工具以所安装版本为准。代码库索引需要另外配置嵌入模型，MCP 服务也需要单独连接，不能由聊天密钥自动替代。

## 常见问题

- **工具调用错误**：核对模型与上游原生 tool calling 支持，查看错误中的 `tools`、`tool_calls` 或参数字段，换成明确兼容的模型再复测读取小文件。
- **404**：Base URL 填 `/v1` 基础地址，避免 `/v1/v1` 或完整聊天端点被再次追加。
- **换模式后模型不一致**：核对该模式当前选中的 API 配置档，修改一个配置档不会自动更新其他档。
- **模型不存在或无权限**：检查真实 ID、分组、订阅与密钥限额，不要使用显示名称。
- **索引失败**：检查嵌入提供商与索引配置，和 Agent 的聊天连接分开排查。
- **任务成本快速增加**：缩小读取范围、减少会话历史，限制专用密钥额度；客户端估算不能代替 Gkotta 账单。

## 官方资料

- [Roo Code OpenAI Compatible 配置与工具调用要求](https://docs.roocode.com/providers/openai-compatible)
- [Roo Code 文档](https://docs.roocode.com/)
- [Roo Code 开源仓库](https://github.com/RooCodeInc/Roo-Code)
- [Gkotta OpenAI API](/openai-api)
