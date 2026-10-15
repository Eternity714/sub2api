# GRS.AI 媒体模型能力核实

核实日期：2026-10-15（Asia/Shanghai）。本文件是当前 PRD 的事实输入，不代表已经实现这些能力。仅访问公开官方文档和仓库源码；未调用生成接口、查询真实任务或读取账号凭证。

## 1. 覆盖口径与结论

- 官方当前模型目录登记 **33 个模型，其中图片 16 个、视频 2 个、文本 15 个**；本文件完整列出这 18 个媒体模型。[S1]
- 18 个媒体模型中，目录将 `gpt-image-2`、`gpt-image-2.5`、`gpt-image-2.5-flare`、`gpt-image-2.5-sunburst` 标为维护中；其余 14 个未标维护。这是公开目录状态，不是对真实账号调用成功率的保证。[S1]
- 2026-10-15 09:20 公告称 OpenAI 算法更新影响 `gpt-image-2` / `gpt-image-2.5`；10-05 公告称 flare/sunburst 因资源不足维护。旧公告中的“已恢复”不可覆盖更新公告。[S2]
- 新官方文档的原生媒体目录有 **4 个生成接口文档页（2 图片族、2 视频族）+ 1 个统一异步结果页**，均使用 `POST /v1/api/generate` 和 `GET /v1/api/result?id=...`。[S3–S8]
- 新生成文档额外列出 `nano-banana`、`nano-banana-pro-vt`，却未列目录中的 `nano-banana-2-lite`。因此目录与接口文档的模型名并集为 **20 个**，不能直接把文档并集当作当前可用名单。[S1、S2、S4]
- 仓库已有 GRS 原生 HTTP 客户端、异步轮询与图片/视频转存，但**没有逐模型能力白名单**。可透传某模型名不等于已实现该模型的严格 schema、完整参数映射和覆盖验证（见第 6 节）。

## 2. 当前目录的逐模型矩阵

所有下列模型的原生创建/查询/响应方式共用第 3 节；`n` 不是 GRS 原生数量字段，统一接口的多张需求须按已确认规则拆成子任务。[S4–S8]

| 上游 model（精确 ID） | 媒体 | 目录明确的输出规格 | 原生参数族 | 2026-10-15 目录状态 |
|---|---|---|---|---|
| `nano-banana-2.1` | image | 1K / 2K / 4K | Nano | 未标维护 |
| `nano-banana-pro` | image | 1K / 2K / 4K | Nano | 未标维护 |
| `nano-banana-2` | image | 1K / 2K / 4K | Nano | 未标维护 |
| `nano-banana-2-lite` | image | 目录未逐一说明分辨率 | Nano；新接口文档漏列此 ID | 未标维护 |
| `nano-banana-fast` | image | 目录未逐一说明分辨率 | Nano | 未标维护 |
| `nano-banana-pro-cl` | image | 仅 1K | Nano | 未标维护 |
| `nano-banana-2-cl` | image | 仅 1K | Nano | 未标维护 |
| `nano-banana-2-2k-cl` | image | 仅 2K | Nano | 未标维护 |
| `nano-banana-pro-vip` | image | 1K / 2K | Nano | 未标维护 |
| `nano-banana-pro-4k-vip` | image | 仅 4K | Nano | 未标维护 |
| `nano-banana-2-4k-cl` | image | 仅 4K | Nano | 未标维护 |
| `gpt-image-2` | image | 1K | GPT | 维护中 |
| `gpt-image-2-vip` | image | 1K / 2K / 4K | GPT | 未标维护 |
| `gpt-image-2.5` | image | 1K | GPT | 维护中 |
| `gpt-image-2.5-flare` | image | 1K / 2K / 4K；quality: low / medium / high | GPT | 维护中 |
| `gpt-image-2.5-sunburst` | image | 1K / 2K / 4K；quality: low / medium / high / xhigh / max | GPT | 维护中 |
| `minimax-h3` | video | 480p / 768p：1–15 秒；1080p：1–10 秒 | MiniMax | 未标维护 |
| `agnes-video-2.5` | video | 720p / 1080p / 2k；4–12 秒 | Agnes | 未标维护 |

依据：[S1] 当前模型目录；字段细节用 [S4–S7] 的 OpenAPI 原文核对。此表不使用上游价格作为 Sub2API 下游收费依据；下游仍按 Sub2API 模型与输出规格定价。

