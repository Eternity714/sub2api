# Sub2API 工程约束

本文件提炼仓库 AGENTS.md 和已确认需求，作为 Spec-Kit 检查入口，不新增审批或产品决策。

1. 提交日志使用中文。遵循既有 Go/Gin/Ent 分层与 Vue 管理端组件规范。
2. 本机所有业务测试、类型检查和前端回归只能通过 deploy/docker-compose.test.yml 中对应容器执行，不在宿主机运行。
3. 需求依据为 specs/unified-media-generation/prd.md；能力证据为 capability-matrix.md；实现按 feature-breakdown.md 的依赖推进。已经确认的协议、范围和账务规则不得静默更改。
4. PRD/spec 描述可验证行为；实现细节写入 plan。新功能先编写会失败的行为测试，再实现和验证。
5. 不泄露账号凭证；管理员配置必须经过服务端鉴权与校验。配置变更不能改变已受理任务的语义和计价依据。
6. 迁移保持可追溯与兼容，不修改已有迁移内容。生产部署另按 AGENTS.md 灰度规则执行；本次工作不授权生产流量切换。

Version: 1.0.0 | Derived: 2026-10-15 | Source: AGENTS.md + 已确认媒体 PRD
