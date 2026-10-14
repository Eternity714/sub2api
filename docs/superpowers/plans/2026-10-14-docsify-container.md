# Docsify Container Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task. User has authorized execution and production rollout; do not ask again for execution mode.

**Goal:** 独立容器承接全部 `/docs`，外置 Markdown 修改后刷新生效，并上线生产。

**Architecture:** Docsify 在浏览器解析原始 Markdown，Nginx 容器提供文档和本地固定资源。生产主机 Nginx 用独立文档规则分流，主应用蓝绿权重保持由现有灰度脚本管理。

**Tech Stack:** Docsify 4.13.1、Nginx、Docker/Podman Compose、现有前端与灰度脚本。

**Spec:** `docs/superpowers/specs/2026-10-14-docsify-container-design.md`

## Global Constraints

- 提交日志使用中文。
- 所有本地测试、前端类型检查与回归测试通过 `deploy/docker-compose.test.yml` 的容器执行。
- 生产每次操作前运行 `gray-status.sh`；只使用服务器灰度脚本和不可变 GHCR `sha-<commit>` 镜像。
- 不初始化灰度、不启动旧 sub2api、不 down、不删除卷、不恢复数据库；回滚只切流并保留现场。

## Task 1: 行为回归与容器验证

**Files:** Create `deploy/tests/docsify-http-test.py`, `deploy/tests/docsify-browser-test.mjs`; Modify `deploy/docker-compose.test.yml`.

**Interfaces:** HTTP 环境变量 `DOCSIFY_TEST_URL` 指向真实文档容器；内容目录 `/workspace/frontend/docs`。检查真实网络行为，不复刻配置。

- [x] 先写 HTTP 回归，验证入口重定向、文章/资源 404、维护文件隔离、旧业务跳转、缓存和挂载更新。
- [x] 在现有服务或无 Docsify 的基线上运行并记录预期失败。
- [x] 加入浏览器容器服务，检查深链接、侧栏、搜索、Mermaid和正文更新，不在宿主机执行测试。
- [x] 实现后通过 Compose 执行 HTTP 与浏览器回归。

## Task 2: 文档内容迁移

**Files:** Modify `frontend/docs/**/*.md`; Create `_sidebar.md`, `_navbar.md`, `_404.md`; Remove obsolete `.vitepress` config/theme.

**Interfaces:** 正文内链显式 `/docs/<slug>`，首页保留 `index.md`，目录首页保留 `scenarios/index.md`；运行时配套 alias 将目录请求映射到 index.md。

- [x] 保留已有教程文字和示例，移除 VitePress frontmatter，首页卡片改为普通 Markdown 快速入口。
- [x] 从现有 config.ts 导航迁移 `_sidebar.md` 和 `_navbar.md`，覆盖所有原导航文章。
- [x] 提示框改为普通引用；Mermaid 代码块由固定运行时资源渲染。
- [x] 更新维护 README，说明 Compose 启动、挂载更新与新发布流程。

## Task 3: Docsify 运行时与主应用解耦

**Files:** Create `deploy/docsify/Dockerfile`, `deploy/docsify/nginx.conf`, `deploy/docsify/runtime/{index.html,config.js,style.css,assets/}`, `deploy/docker-compose.docsify.yml`; Modify `frontend/package.json`, `frontend/pnpm-lock.yaml`, `Dockerfile`, `deploy/docker-compose.docs-preview.yml`, `.dockerignore`.

- [x] 固定版本资源下载到 runtime/assets，记录来源、许可证与 SHA256。
- [x] 实现 history 路由、根目录/嵌套目录映射、实时缓存策略和真实404。
- [x] 提供可单独启动的 Compose 文档服务以及同域入口预览。
- [x] 从主应用 build 删除 docs:build 和 VitePress 依赖、镜像 docs-default 复制。
- [x] 运行 HTTP、浏览器与前端关键回归。

## Task 4: 生产灰度脚本与镜像发布

**Files:** Create `deploy/gray-deploy-docsify.sh`, `deploy/gray-set-docsify-traffic.sh`, `deploy/gray-promote-docsify.sh`, `deploy/gray-docsify-common.sh`, deployment tests and GHCR workflow.

**Interfaces:** 固定镜像仓库 `ghcr.io/eternity714/sub2api-docsify`；宿主机文档端口18083；文档状态保存在 `/opt/sub2api/gray/docsify`；只通过独立 include 修改 `/docs`，主应用 state.env/slots.env 不改。

- [x] 脚本先校验现有 gray-status，部署无流量容器并提取公开 Markdown 到宿主机内容目录。
- [x] 文档专用分流支持0..100，校验健康与Nginx，失败恢复原配置；回滚保留容器和现场。
- [x] 新增发布固定 sha 标签镜像的 workflow；生产脚本支持重复执行并拒绝覆盖已承载流量的候选。
- [x] 通过 Compose 容器运行脚本保护回归，完成独立代码审查。

## Task 5: 生产上线与验收

- [x] 提交中文日志，推送专用分支，发布不可变 Docsify 镜像。
- [x] 安装测试通过的脚本，运行gray-status，部署Docsify到无公网流量端口。
- [x] 浏览器人工检查首页、直达文章、搜索、图表与业务跳转；检查nginx -t、健康、日志。
- [x] 通过文档灰度脚本按10%、50%、100%逐步分流，检查每阶段，观察后晋升。
- [x] 核对主应用槽位、文档镜像与健康，向用户给出URL和Markdown更新位置。