目录之外的两个文档 ID：

| ID | 官方证据 | 处理建议 |
|---|---|---|
| `nano-banana` | 新接口页仍列出；2026-07-01 公告明确下架 | 不生成默认可用模板；保留文档冲突记录，不能据旧示例宣称可用 |
| `nano-banana-pro-vt` | 新接口页列出；当前模型目录无此 ID，未找到当前状态/规格说明 | 不自动启用；需补当前服务及规格依据后才发布 schema |

`nano-banana-2-lite` 与 `nano-banana-fast`：2026-07-01 公告称底层均为 `gemini-3.1-flash-lite-image`，但这不自动证明两者任意分辨率、比例、参考图数量完全相同。[S2]

## 3. 原生协议与统一接口映射

| 项目 | 官方原生协议 | 对统一接口的含义 |
|---|---|---|
| 创建 | `POST /v1/api/generate`，JSON，Bearer API Key | 下游创建路径由图片/视频区分；adapter 按能力配置映射到同一上游路径 |
| 回复控制 | `replyType=json / stream / async`；async 返回 `id` 和 `status=running` | 服务端固定 `replyType=async`，不作为下游可配置 metadata |
| 查询 | `GET /v1/api/result?id={upstream_id}`，Bearer API Key | 绑定已受理账号查询；下游查询只读本地任务 |
| 上游状态 | `running / violation / succeeded / failed` | 下游继续按既定 `queued / in_progress / completed`，不暴露上游内部状态 |
| 成功结果 | `results` 数组，元素含 `url`；图片/视频示例均同形 | 下载并转存后输出统一 `result.data: string[]`；不能将上游临时 URL 直接交付 |
| 失败 | `error: string`；文档示例存在 HTTP 400 且带任务 id/status | 解析受理与失败需看响应体，不能一概认为所有非 2xx 都“未受理，可重试生成” |
| 结果生命周期 | 09-15 公告称图片和视频链接有效期 2 小时 | 尽早转存；该 2 小时不能当 Sub2API S3 的保留时长 |
| 数量 | 4 族原生请求 schema 均无 `n` 数量参数 | 统一 `n` 通过拆分实现；单次 1 张候选仍可以支持统一 `n>1` |

依据：[S2、S4–S8]。本表是协议转换需求输入，受理不明时避免重复提交的具体判据仍应在技术方案中细化。

## 4. 各族字段、约束与缺口

### 4.1 Nano 图片族

原生字段：`model:string`、`prompt:string`（二者必填），`images:string[]`、`aspectRatio:string`、`imageSize:string`、`replyType:string`。[S4]

| 统一字段 | 原生字段 | 官方约束与注意点 |
|---|---|---|
| `model` | `model` | 当前目录型号见第 2 节，必须区分已下架/未证实 ID |
| `prompt` | `prompt` | 文档未给最大长度，不宜凭其他厂商限制直接套用 |
| `image_urls` | `images` | 原生接受 URL 或 Base64；下游目前是 URL 输入，原生最大数量及 URL 图片尺寸/格式限制未完整给出 |
| `size` | `aspectRatio` | `auto, 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 5:4, 4:5, 21:9`；`nano-banana-2` 系列额外列 `1:4,4:1,1:8,8:1`，但“2.1 / lite 是否均继承”未逐一写明 |
| `metadata.resolution` | `imageSize` | 族文档列 1K/2K/4K；逐模型必须再受目录规格约束（如 pro-cl 仅 1K） |
| `n` | 子任务数 | 每次上游请求一个生成任务；不向上游发送 n |

族 schema 的“1K/2K/4K”不能覆盖单型号的较窄能力，不能对所有 Nano ID 默认全放开。文档没有明确省略 `aspectRatio` / `imageSize` 的默认值，示例值不应当作全型号默认值。

### 4.2 GPT 图片族

原生字段：`model:string`、`prompt:string`（必填），`images:string[]`、`aspectRatio:string`、`quality:string`、`background:string`、`mask:string`、`replyType:string`。[S5]

