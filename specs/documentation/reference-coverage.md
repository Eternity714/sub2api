# 参考站文档覆盖清单

更新时间：2026-10-09。范围依据：用户明确“只迁移 Gkotta 已支持功能”；文档部署在 `https://www.gkotta.bid/docs/`，继续采用 VitePress 与 Markdown。

## 盘点来源和判定口径

读取参考站首页 `window.__VP_SITE_DATA__` 的完整 sidebar，并逐页读取公开 HTML，共 **49 个文档导航入口（含首页）**：17 个顶级入口、5 个“产品基础”、27 个“使用场景”。安装入口与使用教程存在重复，因此导航数量不等于需要新增 49 个独立文件。

- **迁移重写**：保留场景、安装、配置、调用验证和排错，按 Gkotta 接口、控制台路径及第三方当前官方文档重写。
- **部分适配**：只保留当前平台能够完成的流程，明确移除依赖参考站专属服务的步骤。
- **合并**：同一工具的安装与使用说明合为一篇，或将固定版本模型文章合入通用国模接入页。
- **排除**：专属服务、没有对应本站界面/端点，或官方实现不能证明可配置 Gkotta 地址。

“对应文档”表示本次迁移的文件去向；客户端允许配置自定义端点只证明接入机制，模型、分组和具体能力仍以当前密钥权限为准。本次验证覆盖官方资料与源码核查、文档构建及本地文档站，没有逐一安装外部客户端或使用真实密钥调用生产模型；此表不把兼容配置等同于客户端端到端实测。

最终分布：**迁移重写 35 条、部分适配 2 条、合并 7 条、排除 5 条**，合计 49 条。44 个参考入口通过迁移、适配或合并得到对应教程，5 个入口按明确理由排除；没有未决定的入口。

## 顶级入口

