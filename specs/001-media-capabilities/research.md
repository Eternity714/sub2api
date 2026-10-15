# F01 技术决策

日期2026-10-15；依据当前代码与[上游能力研究](../unified-media-generation/capability-matrix.md)。

1. Decision: 固定typed schema v1，字段树closed、模式和有限组合算子。Rationale: 多模型差异需要配置，但现无JSON Schema库且任意表达式难审计。Alternatives: 完整JSON Schema/脚本引擎引入范围与依赖；纯模型硬编码无法后台维护。
2. Decision: 单一global(public_model,media_type)能力记录，路由引用与候选参数由F02处理。Rationale: 同名双媒体、管理员配置可复用；不为F01重造路由。Alternatives: 把完整schema塞入GroupsView或每条路由重复存储导致漂移。
3. Decision: JSONB新SQL表+活跃唯一+revision CAS。Rationale: 现GRS仓储支持sql.DB，避免生成大批Ent文件；新迁移保持增量。Alternatives: 用普通唯一键配软删除错误，或直接设置全局缓存忽略变更。
4. Decision: 能力ID/版本/规则摘要/完整schema快照交给F03固定。Rationale: 管理员更新不能改变在途任务；本切片不实现任务资金事务。
5. Decision: 管理页独立，基础字段可视编辑，高级模式JSON。Rationale: GroupsView已有窄路由表单，不适合嵌入全部字段树；沿用BaseDialog/TablePageLayout/通用API client。
6. Decision: 请求解析独立严格，不全局改变Gin配置。Rationale: encoding/json默认允许重复键、大小写匹配和多个值，现其它平台需兼容；只限制新媒体协议。

证据位置：backend/ent/schema/user_attribute_definition.go（JSONB/软删除）；backend/internal/server/routes/admin.go（认证审计）；backend/internal/repository/grsai_task_payload_repo.go（SQL模式）；backend/internal/pkg/response/response.go（公共响应不能用管理envelope）；frontend/src/api/client.ts（统一错误）；frontend/src/views/admin/GroupsView.vue（现路由表单）。

无阻塞F01的NEEDS CLARIFICATION。辅助结果、账号绑定素材和自动时长计价只影响后续特定模型模式，不以默认值代替用户决策。
