# F01 Tasks

## Setup

- [x] T001 初始化隔离分支与Spec-Kit最小项目结构 `.specify/feature.json`。
- [x] T002 完成规格、方案和固定契约 `specs/001-media-capabilities/contracts/`。

## Foundational / US2 核心校验

- [x] T003 [US2] 先写严格JSON、白名单、模式及默认值行为测试 `backend/internal/pkg/mediacapability/capability_test.go` 并在Docker观察失败。
- [x] T004 [US2] 实现typed schema编译与请求校验 `backend/internal/pkg/mediacapability/`。
- [x] T005 [US2] 验证n整项/单次语义、像素映射和reserved操作 `backend/internal/pkg/mediacapability/`。

## US1 管理配置

- [x] T006 [P] [US1] 编写管理服务、仓储CAS与handler测试 `backend/internal/{service,repository,handler/admin}/media_capability*test.go`。
- [x] T007 [US1] 新增兼容迁移与SQL仓储 `backend/migrations/*media_model_capabilities.sql`、`backend/internal/repository/media_capability_repo.go`。
- [x] T008 [US1] 实现服务、基础模板和快照 `backend/internal/service/media_capability.go`。
- [x] T009 [US1] 实现管理员CRUD及无副作用预览 `backend/internal/handler/admin/media_capability_handler.go`。
- [x] T010 [P] [US1] 先写模板编辑/保存错误/版本冲突交互测试 `frontend/src/components/admin/media/__tests__/`。
- [x] T011 [US1] 实现API/types、能力列表和结构化编辑 `frontend/src/{api/admin,types,views/admin,components/admin/media}/`。
- [x] T012 [US1] 接入导航和管理员路由 `frontend/src/router/index.ts`、`frontend/src/components/layout/AppSidebar.vue`。

## US3 一致性与接线

- [x] T013 [US3] 验证快照深复制和revision CAS `backend/internal/service/media_capability_test.go`。
- [x] T014 [US1] 接线Wire/provider/admin路由 `backend/internal/{handler,repository,service}/wire.go`、`backend/internal/server/routes/admin.go`。
- [x] T015 [US1] 验证管理权限和真实持久回读 `backend/internal/handler/admin/media_capability_handler_test.go`。

## Cross-cutting

- [x] T016 完成Docker后端、前端与集成检查 `deploy/docker-compose.test.yml`。
- [x] T017 代码复审并修复重要问题，记录证据 `specs/001-media-capabilities/validation.md`。

依赖：T003→T004→T005；T006与T010可并行先写测试，生产代码依赖T002契约；CRUD依赖核心；UI与后端共享契约；T014/15依赖相应实现；全部通过后才交付F01，F02顺序接续。本特性内部按不重叠文件并行，不并发重写相互依赖的多个特性。

最终状态：后端全量 unit、管理员真实路由认证、逐模型文档校验、真库回读/CAS、前端 lint/类型/关键回归均已通过。详见 [validation.md](validation.md) 和 [整体交付记录](../unified-media-generation/implementation-status.md)。
