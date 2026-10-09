# Codex++

Codex++ 是社区维护的 Codex App 增强启动器。它的“中转注入”功能可以把模型请求交给自定义 Responses 服务。Gkotta 可作为这个模型提供商，Codex App 的账户能力仍按官方登录和计划要求使用。

## 安装与准备

1. 安装并启动 Codex App，按官方流程完成 ChatGPT 账号登录。
2. 从[Codex++ 官方仓库](https://github.com/xianyu110/CodexPlusPlus)及[Releases](https://github.com/xianyu110/CodexPlusPlus/releases)下载对应 Windows 或 macOS、CPU 架构的安装包。
3. 退出 Codex App，备份已有的 `~/.codex/config.toml`。Windows 对应用户目录下的 `.codex/config.toml`。
4. 在 Gkotta 创建支持 Responses 的密钥，确认所选模型在该分组可用。可先按[OpenAI API](/openai-api)验证。

安装后通常有 **Codex++** 启动入口和 **Codex++ 管理工具** 两个入口。配置在管理工具中完成，启动时使用 Codex++ 入口。

## 配置中转注入

1. 打开管理工具的“中转注入”页面，确认它检测到已有的 ChatGPT 登录状态。
2. 添加中转配置，名称填写 `Gkotta`。
3. Base URL 填写 `https://www.gkotta.bid/v1`，API Key 填自己的 Gkotta 密钥。
4. 在模型设置中填写分组实际支持的模型 ID。上下文窗口按模型真实能力设置；不确定时保留默认值。
5. 选择这份配置并应用中转注入。
6. 从 Codex++ 启动 Codex App，选择所配置模型，发送“只回复 Hello，不执行工具”。

在[使用记录](https://www.gkotta.bid/usage)确认请求使用了自己的密钥和预期模型，再验证编程任务。

管理工具会维护 `CodexPlusPlus` provider，协议为 `responses`，Base URL 带 `/v1`。其配置和认证由管理工具生成；不要同时用另一份 CLI 配置覆盖它。[Codex CLI](/codex)可以独立接入，使用方式与该增强启动器不同。

## 恢复官方模式

在管理工具“中转注入”页面清除 API 模式，然后退出并重新启动应用，恢复官方 ChatGPT 登录模式。保留原有登录文件和配置备份，按项目官方恢复说明操作。

Gkotta 只负责配置后经过本站的模型请求；Codex App 登录、插件资格和 Codex++ 自身增强功能由对应项目管理。

## 排错

| 现象 | 检查 |
| --- | --- |
| 管理工具未检测到官方登录 | 先通过原版 Codex App 完成登录，再打开管理工具 |
| API 401 / 403 | 检查密钥、有效期、IP 限制及分组 |
| 404 或协议错误 | Base URL 应为 `/v1`，确认模型支持 Responses |
| 仍使用旧模型或旧地址 | 完全退出 Codex App，重新应用当前配置并从 Codex++ 启动 |
| 增强界面未出现 | 使用 Codex++ 启动入口，在其诊断和日志中检查注入状态 |
| App 更新后增强功能异常 | 查看 Codex++ 更新及兼容说明 |

日志和截图分享前隐藏完整密钥。官方配置说明见[中转注入](https://github.com/xianyu110/CodexPlusPlus#中转注入)，账户与额度问题见[排错指南](/troubleshooting)。
