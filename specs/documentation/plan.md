# 实现方案

需求来源见 [prd.md](prd.md)。

## 选型与集成

采用参考站相同的 VitePress 1.6.4，固定工具版本。每篇正文保存为 `frontend/docs/*.md`，共享默认主题；目录、锚点、代码复制、亮暗主题、搜索和移动阅读由工具提供。文档首页使用 home/hero/features 配置，不逐页编写 HTML。

配置 `base: '/docs/'` 和 `cleanUrls: true`，原前端 build 最后运行文档 build，产物写入 `backend/internal/web/dist/docs`。现有 Dockerfile 把整个 dist 嵌入同一个 Go 二进制，无需独立 DNS 或容器。

## 数据与正文

十篇正文覆盖入门、密钥、Claude Code、Codex、常用客户端、三种 API、计费和排错。协议、路径、鉴权与额度按现有源码核实。内部链接写成 `/<slug>`，由工具加 base；主应用链接使用完整主站 URL，避免产生 `/docs/keys` 等错误地址。样例只使用占位密钥和模型，不实际调用生产 API。

## 界面

`.vitepress/config.ts` 统一配置导航、全文搜索和中文界面；共享 CSS 调整品牌色与圆角。入口使用原生 `<a href="/docs">` 完整跳转至文档应用。

后端两个静态中间件共享文档处理器，支持根路径、clean URL、资源缓存和文档 404。原 `/docs/batch-image` 精确别名继续走主 SPA 与原鉴权。文档 HTML 按请求为脚本注入 CSP nonce，不放宽 CSP；含 nonce 的 HTML 不可缓存，内容 hash 资源长期缓存。

## 验证与交付

所有测试、lint、类型检查和构建均通过 `deploy/docker-compose.test.yml` 容器运行。验证包括入口行为回归、Go 静态文档服务测试和 VitePress 死链接构建检查，并做独立内容与代码审查。交付分支 `codex/documentation`，不合并或发布。

本机 Docker Desktop 启动故障单独记录，不将未执行的检查声明为通过。CLAUDE.md 引用的 `docs/RELEASE_DEPLOYMENT.md` 在当前仓库与 worktree 都不存在，本次不执行任何生产操作。
