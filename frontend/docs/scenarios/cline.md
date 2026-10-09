# Cline（VS Code）

Cline 是 VS Code 中的 AI 编程助手，可以读取项目、提出修改并调用工具。本文使用它的 **OpenAI Compatible** 提供商连接 Gkotta。

## 安装与配置

1. 在 VS Code 扩展商店安装 [Cline](https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev)，打开侧栏中的 Cline 面板。
2. 到 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建编程专用密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认模型及分组支持 Chat Completions。需要 Agent 操作时，还须核对工具调用能力。
3. 点击 Cline 的设置图标，API Provider 选择 **OpenAI Compatible**。
4. 填写并保存以下配置。

| 字段 | 值 |
| --- | --- |
| Base URL | `https://www.gkotta.bid/v1` |
| API Key | `sk-YOUR_API_KEY` |
| Model ID / Model | `YOUR_MODEL_ID` |

5. 按模型能力设置上下文长度、最大输出及图片支持；首次接入先使用基础文本配置，避免同时开启未确认支持的选项。

Base URL 不包含 `/chat/completions`。如果 Plan 与 Act 模式分别保存模型配置，两个模式都要核对提供商、地址和模型，不要只修改其中一个。

## 最小验证工作流

先打开一个小型项目，新建任务并明确要求：

```text
请只回复“连接成功”，不要读取文件、修改文件或运行命令。
```

收到文本后到 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对模型。然后在 Plan 模式提供一个小文件，要求解释函数和提出一个改进建议，暂时不修改代码，以验证项目上下文与工具读取流程。

需要验证实际编辑时，再切到 Act 模式，选择一处小修改，例如「为这个函数补充参数说明，保持逻辑不变」。检查每次请求的操作与修改差异，再批准相应步骤。修改完成后，按自己的项目规则运行验证。

普通文本成功仅说明聊天接口接通。文件操作、命令执行和其他 Agent 步骤还取决于 Cline 版本、工具协议以及当前模型的能力。

## 使用项目规则控制范围

可按 Cline 官方说明在项目中创建 `.clinerules` 规则文件或规则目录，写明项目约束，例如：

```text
每次任务先说明涉及的文件。
读取代码时只包含与当前任务有关的上下文。
修改已有代码前先解释目的。
执行命令前遵守项目的测试与环境约定。
```

需要排除无关文件时按官方说明配置 `.clineignore`。规则和忽略列表用于帮助客户端控制任务范围，不要把 API Key 放入这些可提交的项目文件。

长任务应按功能拆成多个会话。Cline 显示的成本是客户端估算，Gkotta 的实际分组倍率和消耗以使用记录为准。

## 常见问题

- **404**：检查 Base URL 为 `https://www.gkotta.bid/v1`，未附加完整聊天路径。
- **Model Not Found**：核对实际 ID 和分组；不是所有模型都适合编程或工具调用。
- **Plan 成功但 Act 失败**：检查两个模式是否选择了不同提供商或模型，并确认 Act 需要的工具调用协议。
- **工具调用错误或反复循环**：换成确认支持所需工具协议的模型，提供较小任务；文本接口成功不能证明工具协议完整兼容。
- **401/403/429**：检查密钥状态、IP 限制、配额和订阅，避免多个任务同时耗尽额度或并发。
- **上下文过长**：新建任务，减少读取范围，不要一次要求扫描整个大型仓库。

## 官方资料

- [Cline OpenAI Compatible 配置](https://docs.cline.bot/provider-config/openai-compatible)
- [Cline 文档](https://docs.cline.bot/)
- [Cline 开源仓库](https://github.com/cline/cline)
- [Gkotta OpenAI API](/openai-api)
