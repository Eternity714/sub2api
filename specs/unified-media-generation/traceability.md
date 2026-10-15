# 统一媒体 PRD 验收追溯

更新日期：2026-10-15。范围为 [PRD](prd.md) 的 27 条验收标准。

本表记录实际存在的测试断言，不把“函数存在”“模板列入目录”或“某个下层校验通过”当作整条验收已完成。运行结果与覆盖范围是两件事：F01 已执行证据见 [validation.md](../001-media-capabilities/validation.md)，F02—F08 的最终串行验证记录由整体交付记录维护。新增测试未获得最终执行结果前均视为待验证。

| PRD 验收 | 对应可运行测试 | 已覆盖范围与明确边界 |
| --- | --- | --- |
| AC-001 | `handler/unified_media_handler_test.go::TestUnifiedMediaHandlerCreationHasExactlyPublicEnvelope/image`；`repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryHTTPWorkerEndToEnd/n_1` | HTTP 图片 202 的五字段、queued/0、公开模型和秒时间；真实数据库创建先于上游提交。唯一 ID 来自独立任务创建，未以大样本碰撞测试证明。 |
| AC-002 | `handler/unified_media_handler_test.go::TestUnifiedMediaHandlerCreationHasExactlyPublicEnvelope/video` | 视频 HTTP 202 与同一响应契约；handler 使用任务服务替身，尚不能替代真实视频生成、下载及交付全链路。 |
| AC-003 | `service/composite_media_routes_test.go::TestCompositeMediaResolverReturnsAllOrderedExplicitCandidates`；`service/unified_media_task_test.go::TestUnifiedMediaRejectedCandidateCanUseSameAccountForNextModel` | 混合平台有序候选与明确拒绝后换候选模型；查询 envelope 不包含上游字段。真实跨平台故障切换仍需专门账号池场景。 |
| AC-004 | `service/unified_media_task_test.go::TestUnifiedMediaPublicCompletedMeansFinished`；`service/unified_media_upstream_test.go::TestUnifiedMediaUpstreamPollMalformedOrHTTPFailureRetries` | 处理中隐藏结果、progress=50，异常轮询不会当成功；正常 pending→持久化→HTTP 查询完整场景尚未单独断言。 |
| AC-005 | `repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryHTTPWorkerEndToEnd` | 合成上游 HTTP、下载、存储元数据替身、真库 worker 与 S3 URL 投影；真实 S3 服务及 GET 成功响应中两种终态时间字段仍需外部验收。 |
| AC-006 | `service/unified_media_task_test.go::TestUnifiedMediaFirstStorageFailureDoesNotBlockLaterResults`；`repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryCreateClaimPartialFinalize` | 不提前公开部分结果；部分成功只收成功结果费用，事务仅结算一次。分拆请求产生多个上游单元由 AC-023 的集成场景补充。 |
| AC-007 | `service/unified_media_task_test.go::TestUnifiedMediaNoAccountTaskCompletesWithoutSubmit`；`repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryFailedTaskReleasesEntireReservation` | 无账号终结、空结果及错误；失败真库结算释放全部余额、无 expires_at、无用量扣费。 |
| AC-008 | `repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryCreateClaimPartialFinalize`、`TestUnifiedMediaRepositoryFailedTaskReleasesEntireReservation` | 创建冻结、部分 capture、全失败 release、重复 finalization 不重复扣费。真实 S3 与数据库跨资源故障注入不在该替身存储场景内。 |
| AC-009 | `service/unified_media_task_test.go::TestUnifiedMediaPartialStorageRetriesKeepSuccessfulResult`、`TestUnifiedMediaFirstStorageFailureDoesNotBlockLaterResults` | 转存重试保持 in_progress，成功项保留，终态前不交付，失败项不阻断后续结果；资金幂等由 AC-008 覆盖。 |
| AC-010 | `service/unified_media_task_test.go::TestUnifiedMediaRestartResumesAcceptedTaskOnOriginalAccount`；`TestUnifiedMediaExpiredSubmissionIntentCannotResubmit`；`repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryLostLeaseRecoversSubmissionWithoutResubmit` | 新增用例从序列化快照建立新 worker，固定原账号／上游 ID 轮询，提交数为 0，重复运行不重复转存或 finalization；真库另覆盖未知受理禁止重发及旧租约 fencing。新增恢复用例已在最终全量容器回归通过。 |
| AC-011 | `handler/unified_media_handler_test.go::TestUnifiedMediaHandlerQueryUsesAuthenticatedOwnerAndPathMedia`；`repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryCreateClaimPartialFinalize` | handler 固定认证主体；真库拒绝不同 Key 和错误媒体类型。不同用户、未知任务与跨 Key HTTP 错误形状一致性未在同一请求级场景完整比较。 |
| AC-012 | `server/routes/gateway_test.go::TestGatewayRoutesOpenAIImagesPathsAreRegistered`、`TestGatewayRoutesGrokImagesAndVideosPathsAreRegistered`；`server/routes/unified_media_test.go::TestUnifiedMediaCompositePreservesPublicModelAndCandidates` | 原平台图片／视频路由可达，媒体 Composite 保留公开名且不提前改写；未配置媒体的 Composite 保持原行为的完整 dispatch 比较尚需独立用例。 |
| AC-013 | `server/routes/unified_media_test.go::TestUnifiedMediaLegacyGRSEndpointsRetired`；既有 OpenAI／Grok 路由用例 | 旧三个 GRS 入口 404；不增加旧任务排空前置条件，其他平台入口回归由既有路由测试承担。 |
| AC-014 | `pkg/mediacapability/capability_test.go::TestValidateStrictJSONAndFieldRules`；`service/unified_media_upstream_test.go::TestUnifiedMediaPayloadGRSNano`、`TestUnifiedMediaPayloadToAPIsBasic` | metadata 严格校验与两平台转换，保留接受参数；已受理失败的摘要由上游及任务用例分层覆盖，未对所有目录型号做真实付费生成。 |
| AC-015 | `handler/unified_media_validation_test.go::TestUnifiedMediaInvalidRequestsCannotCreateOrFreeze/video_model_on_image_path`、`image_model_on_video_path` | 通过真实能力服务执行错误路径／媒体拒绝，任务仓储创建调用为 0；故无冻结事务及后续上游提交。 |
| AC-016 | `service/media_capability_test.go::TestMediaCapabilityServiceDualMediaAndCAS`；`repository/media_capability_repo_integration_test.go::TestMediaCapabilityRepositoryIntegration` | 同名双媒体持久化与按模型＋媒体读取。尚缺同名模型分别经过两条公开创建路径同时成功的请求级场景。 |
| AC-017 | `handler/unified_media_validation_test.go::TestUnifiedMediaInvalidRequestsCannotCreateOrFreeze/missing_schema`；`service/media_capability_test.go::TestMediaCapabilityServiceActiveSnapshotIsIndependent` | 缺失或禁用能力返回 400；请求级测试确认不创建、不冻结，不进入后续上游阶段。 |
| AC-018 | `frontend/src/components/admin/media/__tests__/MediaCapabilityEditorDialog.spec.ts::从模板复制后用结构化控件修改规则并保存独立配置`；`service/media_capability_test.go::TestMediaCapabilityTemplatesAreCompileableCopies` | 模板复制、字段差异、独立保存与可编译副本；尚未以浏览器＋数据库＋新路由生成联合证明管理员新增同协议型号全过程。 |
| AC-019 | `pkg/mediacapability/capability_test.go::TestValidateStrictJSONAndFieldRules`、`TestValidatePromptDependsOnInputModeAndVideoDoesNotAcceptImageFields`；`handler/unified_media_validation_test.go::TestUnifiedMediaInvalidRequestsCannotCreateOrFreeze` | 图片／视频外来顶层字段严格拒绝、定位字段，公开 HTTP 不创建任务或冻结资金。 |
| AC-020 | `pkg/mediacapability/capability_test.go::TestValidateStrictJSONAndFieldRules`、`TestValidateModesRulesAndDefaultsAreRechecked`；公开 HTTP 非法请求用例 | 元数据未知字段、类型、枚举、范围、必填和组合约束；HTTP 层确认不进入任务创建。 |
| AC-021 | `service/composite_media_routes_test.go::TestCompositeMediaCandidatesFilterCompleteCapabilitiesAndSnapshot`；无账号任务用例 | 4K 请求排除 2K 候选；合格候选消失返回 400；没有账号的后续终结由 AC-007 分层覆盖。尚未在同一个带不合格账号的 worker 场景断言禁止降级。 |
| AC-022 | `service/composite_media_routes_test.go::TestCompositeMediaCandidatesFilterCompleteCapabilitiesAndSnapshot`；`pkg/mediacapability/capability_test.go::TestCandidateMatchDoesNotMutateOrFillAcceptedParameters` | 全部候选不合格返回 400，候选不能注入缺省或修改接受参数；完整 HTTP 侧任务／冻结计数仍由创建前拒绝用例间接保证。 |
| AC-023 | `pkg/mediacapability/capability_test.go::TestMatchCandidateAllowsSemanticSplitWithoutWeakeningParameters`；`repository/unified_media_task_repo_integration_test.go::TestUnifiedMediaRepositoryHTTPWorkerEndToEnd/n_3` | n=3 与 native=1 的匹配、真库创建拆为三个单元，恰好三个提交／轮询／下载／存储及相应费用；HTTP 202 envelope 单独验证。 |
| AC-024 | `service/unified_media_task_test.go::TestUnifiedMediaPublicCompletedMeansFinished`；两个部分成功转存用例；HTTP 创建响应及真库 worker 场景 | queued=0、in_progress=50、全失败／部分成功／成功 completed=100 分布在多层断言，100 的含义不会与成功混用。 |
| AC-025 | `service/media_capability_docs_test.go::TestMediaCatalogDocumentationMatchesSchemasAndExamples` | 检查全部内置目录章节、字段类型／必填／默认／枚举／范围／格式、模式、组合约束、像素映射及可通过 schema 的示例。**管理员后续修改数据库能力后的文档发布同步不由静态目录测试保证。该新增测试已在最终全量容器回归通过。** |
| AC-026 | `pkg/mediacapability/capability_test.go::TestValidatePromptDependsOnInputModeAndVideoDoesNotAcceptImageFields`；`TestValidateStrictJSONAndFieldRules/no_prompt_for_text`；HTTP required_prompt 场景 | 素材模式允许省略且不虚构 prompt；文字模式缺失被拒绝，HTTP 侧不创建任务或冻结。 |
| AC-027 | `service/media_capability_catalog_test.go::TestMediaCatalogTemplatesCompileAndExamplesMatch`、`TestMediaCatalogSpecificConstraintsDoNotLeakAcrossModels` | Seedream 5 Pro 普通示例通过，图层分解被对应 schema 拒绝；公开创建路径无副作用的该特定型号场景尚需补充。 |

