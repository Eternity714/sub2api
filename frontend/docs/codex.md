# Codex

## 安装与准备

安装方式以 [Codex 官方文档](https://developers.openai.com/codex/cli)为准。使用 npm 安装 CLI 时，先安装兼容的 [Node.js LTS](https://nodejs.org/en/download)，再执行：

```bash
npm install -g @openai/codex
codex --version
```

准备一把支持 OpenAI Responses 接口的 Gkotta 密钥，在[可用渠道](https://www.gkotta.bid/available-channels)确认目标模型 ID。密钥创建步骤见[API 密钥管理](/api-keys)。

## 配置文件位置

下面使用用户目录中的配置。若你另外设置过 `CODEX_HOME`，实际配置目录会不同，应以工具当前使用的目录为准。

| 系统 | 默认目录 |
| --- | --- |
| macOS 或 Linux | `~/.codex` |
| Windows | `%USERPROFILE%\.codex` |

macOS 或 Linux 可先创建目录：

```bash
mkdir -p ~/.codex
```

Windows PowerShell：

```powershell
New-Item -ItemType Directory -Path "$env:USERPROFILE\.codex" -Force
```

编辑已有文件前先保留副本，合并下面的 provider 设置，避免丢失你已有的项目、MCP 或其他偏好配置。

## 配置 provider 和模型

在配置目录中编辑 `config.toml`：

```toml
model_provider = "gkotta"
model = "YOUR_MODEL_ID"
disable_response_storage = true

[model_providers.gkotta]
name = "Gkotta"
base_url = "https://www.gkotta.bid/v1"
wire_api = "responses"
requires_openai_auth = true
```

将 `YOUR_MODEL_ID` 换成当前密钥所属分组可用的模型。`model_provider` 必须与 `[model_providers.gkotta]` 中的名称对应。

这里使用 `/v1` 作为 Base URL，并将协议设置为 `responses`。不要填写 `/docs`，也不要把 `wire_api` 改成 Chat Completions。

## 保存 API 密钥

在同一个配置目录中编辑 `auth.json`：

```json
{
  "OPENAI_API_KEY": "sk-YOUR_API_KEY"
}
```

这组示例通过 `requires_openai_auth = true` 与 `auth.json` 配合读取 API 密钥。若客户端版本或密钥页面展示了其他认证模式，按那一组完整配置操作，避免混用不同模式的字段。

macOS 或 Linux 可以限制密钥文件权限：

```bash
chmod 600 ~/.codex/auth.json
```

`auth.json` 包含明文凭证，不能提交到仓库或随配置截图公开分享。把示例占位符替换成真实密钥后再启动工具。

## 启动并验证

1. 保存两个文件，退出已运行的 Codex。
2. 在自己的项目目录启动 `codex`。桌面客户端修改配置后也需要重新启动。
3. 提出简单请求，例如“只回复 Hello，不修改文件”。
4. 在[使用记录](https://www.gkotta.bid/usage)确认当前请求使用了正确的密钥和模型。

需要单独验证 HTTP 接口时，使用[OpenAI API](/openai-api)中的 Responses 请求示例。直接请求成功而工具失败时，检查它使用的配置目录、provider、认证模式和模型。

## 常见配置问题

- **找不到 provider**：核对 `model_provider = "gkotta"` 与配置表 `[model_providers.gkotta]` 的拼写。
- **请求地址不对**：核对 Base URL 为 `https://www.gkotta.bid/v1`，并排除其他 profile 或启动参数覆盖。
- **仍然提示登录或密钥无效**：检查 `auth.json` 是否位于有效配置目录，JSON 是否有效，以及密钥是否停用。
- **模型不可用**：把 `YOUR_MODEL_ID` 替换成分组实际支持的模型 ID；模型列表中有某个名称也不代表所有分组均可用。
- **某个额外功能被拒绝**：先验证基本文本请求，再检查该功能是否属于分组和模型支持的能力；不要把单项能力错误当成整个接口不可用。
- **403、429 或超时**：按[常见问题](/troubleshooting)区分额度、权限、并发和服务状态。

如果使用配置管理工具，确认保存后重新启用 Gkotta provider；否则下次切换时它可能覆盖手工编辑的文件。
