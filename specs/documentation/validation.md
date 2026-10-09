# 验证记录

日期：2026-10-09。恢复后的分支：`codex/documentation-preview`。

## 工作区与预览

原 documentation worktree 消失后，从 `4c1e3c8cfa6c49d787c42683c02e0aec6b67fe1f` 归档清理快照恢复完整实现，创建新的 managed worktree `documentation-preview/sub2api`。快照父提交为原基线 `201e14f51`。被忽略的真实预览 env 未进入快照，已重新生成独立本地凭证。

Docker 已恢复正常，本次没有删除正在使用的 socket，也没有重置 Docker、清理其他栈或数据卷。使用默认构建器绕过不存在的 mirror-builder。首次 Go 镜像构建发现共享模块缓存中空文件和 NUL 内容；预览通过 `GO_CACHE_SCOPE=sub2api-docs-preview` 使用新的专属 Go 缓存，保留原缓存和默认构建行为。Go 测试镜像固定为项目所需的 `golang:1.27.2`。

`sub2api-docs-preview` Compose 镜像构建成功，`up -d --wait` 成功，应用、PostgreSQL、Redis 均 healthy。预览保持运行于 `http://127.0.0.1:18082/home` 和 `/docs/`，使用独立网络与数据卷。其他预览服务保持运行。

## 已通过的检查

- 参考站工具确认是 VitePress 1.6.4，项目固定同版本；十篇教程维护为 Markdown。独立内容与代码静态审查已完成。
- 前端 Compose 的 lint 与类型检查通过。关键回归首次运行 31 文件、569 用例：567 通过，1 项已有账户断言失败，1 项 client hook 超时。单 worker 复验 `client.spec.ts` 与 `documentationEntries.spec.ts`：2 文件、41 用例全部通过，文档入口 11 用例通过。
- Compose 后续关键回归 21 文件、313 用例：312 通过，唯一失败仍为同一已有账户断言。
- `docker compose -p sub2api-docs-tests -f deploy/docker-compose.test.yml run --rm backend-test`：`go test -tags=unit ./...` 全部通过，退出码 0。
- 前端主应用和 VitePress 完整构建在 Compose 内通过。文档构建检查死链接，成功渲染页面并生成本地搜索索引。临时构建命令需像服务默认命令一样激活 pnpm 9.15.9，不能使用 Corepack 的最新默认版本。
- 嵌入模式文档定向回归通过：4 个顶层组、22 个命名子测试，覆盖两个实际嵌入中间件、路由、缓存和 CSP nonce。最终独立复验命令为 `docker compose -p sub2api-docs-tests -f deploy/docker-compose.test.yml run --rm backend-test go test -tags=embed ./internal/web -run 'TestDocumentation|TestInjectDocumentationNonce|TestEmbeddedDocumentation' -count=1`，退出码 0。
- Compose 中运行真实预览 HTTP 检查：`/health`、`/docs` 308、首页、全部十篇文章、文档 404、HEAD、方法限制、脚本 nonce 与 CSP 匹配、hash 资源 immutable 缓存、原 `/docs/batch-image` SPA 别名均通过。
- 浏览器检查主站文档入口跳转、搜索 Codex 与结果锚点跳转、代码复制按钮的“已复制”反馈、亮暗主题、390×844 窄屏布局与目录跳转，均正常。未将浏览器剪贴板 API 的空读值当作复制内容验证；实际粘贴仍由人工验收确认。页面控制台没有捕获到 error/warn。
- `git diff --check` 通过。

## 首轮发现的基线回归失败（后续已修正）

首轮不能声称整个前端关键回归与整个 embed web 包全绿，发现以下已有断言与当前实现不一致：

- `CreateAccountModal.spec.ts:663` 期望三个聚合商，main 的现有界面已包含第四项 GRS.AI。该测试和组件相对 main 无本次变更，单 worker 后续回归仍再现同一失败。
- embed 整包的两个静态文件测试请求 `/logo.png` 并期望 `image/png`，现有源文件和构建产物只有 `logo.svg`；不存在的 PNG 落入原 SPA fallback 返回 HTML。测试、公开 logo 和主构建行为相对 main 无本次变更。本次文档四组嵌入回归已单独通过。

上述为首次本地预览阶段的历史记录，当时没有合并或生产发布。后续已修正这些基线断言并完成定向及完整复验，结果见文末；复制后的实际文本仍待人工确认。

## 验收映射

AC-001 对应 `documentationEntries.spec.ts` 和真实主站入口检查；AC-002、AC-003、AC-005 对应 `docs_test.go`、`docs_embed_test.go` 与真实服务 HTTP 检查；AC-004、AC-006 对应 VitePress 构建、搜索、主题与手机尺寸浏览器检查。逐项覆盖范围及待验子场景见 `traceability.md`。

## 支持主题扩充与预览更新

用户确认“只迁移 Gkotta 已支持的功能”后，逐项盘点参考站 49 个入口：35 条重写、2 条部分适配、7 条合并、5 条排除。44 个入口有对应教程，去向与原因见 `reference-coverage.md`。新增 CLI/桌面/Agent、聊天、编辑器、翻译、工作流、国内模型及图片视频教程，文档共 42 个页面（含首页与使用场景总览）。

所有新增文章经过跨代理独立内容复核，客户端配置按当前官方资料核查，媒体条件按后端源码核查。修正 Gemini CLI Node.js 要求、Hermes 当前 provider 字段、Trae Full URL、NextChat 地址拼接、Grok 组合分组路由，以及 Gemini 原生与 OpenAI Images 权限条件。没有安装所有外部客户端或使用真实密钥进行生产模型调用。