| 编号 | 参考站入口 | 处置 | 对应 Gkotta 文档 | 内容边界与理由 | 官方 / 实现证据 |
| --- | --- | --- | --- | --- | --- |
| 01 | [文档中心][r01] | 迁移重写 | [index.md](../../frontend/docs/index.md) | 保留文档入口与阅读路径；平台介绍、优惠、服务承诺按 Gkotta 实际信息重写。 | [平台接口][platform]; [VitePress][vitepress] |
| 02 | [简易使用][r02] | 迁移重写 | [easy-use.md](../../frontend/docs/easy-use.md) | 保留账号→密钥→配置→小请求验证链路；不保留参考站客服和根地址错误。 | [Codex][codex]; [CC Switch][ccswitch] |
| 03 | [快速开始][r03] | 迁移重写 | [quick-start.md](../../frontend/docs/quick-start.md) | 使用 Gkotta 注册、密钥、分组、模型和用量入口；删除参考站开业福利。 | [平台接口][platform]; [OpenAI SDK][openai-sdk] |
| 04 | [CC Switch 下载安装][r04] | 迁移重写 | [cc-switch.md](../../frontend/docs/cc-switch.md) | 按当前 CC Switch v4.0.4+ 安装和自定义供应商流程重写。 | [CC Switch][ccswitch]; [官方安装][cc-install] |
| 05 | [Claude Code 终端版安装][r05] | 迁移重写 | [claude-code.md](../../frontend/docs/claude-code.md) | 保留 CLI 安装、Messages 地址、认证和模型选择；不强制照搬 Python 依赖或旧截图。 | [Claude Code][claude-code]; [平台鉴权][auth] |
| 06 | [Claude Code 桌面端安装][r06] | 迁移重写 | [claude-desktop.md](../../frontend/docs/claude-desktop.md) | 按官方 3P 网关模式及 CC Switch Desktop profile 重写，说明直连、角色映射和重启。 | [官方 3P][claude-3p]; [CC Switch Desktop][cc-desktop] |
| 07 | [Codex 安装配置][r07] | 迁移重写 | [codex.md](../../frontend/docs/codex.md) | 保留桌面/CLI/IDE 共享配置的核心流程，使用 Gkotta /v1 Responses 与实际模型 ID。 | [Codex][codex]; [Codex 配置][codex-config] |
| 08 | [Codex++ 安装配置][r08] | 迁移重写 | [codex-plus.md](../../frontend/docs/codex-plus.md) | 官方 CodexPlusPlus 支持自定义 Base URL 与 Responses；独立说明增强器安装、认证和中转注入条件。 | [CodexPlusPlus 官方 README][codex-plus] |
| 09 | [Tisoz Image][r09] | 排除 | — | Tisoz Codex 插件固定调用 img-task.ns.tisoz.com，使用 TISOZ_IMAGE_API_KEY；不是 Gkotta 已有客户端或通用端点，不能替换品牌后宣称兼容。 | [原页][r09]; [Gkotta 图片接口][images] |
| 10 | [Hermes 安装与配置指南][r10] | 迁移重写 | [hermes.md](../../frontend/docs/hermes.md) | 保留安装、自定义端点与模型向导；采用当前 providers/transport 配置，不复用过期 llm.* 和移除的环境变量。 | [Hermes 安装][hermes-install]; [AI Providers][hermes-providers] |
| 11 | [OpenClaw 安装与配置指南][r11] | 迁移重写 | [openclaw.md](../../frontend/docs/openclaw.md) | 按官方 models.providers 自定义地址重写，区分添加供应商与选择默认模型。 | [OpenClaw 安装][openclaw-install]; [自定义供应商][openclaw-custom] |
| 12 | [OpenCode 安装与配置指南][r12] | 迁移重写 | [opencode.md](../../frontend/docs/opencode.md) | 保留桌面/CLI 安装、provider、/connect 和模型选择，按实际 SDK 包区分 Chat Completions 与 Responses。 | [OpenCode 供应商][opencode]; [配置][opencode-config] |
| 13 | [API 手册][r13] | 迁移重写 | [api-manual.md](../../frontend/docs/api-manual.md) | 统一索引 Gkotta 的文本、模型列表、密钥用量和媒体接口；每项能力以当前路由及分组条件为准。 | [平台接口][platform]; [OpenAI API 规范][openai-reference] |
| 14 | [国模使用说明][r14] | 迁移重写 | [domestic-models.md](../../frontend/docs/domestic-models.md) | 保留分组、模型 ID 与客户端接入方法；不保证参考站固定国模名单或任意国模适配 Codex。 | [平台接口][platform]; [可用渠道][channels] |
| 15 | [图像生成 API 使用指南][r15] | 迁移重写 | [image-generate.md](../../frontend/docs/image-generate.md) | 按 Gkotta OpenAI 图片、Gemini 原生图片及异步图片任务重写；不保留 Tisoz 独立服务地址与固定模型/尺寸承诺。 | [平台图片路由][platform]; [OpenAI Images][openai-images] |
| 16 | [价格说明][r16] | 迁移重写 | [billing.md](../../frontend/docs/billing.md) | 说明余额、订阅、密钥额度和使用记录；删除原站 30 天退款、活动折扣、发票/SLA及固定定价承诺。 | [Gkotta 使用记录][usage]; [平台用户路由][users] |
| 17 | [疑难杂症][r17] | 迁移重写 | [troubleshooting.md](../../frontend/docs/troubleshooting.md) | 覆盖真实鉴权、分组、额度、限流及路径排错；不继承参考站备用域名、固定 5 并发或 QQ 客服。 | [平台鉴权][auth]; [Google 鉴权][google-auth] |

## 产品基础

