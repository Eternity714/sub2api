# F01 最终验证记录

日期：2026-10-15。目录：`C:/Users/Administrator/.codex/worktrees/unified-media-generation/sub2api`。与运行时一起完成最终串行 Docker 验证；不包含真实平台付费生成。

- 后端全量 unit：59 个包、12,040 个顶层测试通过，新增媒体测试无跳过。日志 `backend/.media-release-unit-results.jsonl`。
- 新增生产路由注册认证测试：能力 CRUD／模板／预览及媒体任务诊断 9 个端点的 401／403 全部通过。
- 内置模型文档契约：115 个目录章节、字段／默认值／范围／枚举、模式、组合约束、像素映射和示例均与能力 schema 一致；四个公开路径及参数文档链接校验通过。
- 真库集成：`TestMediaCapabilityRepositoryIntegration`、多候选迁移及任务/额度回归，共 10 个顶层场景和 4 个子场景全部通过，无 skip。日志 `backend/.media-release-integration-results.jsonl`。
- 前端 lint、vue-tsc 类型检查通过；关键回归 39 文件 659 项、专项回归 21 文件 337 项通过（部分用例重合，不将两者相加作为去重数量）。日志 `.media-final-frontend-results.jsonl`。
- 后端与前端平台目录同步测试在显式挂载前端常量后通过，没有使用原缺文件的 skip 作为证据。
- 需求追溯限定本特性 13 个源文件，27 个 AC 声明／27 引用、无孤儿与幽灵；具体行为及边界见 [追溯表](../unified-media-generation/traceability.md)。

全部 Go/前端业务测试在 `deploy/docker-compose.test.yml` 的 Compose 容器中运行；真库仅用 Testcontainers 临时数据库。独立复审发现的上游错误正文 ID 误受理、坏图片交付、多 URL 首项失败阻塞和跨入口媒体占用缺失等问题已修复并固化测试。

组件测试证明模板复制、独立配置、结构化编辑、保存失败和 CAS 冲突保留草稿。它们不等同于真实浏览器＋真实平台生成实测；内置模板和文档一致不自动保证管理员之后修改数据库配置的文档同步。真实账号、S3 交付及首次管理员确认相关边界见 [整体交付记录](../unified-media-generation/implementation-status.md)。
