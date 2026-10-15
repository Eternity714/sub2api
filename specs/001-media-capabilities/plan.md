# Implementation Plan: F01 统一媒体协议与模型能力

**Branch**: `codex/unified-media-generation` | **Date**: 2026-10-15 | **Spec**: [spec.md](spec.md)

## Summary

复用现有Go/Gin管理鉴权、PostgreSQL和Vue管理页面，新增有限typed能力规则、纯校验核心、SQL持久层和管理员能力维护。公共生成路径待F07接入；本切片校验器不依赖账号、资金或上游客户端。因交付时限，固定v1数据契约，后端核心、CRUD和前端同一特性内按文件职责并行，保留TDD及Docker验证。

## Technical Context

- Language/Version: 后端Go 1.27.2；前端现有TypeScript、Vue3与Node20检查容器。
- Primary Dependencies: 复用Gin、database/sql、项目错误/响应与Vue组件；不新增任意表达式引擎或远程JSON Schema依赖。
- Storage: PostgreSQL JSONB；新表media_model_capabilities，沿现GRS SQL仓储模式避免本切片大量Ent生成代码。
- Testing: backend-test/ backend-integration/ frontend-test Docker Compose服务，严禁宿主机业务测试与类型检查。
- Target Platform: 现有Linux服务及管理员浏览器。
- Performance: 无网络纯校验；请求最多1MiB、metadata256KiB、schema512KiB、嵌套深度8、配置节点256、模式32、组合规则128；超限明确拒绝。
- Scope: 一个能力实体、两个基础模板、一组管理员CRUD/预览接口和管理页；公共生成接入与候选调度不在本切片。

## Constitution Check

- 前置：通过。需求来自已确认PRD；本机测试只通过Docker；无生产部署；不修改既有迁移；不改其它平台入口。
- 设计后：通过。有限规则不执行脚本，管理操作沿既有鉴权/审计/合规链；快照与revision保证后续在途稳定；没有把动态能力查询加入范围。
- 没有新增技术栈或审批流程。缺失的历史RELEASE_DEPLOYMENT.md不构成虚构发布依据；生产仍以AGENTS.md灰度约束为准，本切片不发布。

## Project Structure

```text
backend/internal/pkg/mediacapability/       # typed schema/严格JSON/规范化/候选匹配
backend/internal/service/media_capability* # 管理服务、模板、快照
backend/internal/repository/media_capability* # SQL持久与CAS
backend/internal/handler/admin/media_capability* # 管理API
backend/internal/server/routes/admin.go    # 仅挂管理API
backend/internal/{handler,repository,service}/wire.go # 依赖注入
backend/migrations/*media_model_capabilities.sql # 新的兼容增量迁移
frontend/src/types/mediaCapability.ts
frontend/src/api/admin/mediaCapabilities.ts
frontend/src/views/admin/MediaCapabilitiesView.vue
frontend/src/components/admin/media/
specs/001-media-capabilities/{spec,plan,research,data-model,quickstart,tasks}.md
specs/001-media-capabilities/contracts/
```

## Phase 0: Research decisions

详见[research.md](research.md)。选择固定typed schema v1而非任意JSON Schema；只引入单一能力实体，候选以纯输入契约预留F02接入；基础模板复制后独立。现Composite唯一键暂不修改，F02必须处理同模型多平台候选约束。

## Phase 1: Design

- [data-model.md](data-model.md)：活跃唯一、revision CAS、软删除和快照。
- [contracts/capability-v1.md](contracts/capability-v1.md)：固定字段/模式/规则/规格/拆分契约与校验顺序。
- [contracts/admin-api.md](contracts/admin-api.md)：管理员API与错误约定。
- [contracts/public-media.md](contracts/public-media.md)：供F03/F07复用的公共协议边界。
- [quickstart.md](quickstart.md)：Docker可运行验证。

### Validation pipeline

体积与递归深度→单对象/重复键/精确字段名/EOF/UseNumber→固定model及顶层字段→取得对应媒体schema→按声明补默认且保留显式来源→字段类型与范围→唯一模式→条件必填及组合规则→可选像素映射→规范化结果。无模式或多模式匹配均拒绝。对象递归closed；不接受远程引用、脚本或任意表达式。

schema保存前运行相同编译/静态验证：默认值合法、字段路径存在、边界有序、模式及规则类型受支持。不存在输入的缺省值不能覆盖已显式给出的null/false/0。可选字段的null/空串分别受nullable与min_length约束。

### Candidate contract

F01提供纯MatchCandidate；F02负责选取数据库候选与账号。fields.n.maximum是整项上限，batch.native_max_results是单次上游上限；允许拆分且非语义关联的请求不因native=1排除n=3，但不得绕过整项上限。匹配不修改原规范化输入，不通过候选默认值改已接受参数。

### Image mapping and reserved operations

只有声明image_specs的模型要求size/resolution命中唯一映射；按原生比例/档位处理且不需像素转换的模型可不声明映射。禁止猜测、全局换算和metadata.output_size。图层分解true不在可开放模式；混合结果、绑定资源和自动时长计费未决能力不自动启用。

### Incremental implementation and rollout

先核心行为测试与纯实现，同时编写CRUD/前端行为测试；核心契约稳定后完成CRUD和UI，最后接线并跑相关回归。F01管理配置默认禁用，不自动配置108模型；全部F01—F07通过前不将真实统一生成流量接入，不提前关闭旧GRS接口。

## Complexity Tracking

无需违反既有约束。纯SQL新仓储沿用已有模式，不引入新持久框架；复杂规则限制为固定算子，不开放管理员执行代码。