| 编号 | 参考站入口 | 处置 | 对应 Gkotta 文档 | 内容边界与理由 | 官方 / 实现证据 |
| --- | --- | --- | --- | --- | --- |
| 18 | [素材引用生视频实战][r18] | 部分适配 | [image-to-video.md](../../frontend/docs/image-to-video.md) | 只迁移“输入图片→创建视频任务→轮询→下载”通用场景；删除 icover 素材上传/入库、真人认证、asset:// 账号链路。 | [平台视频接口][platform]; [原页][r18] |
| 19 | [AI 视频工作台使用指南][r19] | 排除 | — | 参考页依赖 FastAIToken /video-studio 四模式专属工作台；Gkotta 没有该界面，不能凭已有视频 API 编造同名操作流程。 | [原工作台页][r19]; [Gkotta 视频教程][videos] |
| 20 | [Seedance 2.0 视频生成 API 参考][r20] | 迁移重写 | [seedance-api.md](../../frontend/docs/seedance-api.md) | 按本站 /api/v3/contents/generations/tasks 原生任务接口写创建、查询、下载与终态；不照搬原站 /seedance 前缀。 | [平台接口][platform]; [Gkotta Seedance API][seedance] |
| 21 | [Seedance 2.0 视频生成][r21] | 迁移重写 | [seedance-api.md](../../frontend/docs/seedance-api.md)<br>[video-generation.md](../../frontend/docs/video-generation.md) | 在 Seedance 接口与视频教程保留任务式接入、参数/结果处理、费用排查；删掉原站实测并发、免认证和充值加赠宣称。 | [平台接口][platform]; [Gkotta Seedance API][seedance] |
| 22 | [Seedance 2.0 素材库（人物一致性视频）][r22] | 排除 | — | icover.ai 素材库及真人活体认证是参考站旗下服务；Gkotta 未提供这些账号、素材 KEY、API 或可信素材库，不能迁移为本站能力。 | [icover 官方入口][icover]; [原页][r22] |

## 使用场景

