# 统一媒体生成测试交付记录

日期：2026-10-15。分支：`codex/unified-media-generation`。实施目录：`C:/Users/Administrator/.codex/worktrees/unified-media-generation/sub2api`。当前交付目标是本地配置与联调；没有生产部署、合并主分支或真实付费生成。

## 已实现

- F01：图片／视频独立能力记录、数据库 CRUD、revision CAS、基础模板、严格嵌套字段及输入模式校验、后台编辑和预览。
- F02：ToAPIs 账号与只读连接测试，GRS／ToAPIs 共享账号额度管理，Composite 多候选与完整参数能力筛选，用户平台额度继续排除两者。
- F03／F06：持久父任务及执行单元、余额和 Key 预留、账号并发与共享额度、提交意图及租约恢复、S3 逐结果重试、部分收费、最终失败释放、幂等终结。已有媒体预留实时进入其它收费入口的 Key 准入。
- F04／F05：GRS 与 ToAPIs 普通生成协议、参数转换、提交及查询、Midjourney 四宫格、特殊独立查询路径；不明受理禁止重复提交。
- F07：四个公开端点、202 五字段、只读归属查询、三状态与 0/50/100、Unix 秒、S3 URL 字符串数组、单一 error 字符串。
- F08：旧三个 GRS 入口直接 404，管理员任务诊断列表／详情、参数文档与回归。

使用及配置顺序见 [统一接口说明](../../docs/UNIFIED_MEDIA_API.md)，逐型号见 [模型参数](../../docs/UNIFIED_MEDIA_MODEL_PARAMETERS.md)。测试与 PRD 的映射见 [追溯表](traceability.md)。

## 当前验证证据

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 后端全量 unit | 通过；59 个包、12,040 个顶层测试，无新增媒体跳过 | Docker Compose backend-test；`backend/.media-release-unit-results.jsonl` |
| 前后端平台目录同步 | 通过；额外挂载前端目录后显式执行，未沿用缺文件 skip | `TestFrontendBuiltinPlatformCatalogInSync` |
| 真库集成 | 通过；10 个顶层场景及 4 个子场景，无 skip | Testcontainers 临时 PostgreSQL／Redis；`backend/.media-release-integration-results.jsonl` |
| 前端 lint／类型／关键回归 | 通过；关键 39 文件 659 项、专项 21 文件 337 项（集合有交叉，不相加为去重总数） | `.media-final-frontend-results.jsonl` |
| 前端与内嵌后端构建 | 通过；CGO_ENABLED=0 内嵌构建及本地运行时镜像成功 | 前端／后端／预览 build 日志 |
| 需求追溯 | 通过；27 声明／27 引用，0 孤儿／幽灵 | 限定本特性 13 源文件清单与脚本输出；不是全场景集成通过声明 |
| 契约变更确认 | 用户已明确确认立即关闭旧三端点；其它媒体入口保留 | PRD AC-013 和原平台路由回归 |

真库场景覆盖能力持久回读与 CAS、媒体多候选迁移唯一约束、实时 Key 未结占用、并发媒体创建不透支、共享账号总额／并发、部分 capture 幂等、全失败释放、未知提交与租约接管、软删除 Key／账号仍可结算，以及合成 HTTP 上游→worker→下载→存储元数据→正式扣费。`n=3` 明确验证三次独立提交／轮询／下载／存储，扣费 1.5，冻结为 0，重复执行不再收费。

## 模型开放边界

目录共 115 条，103 条提供可编译的基础能力模板，加 2 个通用脚手架，共 105 个模板；默认均禁用，管理员需配置公开模型、路由和价格。12 条 ToAPIs 目录项缺少完整依据或涉及独立特殊操作，未提供可启用 schema，不能宣称所有目录项已可调用：

- `gemini-2.5-flash-image-preview-vip`（image）
- `gemini-3.1-flash-lite-image-official`（image）
- `grok-imagine-1.0-edit`（image）
- `nano_banana`（image）
- `nano_banana_2`（image）
- `nano_banana_pro`（image）
- `gemini_omni`（video）
- `viduq3-ad`（video）
- `viduq3-drama`（video）
- `viduq3-mix`（video）
- `viduq3-pro-fast`（video）
- `wan3.0-video-prime`（video）

Seedream 5.0 Pro 的普通生成已覆盖，图层分解及其他未声明特殊操作返回创建前错误。没有调用真实 GRS／ToAPIs 付费生成，内置模板通过不代表账号实测。管理员改动数据库 schema 后需同步发布对应下游参数说明；静态文档测试只保证内置模板与仓库文档一致。

## 本地联调与边界

独立预览配置位于 `.dev/unified-media-preview/`，使用独立 app／PostgreSQL／Redis／MinIO 卷，预期入口 `http://127.0.0.1:18084`。已启动并检查：app、PostgreSQL、Redis、MinIO 均 healthy。`/health`、内嵌前端与 JS、管理员登录、未认证 401、首次管理员确认 423 检查通过。凭据仅保存在其忽略的 `.env.local`，不进入提交或日志。

预览不预置真实上游账号。首次管理员沿用系统已有部署确认流程，自动检查不会代为确认；确认前 CRUD 冒烟会明确标为待人工确认，不能作为成功证据。本次冒烟返回 `pending_admin_confirmation`（退出码 2），`crud_executed=false`、生成任务数为 0，未代用户完成部署承诺。用户在页面按既有流程确认后可重跑 `.dev/unified-media-preview/smoke.py` 的容器检查，验证运行中服务的完整 CRUD。真实上游和实际媒体转存仍需使用授权测试账号联调。

新媒体之间的预留通过数据库事务限制；普通文字请求已在途、后结算的原有机制保留。实时未结媒体占用用于后续新请求准入，不能解释成所有旧协议在途成本均已有硬预留。账号用量按本地基础价与账号倍率保守计量，独立于用户计费，不等于上游平台真实账单。

测试服务管理员：`admin@unified-media.local`；密码见忽略文件 `.dev/unified-media-preview/.env.local` 的 `ADMIN_PASSWORD`。镜像标记使用 `workingtree-20261015`，标明基线与未提交源构建，不冒充生产 SHA 镜像。本地启动先修正失效 Docker builder，再因旧文档服务占用 18083 改用 18084，没有更改旧服务或其数据。