上述后端路径均相对 `backend/internal/`；前端路径相对仓库根目录。子测试名称展示为 Go 对空格归一后的名称。

## 机械检查

已执行技能规定的脚本，只读取输出，没有读取脚本源代码：

```powershell
python D:/work/个人/sub2api/.agents/skills/ai-product-dev-pack/scripts/traceability_check.py specs/unified-media-generation backend
```

首次诊断结果：27 条 AC 声明、34 条测试引用，`no-go`；当时孤儿为 AC-010、AC-018，另有 9 个支付特性 031 系列幽灵引用。AC-018 的前端用例不在该次 `backend` 扫描范围；AC-010 随后补充了上表的真实恢复场景。仓库其他文档测试也使用 AC-001～008，存在 ID 跨特性串号，不能据该宽目录扫描宣称本特性已通过。

最终应以本特性明确列举的后端与前端测试文件集运行该脚本，并保留范围清单；不得把其他特性的同名 ID 算作覆盖，也不得仅增加注释而省略行为断言。即使机械检查通过，上表的“部分覆盖”和外部服务未实测边界仍然有效。

最终已对 [traceability-tests.txt](traceability-tests.txt) 列明的 13 个源文件运行限定校验。临时目录只包含这些源文件的静态副本，保留仓库相对路径，排除支付与原文档特性，避免同名 ID 串号。结果：**27 条 AC 声明、27 条测试引用，0 个孤儿、0 个幽灵，退出码 0**。这仅表示逐条可追溯，不能替代用例通过记录和上表的覆盖边界。

复现方式（在仓库根目录）：

```powershell
$mediaTraceRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('sub2api-media-trace-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $mediaTraceRoot | Out-Null
foreach ($mediaTraceSource in (Get-Content specs/unified-media-generation/traceability-tests.txt)) {
  $mediaTraceTarget = Join-Path $mediaTraceRoot $mediaTraceSource
  New-Item -ItemType Directory -Force -Path ([System.IO.Path]::GetDirectoryName($mediaTraceTarget)) | Out-Null
  Copy-Item -LiteralPath $mediaTraceSource -Destination $mediaTraceTarget
}
python D:/work/个人/sub2api/.agents/skills/ai-product-dev-pack/scripts/traceability_check.py specs/unified-media-generation $mediaTraceRoot
```
