# Claude Desktop

Claude Desktop 可以通过第三方供应商模式（3P）连接兼容的 Anthropic 网关。Gkotta 提供 Anthropic Messages 接口；下面使用 CC Switch 管理 Desktop 的 3P profile，并用当前分组支持的 Claude 模型验证接入。

## 版本与使用方式

从 [Claude 官方下载页](https://claude.com/download)安装支持 3P 的当前桌面版本，再安装 [CC Switch](/cc-switch)。本文按 **CC Switch v4.0.4+** 的[Claude Desktop 手册](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/2-providers/2.6-claude-desktop.md)说明操作。

Claude Desktop 的官方账号模式与 3P 模式使用不同认证来源：

- 官方账号模式使用 Anthropic 登录和对应计划。官方 Code 标签页的订阅要求见[Desktop 入门](https://code.claude.com/docs/en/desktop-quickstart)。
- 3P 模式把推理请求交给配置的供应商，使用该供应商的密钥与计费。官方将它定位为组织部署模式；受企业 MDM 或组织策略管理的设备，需要管理员允许或提供配置。

配置 3P 后，应用在启动时识别 provider 和凭证，按当前界面选择以该供应商启动。账户、系统与企业部署要求以[官方 3P 概览](https://claude.com/docs/third-party/claude-desktop/overview)及桌面应用提示为准。不能只把 API Key 粘贴进普通登录页面来替代 3P 配置。

## 准备密钥并验证 API

1. 在[API 密钥](https://www.gkotta.bid/keys)创建密钥。
2. 在[可用渠道](https://www.gkotta.bid/available-channels)确认分组支持 Anthropic Messages 和实际 Claude 模型 ID。
3. 可先用[Anthropic API](/anthropic-api)中的最小请求验证，确认密钥与模型能正常回复。

Base URL 使用 `https://www.gkotta.bid`，请求协议为 Anthropic Messages。直连需要模型 ID 属于 Desktop 识别的 `claude-sonnet-*`、`claude-opus-*` 或 `claude-haiku-*` 角色形式；模型的实际可用性以当前分组为准。

## 在 CC Switch 添加供应商

1. 完全退出 Claude Desktop。
2. 打开 CC Switch，在侧栏选择 **Claude Desktop**，不是 **Claude Code**。
3. 如果没有该入口，到“应用”页面确认它没有被隐藏，并核对 CC Switch 版本。
4. 点击添加供应商，选择自定义供应商。
5. 名称填写 `Gkotta`，API Key 填当前密钥，接口地址填写 `https://www.gkotta.bid`。
6. 选择 Anthropic Messages，使用已验证的 Claude 模型，保持“需要模型映射”关闭。
7. 保存，在供应商卡片上点击切换，确认它显示为使用中。
8. 重新打开 Claude Desktop，按 3P 配置启动。

如果已经在 CC Switch 的 Claude Code 页配置了 Gkotta，可使用“将 Claude Code 中已有的供应商导入”，然后核对 Desktop 页的地址、密钥与模型。Claude Code 的 `settings.json` 与 Desktop 的 3P profile 是不同配置，不能仅依靠修改前者完成 Desktop 接入。

## 模型映射何时需要

只有直连接口与 Desktop 识别的模型 ID 匹配时才用直连。使用旧式 Claude ID、其他模型 ID，或需要转换协议时，CC Switch 提供模型映射：

1. 编辑供应商，开启“需要模型映射”。
2. 协议仍选择与你当前分组匹配的实际协议。
3. 添加至少一条映射：模型角色选择 Sonnet / Opus / Haiku，实际请求模型填分组真实支持的模型 ID，菜单显示名自定义。
4. 不确定目标模型支持 1M 上下文时，不勾选 1M。
5. 保存并重新切换供应商，确认本地路由显示运行中，再重启 Claude Desktop。

映射模式下 Desktop 请求经过 CC Switch 本地路由，因此使用期间需要保持 CC Switch 运行。菜单中的角色或显示名称不改变真实上游模型；在 Gkotta 使用记录中核对实际模型。

## 3P profile 的维护

CC Switch 自动维护 Desktop 的 3P 配置。其官方手册给出的直连 profile 字段如下，仅用于理解配置，不需要手工创建另一份文件：

```json
{
  "inferenceProvider": "gateway",
  "inferenceGatewayBaseUrl": "https://www.gkotta.bid",
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceGatewayApiKey": "sk-YOUR_API_KEY"
}
```

Gkotta 的 Messages 接口接受 Bearer 凭证。完整 profile、元数据及各系统目录由 CC Switch 管理，排错时使用[官方 profile 位置说明](https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/2-providers/2.6-claude-desktop.md#配置文件位置)，不要把上述片段覆盖进任意 `claude_desktop_config.json`。

手动或企业部署网关的方法见[Claude Desktop 官方网关指南](https://claude.com/docs/third-party/claude-desktop/gateway)。企业托管配置可能具有更高优先级，由管理员调整后再重启应用。

## 验证与恢复

启动后选择已配置的模型，发送“只回复 Hello，不执行工具”。到[使用记录](https://www.gkotta.bid/usage)检查密钥、模型与消耗，再测试需要的其他功能。

需要回到官方账号模式时，在 CC Switch 选择 **Claude Desktop Official**，点击切换，完全退出并重新打开 Desktop，按官方账号流程登录。不要将 Gkotta 密钥填入官方账号认证入口。

## 常见问题

| 问题 | 处理方式 |
| --- | --- |
| 没有 Claude Desktop 配置入口 | 核对 CC Switch v4.0.4+，确认侧栏应用未隐藏 |
| 切换后界面没有变化 | 完全退出应用再重启，只关闭窗口可能仍保留后台进程 |
| 仍要求官方账号或升级 | 确认当前是 3P profile 而非官方模式，核对桌面版本和组织策略 |
| 模型列表为空或 ID 被拒绝 | 确认直连使用可识别的 Claude 角色 ID，必要时使用映射 |
| 映射模式请求失败 | 确认 CC Switch 运行、本地路由正常、映射有真实模型 ID |
| 401 / 403 / 429 | 检查密钥、分组、IP 限制及额度，见[常见问题](/troubleshooting) |
| API 正常但某个桌面工具失败 | 分别检查客户端工具权限、系统依赖与目标模型能力 |

3P 接入成功表示模型请求可用，Desktop 的全部工具、沙箱和组织功能仍需满足它们各自的使用要求。
