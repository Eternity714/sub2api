# Docsify 文档维护说明

## 内容维护约定

本目录存放公开文档正文，不用于记录账号凭证，也不承担用户教程之外的运行时逻辑。`README.md` 是维护说明，不应进入用户目录、搜索索引或正文导入列表；文档服务器必须拒绝访问该文件和隐藏目录。

- 文档使用 **Docsify 4.13.1** 在浏览器解析 Markdown。固定版本的 Docsify、搜索、代码复制和 Mermaid 运行资源保存在独立文档镜像中，不依赖第三方 CDN。每篇教程只维护普通 Markdown，不编写独立 HTML 页面。
- 文章使用 H1 标题、H2 章节和 H3 子章节。侧栏维护在 `_sidebar.md`，顶部导航维护在 `_navbar.md`，未找到文档的提示维护在 `_404.md`。首页保留 `index.md`，使用场景目录首页保留 `scenarios/index.md`。
- 内部教程链接显式写为 `/docs/<slug>`（例如 `/docs/api-keys`），文档首页写为 `/docs/`，目录首页写为 `/docs/scenarios/`。运行时使用 history 路由并将目录首页映射到对应的 `index.md`。
- 主站账户功能链接使用完整 `https://www.gkotta.bid/<path>`，避免被文档工具误加 `/docs/` 前缀。
- 提示内容使用普通 Markdown 引用；流程图使用 `mermaid` 代码块。文章摘要写在 H1 之后，不使用 VitePress frontmatter、容器语法或 Vue 组件。
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

