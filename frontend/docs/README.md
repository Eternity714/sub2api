## 内容维护约定

本目录存放公开文档正文，不用于记录账号凭证，也不承担用户教程之外的运行时逻辑。`README.md` 是维护说明，不应进入用户目录、搜索索引或正文导入列表。

- 文档工具为与参考站相同的 **VitePress 1.6.4**，版本在 `frontend/package.json` 中固定。每篇教程只维护 Markdown，不编写独立 HTML 页面。
- 文章使用 H1 标题、H2 章节和 H3 子章节，VitePress 自动生成页内目录。导航和站点设置统一维护在 `.vitepress/config.ts`。
- 内部教程链接写为 `/<slug>`（例如 `/api-keys`），VitePress 自动加上 `/docs/` base。不要重复写 `/docs/<slug>`，否则会变成 `/docs/docs/<slug>`。
- 主站账户功能链接使用完整 `https://www.gkotta.bid/<path>`，避免被文档工具误加 `/docs/` 前缀。
- 文档只描述已核实的产品行为。路由存在不代表所有分组、模型和上游都支持该能力。
- 样例统一使用 `sk-YOUR_API_KEY` 和 `YOUR_MODEL_ID`，不得提交真实密钥或未经验证的固定模型名称。
- 不使用未经确认的退款、价格、活动、SLA 或客服信息。涉及价格、可用模型、功能开关和订阅状态时引导用户查看当前控制台。
- 请求样例不在维护过程中向生产环境实际发送，不以真实调用验证文章。

## 本批内容的事实来源

核查时以仓库源码为准：

| 内容 | 源码入口 |
| --- | --- |
| OpenAI、Anthropic 和 Gemini 接口路径 | `backend/internal/server/routes/gateway.go` |
| 常规 API 认证、额度、余额、有效期与分组权限 | `backend/internal/server/middleware/api_key_auth.go` |
| Gemini 鉴权头优先级 | `backend/internal/server/middleware/api_key_auth_google.go` |
| Gemini 原生方法与 SSE | `backend/internal/handler/gemini_v1beta_handler.go` |
| Claude Code、Codex 与 Gemini CLI 配置 | `frontend/src/components/keys/UseKeyModal.vue` |
| 密钥额度、IP 限制和到期时间 | `frontend/src/views/user/KeysView.vue` |
| 余额充值、订阅入口及功能开关 | `frontend/src/views/user/PaymentView.vue` |
| 分组订阅与日/周/月额度 | `frontend/src/views/user/SubscriptionsView.vue` |
| Token、缓存用量和费用明细 | `frontend/src/views/user/UsageView.vue` |
| 用户页面地址与访问要求 | `frontend/src/router/index.ts` |

Claude Code 地址不含 `/v1`，其配置使用 `ANTHROPIC_AUTH_TOKEN`。Codex 示例使用 `/v1`、`wire_api = "responses"` 和配套的 `requires_openai_auth = true` / `auth.json` 认证方式。Gemini 原生 HTTP 示例使用 `x-goog-api-key` 与 `/v1beta`，CLI 使用根地址。

本批内容不提供 `/v1/usage` 余额接口教程；账户和用量查询引导至控制台页面。错误说明区分密钥配额、账户余额和订阅周期额度，避免将所有 `429` 写成余额不足。

## 参考站证据

[参考文档中心](https://docs.fastaitoken.com/docs/)采用 VitePress v1.6.4，具有三栏阅读布局、快速入口卡片、工具安装配置、API 与计费排错目录。版式细节见[公开样式文件](https://docs.fastaitoken.com/assets/style.DphkpMR2.css)。

只借鉴信息架构和阅读方式。文章重新按 Gkotta 源码编写，不复制参考站的模型清单、充值活动、退款、SLA、客服及其他运营承诺。

## 更新流程

修改接口或工具配置时同步核对相关教程、页面元数据和导航配置。外部安装命令以文章链接的官方资料为准，避免固化过时的依赖版本或安装界面。

本地文档开发命令为 `pnpm --dir frontend docs:dev`，静态预览为 `pnpm --dir frontend docs:preview`。文档开发服务器仅用于单独编写页面，生产入口始终是主站 `/docs`。

`pnpm --dir frontend build` 先构建主应用，再运行 `docs:build` 将 VitePress 产物写入 `backend/internal/web/dist/docs`。现有 Dockerfile 会把整个 dist 嵌入同一个 Go 二进制。构建顺序不能反过来，否则主应用构建清空 dist 时会删除文档产物。

后端同时支持 `/docs`、clean URL 文章直达、资源缓存和文档 404，并为 HTML 脚本注入 CSP nonce。原 `/docs/batch-image` 属于主应用登录功能，保留原入口，禁止复用该文档 slug。

文章死链接由 VitePress 构建检查，入口与后端文档深链接有回归测试。所有测试、类型检查与构建均按根目录 `AGENTS.md` 的 Docker Compose 要求执行。

## 本地人工预览

在仓库根目录，将 `deploy/docs-preview.env.example` 复制为 `deploy/docs-preview.env`，填写三个必填的独立随机凭证。真实 env 文件已被 Git 忽略。随后运行：

```powershell
docker compose --env-file deploy/docs-preview.env -p sub2api-docs-preview -f deploy/docker-compose.docs-preview.yml build --builder default
docker compose --env-file deploy/docs-preview.env -p sub2api-docs-preview -f deploy/docker-compose.docs-preview.yml up -d
docker compose --env-file deploy/docs-preview.env -p sub2api-docs-preview -f deploy/docker-compose.docs-preview.yml ps
```

应用健康后，打开 [本地主页](http://127.0.0.1:18082/home)，点击文档入口，或直接访问 [文档首页](http://127.0.0.1:18082/docs/) 和 [快速开始](http://127.0.0.1:18082/docs/quick-start)。配置不同的 `DOCS_PREVIEW_PORT` 时替换地址中的端口。预览运行真实 Go 嵌入服务，可检查 CSP、文章直达、搜索、代码复制、主题和移动端布局。

该 Compose 使用专属项目网络和数据卷，首次自动创建本地管理员 `docs-preview@example.invalid`，密码为 env 中设置的值。数据库与 Redis 不暴露宿主机端口，批量生图关闭。文档中的主站、控制台按钮和业务链接仍指向生产站；本地入口测试从上述 `/home` 地址开始。

构建命令显式选择 Docker 默认构建器，避免本机残留的构建器配置影响预览。修改文档后重新执行构建和启动命令即可更新镜像。停止预览并保留数据：

```powershell
docker compose --env-file deploy/docs-preview.env -p sub2api-docs-preview -f deploy/docker-compose.docs-preview.yml stop
```