| 统一字段 | 原生字段 | 官方约束与注意点 |
|---|---|---|
| `image_urls` | `images` | URL/Base64；参考图最大数量、总大小与相互依赖未明确 |
| `size` + `metadata.resolution` | `aspectRatio` | `gpt-image-2` / `2.5` 支持比例或 1K 像素值；vip / flare / sunburst 只接收具体像素值，不能直接转发比例 |
| `metadata.quality` | `quality` | `2/2.5:auto`；`2-vip:medium`；`2.5-flare:low/medium/high`；`2.5-sunburst:low/medium/high/xhigh/max`。不得把族枚举全开放给每个型号 |
| `metadata.background` | `background` | 已列 `transparent`，支持者：2-vip / 2.5-flare / 2.5-sunburst |
| `metadata.mask_url` | `mask` | 原文只给 PNG URL 示例；透明通道要求、与输入图数量/尺寸的组合约束、逐型号支持未明确，需要补证据后发布相应 schema |
| `n` | 子任务数 | 原生 schema 无数量字段 |

官方给出的 1K / 2K / 4K 像素映射（vip / flare / sunburst）如下。统一接口只接受比例，因此这些属于 adapter 的规格映射依据，不能要求下游改传像素字符串。

| 比例 | 1K | 2K | 4K |
|---|---|---|---|
| `1:1` | 1024×1024 | 2048×2048 | 2880×2880 |
| `16:9` | 1280×720 | 2048×1152 | 3840×2160 |
| `9:16` | 720×1280 | 1152×2048 | 2160×3840 |
| `4:3` | 1152×864 | 2304×1728 | 3264×2448 |
| `3:4` | 864×1152 | 1728×2304 | 2448×3264 |
| `3:2` | 1536×1024 | 2048×1360 | 3504×2336 |
| `2:3` | 1024×1536 | 1360×2048 | 2336×3504 |
| `5:4` | 1120×896 | 2240×1792 | 3200×2560 |
| `4:5` | 896×1120 | 1792×2240 | 2560×3200 |
| `21:9` | 1456×624 | 2912×1248 | 3840×1648 |
| `9:21` | 624×1456 | 1248×2912 | 1648×3840 |
| `2:1` | 1536×768 | 3072×1536 | 3840×1920 |
| `1:2` | 768×1536 | 1536×3072 | 1920×3840 |

文档另列 `1:3: 688×2048、1280×3840` / `3:1: 2048×688、3840×1280`，未明确对应分辨率档位；不得自行补出三档。自定义像素约束注明“仅限 vip”：边长 ≤3840、每边为 16 倍数、长短比 ≤3、总像素 655,360–8,294,400；其对 flare/sunburst 的适用范围没有写清。上表有近似比例，统一文档应说明尺寸按模型规格取值，不能承诺任意精确像素。

### 4.3 MiniMax 视频族

`model=minimax-h3`；必填：`prompt`、`aspectRatio`、`resolution`、`model`、`duration`、`replyType`。可选：`images:string[]`（最多 9）、`audios:string[]`（最多 3）、`seed:integer`。[S6]

| 统一字段 | 原生字段 | 能力 |
|---|---|---|
| `duration` | `duration` | 整数；480p/768p: 1–15；1080p: 1–10 |
| `aspect_ratio` | `aspectRatio` | 原生只有 `portrait` / `landscape`；计划可将 9:16/16:9 映射到竖屏/横屏，其精确实际像素比需验证，禁止放开任意比例 |
| `image_urls` | `images` | 最多 9；原生也可 Base64 |
| `metadata.resolution` | `resolution` | `480p / 768p / 1080p` |
| 待确定的 metadata 音频字段 | `audios` | 最多 3，URL/Base64；音频时长/格式/大小/与图片组合约束未明 |
| `metadata.seed` | `seed` | integer，未给范围/默认值 |

### 4.4 Agnes 视频族

`model=agnes-video-2.5`；必填同 MiniMax；可选 `images:string[]`、`audios:string[]`、`seed:integer`。[S7]

| 统一字段 | 原生字段 | 能力 |
|---|---|---|
| `duration` | `duration` | 整数 4–12 秒 |
| `aspect_ratio` | `aspectRatio` | `16:9 / 9:16 / 21:9 / 4:3 / 1:1 / 3:4` |
| `image_urls` | `images` | 最多 5；URL/Base64 |
| 待确定的 metadata 音频字段 | `audios` | 最多 3；URL/Base64，其他输入限制未明 |
| `metadata.resolution` | `resolution` | 原生枚举大小写为 `720p / 1080p / 2k`，与图片 2K 应分别做规范化映射 |
| `metadata.seed` | `seed` | integer，未给范围/默认值 |

