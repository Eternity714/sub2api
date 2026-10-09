# 简易使用

如果你只想尽快在编程工具中使用 Gkotta，按下面的顺序完成账号、密钥和客户端配置即可。第一次接入可以选择 Codex；已经在用其他工具时，在[客户端教程](/clients)选择对应入口。

## 1. 准备账号与密钥

1. 打开 [Gkotta](https://www.gkotta.bid/home)，注册或登录账号。
2. 在[可用渠道](https://www.gkotta.bid/available-channels)确认目标分组、协议和模型。
3. 打开[API 密钥](https://www.gkotta.bid/keys)，创建一把密钥并选择该分组。
4. 复制密钥。在密钥的使用入口查看当前平台生成的配置，记下模型 ID。

文档里的 `sk-YOUR_API_KEY` 和 `YOUR_MODEL_ID` 是占位符，分别换成你的密钥和当前分组可用的模型 ID。模型显示名称可能与接口 ID 不同，以使用配置或模型列表为准。

密钥的额度上限是这把密钥的消费限制。账户有余额、订阅有效，也仍需检查密钥额度及有效期；设置方法见[API 密钥管理](/api-keys)。

## 2. 安装客户端

按 [Codex 官方安装指南](https://developers.openai.com/codex/cli)安装 CLI、桌面版或编辑器扩展。使用 CLI 时，可以先安装兼容的 [Node.js LTS](https://nodejs.org/en/download)，再运行：

```bash
npm install -g @openai/codex
codex --version
```

不想手动编辑配置，可以同时从 [CC Switch Releases](https://github.com/farion1231/cc-switch/releases)安装配置管理工具。CC Switch 帮你把接口地址、密钥和模型写入客户端配置；它不代替 Codex 本体。

## 3. 用 CC Switch 配置 Codex

1. 打开 CC Switch，选择 **Codex**。
2. 添加自定义供应商，名称填 `Gkotta`。
3. 设置 Base URL 为 `https://www.gkotta.bid/v1`，协议选择 **OpenAI Responses**。
4. 输入密钥，把默认模型改成当前分组可用的模型 ID。
5. 保存并启用该供应商，完全退出后重新打开 Codex。

不同版本的界面名称可能有变化。完整配置字段、其他工具的地址和恢复方法见 [CC Switch](/cc-switch)。如果工具仍提示官方登录，请核对当前启用的供应商、认证方式与实际配置目录。

## 4. 手工配置 Codex

也可以直接编辑用户目录下的 `.codex` 文件夹。Windows 默认位置是 `%USERPROFILE%\.codex`，macOS / Linux 是 `~/.codex`。已有配置先保留副本，只合并需要修改的字段。

在 `auth.json` 中保存密钥：

```json
{
  "OPENAI_API_KEY": "sk-YOUR_API_KEY"
}
```

在 `config.toml` 中选择供应商与模型：

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

全局的 `model_provider` 与 `model` 放在 TOML 文件的表头之前；供应商字段放在 `[model_providers.gkotta]` 下。详细说明见 [Codex](/codex)。

## 5. 发送一次小请求

保存配置、重新启动 Codex，在项目目录中输入：

```text
只回复 Hello，不修改文件，也不执行命令。
```

收到回复后，到[使用记录](https://www.gkotta.bid/usage)核对时间、密钥、模型和费用。能查询模型列表与能生成内容是不同检查，生成还受分组权限、余额或订阅以及密钥限额影响。

## 连接失败时

| 现象 | 优先检查 |
| --- | --- |
| 仍提示官方登录 | 是否启用 Gkotta 供应商，是否读取了正确的 `auth.json` |
| 401 / 密钥无效 | 密钥是否复制完整、停用或过期 |
| 404 / 模型不可用 | `/v1` 是否只出现一次，模型 ID 是否属于当前分组 |
| 额度不足或 429 | 账户余额、订阅额度、密钥限额、并发限制 |
| 配置改了但无变化 | 客户端是否完全退出，配置管理工具是否覆盖了手工配置 |

需要定位接口本身时，先运行[快速开始](/quick-start)中的最小 HTTP 请求。更多错误处理见[常见问题](/troubleshooting)。密钥文件含有凭证，分享配置或截图前先移除密钥。
