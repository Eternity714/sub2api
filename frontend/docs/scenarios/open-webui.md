# Open WebUI

Open WebUI 是可自行部署的多用户聊天界面。通过它的 OpenAI 兼容连接，可以让服务端使用 Gkotta 密钥调用模型，再把结果返回给浏览器。

## 添加连接

按 [Open WebUI 官方快速开始](https://docs.openwebui.com/getting-started/quick-start/)安装并创建管理员账号。从 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建专用密钥，确认[分组与模型](https://www.gkotta.bid/available-channels)支持 Chat Completions。

1. 以管理员登录，打开 **Settings → Admin → Connections**。
2. 在 **Manage OpenAI API Connections** 中点击 **Add Connection**。
3. URL 填 `https://www.gkotta.bid/v1`，API Key 填 `sk-YOUR_API_KEY`，保存并启用连接。
4. 点击连接检查按钮，确认能查询 `/v1/models`。
5. 如需限制模型，在 **Model IDs** 中逐个添加已确认可用的真实 ID，例如 `YOUR_MODEL_ID`，再保存。

这里的 URL 是 API Base URL，客户端会追加 `/models` 或 `/chat/completions`，不要填写完整的聊天端点。模型白名单只限制客户端显示范围，实际权限仍由 Gkotta 分组决定。

## 使用环境变量配置

自行部署时也可在 Open WebUI 服务端设置：

```dotenv
ENABLE_OPENAI_API=true
OPENAI_API_BASE_URLS=https://www.gkotta.bid/v1
OPENAI_API_KEYS=sk-YOUR_API_KEY
```

多个连接的 URL 与 Key 使用分号分隔，并按顺序一一对应。密钥应保存在服务端环境或连接配置中，避免把管理员共用密钥嵌入公开前端。

已有实例可能从持久化数据库读取连接配置，改变环境变量后不一定会覆盖之前保存的值。进入管理员 Connections 页面核对实际地址与密钥，按官方配置说明处理，不要为更新连接删除数据卷。

## 最小验证工作流

新建聊天，在模型选择器选择 `YOUR_MODEL_ID`，先关闭知识库检索、联网工具和附件，发送「用一句话解释 HTTP」。收到回复后再追问「给出一个请求方法示例」，确认上下文正常，到 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对模型和消耗。

Open WebUI 的文件上传、知识库、检索和图像生成需要各自的组件或模型配置。文本接口连通后，仍须单独确认嵌入、重排或图像接口是否已配置；本教程不把它们视为 Gkotta 聊天密钥自动提供的能力。

需要通过 Open WebUI 自身的 `/api/chat/completions` 调用时，应使用 **Open WebUI 账号生成的 API Key**。Gkotta Key 只用于上游连接，两者不能互换。

## 常见问题

- **列表没有模型**：检查连接启用状态、`/v1/models` 响应和 Model IDs 白名单；必要时手动添加当前分组允许的 ID。
- **检查成功但生成失败**：实际生成仍检查余额、订阅和配额，查看 Gkotta 错误响应及使用记录。
- **404**：URL 应为 `https://www.gkotta.bid/v1`，不能缺少版本路径或含重复聊天路径。
- **环境变量更新后仍调用旧地址**：核对管理界面的持久化连接；重启容器不会自动清除数据库配置。
- **上传文档失败或检索无结果**：检查 Open WebUI 存储、解析及嵌入模型配置，与聊天连接分别排查。
- **浏览器能访问但服务调用超时**：从 Open WebUI 所在服务器检查出站 HTTPS、代理和 DNS；请求由该服务器发出。

## 官方资料

- [OpenAI 兼容连接说明](https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai-compatible)
- [官方环境变量配置源码](https://github.com/open-webui/open-webui/blob/main/backend/open_webui/config.py)
- [Gkotta OpenAI API](/openai-api)