模型目录称支持“首尾帧”，新接口 schema 却没有 `first_frame` / `last_frame` / `mode` 字段，也没有说明 images 前两项是否自动代表首尾帧。**不能据模型介绍自行发明字段或排序语义。** 文生视频、一般图片/音频参考有明确字段；指定首尾帧行为需要进一步官方证据。[S1、S7]

## 5. 旧文档、兼容格式与本期边界

当前官方站仍保留旧文档，但以下模型/族没有进入 2026-10-15 当前模型目录；旧页面存在不能证明当前账号可调用。[S1、S9–S11]

| 旧文档族/操作 | 创建路径 | 查询/特有输入 | 本期处理依据 |
|---|---|---|---|
| Veo | `POST /v1/video/veo` | `POST /v1/draw/result`，页面展示 16:9/9:16 | 未列当前目录，不当作本期已核实可用模型；不改项目其他平台入口 |
| Sora 2 | `POST /v1/video/sora-video` | `POST /v1/draw/result`；旧页展示参考图、10/15秒、small/large | 同上；旧页面不构成新原生 async 路径支持证明 |
| Sora 上传角色 | `POST /v1/video/sora-upload-character` | 特定角色资源创建操作 | 非单纯生成；须单独讨论，不塞入本期通用生成协议 |
| Sora 从视频创建角色 | `POST /v1/video/sora-create-character` | 从既有视频创建角色 | 同上 |
| Imagen | `POST /v1/draw/imagen` | `POST /v1/draw/result`，页面展示 1:1/16:9/9:16/4:3/3:4 | 未列当前目录，不当作已核实可用模型 |

新文档还列 Gemini `generateContent` / `streamGenerateContent`、OpenAI `chat/completions`、`images/generations`、`images/edits` 兼容格式。[S3] 本期新统一 adapter 优先覆盖已核实的原生媒体协议，不重构这些已有平台格式入口。`image_urls` 参考图和用户已举例的 `metadata.mask_url` 属于生成请求的输入能力；不能仅因相关上游文档叫“编辑”就将其一概排除。增强、变体、延长、角色资源管理是独立操作，另议。

没有在当前 GRS 新官方目录/接口页找到 Midjourney 四宫格或增强/变体模型。用户已经定下的四宫格交付计费规则仍适用于支持它的候选平台；不能把它记作 GRS 已验证能力。

## 6. 仓库支持情况（与官方支持分开）

通过 CodeGraph 先定位，再核对当前源码。以下证据是本地实现事实，不能替代上游能力文档。

| 本地事实 | 代码依据 | 对本期工作的含义 |
|---|---|---|
| 原生创建/查询路径固定为 `/v1/api/generate` / `/v1/api/result` | `backend/internal/service/grsai_native.go:18` | 现有 HTTP 协议骨架可复用 |
| async 提交要求上游 body 为 `replyType=async`，查询使用 GET 的 id 参数 | `backend/internal/service/grsai_native.go:128`、`:155` | 已有异步协议边界，无需重造 GRS 查询路径 |
| legacy body 仅处理 stream/async/replyType 控制，其他模型字段不做能力解释 | `backend/internal/service/grsai_native.go:147`、`:292` | 属于宽松透传，不能称为已覆盖全模型 schema |
| durable 解析保留其他字段，强制 `replyType=async`，删除 n/numImages 等数量别名，固定 `ImageCount=1` | `backend/internal/service/grsai_delivery.go:89`、`:112` | 当前无统一 n 拆分；未来需新增父/子任务语义而非把 n 转发 |
| 视频能力硬编码只对 minimax-h3 限定 480p/768p/1080p 及最长时长；其他型号只要求 duration>0 | `backend/internal/service/grsai_video_pricing.go:11` | Agnes 4–12秒/比例/参考图/音频等完整约束尚未实现；不能据透传断言支持 |
| 创建按后台配置的 billing mode 判定 image/video；原始分组须为 grsai | `backend/internal/service/grsai_task_service.go:46`、`:57`、`:97` | 现有任务媒体类型不是由公共端点+能力 schema 决定，需要改造 |
| 视频按配置分辨率价格×duration 冻结，image/per_request 同样要求价格配置 | `backend/internal/service/grsai_task_service.go:53`、`:85`；`backend/internal/service/grsai_settlement.go:237` | 没有逐个型号固定支持清单；定价配置与可调用能力不同，不能互相替代 |
| worker 绑定上游 id，按原账号轮询，成功后按媒体类型持久化再结算 | `backend/internal/service/grsai_task_runtime.go:158`、`:194`、`:201`、`:229` | 保留异步 worker、S3 转存与结算顺序，增加统一协议所需严格状态与部分成功处理 |

