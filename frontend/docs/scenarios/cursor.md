# Cursor

Cursor 可以使用自己的 API Key，并允许覆盖 OpenAI Base URL。本教程先验证通过 Gkotta 的自定义聊天模型，其他编辑器能力再按当前 Cursor 版本与模型支持情况逐项确认。

## 配置自定义地址

从 [Cursor 官网](https://cursor.com/)安装编辑器，在 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建专用密钥，到[可用渠道](https://www.gkotta.bid/available-channels)确认模型及分组支持 Chat Completions。

1. 打开 **Cursor Settings → Models**，展开 **API Keys**。
2. 在 **OpenAI API Key** 中填写 `sk-YOUR_API_KEY`，开启 **Use OpenAI API Key**。
3. 开启 **Override OpenAI Base URL**，填写 `https://www.gkotta.bid/v1`。
4. 按当前界面的 **Save/Verify** 保存或验证。
5. 在 Models 中添加自定义模型，名称填写 `YOUR_MODEL_ID`，必须与 Gkotta 接口接受的 ID 完全一致。
6. 重启编辑器或新建聊天，选择刚添加的自定义模型。

Base URL 填基础地址，不能填完整 `/chat/completions` 路径。模型只有被当前 Cursor 版本和所选调用协议支持时才可使用，不能把控制台所有模型都视为 Cursor 可用模型。

## 最小验证工作流

在新的聊天中明确选择自定义模型，发送「请只回复：连接成功」。收到文字后再附加一个小函数，要求解释输入与返回值；暂时不启用工具或跨文件操作。到 [Gkotta 使用记录](https://www.gkotta.bid/usage)确认请求确实使用了对应模型。

连接验证可能使用一个预设模型。Verify 失败时应结合返回错误判断是否是预设模型无权限，仍要用自己分组允许的模型完成实际文本生成验证，不能只反复点击 Verify。

## 功能范围

Cursor 官方说明自带 API Key 用于聊天模型，**Tab 补全仍使用 Cursor 内置模型**。自定义地址用于 Agent、工具调用或其他编辑流程时，还取决于 Cursor 版本、请求格式、模型能力和当前分组支持情况，需在基础聊天成功后逐项验证。

自定义模型和 Cursor 内置模型混用时，如果内置模型也被错误送到覆盖地址，切回内置模型前关闭 Override OpenAI Base URL，重新打开聊天确认配置。不要为解决这类路由问题给 Gkotta 增加不存在的模型别名。

Cursor 官方说明 BYOK 请求会经过 Cursor 的服务端构建提示词。因此，服务端是否能访问自定义地址、密钥的 IP 限制，以及 Cursor 账户/团队的模型政策都可能影响调用。两边的使用限制和费用规则分别以各自平台为准。

## 常见问题

- **Model name is not valid**：检查 Use OpenAI API Key 开关、实际模型 ID 和当前选择的提供商，再重启并新建聊天。
- **404**：确认覆盖地址为 `https://www.gkotta.bid/v1`，未重复添加版本或聊天端点。
- **本地 cURL 成功但 Cursor 超时**：检查 Cursor 服务端到自定义地址的访问条件及密钥 IP 限制；两者请求来源不同。
- **聊天成功但工具调用失败**：核对 Cursor 发送的工具格式与模型支持情况，先恢复无工具聊天，再验证单个读取步骤。
- **Tab 没走 Gkotta**：Tab 使用 Cursor 内置模型，修改自定义 API Key 不会切换它。
- **选回内置模型时报错**：关闭地址覆盖，新建聊天，再选择内置模型。

## 官方资料

- [Cursor 自带 API Key 说明](https://cursor.com/docs/settings/api-keys)
- [官方支持人员说明自定义 Base URL 的配置步骤](https://forum.cursor.com/t/how-to-adjust-settings-for-custom-added-models/173253)
- [Gkotta OpenAI API](/openai-api)