[参考文档中心](https://docs.fastaitoken.com/docs/)采用 VitePress v1.6.4，具有三栏阅读布局、快速入口卡片、工具安装配置、API 与计费排错目录。版式细节见[公开样式文件](https://docs.fastaitoken.com/assets/style.DphkpMR2.css)。本站教程已迁移为普通 Markdown，由独立 Docsify 容器提供。

逐项核对参考站的 49 个导航入口，迁移 Gkotta 已支持的主题，并合并同一工具的安装与使用文章。具体对应关系、排除原因和官方依据见 `specs/documentation/reference-coverage.md`。兼容第三方客户端包含完整配置、验证和排错步骤；图片、视频与国内模型按实际分组及源码条件编写。

参考站存在部分过期配置和专属产品依赖，正文据当前工具官方资料与 Gkotta 源码重写。未迁移 Tisoz 专属服务、AI 视频工作台、人物素材库，以及尚无经核实 Gkotta 接入方式的 ComfyUI 节点。飞书教程是用户自建集成流程，Paper2Any 明确文本模型入口与外部处理依赖。模型清单、充值活动、退款、SLA、客服及其他运营承诺不照搬。

## 更新流程

修改接口或工具配置时同步核对相关教程、摘要和导航。外部安装命令以文章链接的官方资料为准，避免固化过时的依赖版本或安装界面。新增、重命名或删除文章时，同时调整 `_sidebar.md` 和引用该文章的正文链接。

文档图片与 Markdown 一起放在本目录，图片链接使用 `/docs/` 前缀；站点标志位于 `logo.svg`，公开地址为 `/docs/logo.svg`。维护 README 和隐藏目录不属于公开内容，不要将它们加入导航或导入清单。

## 本地编写与预览

在仓库根目录启动独立文档容器：

```powershell
docker compose -f deploy/docker-compose.docsify.yml build --builder default
docker compose -f deploy/docker-compose.docsify.yml up -d --no-build --wait
```

打开[本地文档首页](http://127.0.0.1:18083/docs/)，也可直接打开[快速开始](http://127.0.0.1:18083/docs/quick-start)或[使用场景](http://127.0.0.1:18083/docs/scenarios/)。宿主机默认只绑定 `127.0.0.1:18083`，容器内部使用 8080。

Compose 将 `frontend/docs/` 中的原始 Markdown、导航和图片只读挂载到容器的 `/usr/share/nginx/html/docs`。修改文章并保存后，刷新浏览器即可看到新正文，无需构建、重建镜像或重启容器。HTML、Markdown 和导航不持久缓存。

Docsify 的入口页面和固定静态运行资源位于镜像中的 `/opt/docsify/runtime`，独立于内容挂载目录。修改这些运行资源或容器配置时，才需要重新构建并启动文档容器。主应用构建不再运行 `docs:build`，也不再将文档编译进 Go 二进制。

同域完整预览使用 `deploy/docker-compose.docs-preview.yml` 的入口代理；文档内容仍来自 `frontend/docs/`。预览的主站、控制台按钮和业务链接保持指向文中明确给出的地址。

所有本地测试、HTTP 与浏览器回归、前端类型检查均按根目录 `AGENTS.md` 要求通过 Docker Compose 容器执行，不在宿主机直接运行测试命令。

## 生产内容与发布

生产每个文档槽位有独立内容目录 **`/opt/sub2api/gray/docsify/releases/<slot>-sha-<commit>/content`**，只读挂载到对应 Docsify 容器。先运行 `gray-docsify-status.sh`，以输出的稳定槽位 `docsify_<slot>_content` 为准。已发布后更新文章、导航或图片，直接修改这个目录中的对应文件并刷新即可，不需要重新编译主应用或重建文档镜像。保持挂载根目录不变，在目录内更新文件；不要替换整个挂载目录或切换为符号链接。同步修改仓库 `frontend/docs/`，保证后续发布包含已维护的正文。

文档镜像仓库为 `ghcr.io/eternity714/sub2api-docsify`，发布时只使用不可变 `sha-<commit>` 标签，禁止使用 `latest`。镜像和脚本的更新与正文维护分开进行。公开内容同步时排除 `README.md`、隐藏目录及仓库其它文件。

生产操作通过 SSH 别名 `tenxunyun.guigu` 访问 `/opt/sub2api`。每次操作前先运行服务器中已安装的 `gray-status.sh`，核对主应用稳定、候选槽位及健康状态；不得重新初始化灰度或启动旧 `sub2api` 容器。

使用 `/opt/sub2api/scripts` 中的文档灰度脚本依次完成部署、放量和晋升：`gray-deploy-docsify.sh` 先部署到无流量的独立端口，完成容器健康、`/health`、文章、搜索、代码复制、Mermaid、移动布局及 Nginx 配置检查后，再由 `gray-set-docsify-traffic.sh` 逐步扩大文档流量。达到 100% 并完成观察后，由 `gray-promote-docsify.sh` 确认晋升。文档流量通过独立 Nginx include 管理，不修改主应用蓝绿权重。

将同一发布版本的 `gray-docsify-common.sh`、`gray-docsify-nginx-install.py`、`gray-docsify-status.sh`、`gray-deploy-docsify.sh`、`gray-set-docsify-traffic.sh` 和 `gray-promote-docsify.sh` 安装在已有的 `/opt/sub2api/scripts` 目录，保留已有 `gray-common.sh` 和主应用灰度脚本。服务器需要 `python3`、`flock`、Podman、curl 和免交互 sudo。`Publish Docsify GHCR Image` workflow 在 `main` 或 `codex/**` 分支的相关变更推送后验证并发布 `sha-<完整提交SHA>` 镜像；部署时把下面的标签占位符替换为 workflow 输出的实际标签。

2026-10-14 首次发布时，GitHub 平台拒绝运行本仓库 Actions，手动触发返回 `HTTP 422: Actions has been disabled for this repository`。本次通过 Docker 构建并由生产服务器现有 Podman GHCR 登录发布固定 SHA 镜像；自动工作流恢复前，运行时或镜像更新需手动发布。已挂载的 Markdown、导航和图片更新不受此限制。

```bash
cd /opt/sub2api
./scripts/gray-status.sh
./scripts/gray-deploy-docsify.sh 'sha-<完整提交SHA>'
./scripts/gray-docsify-status.sh
# 首次候选是 127.0.0.1:18083，下一候选使用 18084；按状态输出核对。
# 先完成候选直连健康检查与浏览器人工验收，再逐步放量。
./scripts/gray-set-docsify-traffic.sh 10
./scripts/gray-docsify-status.sh
# 检查 Nginx、容器健康、最近错误日志和主站功能后继续。
./scripts/gray-set-docsify-traffic.sh 50
./scripts/gray-docsify-status.sh
./scripts/gray-set-docsify-traffic.sh 100
./scripts/gray-docsify-status.sh
# 在 100% 连续观察至少 300 秒，确认文档和主站正常后晋升。
./scripts/gray-promote-docsify.sh
./scripts/gray-docsify-status.sh
./scripts/gray-status.sh
```

晋升脚本默认要求 100% 流量后至少观察 300 秒，起始时间以状态输出的 `docsify_full_traffic_at`（Unix 秒）为准。放量会更新 `docsify_cookie_epoch`，让此前固定到旧比例的会话按新比例重新分流。首次迁移期间，Markdown 和 `/docs/_assets/` 始终由候选 Docsify 提供，旧站 `/docs/assets/` 仍走主应用；晋升后所有 `/docs` 地址都归 Docsify，缺失的旧资源返回真实 404。

主机规则位于独立的 `/etc/nginx/conf.d/sub2api-docsify-policy.conf` 和 `/etc/nginx/snippets/sub2api-docsify-locations.conf`。policy 由 Nginx 的 `conf.d/*.conf` 自动加载，不要在主站配置中重复 include；locations 只安装到 `www.gkotta.bid` 的 HTTPS 业务 server。配置变更前的备份保存在 `/opt/sub2api/gray/docsify/backups`，主应用 `state.env`、`slots.env` 和蓝绿 policy 不会被文档脚本改写。

`/docs` 转到 `/docs/`，历史 `/docs/batch-image` 及其尾斜杠形式转到主应用 `/batch-image` 并保留查询参数；不要复用 `batch-image` 文档 slug。入口代理保留 `/docs` URI，不剥离前缀；未知文章、Markdown、资源和维护文件返回 404，不回落到主应用 SPA。

发现候选文档不健康、Nginx 检查失败或主站异常时，先通过文档灰度脚本将新文档请求切回稳定入口，并保留候选容器、镜像、日志和备份现场。回滚仅切流，不销毁容器，不删除数据卷，不恢复数据库；后续处理按根目录 `AGENTS.md` 的要求执行。

候选晋升前的人工回切命令为 `./scripts/gray-set-docsify-traffic.sh 0`，随后运行 `./scripts/gray-docsify-status.sh` 和 `./scripts/gray-status.sh` 核对文档与主应用状态。脚本发现活动候选不健康或切流状态保存失败时会先执行同样的入口回切；如回切或状态保存仍失败，会明确报告 `CRITICAL`，应检查保留的 Nginx 备份与状态后再操作。