| 编号 | 参考站入口 | 处置 | 对应 Gkotta 文档 | 内容边界与理由 | 官方 / 实现证据 |
| --- | --- | --- | --- | --- | --- |
| 23 | [沉浸式翻译][r23] | 迁移重写 | [scenarios/immersive-translate.md](../../frontend/docs/scenarios/immersive-translate.md) | 保留浏览器翻译安装、自定义完整聊天地址、小段验证和批量/费用控制；不承诺扩展付费功能均免费。 | [沉浸式翻译 OpenAI 服务][immersive] |
| 24 | [飞书多维表格 AI 生图方案][r24] | 部分适配 | [scenarios/lark-images.md](../../frontend/docs/scenarios/lark-images.md) | 改写为 Gkotta 现有 Images 接口、用户自建 worker、飞书官方附件上传和记录回写；不依赖原站未公开插件、Coze 包及专属存储。 | [飞书多维表格 API][lark-base]; [附件上传][lark-upload]; [Gkotta 图片接口][images] |
| 25 | [使用场景总览][r25] | 迁移重写 | [scenarios/index.md](../../frontend/docs/scenarios/index.md) | 保留聊天、编程、自动化、翻译与媒体接入导航；仅链接本次有真实兼容教程的工具，删除残留其他站注册入口。 | [平台接口][platform]; [可用渠道][channels] |
| 26 | [Bob 翻译][r26] | 迁移重写 | [scenarios/bob.md](../../frontend/docs/scenarios/bob.md) | 明确 Bob 本体与 OpenAI Translator 插件；当前插件填完整请求地址，修正原页只填域名的做法。 | [Bob 官方指南][bob]; [插件配置手册][bob-plugin] |
| 27 | [CC Switch][r27] | 合并 | [cc-switch.md](../../frontend/docs/cc-switch.md) | 与顶级 CC Switch 安装入口合并，保留供应商、配置备份、切换、生效和路由边界；避免两页版本漂移。 | [CC Switch 手册][ccswitch] |
| 28 | [Chatbox AI][r28] | 迁移重写 | [scenarios/chatbox.md](../../frontend/docs/scenarios/chatbox.md) | 保留安装、自定义 provider、Host/Path 区分、多轮验证和排错；只文档化当前分组可用能力。 | [Chatbox 帮助][chatbox]; [官方地址处理源码][chatbox-source] |
| 29 | [ChatGPT Next Web][r29] | 迁移重写 | [scenarios/nextchat.md](../../frontend/docs/scenarios/nextchat.md) | 按现名 NextChat 重写部署、服务器变量与模型；该工具 BASE_URL 使用根地址，避免重复 /v1。 | [NextChat 环境变量][nextchat]; [官方路径拼接][nextchat-source] |
| 30 | [Cherry Studio][r30] | 迁移重写 | [scenarios/cherry-studio.md](../../frontend/docs/scenarios/cherry-studio.md) | 保留 OpenAI/Anthropic/Gemini 三类协议配置，按实际客户端路径规则重写，删除原站 400+ 模型与缓存价格保证。 | [Cherry Studio 文档][cherry]; [官方地址预览实现][cherry-source] |
| 31 | [Claude Code][r31] | 合并 | [claude-code.md](../../frontend/docs/claude-code.md) | 与终端版安装页合并，补足环境变量、配置、/status 与使用记录验证；不复用原平台专属分组折扣。 | [Claude Code][claude-code]; [网关接入][claude-gateway] |
| 32 | [Cline (VS Code)][r32] | 迁移重写 | [scenarios/cline.md](../../frontend/docs/scenarios/cline.md) | 保留扩展安装、OpenAI Compatible 地址、模型选择、上下文/费用管理与工具权限；不把参考页虚构配置当作当前标准。 | [Cline 官方文档][cline] |
| 33 | [Cursor][r33] | 迁移重写 | [scenarios/cursor.md](../../frontend/docs/scenarios/cursor.md) | 保留自定义 API Key/Base URL 和短对话验证；按当前官方 BYOK 说明区分可用模式，不继承“Cursor 无 Agent”绝对结论。 | [Cursor API Keys][cursor] |
| 34 | [Dify][r34] | 迁移重写 | [scenarios/dify.md](../../frontend/docs/scenarios/dify.md) | 按官方 OpenAI-compatible 插件添加文本模型和最小应用；知识库/嵌入/其他能力须单独确认，避免从聊天兼容推导。 | [Dify 官方文档][dify]; [插件配置字段][dify-source] |
| 35 | [FastClaw][r35] | 迁移重写 | [scenarios/fastclaw.md](../../frontend/docs/scenarios/fastclaw.md) | 官方 dev 分支的 provider 配置支持 apiBase/apiKeyEnv；按自定义 OpenAI 地址写模型绑定与验证，注明版本差异。 | [FastClaw dev README][fastclaw] |
| 36 | [Gemini CLI][r36] | 迁移重写 | [gemini-cli.md](../../frontend/docs/gemini-cli.md) | 保留安装、API Key auth 和自定义根地址；更新到 Node.js 20+，明确 v1beta，删除参考页损坏的 PowerShell 代码。 | [Gemini CLI 安装][gemini-install]; [官方配置][gemini-config] |
| 37 | [GLM-5.2][r37] | 合并 | [domestic-models.md](../../frontend/docs/domestic-models.md) | 保留国产模型接入与选型原则；不保证 GLM-5.2 已上架或复制固定窗口、价格和性能排行。 | [可用渠道][channels]; [智谱官方文档][glm] |
| 38 | [Hermes Agent][r38] | 合并 | [hermes.md](../../frontend/docs/hermes.md) | 与顶级 Hermes 安装入口合并，纳入 CLI/命名 provider/后台环境排错；按当前官方版本配置。 | [Hermes AI Providers][hermes-providers] |
| 39 | [Image Annotator - ComfyUI 节点][r39] | 排除 | — | 官方 luckdvr/comfyui-image-annotator 是本地视觉标注节点，输出图像/JSON，没有参考页声称的自定义模型 API 接入；不能编写不存在的密钥配置。 | [Image Annotator 官方 README][annotator] |
| 40 | [LangChain][r40] | 迁移重写 | [scenarios/langchain.md](../../frontend/docs/scenarios/langchain.md) | 使用官方 ChatOpenAI 的 base_url/api_key 保留文本、流式、异步和工作流；RAG/embedding 独立能力不从兼容聊天自动推导。 | [ChatOpenAI 官方参考][langchain] |
| 41 | [Luck GPT-Image 2 - ComfyUI 节点][r41] | 排除 | — | 官方 Luck GPT 图像节点 README/实现只列 APIYi 固定地址选项；未确认开放任意 Gkotta Base URL，不能称为可直接兼容的节点。 | [节点官方 README][luck-gpt]; [节点官方仓库][luck-gpt-repo] |
| 42 | [Make.com 接入 Gemini 图像理解][r42] | 迁移重写 | [scenarios/make-gemini-vision.md](../../frontend/docs/scenarios/make-gemini-vision.md) | 保留 Make HTTP 模块发送 Gemini 原生图像理解请求、读取结果和错误分支；模型能力以当前分组为准。 | [Make HTTP 文档][make]; [Gemini 图像理解][gemini-image] |
| 43 | [Open WebUI][r43] | 迁移重写 | [scenarios/open-webui.md](../../frontend/docs/scenarios/open-webui.md) | 按官方 OpenAI-compatible 连接和环境变量配置文本模型；知识库、联网与其他工具另需自己的依赖和授权。 | [Open WebUI 官方连接][webui]; [官方变量源码][webui-source] |
| 44 | [OpenAI Codex][r44] | 合并 | [codex.md](../../frontend/docs/codex.md) | 与顶级 Codex 安装入口合并，统一 auth.json/config.toml；修正参考页旧协议兜底及混用认证字段。 | [Codex 官方配置][codex-config] |
| 45 | [OpenCode][r45] | 合并 | [opencode.md](../../frontend/docs/opencode.md) | 与顶级 OpenCode 安装入口合并，包含自定义 provider、/connect、/models 与 SDK 协议选择。 | [OpenCode 官方供应商][opencode] |
| 46 | [Paper2Any 论文多模态工作流][r46] | 迁移重写 | [scenarios/paper2any.md](../../frontend/docs/scenarios/paper2any.md) | 按当前 OpenDCAI README 的 SIMPLE_TEXT_API_URL/API_KEY 配置文本能力；图像与 GPU 工作流独立确认，不搬旧 DEFAULT_LLM 变量。 | [Paper2Any 官方 README][paper2any] |
| 47 | [Qwen3.6 系列文本模型][r47] | 合并 | [domestic-models.md](../../frontend/docs/domestic-models.md) | 保留 Qwen 类模型的协议接入原则；不承诺 Qwen3.6 全部版本、价格阶梯、开源托管或性能数据。 | [可用渠道][channels]; [阿里云模型文档][qwen] |
| 48 | [Roo Code (VS Code)][r48] | 迁移重写 | [scenarios/roo-code.md](../../frontend/docs/scenarios/roo-code.md) | 保留扩展安装、自定义地址、多模式模型和验证；使用真实 model ID，不承诺任意模型完整 Agent 能力。 | [Roo Code 官方 OpenAI Compatible][roo] |
| 49 | [Trae][r49] | 迁移重写 | [scenarios/trae.md](../../frontend/docs/scenarios/trae.md) | 保留自定义模型入口及协议选择，按当前版本实际字段区分完整路径与根地址；不复制原站优惠或固定模型清单。 | [Trae 官方文档][trae] |

