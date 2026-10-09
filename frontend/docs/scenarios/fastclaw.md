# FastClaw

FastClaw 是独立部署的 Agent 工具，支持 OpenAI 兼容供应商和自定义 `apiBase`。将其模型供应商指向 Gkotta 后，可以使用当前密钥分组支持的聊天与工具调用模型。

## 安装并启动

按[FastClaw 官方仓库](https://github.com/fastclaw-ai/fastclaw)的当前安装说明获取客户端。首次运行：

```bash
fastclaw
```

首次向导用于配置模型供应商并创建默认 Agent。随后打开本机 `http://localhost:18953`，使用 FastClaw 创建的本地管理员凭证进入 Dashboard。

FastClaw 的管理员令牌用于管理本地服务，Gkotta API Key 用于支付和认证模型请求，两者需要分别保存与配置。

## 设置 Gkotta 模型供应商

在 Dashboard 的 **Models** 管理模型供应商，选择支持自定义地址的 OpenAI 兼容提供商：

| 配置 | 值 |
| --- | --- |
| 名称 | Gkotta |
| API Base / `apiBase` | `https://www.gkotta.bid/v1` |
| API Key | 自己的 Gkotta 密钥 |
| Model | 当前分组实际可用的模型 ID |

保存后进入目标 Agent 的设置，选择刚配置的供应商和模型。Agent 自己的 Models 设置可以覆盖系统同名配置，连接失败时也要检查 Agent 层的地址和密钥。

当前项目把供应商和运行配置保存在数据库，通过 Dashboard 或 CLI 更新。无需创建 `fastclaw.json`。若使用 CLI，按官方当前 `fastclaw agents config` 的 `provider.<name>.<field>` 规则操作，修改后确认已加载到实际运行的服务。

## 验证请求

1. 先使用[OpenAI API](/openai-api)中的最小请求确认密钥和模型可用。
2. 在 Agent 的 Chat 页面发送“只回复 Hello，不调用工具”。
3. 在 Gkotta [使用记录](https://www.gkotta.bid/usage)检查实际模型和密钥。
4. 简单聊天成功后，再测试 Agent 工具。所选模型必须支持客户端要求的工具调用格式。

IM Channels、沙箱、文件访问与定时任务在 FastClaw 内单独配置，具体依赖及权限按其官方文档处理。

## 常见问题

| 现象 | 处理 |
| --- | --- |
| Dashboard 无法登录 | 检查 FastClaw 本地管理员凭证和服务状态 |
| 模型请求 401 | 检查供应商栏中的 Gkotta 密钥及服务实际加载的配置 |
| 配置已修改但请求走旧地址 | 检查 Agent 层覆盖项；按官方提示重载或重启 FastClaw |
| 普通聊天成功但工具失败 | 检查模型工具能力、FastClaw 工具权限和本地依赖 |
| 找不到模型 | 核对分组、实际模型 ID，见[API 密钥与分组](/api-keys) |

官方配置、存储及平台支持说明见[FastClaw README](https://github.com/fastclaw-ai/fastclaw#configuration)，额度和网络排错见[常见问题](/troubleshooting)。
