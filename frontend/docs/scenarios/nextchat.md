# NextChat

NextChat（原 ChatGPT Next Web）是可自行部署的网页聊天客户端。本文通过 NextChat 的服务器代理接入 Gkotta，让浏览器使用访问密码进入应用，Gkotta 密钥保存在服务器配置中。

## 准备工作

按 [NextChat 官方仓库](https://github.com/ChatGPTNextWeb/NextChat)部署应用，可选择它支持的容器或托管方式。部署需要包含 NextChat 服务端，纯静态网页不能使用本教程的服务器代理。

在 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建专用密钥，设置合适的额度，到[可用渠道](https://www.gkotta.bid/available-channels)确认分组支持 OpenAI Chat Completions 和要使用的模型。

## 配置服务器环境变量

在部署平台的服务器环境变量或容器环境中设置：

```dotenv
OPENAI_API_KEY=sk-YOUR_API_KEY
BASE_URL=https://www.gkotta.bid
CODE=CHANGE_TO_A_PRIVATE_ACCESS_PASSWORD
CUSTOM_MODELS=-all,+YOUR_MODEL_ID@OpenAI
DEFAULT_MODEL=YOUR_MODEL_ID
HIDE_USER_API_KEY=1
```

`OPENAI_API_KEY` 替换为密钥，`YOUR_MODEL_ID` 替换为实际 ID，`CODE` 换成自己的访问密码。`CUSTOM_MODELS` 的 `-all` 清除默认模型列表，只添加已验证模型；需要多个模型时，用逗号追加 `+模型ID@OpenAI`。

**NextChat 的 `BASE_URL` 是根地址。** 它的服务器代理会从客户端请求中取得 `v1/chat/completions` 并追加到这里，因此不要填写 `https://www.gkotta.bid/v1`，否则可能产生重复 `/v1`。这是 NextChat 的变量规则；其他 OpenAI SDK 的 Base URL 仍使用 `/v1`。

修改环境变量后重新部署或重启服务。浏览器中的 API Key 保持空白，使用访问密码进入服务端代理。不要把密钥写入 `NEXT_PUBLIC_*`、`VITE_*`、网页源代码或分享链接。

## 完成第一次调用

1. 打开自己的 NextChat 实例，在设置中输入上面配置的访问密码。
2. 新建聊天，选择 `YOUR_MODEL_ID` 对应的 OpenAI 提供商模型。
3. 发送「请只回复：连接成功」，确认收到文本。
4. 再发送「将上一条回复翻译成英文」，确认多轮对话正常。
5. 在 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对请求模型和费用。

部署自己的实例后可以使用 NextChat 的面具、提示词和导出功能。图片能力要另行确认模型与客户端支持，先用纯文本验证连接。访问密码保护的是 NextChat 应用入口，不会替代 Gkotta 密钥的额度和权限控制。

## 排错

- **401 且没有 Gkotta 使用记录**：检查 NextChat 访问密码和服务端 `CODE` 是否一致；不要将访问密码填成 Gkotta 密钥。
- **上游认证失败**：确认服务器实际读取到了 `OPENAI_API_KEY`，更新变量后已重新部署。
- **404 或 `/v1/v1/chat/completions`**：把服务器 `BASE_URL` 改回 `https://www.gkotta.bid`，并检查浏览器是否还保存旧的自定义接口地址。
- **模型没有显示**：核对 `CUSTOM_MODELS` 和 `DEFAULT_MODEL` 的大小写、逗号和 `@OpenAI`，重新部署后刷新页面。
- **新增模型能显示却无法生成**：检查真实模型 ID、分组协议、余额及订阅；模型列表配置只控制客户端选项。
- **流式回复断开**：确认部署平台和反向代理允许持续的流式响应，先缩短上下文再测试。

## 官方资料

- [NextChat 环境变量与部署说明](https://github.com/ChatGPTNextWeb/NextChat#environment-variables)
- [官方服务器代理的路径拼接实现](https://github.com/ChatGPTNextWeb/NextChat/blob/main/app/api/common.ts)
- [Gkotta OpenAI API](/openai-api)