## 不继承的原站信息

所有迁移页移除原站的联系人、客服群、独立域名、活动有效期、退款/发票/SLA 承诺、固定折扣、固定并发、固定上架模型数量与性能保证。实际费用、余额、订阅和密钥额度指向 Gkotta 控制台。

固定型号文章不通过改品牌就宣称本站上架；客户端教程使用可替换的真实模型 ID。图像、视频、工具调用、嵌入与缓存等能力分别按已有接口及分组说明，不从基础聊天成功推导额外功能。

参考页还存在其他站注册链接、损坏的命令块及过期配置。以第三方官方文档和本站现有实现核对后重写：例如 Codex 用 Responses、Gemini CLI 用 API Key 认证与自定义根地址、Hermes 使用当前 provider/transport，而非原页旧式 `llm.*` 字段。

## 证据链接

[platform]: ../../backend/internal/server/routes/gateway.go
[auth]: ../../backend/internal/server/middleware/api_key_auth.go
[google-auth]: ../../backend/internal/server/middleware/api_key_auth_google.go
[users]: ../../backend/internal/server/routes/user.go
[images]: ../../frontend/docs/image-generate.md
[videos]: ../../frontend/docs/video-generation.md
[seedance]: ../../frontend/docs/seedance-api.md
[channels]: https://www.gkotta.bid/available-channels
[usage]: https://www.gkotta.bid/usage
[vitepress]: https://vitepress.dev/
[codex]: https://developers.openai.com/codex/cli
[codex-config]: https://developers.openai.com/codex/config-reference
[openai-sdk]: https://github.com/openai/openai-python
[openai-reference]: https://platform.openai.com/docs/api-reference
[openai-images]: https://platform.openai.com/docs/api-reference/images
[ccswitch]: https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/README.md
[cc-install]: https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/1-getting-started/1.2-installation.md
[cc-desktop]: https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/2-providers/2.6-claude-desktop.md
[claude-code]: https://code.claude.com/docs/en/quickstart
[claude-3p]: https://claude.com/docs/third-party/claude-desktop/overview
[claude-gateway]: https://code.claude.com/docs/en/llm-gateway
[codex-plus]: https://github.com/xianyu110/CodexPlusPlus/blob/master/README.md
[hermes-install]: https://hermes-agent.nousresearch.com/docs/getting-started/installation
[hermes-providers]: https://hermes-agent.nousresearch.com/docs/integrations/providers
[openclaw-install]: https://docs.openclaw.ai/install
[openclaw-custom]: https://docs.openclaw.ai/concepts/model-providers/custom-providers
[opencode]: https://opencode.ai/docs/providers/#custom-provider
[opencode-config]: https://opencode.ai/docs/config/
[icover]: https://icover.ai/zh/seedance-official/asset-library
[immersive]: https://immersivetranslate.com/docs/services/openai/
[lark-base]: https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/create
[lark-upload]: https://open.feishu.cn/document/server-docs/docs/drive-v1/media/upload_all
[bob]: https://bobtranslate.com/guide/
[bob-plugin]: https://github.com/nextai-translator/bob-plugin-openai-translator/blob/main/docs/configuration_manual_CN.md
[chatbox]: https://chatboxai.app/en/help-center
[chatbox-source]: https://github.com/chatboxai/chatbox/blob/main/src/shared/utils/llm_utils.ts
[nextchat]: https://github.com/ChatGPTNextWeb/NextChat#environment-variables
[nextchat-source]: https://github.com/ChatGPTNextWeb/NextChat/blob/main/app/api/common.ts
[cherry]: https://docs.cherry-ai.com/
[cherry-source]: https://github.com/CherryHQ/cherry-studio/blob/main/src/renderer/pages/settings/ProviderSettings/hooks/providerSetting/buildHostEndpointPreviews.ts
[cline]: https://docs.cline.bot/provider-config/openai-compatible
[cursor]: https://docs.cursor.com/settings/api-keys
[dify]: https://docs.dify.ai/
[dify-source]: https://github.com/langgenius/dify-official-plugins/blob/main/models/openai_api_compatible/provider/openai_api_compatible.yaml
[fastclaw]: https://github.com/fastclaw-ai/fastclaw/blob/dev/README.md
[gemini-install]: https://www.geminicli.com/docs/get-started/installation
[gemini-config]: https://www.geminicli.com/docs/reference/configuration
[glm]: https://docs.bigmodel.cn/
[annotator]: https://github.com/luckdvr/comfyui-image-annotator/blob/main/README.md
[langchain]: https://reference.langchain.com/python/integrations/langchain_openai/ChatOpenAI/
[luck-gpt]: https://github.com/luckdvr/Comfyui-Luck-gpt2.0/blob/main/README.md
[luck-gpt-repo]: https://github.com/luckdvr/Comfyui-Luck-gpt2.0
[make]: https://help.make.com/http
[gemini-image]: https://ai.google.dev/gemini-api/docs/image-understanding
[webui]: https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai-compatible
[webui-source]: https://github.com/open-webui/open-webui/blob/main/backend/open_webui/config.py
[paper2any]: https://github.com/OpenDCAI/Paper2Any/blob/main/README.md
[qwen]: https://help.aliyun.com/zh/model-studio/models
[roo]: https://docs.roocode.com/providers/openai-compatible
[trae]: https://docs.trae.ai/

