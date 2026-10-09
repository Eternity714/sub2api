# 需求与验证追踪

日期：2026-10-09。验收编号采用检查器识别的 `AC-001` 至 `AC-008`。

需求源为 `prd.md`。下表严格区分自动化断言、已有人工证据和待验子场景；编号出现在测试或人工证据登记中，仅表示有对应记录，不表示整条验收标准已经完成。人工证据原文见 `validation.md`，`traceability/manual_evidence.py` 是检查器的人工证据输入，不能作为测试运行或计入自动化测试数量。

| 验收项 | 自动化验证 | 已有人工／真实服务证据 | 待验子场景 |
| --- | --- | --- | --- |
| AC-001 | `frontend/src/components/layout/__tests__/documentationEntries.spec.ts`：11 个组件用例覆盖默认／紧凑首页、页头、密钥用量、各模式侧栏及移动侧栏链接 | 主站入口真实浏览器跳转 | 无已知缺口 |
| AC-002 | `backend/internal/web/docs_test.go::TestDocumentationRoutes`、`docs_embed_test.go::TestEmbeddedDocumentationEntrypoints`：公开文章路由和旧 SPA 别名边界；挂载测试覆盖同一边界。组件测试覆盖后端模式入口可见性 | Compose HTTP 访客直接读取文档、文章；无认证 localhost 浏览器访问 `/docs/batch-image` 自动跳转 `/login?redirect=/docs/batch-image` 并显示表单；已登录本地管理员访问别名显示原批量任务 SPA | 真实后端模式访客会话；SPA HTTP 200 本身不证明客户端鉴权 |
| AC-003 | 上述路由／实际中间件测试及 `TestMountedDocumentationMissing404DoesNotUseEmbedded404`：文章响应和真实文档 404 | 首页入口、侧栏与 Paper2Any 点击；42 页、43 个内部链接 HTTP 200；quick-start 下一篇点击至 api-keys 后上一篇返回，均核对 URL 与标题 | 无已知缺口 |
| AC-004 | VitePress 构建成功并生成本地搜索索引（构建证据，不是搜索交互测试） | Codex、Seedance 标题／章节搜索与结果锚点跳转；清空查询不显示结果列表且 clear disabled，罕见字符串显示“没有找到相关结果” | 无已知缺口 |
| AC-005 | `TestDocumentationCSPNonce`、`TestMountedDocumentationPreservesHTTPAndCSP` 及两种真实嵌入入口：nonce 与 CSP 一致 | 目录／搜索锚点跳转、复制按钮“已复制”反馈 | 实际复制文本粘贴；反馈不能证明剪贴板内容正确 |
| AC-006 | VitePress 配置／中文教程构建检查（构建证据，不是持久化交互测试） | 亮暗切换、390×844 窄屏、目录跳转及 `scrollWidth == clientWidth == 375`；中文内容复核；手机导航切换浅色后 reload 仍浅色且 switch `aria-checked=false` | 无已知缺口 |
| AC-007 | VitePress 死链接与完整构建、Compose 全页 HTTP 检查 | `reference-coverage.md` 完整 49 条映射；官方配置和后端能力独立复核 | 没有以所有外部客户端安装或生产模型调用作为已完成证据；教程可用性仍取决于实际密钥分组 |
| AC-008 | `TestMountedDocumentationSelectsEntireSite`、`TestMountedDocumentationFallbackRequiresIndex`、`TestMountedDocumentationUpdatesWithoutRestart`、`TestMountedDocumentationCannotEscapeDirectoryThroughSymlinks` 及实际挂载中间件：整站选择、请求刷新读取、回退、不混站和目录越界 | 后端挂载相关 Compose unit/embed 回归已通过；Compose 真实服务 mountprobe 写入、更新、删除均即时生效；生产槽位目录配置由发布操作复核 | 生产两个灰度槽位挂载互不覆盖的部署证据 |

## 基线断言修正后的复验

已有账户与静态 logo 断言按当前实现修正后，以下定向 Compose 检查通过：

- `docker compose -p sub2api-docs-check -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test bash -c 'set -euo pipefail; corepack prepare pnpm@9.15.9 --activate; corepack pnpm --dir frontend exec vitest run src/components/account/__tests__/CreateAccountModal.spec.ts'`：44 个用例通过，退出码 0。
- `docker compose -p sub2api-docs-check -f deploy/docker-compose.test.yml run --rm --no-deps backend-test go test -count=1 '-tags=unit,embed' ./internal/web`：web 包通过，退出码 0。

这两条复验只解决前文记录中的基线断言失败；全套回归最终结果由对应的完整运行记录确认。

## 追踪检查

通过 `frontend-test` Compose 容器临时收集上述三个真实 Go/TS 测试文件和显式人工证据登记，再运行检查器；未读取或修改检查器逻辑：

```powershell
docker compose -p sub2api-docs-check -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test bash -c 'set -euo pipefail
trace_dir=$(mktemp -d)
cp backend/internal/web/docs_test.go backend/internal/web/docs_embed_test.go frontend/src/components/layout/__tests__/documentationEntries.spec.ts specs/documentation/traceability/manual_evidence.py "$trace_dir/"
python3 /workspace/deploy/docs-preview.traceability.tmp /workspace/specs/documentation "$trace_dir"'
```

输出：需求声明 8 条、测试引用 8 条，无孤儿需求、无幽灵需求，退出码 0。输出中的“测试引用”包含显式人工证据登记；它只证明需求与验证记录对齐，不执行测试、不证明待验子场景通过，也不代表已接入 CI。