本轮新增验证均通过 Docker Compose 容器运行：

- `sub2api-docs-tests` 的 `frontend-test` 执行 lint 与 `docs:build`，退出码 0，VitePress 渲染及死链接检查通过。
- `sub2api-docs-preview` 使用默认构建器完整重建镜像，主应用类型检查、i18n 3 用例、Vite 和 VitePress 构建以及 Go embed 二进制构建成功。随后 `up -d --wait --wait-timeout 180` 退出码 0，三个服务均 healthy。
- 新产物通过同一 Compose 后端文档定向回归，`go test -tags=embed ./internal/web -run 'TestDocumentation|TestInjectDocumentationNonce|TestEmbeddedDocumentation' -count=1` 退出码 0。
- Compose 内运行全页 HTTP 检查，42 个页面和 43 个去重内部链接全部 200，文章标题匹配且没有双重 `/docs/docs/`。健康、308、404、HEAD、405、CSP nonce、hash 资源缓存及旧批量生图别名均通过，退出码 0。
- 浏览器刷新后确认新增首页入口、搜索 Seedance 的标题与章节结果、结果锚点跳转、展开工作流目录和 Paper2Any 文章跳转。390×844 手机尺寸下页面 `scrollWidth` 与 `clientWidth` 均为 375（扣除滚动条），无整体横向溢出，代码区域自身滚动。浏览器未捕获 warn/error。已恢复正常尺寸并保留文档首页。

预览继续运行于 `http://127.0.0.1:18082/docs/`；本轮未修改后端业务或生产环境，也未重跑与文档内容无关的完整回归。前述现有基线失败仍保留在记录中。AC-007 对应完整覆盖表、源码/官方资料审核、VitePress 构建与全页 HTTP 检查。

## 发布门前补充验证

已有账户聚合商断言更新为当前四项（包含 GRS.AI）；静态资源回归改为现有 `/logo.svg` 及 `image/svg+xml`。定向 Compose 复验账户 44 用例和 unit/embed web 整包均退出 0。后续完整 Compose 前端回归分别 31 文件／569 用例、21 文件／313 用例全部通过；完整后端 unit 回归退出 0，包含挂载文档回归的 unit/embed web 整包退出 0。先前失败记录为已解决的历史问题。

随后后端完整 integration 检查通过项目规定的 Compose `backend-integration` 服务执行，退出码 0；前端 smoke 检查也通过 Compose 容器执行，退出码 0。用户已授权提交推送、生产灰度发布和晋升，生产结果在实际候选槽位检查后记录。

本地预览补充浏览器证据：在 quick-start 点击下一篇至 api-keys，再点击上一篇返回，URL 和页面标题均正确；搜索清空后没有结果列表且清除按钮禁用，罕见字符串查询显示“没有找到相关结果”；手机导航将暗色切换为浅色，刷新后仍为浅色，主题 switch 的 `aria-checked=false`。

复制按钮仍显示已复制；尝试 Control+V 时浏览器工具报告虚拟剪贴板无数据，未能验证实际粘贴内容，这一工具限制不作为复制失败或内容正确的证据。生产两个灰度槽位的外置文档挂载隔离尚待部署阶段记录；真实后端模式访客会话也未以浏览器证据完整确认。

无认证的 localhost 浏览器访问 `/docs/batch-image` 自动跳转至 `/login?redirect=/docs/batch-image` 并显示登录表单；已登录本地管理员访问原 127.0.0.1 别名显示原批量任务 SPA，而非 VitePress，确认旧入口继续使用认证流程。预览镜像完整构建退出 0，Compose `up --wait` healthy；容器内真实 HTTP 验证 mountprobe 文件写入、更新、删除均即时生效，42 页面／43 内链、CSP／缓存／404 等检查全部通过。

需求追踪检查经 Compose 运行通过：声明 8 条、引用 8 条，无孤儿或幽灵需求，退出码 0。详细命令、自动化／人工证据及未验子场景见 `traceability.md`；追踪门通过不等于所有浏览器场景已验证或自动强制接入 CI。

## 发布准备脚本检查

Compose 中执行 `bash -n deploy/gray-prepare-documentation.sh` 和 `bash deploy/tests/gray-prepare-documentation-test.sh`：15 个模拟场景全部通过，退出码 0。覆盖完整站点与子目录首页、双代资源、备份权限、候选非 0%、提取期间状态变化、两槽递归符号链接、发布标记、资源失败和清理边界。恢复旧的宽泛首页排除规则时，子目录首页回归明确失败；恢复修复版本后全部通过。对应语法检查和模拟场景已加入 CI release-helpers。

默认 Node 测试镜像缺少 Docker CLI，Compose 环境回归首轮因缺少 `docker` 无法执行；改用 Compose 中的 `docker:28-cli` 镜像并安装 Python 后，`deploy/tests/docker-compose-simple-mode-env-test.sh` 全部通过，退出码 0。没有在宿主直接执行测试。

本次没有修改业务 API 的请求、响应或数据库结构；公开 `/docs` 命名空间继续排除原受保护的批量生图别名。现有灰度脚本无共享锁，发布准备、部署与流量操作必须串行。生产发布尚待匹配提交的 CI、安全扫描和不可变镜像完成后执行，不提前记录晋升成功。