固定路径的现有客户端不能直接用来覆盖旧 `/v1/draw/*` / `/v1/video/*` 协议；这些路径的请求、返回、回调/流式语义另有契约，需要独立 adapter 才能宣称支持。原生 async 的 HTTP 传输、Bearer 鉴权、响应解析和已有轮询骨架才是直接可复用部分。

因此当前源码能够证明的是“通用 GRS 原生 transport + 持久图片任务 + 带 MiniMax 特定时长校验的视频任务”，不能给出一个等同于 18 模型完整验证的支持数量。仓库测试中的 `gpt-image-2`、`nano-banana`、`nano-banana-2`、`minimax-h3` 是测试输入示例，不是实时上游可用声明。

## 7. 收敛项：工程核实与产品决策分开

### 可在技术方案中处理，不必让用户重复定稿

1. 定义“目录登记、维护暂停、schema已确认、管理员启用”的区别，避免把 4 个维护模型或文档残留型号默认当当前可调度账号。
2. 核实 fast/lite 分辨率、Nano 参考图数、mask 组合约束和缺省行为；未证实能力不扩充严格白名单。
3. 用显式比例×分辨率表映射 GPT 像素尺寸，并记录供应商输出规格近似值；缺失组合不降规格。
4. 核实 Agnes 首尾帧在原生协议中的表达方式；没有证据时不要宣称已支持。
5. 根据真实受理响应分类，明确带 id 的错误、超时及空响应的“已受理/不明”处理，复用既有避免重复提交约束。

### 可能需要产品定稿的具体边界

- 两个当前视频族都具备参考音频输入。统一视频顶层已定不新增字段，需在跨平台对照完成后，确认是否统一用一个有明确类型的 metadata 音频 URL 字段（例如 `metadata.audio_urls:string[]`），以及首期是否包含这类生成输入。此处仅提候选字段，**未当作已定协议**。
- `agnes-video-2.5` 目录宣传的指定首尾帧与仅普通图片参考不同；先补官方语义证据，再判断是否需新增 metadata 模式/字段。无法证实不是要求用户替上游定义语义。

模型 ID 的机器可读清单见同目录 `grsai-models.json`；当前目录、额外文档型号和历史族分列，不合并统计。

## 8. 官方来源

| 编号 | 官方来源 | 核实内容 |
|---|---|---|
| S1 | https://grsai.com/zh/dashboard/models | 当前全目录、型号媒体类型、逐模型分辨率、维护标记 |
| S2 | https://grsai.com/zh/dashboard/announcements | 10-15维护、10-07新增2.1、09-15 MiniMax规格、07-01下架/替代信息、2小时链接期限 |
| S3 | https://qmy27nhsd9.apifox.cn/llms.txt | 官方新接口索引（由官网公告直接链接该 Apifox 项目，非第三方教程） |
| S4 | https://qmy27nhsd9.apifox.cn/452392911e0.md | Nano 原生完整 OpenAPI |
| S5 | https://qmy27nhsd9.apifox.cn/452409160e0.md | GPT 原生完整 OpenAPI及像素/quality约束 |
| S6 | https://qmy27nhsd9.apifox.cn/514679297e0.md | MiniMax 原生完整 OpenAPI |
| S7 | https://qmy27nhsd9.apifox.cn/523334530e0.md | Agnes 原生完整 OpenAPI |
| S8 | https://qmy27nhsd9.apifox.cn/452409577e0.md | 统一异步结果查询 OpenAPI |
| S9 | https://grsai.com/zh/dashboard/documents/veo | 旧 Veo 页面及路径 |
| S10 | https://grsai.com/zh/dashboard/documents/sora-2 | 旧 Sora 页面、角色管理路径 |
| S11 | https://grsai.com/zh/dashboard/documents/imagen | 旧 Imagen 页面及路径 |

检索工具能够读取 S1/S2 与部分 HTML，但多次读取 Apifox HTML超时；S3–S8 已通过公开 HTTPS GET 直接读取其 `.md` OpenAPI，未使用缓存教程替代。此处报告“文档未明确”表示所列官方证据未明确，不表示服务一定不支持。