[r01]: https://docs.fastaitoken.com/docs/
[r02]: https://docs.fastaitoken.com/docs/easyuse
[r03]: https://docs.fastaitoken.com/docs/getting-started
[r04]: https://docs.fastaitoken.com/docs/cc-switch
[r05]: https://docs.fastaitoken.com/docs/claude-code
[r06]: https://docs.fastaitoken.com/docs/claude-desktop
[r07]: https://docs.fastaitoken.com/docs/codex
[r08]: https://docs.fastaitoken.com/docs/codex-plus
[r09]: https://docs.fastaitoken.com/docs/fastai_%E5%9B%BE%E7%89%87codex%E8%B0%83%E7%94%A8
[r10]: https://docs.fastaitoken.com/docs/hermes
[r11]: https://docs.fastaitoken.com/docs/openclaw
[r12]: https://docs.fastaitoken.com/docs/opencode
[r13]: https://docs.fastaitoken.com/docs/api-manual
[r14]: https://docs.fastaitoken.com/docs/%E5%9B%BD%E6%A8%A1%E4%BD%BF%E7%94%A8%E6%96%87%E6%A1%A3%EF%BC%88%E9%80%9A%E7%94%A8%EF%BC%89
[r15]: https://docs.fastaitoken.com/docs/image-generate
[r16]: https://docs.fastaitoken.com/docs/pricing
[r17]: https://docs.fastaitoken.com/docs/help
[r18]: https://docs.fastaitoken.com/docs/%E4%BA%A7%E5%93%81%E5%9F%BA%E7%A1%80/%E7%B4%A0%E6%9D%90%E5%BC%95%E7%94%A8%E7%94%9F%E8%A7%86%E9%A2%91%E5%AE%9E%E6%88%98
[r19]: https://docs.fastaitoken.com/docs/%E4%BA%A7%E5%93%81%E5%9F%BA%E7%A1%80/AI%E8%A7%86%E9%A2%91%E5%B7%A5%E4%BD%9C%E5%8F%B0
[r20]: https://docs.fastaitoken.com/docs/%E4%BA%A7%E5%93%81%E5%9F%BA%E7%A1%80/Seedance%202.0%20%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90%20API%20%E5%8F%82%E8%80%83
[r21]: https://docs.fastaitoken.com/docs/%E4%BA%A7%E5%93%81%E5%9F%BA%E7%A1%80/Seedance%202.0%20%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90
[r22]: https://docs.fastaitoken.com/docs/%E4%BA%A7%E5%93%81%E5%9F%BA%E7%A1%80/Seedance%202.0%20%E7%B4%A0%E6%9D%90%E5%BA%93%EF%BC%88%E4%BA%BA%E7%89%A9%E4%B8%80%E8%87%B4%E6%80%A7%E8%A7%86%E9%A2%91%EF%BC%89
[r23]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/%E6%B2%89%E6%B5%B8%E5%BC%8F%E7%BF%BB%E8%AF%91
[r24]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/%E9%A3%9E%E4%B9%A6%E5%A4%9A%E7%BB%B4%E8%A1%A8%E6%A0%BC%20AI%20%E7%94%9F%E5%9B%BE%E6%96%B9%E6%A1%88
[r25]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF%E6%80%BB%E8%A7%88
[r26]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Bob%20%E7%BF%BB%E8%AF%91
[r27]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/CC%20Switch
[r28]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Chatbox%20AI
[r29]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/ChatGPT%20Next%20Web
[r30]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Cherry%20Studio
[r31]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Claude%20Code
[r32]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Cline%20%28VS%20Code%29
[r33]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Cursor
[r34]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Dify
[r35]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/FastClaw
[r36]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Gemini%20CLI
[r37]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/GLM-5.2
[r38]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Hermes%20Agent
[r39]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Image%20Annotator%20-%20ComfyUI%20%E8%8A%82%E7%82%B9
[r40]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/LangChain
[r41]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Luck%20GPT-Image%202%20-%20ComfyUI%20%E8%8A%82%E7%82%B9
[r42]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Make.com%20%E6%8E%A5%E5%85%A5%20Gemini%20%E5%9B%BE%E5%83%8F%E7%90%86%E8%A7%A3
[r43]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Open%20WebUI
[r44]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/OpenAI%20Codex
[r45]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/OpenCode
[r46]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Paper2Any%20%E8%AE%BA%E6%96%87%E5%A4%9A%E6%A8%A1%E6%80%81%E5%B7%A5%E4%BD%9C%E6%B5%81
[r47]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Qwen3.6%20%E7%B3%BB%E5%88%97%E6%96%87%E6%9C%AC%E6%A8%A1%E5%9E%8B
[r48]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Roo%20Code%20%28VS%20Code%29
[r49]: https://docs.fastaitoken.com/docs/%E4%BD%BF%E7%94%A8%E5%9C%BA%E6%99%AF/Trae
