# Docsify 独立容器生产验收记录

日期：2026-10-14（Asia/Shanghai）

状态：**已完成生产上线、341 秒观察与 Docsify 晋升。**

生产通过独立文档灰度脚本按 10%→50%→100% 放量，观察通过后晋升 blue。所有 `/docs` 路由现由独立 Docsify 容器提供，主应用槽位和权重保持原值。

## 发布身份

| 项目 | 已核实值 |
| --- | --- |
| 仓库 | `Eternity714/sub2api` |
| 实现分支 | `codex/docsify-container` |
| Docsify 镜像源码提交 | `411025133ebf4728eba0700bc4af959c128c1df9` |
| 已发布不可变镜像 | `ghcr.io/eternity714/sub2api-docsify:sha-411025133ebf4728eba0700bc4af959c128c1df9` |
| PR | [新增独立Docsify容器并支持文档实时更新 #3](https://github.com/Eternity714/sub2api/pull/3)，草稿 |
| 生产文档入口 | [文档中心](https://www.gkotta.bid/docs/) |
| Docsify 稳定 / 候选 | blue / green，候选比例 0%，cookie epoch 5；blue 端口 `127.0.0.1:18083` |
| 稳定容器启动时刻 | `2026-10-14 11:04:18.661038399 +0800 CST` |

首次迁移期间，Markdown 与 `/docs/_assets/` 由候选提供；旧文档 `/docs/assets/` 保留主应用。晋升后，包括旧资源路径在内的所有 `/docs` 均交给稳定 Docsify，未知资源返回真实 404。历史批量生图路径由文档容器跳转到主应用 `/batch-image`。

## 已完成的实现

- 将 42 篇教程迁移为普通 Markdown，保留首页 `index.md`、场景首页 `scenarios/index.md`，维护 `_sidebar.md`、`_navbar.md` 和 `_404.md`。
- 独立容器以 Docsify 4.13.1 在浏览器解析正文。搜索、代码复制、Mermaid 11.12.0 和 Prism 1.30.0 等资源固定版本并放入镜像，配有许可证及 `SHA256SUMS`，运行时无需第三方 CDN。
- 镜像运行资源位于 `/opt/docsify/runtime`，正文只读挂载至 `/usr/share/nginx/html/docs`。以 UID/GID 101 运行 Nginx，监听 8080，根文件系统只读，移除全部 capabilities，并使用 `/tmp` tmpfs；`/health` 返回 `ok`。
- 主应用前端构建已移除 VitePress 文档编译，正文维护与主应用构建解耦。修改挂载的 Markdown、导航或图片后刷新即可生效；入口 HTML、Markdown 与导航使用不持久缓存的策略。
- 代理保留完整 `/docs` URI 与查询参数。`/docs` 重定向到 `/docs/`；历史 `/docs/batch-image` 及尾斜杠形式重定向到 `/batch-image`，保留查询参数。不存在的文章、Markdown、资源、维护 README 与隐藏目录返回真实 404。
- Docsify 使用独立 blue/green 槽位，端口分别为 18083/18084，只绑定回环地址。状态保存在 `/opt/sub2api/gray/docsify/state.env`，与主应用灰度分离。
- 文档 policy 位于 `/etc/nginx/conf.d/sub2api-docsify-policy.conf`，通过已有 `conf.d/*.conf` 加载；文档 locations 位于 `/etc/nginx/snippets/sub2api-docsify-locations.conf`，只在 `www.gkotta.bid` 的 HTTPS 业务 server 安装。保留已有 HSTS、安全响应头及发布标记。
- 放量更新 `docsify_cookie_epoch`，避免旧比例的会话继续固定到过期分流结果。首次迁移晋升后，所有 `/docs` 路由交给 Docsify，缺失的旧资源返回 404。
- 配置安装与 Nginx reload 具备备份、校验和恢复流程；切流后状态保存失败时先回到稳定入口 0%，保留候选容器、内容、日志及备份。晋升要求 100% 流量并完成默认至少 300 秒的观察。
- Podman 启动时显式设置健康检查，避免镜像运输转换丢失 Docker Healthcheck 元数据：`wget -q -T 4 -O /dev/null http://127.0.0.1:8080/health`，interval 10s、timeout 5s、start-period 5s、retries 3。

## 本地验证结果

全部本地测试通过 Docker Compose 容器执行，未在宿主机直接运行测试或前端类型检查。

| 验证范围 | 已确认结果 |
| --- | --- |
| 独立文档 HTTP 回归 | 8 项通过，覆盖路由、缓存、404、维护文件隔离与挂载正文更新 |
| 同域预览 HTTP 回归 | 10 项通过 |
| 浏览器回归 | 首页与导航、深链接刷新、正文搜索、真实剪贴板复制、Mermaid SVG、移动布局、挂载正文刷新、运行资源同域请求通过 |
| 灰度部署保护回归 | 20/20 通过 |
| 真实 Nginx / HTTP 代理场景 | 5/5 通过：首次迁移 0%、50%、100%，晋升后 0%，健康异常回切 |
| 前端 | 类型检查、14 项相关回归及完整生产构建通过 |
| 工作流静态检查 | `publish-docsify-image.yml` 与 `backend-ci.yml` 的 actionlint 通过 |
| 独立代码审查 | 未发现新增 P0/P1/P2 问题 |
| 补丁检查 | `git diff --check` 通过 |

真实 Nginx 回归覆盖完整 URI/查询参数、分流 Cookie、HSTS、Markdown/运行资源路由、旧资源去向与真实 404。验证命令为：

```bash
docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test bash -c 'set -euo pipefail; apt-get update -qq; apt-get install -y --no-install-recommends nginx-light >/dev/null; DOCSIFY_REAL_NGINX=/usr/sbin/nginx bash deploy/tests/docsify-gray-test.sh'
```

显式 Podman 健康检查修复先加入回归断言，测试按预期失败并报告 `Container health command depends on image metadata`；实现后通过 Compose 的 `bash -n` 与全部 20 项部署保护测试。

## 镜像发布及 Actions 限制

工作流代码已准备并通过静态检查，但 GitHub 平台未执行本仓库的新推送工作流。对已注册工作流的实际手动触发返回：

```text
HTTP 422: Actions has been disabled for this repository
```

因此，本次采用手动固定 SHA 发布：由本机 Docker 构建，再使用生产服务器已有的 Podman GHCR 登录发布上述不可变镜像。没有输出或修改已有注册表凭据。本机已有 `gh` 登录用于 Docker 登录后，推送曾因所需权限范围不足而失败；最终发布使用服务器现有认证成功。

Actions 恢复前，运行时或镜像更新需手动发布不可变 `sha-<完整提交SHA>` 标签。正文、导航和图片的挂载更新不受此限制，不需要触发 Actions。

部署预检中已处理两项实际问题：镜像内容种子残留空 `.vitepress` 目录，现已在 Dockerfile 中排除隐藏及废弃工具路径；Docker/OCI 到 Podman 的转换丢失镜像 Healthcheck，现已改为运行时显式健康检查。前者在预检阶段拒绝继续，后者由健康守卫阻止安装代理及放量；相关零流量候选、内容、日志和备份保留。

## 主应用基线及隔离证据

以下值来自本次生产核对，主应用仍保持稳定 green、候选 blue、0% 候选比例，两个容器均 healthy。

| 项目 | 基线 |
| --- | --- |
| 主应用稳定 green | `sha-53164e851a01e1d7356e357efe62f99c86bcddd3` |
| 主应用候选 blue | `sha-671568813bed315a88a8c76edae9b7ecc780e676` |
| 主应用候选比例 | `0` |
| 主应用 `state.env` SHA-256 | `e46ed023f47403d054ea38267995d178f4d2862c1f6114d1477fcb7af563f819` |
| 主应用 `slots.env` SHA-256 | `5fb7225682a691a6f6fc83538160bcf59a5528bb6c5316d5afe474a932f0ed26` |

晋升后的生产核对中，上述两个文件哈希与基线完全一致，主应用槽位和候选比例未变，两槽位均 healthy。主应用 policy 未采集哈希证据，本记录不作哈希一致性结论。

## 生产验收进度

| 阶段 | 当前状态 | 已确认结果 / 待回填证据 |
| --- | --- | --- |
| 不可变镜像发布 | 完成 | 镜像标签见发布身份 |
| 0% 候选部署与验收 | 完成 | 直连 HTTP 执行 9 项，2 项因直连无主应用及边缘发布头而合理跳过，其余通过；浏览器桌面前六项先通过，挂载更新、移动布局及网络边界修复后通过，截图人工检查通过 |
| 10% 文档放量 | 完成 | `2026-10-14 11:30:21+08`，epoch 2 |
| 50% 文档放量 | 完成 | `2026-10-14 11:31:03+08`，epoch 3 |
| 100% 文档放量 | 完成 | `2026-10-14 11:31:46+08`，epoch 4；`docsify_full_traffic_at=1791948705`（11:31:45） |
| 100% 公网验收 | 完成 | 浏览器 10 项通过；最终公网 HTTP 9 项通过，耗时 36.233 秒；截图人工检查正常 |
| 100% 观察至少 300 秒 | 完成 | 11:31:45 至 11:37:26，341 秒；持续 healthy，最近 5 分钟文档 5xx 为 0，主站健康与登录正常 |
| 文档槽位晋升 | 完成 | `docsify_promoted_at=1791949046`；稳定 blue / 候选 green / 候选 0%，epoch 5，镜像保持上述 SHA |
| 晋升后最终复核 | 完成 | 公网 HTTP 10 项通过，耗时 50.082 秒；旧 epoch、stable 与 candidate Cookie 均进入稳定 Docsify，旧资源真实 404；Nginx、容器健康和主应用配置哈希均通过 |

每个放量阶段均确认 stable/candidate Cookie 对应的 HTML 路由、主应用 `/health` 与登录页、`nginx -t` 和容器 healthy；候选最近 3–5 分钟的 5xx 计数均为 0。11:35:46 及晋升后再次核对仍 healthy、最近 5 分钟 5xx 为 0。

100% 公网浏览器包含新增的 async false/pending 负例，实际将挂载测试正文 `Initial` 改为 `Revised` 后刷新，确认旧文本消失。容器 `StartedAt` 始终为上述 11:04:18.661038399，验证更新未重启。人工截图检查发现首页代码周围 Docsify 未解析的粗体星号，已同步修正源码 `index.md` 并 scp 到候选 content；100% 截图确认生效，未重建镜像或重启容器。

公网 HTTP 和阶段验证脚本位于本结果目录的 `2026-10-14-docsify-production-http.py` 和 `docsify-production-stage.py`。完整浏览器使用 `deploy/tests/docsify-browser-test.mjs` 的生产副本，额外保存截图 `docsify-production.png`。生产 HTTP 与浏览器验证均通过 Compose 容器执行。

自建浏览器测试文章已删除并确认返回 404；本次 SSH 隧道已关闭。生产容器、文档内容、灰度备份及旧的零流量现场保留。

## 验收工具调试与公网边界

Playwright 1.58.2 的 `waitForFunction` 在本次验证中未等待返回的 Promise：异步 false 立即 resolved，导致挂载测试在修改完成前提前 reload。现改为 Node 轮询真正 await 布尔值，并限制 deadline；async false/pending 负例及 Compose 验证通过，最终挂载刷新验证通过。

原 Python User-Agent 被 Cloudflare 1010 拒绝，已调整公网检查请求的 User-Agent。原始严格 HTTP 脚本还检测到 Cloudflare 注入的可选 `static.cloudflareinsights.com` beacon；实际浏览器的 CSP 阻止此外部请求，该 beacon 不是 Docsify 运行依赖。公网 wrapper 仅识别这个指定 beacon，并新增 CSP `self` 断言；容器原测试继续严格检查外部依赖。调整后的最终公网 HTTP 9 项通过。

每次生产操作前执行主应用 `gray-status.sh`，并按文档 `gray-docsify-status.sh` 的实际状态选择操作；依次使用服务器现有 `gray-deploy-docsify.sh`、`gray-set-docsify-traffic.sh` 和 `gray-promote-docsify.sh`。候选晋升前如有健康、Nginx 或主站异常，应立即通过 `gray-set-docsify-traffic.sh 0` 回切文档入口并保留现场，再核对两套灰度状态。

## Markdown 维护位置及更新方法

本次 **blue 稳定槽位** 的正文目录为：

```text
/opt/sub2api/gray/docsify/releases/blue-sha-411025133ebf4728eba0700bc4af959c128c1df9/content
```

后续维护前运行 `/opt/sub2api/scripts/gray-docsify-status.sh`，以当次稳定槽位的 `docsify_<slot>_content` 为维护路径；后续发布晋升会改变目录。

保持挂载根目录及其 inode 不变，在其中更新单个 Markdown、导航或图片文件，然后刷新浏览器；无需构建或重启容器。不要替换整个目录或将根目录改为符号链接。同步修改仓库 `frontend/docs/`，使下一次镜像发布保留正文变更；新增、重命名或删除页面时同步更新 `_sidebar.md` 与正文链接。不要上传维护 `README.md`、隐藏目录或仓库其他文件。

以下为后续状态确认后的正文更新示例：

```powershell
ssh tenxunyun.guigu '/opt/sub2api/scripts/gray-status.sh'
ssh tenxunyun.guigu '/opt/sub2api/scripts/gray-docsify-status.sh'
# 仅在状态确认 blue 已是稳定槽位且路径一致后，使用下面的路径。
$docsifyContent = '/opt/sub2api/gray/docsify/releases/blue-sha-411025133ebf4728eba0700bc4af959c128c1df9/content'
scp 'C:/Users/Administrator/.codex/worktrees/docsify-container/sub2api/frontend/docs/quick-start.md' "tenxunyun.guigu:$docsifyContent/quick-start.md"
```

完整维护说明见 `frontend/docs/README.md`；设计与执行计划分别见 `docs/superpowers/specs/2026-10-14-docsify-container-design.md` 和 `docs/superpowers/plans/2026-10-14-docsify-container.md`。
