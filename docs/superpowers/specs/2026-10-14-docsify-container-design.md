# 独立 Docsify 文档容器

## 用户目标

在当前项目增加独立 Docsify 容器，同域名的 `/docs` 和 `/docs/*` 都由它接管。修改宿主机挂载目录中的 Markdown 后刷新即可生效，不重新编译、不重建镜像。用户已要求本次同时上线生产。

## 行为与边界

- 文档正文继续位于 `frontend/docs/`，转换为普通 Markdown；只公开教程、导航和图片，不公开维护 README、隐藏目录或仓库其它文件。
- 固定版本 Docsify、搜索、代码复制、Mermaid 资源保存在镜像中，运行时不依赖第三方 CDN。
- 使用 history 路由，保留 `/docs/quick-start`、`/docs/scenarios/` 等现有地址；文档入口、内容基址和内链统一包含正确的 `/docs` 前缀。
- `/docs` 以 308 转到 `/docs/`；`/docs/batch-image` 和其尾斜杠形式以 308 转到现有 `/batch-image` 业务地址，保留查询参数。
- HTML、Markdown 与导航禁止持久缓存，缺失文章、Markdown 和资源返回 404，不回落主应用 SPA。
- 文档主容器使用 8080，宿主机默认只绑定 `127.0.0.1:18083`。入口代理保留 URI，不剥离 `/docs`。
- Markdown 单独挂载到 `/usr/share/nginx/html/docs`；入口和静态运行资源位于 `/opt/docsify/runtime`，挂载内容不会覆盖运行资源。
- 主应用构建解除 VitePress 依赖；旧的 Go 文档处理作为旧镜像/回滚兼容路径保留。
- 所有本地验证经 Docker Compose 容器执行，包括 HTTP 检查、浏览器检查、前端检查和部署脚本回归。

## 生产发布

使用不可变 `ghcr.io/eternity714/sub2api-docsify:sha-<commit>`，通过安装到 `/opt/sub2api/scripts` 的 Docsify 灰度脚本操作。先 `gray-status.sh`，在无流量独立端口部署 Docsify，健康与人工浏览检查通过后逐步分流 `/docs`。文档规则采用独立 Nginx include，不修改主应用蓝绿权重。现有 `gray-set-traffic.sh` 只重写主应用 policy，不会覆盖独立文档规则。

生产正文固定为真实目录 `/opt/sub2api/content`（脚本使用 `$DEPLOY_DIR/content`），新容器只读挂载该目录。目录不存在时优先从当前稳定 Docsify 正文初始化，全新安装才从镜像初始化；已存在时验证并复用，绝不覆盖正文或替换根目录 inode。运行时发布不自动同步正文，维护时明确更新固定目录并同步仓库源码。旧版本目录和容器保留，迁移期允许回切旧稳定槽位；放量及晋升必须核对候选状态和实际 Podman bind mount 均指向固定目录且只读。

旧 VitePress 页面引用旧资源而 Docsify 使用保留的 `/docs/_assets/` 命名空间；首次迁移观察期内 Markdown/Docsify资源始终从新文档容器获取，文章入口按文档专用 cookie 固定分流，`/docs/assets/` 暂时保留旧资源入口。晋升后全部 `/docs` 路径转到文档容器。首次迁移失败时将文档入口切回原主应用；后续发布及固定目录迁移失败时切回当前稳定 Docsify 槽位。回滚保留容器、正文、日志与备份。

## 验收

1. 通过同一入口访问 `/docs/`、所有现有教程及主应用其它路径，分流正确。
2. 直接刷新深链接正常；首页、侧栏、搜索、代码复制、Mermaid、移动布局可用。
3. 修改挂载的 Markdown 后，不重启容器，刷新出现新正文。
4. 未知文章/资源和维护文件返回 404；历史批量生图入口跳转到正确业务地址。
5. 发布脚本对标签、端口、主应用状态、Nginx 校验失败与流量回滚有明确保护和回归验证。
6. 生产逐步分流后检查文档、主应用健康、Nginx、错误日志及槽位状态。
