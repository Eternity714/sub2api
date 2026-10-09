# 验证记录

日期：2026-10-09。分支：`codex/documentation`。

## 已完成的静态核查

- 参考站的 generator 和官方源码确认工具为 VitePress 1.6.4；本项目固定该版本，正文为十篇 Markdown。
- 依赖锁文件已由 pnpm 9.15.9 重新读取，保留现有依赖版本并增加文档工具依赖。
- `git diff --check` 通过。后端新增 Go 文件已 gofmt。
- 独立内容审查核对协议、配置、额度及所有教程链接；文档 404 按钮漏设中文可见文字的问题已修复。
- 独立代码静态审查未发现阻塞问题，核对 VitePress base/outDir、现有镜像构建链、静态文档入口、旧别名、缓存及 CSP nonce；尚未验证实际生成页面或浏览器运行时。
- 入口回归加入 `Makefile` 的前端关键回归列表；后端测试覆盖共享 handler 和两个实际嵌入入口。

## 容器验证阻塞

前端及后端 Docker Compose 命令均在连接 Docker API 时退出，错误为 `dockerDesktopLinuxEngine` 命名管道不存在，未执行测试。

启动 Docker Desktop 后，后台日志报 `sailor-ingest.sock` 无法访问，初始化 Ingest server 失败。重命名残留 socket 失败；自动审批策略拒绝清理 socket。未重置 Docker、删除数据卷或改用宿主机测试。

因此 lint、类型检查、单元测试、静态文档构建和浏览器验证均待 Docker 恢复后执行。当前不得合并或发布，不宣称测试通过。

## Docker 恢复后的验证命令

先运行项目要求的检查：

```powershell
docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test
docker compose -f deploy/docker-compose.test.yml run --rm backend-test
```

在前端服务容器内生成主应用及文档（不能在宿主机执行包含检查的 build）：

```powershell
docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test bash -lc "corepack enable && corepack prepare pnpm@9.15.9 --activate && pnpm --dir frontend install --frozen-lockfile --store-dir /pnpm/store && pnpm --dir frontend build"
docker compose -f deploy/docker-compose.test.yml run --rm backend-test go test -tags=embed ./internal/web
```

验收映射：AC-DOC-001 对应 `documentationEntries.spec.ts`；AC-DOC-002/003/005 对应 `docs_test.go` 和 `docs_embed_test.go`；AC-DOC-004/006 通过容器构建后的真实浏览器检查 VitePress 搜索、复制、窄屏和主题。VitePress 构建同时检查正文死链接。
