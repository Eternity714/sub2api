# ToAPIs 图片／视频生成能力核实

核实日期：2026-10-15。本文只读取 ToAPIs 官方公开文档，未请求真实生成、未读取任何账号或密钥。入口：[公开模型目录](https://docs.toapis.com/docs/cn/api-reference/models)、[完整文档索引](https://docs.toapis.com/llms.txt)。公开目录自身标注的目录快照日期是 **2026-09-30**；核实日期不代表目录已刷新到当天。

本文是上游能力证据和映射研究，不替代 PRD，也不自行扩大已确认的首期操作范围。下文“待核实”不代表模型不可用，而是官方公开证据尚不足以安全启用严格 schema。

## 1. 覆盖范围与目录边界

- 对公开目录的 Markdown 表格机械计数：图片区 **41 条**（标题写 38），视频区 **49 条**。不能直接使用目录标题作覆盖率分母。
- 图片目录含 `grok-imagine-1.0-edit` 编辑专用型号；并非全部条目都是普通生成。`gpt-4o-image` 在文本区但有独立图片生成文档。`grok-4-1-image`、`grok-4-2-image`、`midjourney` 有生成文档，却不在图片区。
- 视频文档还明确列出目录视频区之外的 `sora-2-vvip`、`gemini-omni-flash`；后者文档明确给出 `gemini_omni_flash` 别名。Midjourney 的 `video` 操作另有生成文档。
- 文本区的 `gemini-3.1-pro`、`gemini-3.5-flash`、`gemini-3.5-thinking` 标有 `image-generation`，但本轮没有发现相应的专属图片参数页；仅登记为待核实线索，不从端点名称猜出能力 schema。
- 图片区大量型号同时标注 `openai-video`。这是目录端点元数据，**不能据此把图片模型注册成视频输出模型**。
- 已沿索引读取 98 份图片、视频、任务、模型列表及相关说明页；以下逐族表保留全部目录 ID，并另列文档新增 ID。抓取数量不等于模型数量。
- 实际账号可用范围必须以后端账号接入时取得的能力为准。[列出模型](https://docs.toapis.com/docs/cn/api-reference/chat/list-models)说明：`GET /v1/models` 默认只返回文字；`?type=image|video|all` 按 API Key 限制、用户组、可用渠道和计费配置动态过滤。公开价格目录、模型列出成功、报价成功均不能证明每种输入模式都有可用生成渠道。此接口属于上游管理员接入验证，不等于本期向下游新增动态能力查询。

## 2. 通用提交、轮询与结果契约

| 类型 | 上游提交 | 上游查询 | 执行与结果 |
| --- | --- | --- | --- |
| 普通图片 | `POST /v1/images/generations` | `GET /v1/images/generations/{task_id}` | 全部为异步图片任务；终态成功 `result.type=image`，`result.data[]` 是含 `url` 的对象，不能原样当成 Sub2API 的 `string[]` |
| 普通视频 | `POST /v1/videos/generations` | `GET /v1/videos/generations/{task_id}` | 全部为异步视频任务；成功 `result.type=video`，`data[]` 对象含 `url`、可选 `format`、可选 `last_frame_url` |
| Midjourney Imagine | `POST /v1/midjourney/generations/imagine`；默认 `/generations` 等价 | `GET /v1/midjourney/{task_id}`；另有 `/v1/tasks/{task_id}` 别名 | 独立异步协议；四宫格 `grid_image_url`，另可能有 `image_urls`、`buttons` 等 |
| Midjourney Video | `POST /v1/midjourney/generations/video` | 同 Midjourney 查询 | 独立异步任务；结果 `video_urls` |

通用鉴权为 `Authorization: Bearer <ToAPIs API Key>`；使用平台 Key，不使用原厂 Azure／Google Key。普通提交为 JSON；不需要下游调用素材上传或传原厂凭证。Omni 1.1 普通／VIP 的独立页面明确使用 `GET /v1/videos/{id}`，与通用视频任务页路径不同，应为该具体渠道保留差异并验证查询适配。大多数页面使用 `https://toapis.com` / `https://toapis.cn`；较新的 GPT-Image-2.5 页使用 `https://api.toapis.com` / `https://api.toapis.cn`。配置 ToAPIs 账号时应验证管理员所选 Base URL 的完整路径与认证，不能仅按文档写法静默切换域名。

[图片任务查询](https://docs.toapis.com/docs/cn/api-reference/tasks/image-status)、[视频任务查询](https://docs.toapis.com/docs/cn/api-reference/tasks/video-status)共同说明：上游 `queued / in_progress / completed / failed`；部分新 GPT 页的提交示例还会返回 `pending`。ToAPIs 的 `completed` 表示生成成功，而 Sub2API 的 `completed` 表示整项处理结束，须显式转换。上游错误是 `{code,message}` 对象，需转为 Sub2API 顶层字符串 `error`。上下游任务 ID 不共用。

普通图片／视频结果 URL 文档有效期为 24 小时，必须先下载转存 Sub2API S3；Sub2API 的 `expires_at` 依自己的保留策略生成。上游查询可返回独立于生成状态的 `billing.status=pending|settled|refunded`、实际金额与部分用量；缺失不等于免费。Sub2API 下游仍按已确认的本地模型／规格定价，不能用上游计费状态替代自己的正式结算。

任务查询页允许携带 `client_business_id` 并按该值查询；这不等于已证明上游创建接口幂等，也不构成在“受理不明”时重新生成的依据。轮询至少 5–10 秒并加入抖动，429 读取 `Retry-After`；本期继续保留 worker 轮询，Webhook 不是必要前置。

## 3. 图片能力表

记号：`R7` = `1:1,4:3,3:4,16:9,9:16,3:2,2:3`；`R10` = R7 加 `5:4,4:5,21:9`；`R13` = R10 加 `2:1,1:2,9:21`。图片顶层公共 `size` 保留比例；上游的其他字段由 adapter 从已登记的 `metadata` 映射。**以下集合均来自对应文档，不表示可向整个模型族统一套同一默认值。**

| 上游精确 model ID | 比例／分辨率；上游默认 | 数量／参考图 | 特有参数与严格校验要点 | 官方来源 |
| --- | --- | --- | --- | --- |
| `gemini-nano-banana-2.1`、`gemini-nano-banana-2.1-vip` | R10 + `1:4,4:1,1:8,8:1`；resolution=`1K,2K,4K` 默认1K | n固定1；字符串URL≤14，每张≤10MB | 仅VIP支持grounding=`none,web,image,web_image` 默认none；不能把VIP扩展放到普通候选 | [Nano2.1](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1/generation) |
| `gemini-nano-banana-2.1-official` | R10 + `9:21,1:4,4:1,1:8,8:1`；resolution=1K/2K/4K 默认1K | n=1；字符串URL≤14，≤10MB | Vertex原生参数组V；grounding为none/web/image/web_image | [Official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1-official/generation) |
| `gemini-3.1-flash-image-preview` | R10 + `1:4,4:1,1:8,8:1`；0.5K/1K/2K/4K 默认1K | n整数默认1，但本页未给最大值；对象URL≤6 | google_search:boolean=false；google_image_search:boolean=false，后者要求前者true | [3.1普通](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash/generation) |
| `gemini-3.1-flash-image-preview-vip` | R10；1K/2K/4K 默认1K | n=1；对象URL≤14 | grounding=none/web/image/web_image | [3.1VIP](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash-vip/generation) |
| `gemini-3.1-flash-image-official` | R10 + `4:1,8:1`；1K/2K/4K 默认1K | n=1；字符串URL≤14，≤10MB | 参数组V；grounding=none/web/image/web_image | [3.1Official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash-official/generation) |
| `gemini-3-pro-image-preview` | R10；1K/2K/4K 默认1K | n=1；对象URL≤6，≤10MB | prompt上限为上游65536 tokens；orientation=landscape/portrait | [3Pro普通](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image/generation) |
| `gemini-3-pro-image-preview-vip` | R10；1K/2K/4K，**默认2K** | n=1；对象URL≤14 | grounding=none/web | [3ProVIP](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image-vip/generation) |
| `gemini-3-pro-image-official` | R10；1K/2K/4K 默认1K | n=1；字符串URL≤14，≤10MB | 参数组V；grounding=none/web | [3ProOfficial](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image-official/generation) |
| `gemini-2.5-flash-image-preview` | R7；仅1K | n=1；对象URL，单张≤10MB，页未给总张数 | prompt≤1000字符；orientation=landscape/portrait；文档别名`nano-banana`，不能擅自等同目录下划线别名 | [2.5普通](https://docs.toapis.com/docs/cn/api-reference/images/gemini-2.5-flash/generation) |
| `gemini-2.5-flash-image-official` | R10，未公布resolution档位字段 | n=1；字符串URL≤3，≤10MB | Vertex组V的temperature/topP/maxOutputTokens/personGeneration/imageOutputOptions/safetySettings；未声明thinkingConfig/resolution，禁止从3系列复制 | [2.5Official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-2.5-flash-official/generation) |
| `gpt-image-2` | R13；resolution=1k/2k/4k 默认1k；精确像素按比例表，4K方图2880² | n整数默认1、上限未声明；reference_images≤14，image_urls为兼容别名 | prompt≤32000字符；background仅transparent或省略；response_format固定url | [GPT2](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2/generation) |
| `gpt-image-2-vip` | R13以及任意比例／auto；1k/2k/4k 默认1k | n默认1上限未声明；参考图最大数未声明 | quality=low/medium/high默认medium；background=auto/transparent/opaque默认auto；任意尺寸约束见下文 | [GPT2VIP](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2-vip/generation) |
| `gpt-image-2-official` | R13及任意比例／auto；1k/2k/4k 默认1k | 参考图≤16；**n参数写1–10，但页首写最多4，存在冲突** | quality=low/medium/high默认high；background=auto/transparent/opaque；output_format=png/jpeg；compression=0–100；mask_url需同首图尺寸+Alpha | [GPT2Official](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2/official) |
| `gpt-image-2.5-flare`、`gpt-image-2.5-sunburst` | 比例size，例列R10；resolution=1K/2K/4K 默认1K | n=1；URL参考图，最大数未声明 | quality固定high，上游会忽略其他值，下游应拒绝不支持值；background=transparent或省略 | [GPT2.5普通](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5/generation) |
| `gpt-image-2.5-flare-vip`、`gpt-image-2.5-sunburst-vip` | **像素size**默认1024x1024；需符合R10，任意像素范围以接口校验为准；无需resolution | n=1；URL参考图，最大数未声明 | quality=auto/low/medium/high/xhigh/max默认high；background=transparent或省略；生成JSON支持参考图；另有multipart编辑路径 | [GPT2.5VIP](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-vip/generation) |
| `gpt-image-2.5-flare-official`、`gpt-image-2.5-sunburst-official` | **像素size**默认1024x1024；自定义尺寸约束未公布完整范围 | n=1；image_urls支持参考图、总数未声明 | quality六档默认high；background=transparent；上游成功后先存R2；按token结算不改变Sub2API本地按规格收费原则 | [GPT2.5Official](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-official/generation) |
| `doubao-seedream-4-0` | R7+21:9/9:21及有参考图才可用的auto；1K/2K/4K 默认2K | n=1–15；参考≤10且参考+n≤15 | prompt≤1000字符；optimize_prompt_options.mode=standard/fast默认standard；sequential模式参数；watermark默认true | [Seedream4](https://docs.toapis.com/docs/cn/api-reference/images/seedream-4.0/generation) |
| `doubao-seedream-4-5` | R7+21:9/9:21；仅2K/4K 默认2K | n=1–15；参考≤10且参考+n≤15 | 同4.0顺序与优化字段；不接受1K；watermark默认true | [4.5](https://docs.toapis.com/docs/cn/api-reference/images/seedream-4.5/generation) |
| `doubao-seedream-5-0` | R7+21:9/9:21；仅2K/3K 默认2K | n=1–15；参考≤10且参考+n≤15 | 顺序参数；watermark默认false；文档标题提联网搜索，但本页参数未给具体字段，不猜 | [5.0](https://docs.toapis.com/docs/cn/api-reference/images/seedream-5.0/generation) |
| `doubao-seedream-5-0-pro` | R7+21:9/9:21；仅1K/2K 默认2K | n=1–10；参考1–10（无参考则文生图），参考+n≤15 | 普通图生成可映射；layer_decomposition=true改变size语义且返回图层结构，见待决范围 | [5Pro](https://docs.toapis.com/docs/cn/api-reference/images/seedream-5.0-pro/generation) |
| `flux-kontext-pro`、`flux-kontext-max` | R7+21:9/9:21；无参考默认16:9，有参考默认match_input_image | 参考≤1；n未声明 | response_format=png/jpg默认png；safety_tolerance整数0–6默认2；prompt_upsampling=false | [Kontext](https://docs.toapis.com/docs/cn/api-reference/images/flux-kontext/generation) |
| `flux-2-flex`、`flux-2-pro` | R7默认1:1；1K/2K默认1K | 参考≤8；n未声明 | image_urls字符串；像素映射表为模型专属，不能套GPT表 | [Flux2](https://docs.toapis.com/docs/cn/api-reference/images/flux-2/generation) |
| `qwen-image-3.0`、`qwen-image-3.0-pro` | R7；1K/2K默认1K；未给size时自动；还支持像素形式 | n=1；参考≤3 | prompt_extend=true；prompt_extend_mode=direct/agent，agent仅纯文，上游有图会降为direct，下游应拒绝该组合；negative_prompt,seed,watermark=false | [Qwen](https://docs.toapis.com/docs/cn/api-reference/images/qwen-image-3.0/generation) |
| `grok-imagine-1.0` | 1:1/16:9/9:16/3:2/2:3默认1:1；未公布resolution | n=1–10；生成页未声明参考图 | 纯文生成schema；图像编辑型号单列下文 | [Imagine](https://docs.toapis.com/docs/cn/api-reference/images/grok-imagine-1.0/generation) |
| `vidu-image`、`viduq3-fast`、`viduq2-pro` | R7默认1:1；1K/2K/4K默认1K | n=1；参考0–7 | seed:integer；watermark=false；支持上游像素size，但统一比例足以表达基础生成 | [Vidu图片](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation) |
| `viduq2-fast` | R7默认1:1；**仅1K** | n=1；参考0–7 | 2K/4K上游明确400；禁止套用另3个Vidu模型分辨率 | 同上 |

### 3.1 目录中有 ID，但缺少可直接使用的生成 schema

| 目录精确 ID | 证据／处理 |
| --- | --- |
| `gemini-2.5-flash-image-preview-vip` | 本轮官方索引没有独立参数页；不能复制preview普通版或2.5官方版 |
| `gemini-3.1-flash-lite-image-official` | 目录存在，未找到独立图片参数文档；不能因名称类似3.1 Flash就复制其限制 |
| `nano_banana`、`nano_banana_2`、`nano_banana_pro` | 目录为下划线名字；详细页提供的是`nano-banana`/`nano-banana-pro`等不同别名。需上游或账号验证确认准确映射，不能自动注册同义词 |
| `grok-imagine-1.0-edit` | [编辑页](https://docs.toapis.com/docs/cn/api-reference/images/grok-imagine-1.0/edits)要求至少1张原图，n建议1–10，是编辑操作；应与普通生成能力范围分开登记 |

目录图片41条已全部出现在3节和3.1节；35条可找到普通生成页，5条参数证据不足，1条为编辑专用。此统计不代表35条已验证生产账号可调用。

### 3.2 文档发现的目录图片区外 ID

| 精确 ID | 普通生成能力 | 官方来源 |
| --- | --- | --- |
| `gpt-4o-image` | 异步；prompt≤1000字符；size=1:1/2:3/3:2默认1:1；n=1/2/4；参考≤5，每张≤10MB；mask_url为PNG、同图尺寸、≤4MB（mask作为特殊操作另定范围） | [GPT4o](https://docs.toapis.com/docs/cn/api-reference/images/gpt-4o/generation) |
| `grok-4-1-image`、`grok-4-2-image` | 异步；prompt≤1000字符；size=1:1/2:3/3:2/16:9/9:16；n=1/2/4；参考≤5、每张≤10MB | [Grok旧图片](https://docs.toapis.com/docs/cn/api-reference/images/grok-image/generation) |
| `midjourney` | Imagine异步返回四宫格；prompt必填；可传image_urls，图片≤12MiB；speed=relax/fast/turbo默认relax；version例v8.2/v8.1/v7/v6.1。size/quality/style/seed/stylize/chaos/weird/negative_prompt有字段名，但枚举／范围未在Imagine页完整定义 | [MJ总览](https://docs.toapis.com/docs/cn/api-reference/images/midjourney/generation)、[Imagine](https://docs.toapis.com/docs/cn/api-reference/images/midjourney/imagine) |

### 3.3 图片通用扩展字段的精确边界

- **Vertex参数组V**：temperature:number 0–2（3.1默认1；3Pro/Nano2.1页未给默认）；topP:number 0–1默认0.95；maxOutputTokens:integer默认32768；personGeneration枚举ALLOW_ALL/ALLOW_ADULT/ALLOW_NONE；imageOutputOptions.mimeType=image/png|image/jpeg|image/webp及JPEG compressionQuality:integer（范围未声明）；thinkingConfig.thinkingBudget整数0–24576，thinkingLevel=LOW/MEDIUM/HIGH/MINIMAL；safetySettings[]的category为HARM_CATEGORY_HATE_SPEECH/DANGEROUS_CONTENT/SEXUALLY_EXPLICIT/HARASSMENT，threshold=OFF/BLOCK_LOW_AND_ABOVE/BLOCK_MEDIUM_AND_ABOVE/BLOCK_ONLY_HIGH。只在明确支持该字段的具体型号启用。
- **Seedream顺序参数**：sequential_image_generation=disabled/auto默认disabled；n>1上游自动auto；sequential_image_generation_options.max_images为整数，对4.0/4.5/5.0是1–15，对5Pro是1–10，且参考张数+最终张数≤15；必须防止n和metadata的数量字段互相冲突。自动拆分可绕过单次张数上限，但不能把“有语义连续关系的一组图”误当相互独立的生成后承诺相同效果。
- Seedream参考图约束为JPEG/PNG、≤10MB、宽高>14px、比例1/3–3、总像素≤6000²。其n与参考联合上限应按每个实际子请求校验。
- GPT2 VIP/Official任意像素约束：宽高16倍数、长边≤3840、比例不超过3:1、面积655360–8294400；但官方预设表有宽屏21:9／9:21等，仍须逐预设验证，不能从“4K”直接推出4096边长。
- Qwen像素输入面积512²–2048²，比例1:8–8:1。统一接口目前只接受比例size，因此可先提供模型明确的比例档；如提供自定义精确尺寸，应由metadata字段单独表达并校验与size一致，不能悄悄替换下游比例语义。

## 4. 视频能力表

除Midjourney外均使用第2节的统一异步提交／查询路径；resolution等上游顶层扩展拟由统一请求metadata映射，**保留该模型实际枚举**（例如720p、768P、2K、4k），不能把视频1080p改名成图片1K／2K。视频多输入模式是生成能力的一部分；独立remix／edit／extend仍另定范围。

| 上游精确 model ID | 时长／分辨率／比例 | 生成输入与关键组合约束 | 其他模型字段与默认 | 官方来源 |
| --- | --- | --- | --- | --- |
| `doubao-seedance-1-0-pro-fast`、`doubao-seedance-1-0-pro-quality` | duration整数2–12默认5；480p/720p/1080p，参数栏默认720p；比例16:9/9:16/1:1/4:3/3:4/21:9默认16:9 | image_urls与image_with_roles互斥；role=first_frame/last_frame/reference，每种≤1；仅quality支持last_frame；reference不支持1080p | seed整数-1至2³²-1；private_asset_review=false。1.5对比表却称1.0默认1080p，与本参数页冲突，需核实 | [1.0](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance/generation) |
| `doubao-seedance-1-5-pro` | 4–12默认5；480p/720p/1080p默认720p；同比例集合 | 1/2张image_urls对应首／首尾帧；image_with_roles明确first_frame/last_frame且与image_urls互斥；不支持reference；上游多图会截断，下游应拒绝>2 | seed -1至2³²-1；audio=false；camerafixed=false；private_asset_review=false | [1.5](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance-1-5/generation) |
| `seedance-2` | 4–15；0/-1自动时长；480p/720p/1080p/4k默认720p；21:9/16:9/4:3/1:1/3:4/9:16/adaptive | 首帧1、尾帧1或参考图≤9，首尾与参考互斥；video/audio为reference角色，上限依通道；音频不能单独输入；image_urls与roles互斥 | generate_audio=true；return_last_frame=false；tools仅web_search且纯文；seed、private_asset_review、callback_url、client_business_id、trace_id | [Seedance2](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation) |
| `seedance-2-fast` | 4–15；0/-1自动；480p/720p默认720p；比例同上 | 首尾／多模态规则同上；视频音频上限依通道 | 同上 | 同上 |
| `seedance-2-mini` | 4–15；不支持0/-1；480p/720p默认720p | 首尾／参考模式同上；参考图≤9、视频≤3、音频≤3；音频不可单独；单次仅1结果 | 同上；明确支持音频开关和首尾帧 | 同上 |
| `seedance-2-5` | 4–30或-1自动；480p/720p/1080p；比例同2系列 | 普通generate支持首尾或参考，图片≤30、视频≤10、音频≤10，总≤50；支持纯音频参考；首尾与参考互斥 | video_operation=generate默认；output_format=mp4/mov默认mp4；generate_audio=true；return_last_frame=false；tools仅纯文web_search；edit/extend/draft另列范围 | [2.5](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-5/generation) |
| `seedance-2-ext` | seconds/duration 4–15；固定720p；size/aspect_ratio=16:9/9:16/4:3/3:4/1:1/21:9 | 纯文或参考图≤9；不支持参考视频、音频、首尾帧、编辑 | reference_image或reference_images；callback_url/client_business_id/trace_id | [2ext](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-ext/generation) |
| `seedance-2-fast-ext` | seconds/duration仅5/10/15默认5；480p/720p/1080p默认720p；比例16:9/9:16/4:3/3:4/1:1 | 纯文或参考图≤9；不支持视频音频／首尾帧 | 同上 | [FastExt](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-fast-ext/generation) |
| `seedance-2.5-ext` | seconds/duration 10–30；固定720p；比例16:9/9:16/4:3/3:4/1:1 | 参考图≤30、视频≤10、音频≤10；音频不可单独；带视频、音频或>10张图时不支持15秒；不支持首尾、编辑、延长 | reference_image(s)、video_with_roles、audio_with_roles、callback_url/client_business_id/trace_id | [2.5ext](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-5-ext/generation) |
| `gemini_omni_flash`，文档原名`gemini-omni-flash` | 4/6/10默认6；720p/1080p默认720p；16:9/9:16，1080p只横屏 | 文生或参考图≤3；不支持视频输入 | 提供公开HTTP(S)图片，不收base64 | [旧Omni](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation) |
| `gemini-omni-1.1-flash-preview` | 固定10秒/720p；16:9/9:16默认16:9 | 文生或图≤5 | 无其它明确扩展 | [Omni1.1普通](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation-1-1) |
| `gemini-omni-1.1-flash-preview-vip` | 4/6/8/10默认6；360p/720p/1080p/4k默认720p；16:9/9:16 | generation_type=frame仅1图/reference仅1或3图，不支持2图首尾；video_urls≤1，可与图片混用，但有视频必须省略duration/seconds | 无图须省略generation_type；视频参考输出时长按实际，文档不保证等于输入 | [Omni1.1VIP](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation-1-1-vip) |
| `gemini-omni-flash-preview-official` | duration整数≤10默认6，页面预设4/6/8/10但未给下限；固定720p；16:9/9:16 | 图片≤10与输入视频≤3互斥；视频输入用于edit；普通text/image/reference路径可纳入生成 | metadata.task=text_to_video/image_to_video/reference_to_video/edit；temperature0–2，topP/top_p0–1，candidateCount/candidate_count，previous_interaction_id | [OmniOfficial](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash-preview-official/generation) |
| `gemini-omni-1.1-flash-preview-official` | 3–10默认6；360p/720p/1080p/4K默认720p；16:9/9:16默认16:9 | 图片≤10与输入视频≤3互斥；video_list视频MP4/MOV单项≤50MiB；edit时不发送aspect_ratio | metadata.task四类同上；拒绝Veo专属generateAudio/negativePrompt/seed | [Omni1.1Official](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-1.1-flash-preview-official/generation) |
| `veo3.1-fast`、`veo3.1-quality`、`veo3.1-lite` | 固定8秒；720p/1080p/4k默认720p；16:9/9:16 | generation_type=frame/reference；2图推frame、3图推reference；quality不支持reference | enable_gif=false，GIF不能1080p/4k；图片≤10MB；不同lite规格未单独列，不能自动认为全部组合可用 | [Veo普通](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation) |
| `Veo3.1-quality-official`、`Veo3.1-fast-official`、`Veo3.1-lite-official` | 4/6/8默认8；720p/1080p/4k预览默认720p；size比例16:9/9:16；Lite介绍仅720p | image_urls只取首张为首帧，与referenceImages≤3互斥；lastFrame必须有首帧 | generateAudio=false；negativePrompt；personGeneration=allow_adult/dont_allow；compressionQuality=optimized/lossless默认optimized；resizeMode=pad/crop | [VeoOfficial](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation) |
| `sora-2-official` | 4/8/12默认4；比例16:9/9:16默认9:16，对应1280x720/720x1280 | 首图生成，最多只使用第一张；输入必须精确对应上述尺寸，JPEG/PNG/WebP | remix另有接口，普通生成不携带remix参数 | [AzureSora](https://docs.toapis.com/docs/cn/api-reference/videos/sora-2-official/generation) |
| `happyhorse-1.1` | 3–15默认5；720P/1080P默认1080P；16:9/9:16/1:1/4:3/3:4默认16:9，仅文生比例有效 | action=text-to-video/image-to-video/reference-to-video/video-edit；首帧只取1图；参考1–9；首图≥300px、≤10MB、比例0.4–2.5 | seed整数；watermark=true；image-to-video prompt可省；video-edit须url，audio_setting=auto/origin属编辑范围 | [HappyHorse](https://docs.toapis.com/docs/cn/api-reference/videos/happyhorse/generation) |
| `MiniMax-Hailuo-02` | 6/10默认6；512P/768P/1080P默认768P；1080P仅6秒 | 0/1首帧；metadata.last_frame_image尾帧必须有首帧，首尾不支持512P；首图短边>300、≤20MB、比例0.4–2.5 | prompt文生必填、图生可省，≤2000字符；prompt_optimizer=true、fast_pretreatment=false、aigc_watermark=false；512P文生支持与正文说明有冲突 | [Hailuo02](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo/generation) |
| `MiniMax-Hailuo-2.3`、`MiniMax-Hailuo-2.3-Fast` | 6/10默认6；768P/1080P默认768P；1080P仅6秒 | 普通支持文生/首图，Fast必须1张首图；图片约束同02 | prompt必填≤2000；prompt_optimizer/fast_pretreatment/aigc_watermark同02 | [Hailuo2.3](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo-2.3/generation) |
| `MiniMax-H3` | 4–15默认5；2K/768p默认2K；21:9/16:9/4:3/1:1/3:4/9:16/adaptive | 首尾模式或多模态参考，互斥；参考图≤9、视频≤3、音频≤3，总≤12；音频不可单独；首尾按图自适应 | prompt必填≤7000；watermark=false；视频总≤15秒/单段2–15秒≤50MB；音频总≤15秒/单段2–15秒≤15MB | [H3](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation) |
| `MiniMax-H3-Max` | 5–15；480P/768P；其它同H3 | 同H3多模态能力 | 文档没有明确本型号默认resolution，不应采用共享字段默认2K | 同上 |
| `MiniMax-H3-Max-Turbo` | 5–15；480P/768P | 仅文生／首尾帧，**不支持任何reference媒体角色** | 不从Max复制reference schema | 同上 |
| `kling-v2-6` | 5/10默认5；mode=std(720P)/pro(1080P)默认std；常用16:9/9:16/1:1 | reference_images为普通参考；首尾必须image_with_roles显式first_frame/last_frame，允许和reference混用；不猜第二图尾帧 | prompt≤2500；negative_prompt；audio=false且仅pro可true；watermark:boolean | [Kling2.6](https://docs.toapis.com/docs/cn/api-reference/videos/kling-2-6/generation) |
| `kling-v3` | 3–15默认5；std720P/pro1080P默认std；常用16:9/9:16/1:1 | 普通参考与显式首尾语义同2.6 | audio=false；negative_prompt、watermark | [Kling3](https://docs.toapis.com/docs/cn/api-reference/videos/kling-v3/generation) |
| `kling-3.0-turbo` | 3–15默认5；720p/1080p默认720p；16:9/9:16/1:1默认16:9，仅文生有效 | 图生仅1张首帧；不支持尾帧；上游多余图会忽略，统一schema应拒绝 | watermark=false布尔 | [KlingTurbo](https://docs.toapis.com/docs/cn/api-reference/videos/kling-3.0-turbo/generation) |
| `kling-v3-omni` | 3–15默认5；std720P/pro1080P；常用16:9/9:16/1:1 | image_list/element_list按prompt占位符引用；video_list≤1，refer_type=base/feature，keep_original_sound=yes/no；视频与audio=true互斥；end_frame要求first_frame | audio=false；watermark；client_business_id。base为编辑，feature为参考生成，不能一起粗暴排除 | [KlingOmni](https://docs.toapis.com/docs/cn/api-reference/videos/kling-v3-omni/generation) |
| `kling-video-o1` | 3–10默认5；std720P/pro1080P；16:9/9:16/1:1 | Omni图片/主体；video≤1，3–10秒base模式属编辑且不能首尾；feature属参考；MP4/MOV、720–2160px、24–60fps、≤200MB；end_frame要求first_frame | watermark；prompt占位符顺序不能改；视频时禁audio/sound | [KlingO1](https://docs.toapis.com/docs/cn/api-reference/videos/kling-video-o1/generation) |
| `viduq3-pro`、`viduq3-turbo` | 1–16默认5；540p/720p/1080p默认720p；任意比例 | 0图文生、1首帧、2首尾帧；输入图时实际比例通常由图决定 | prompt≤5000；audio默认true；seed；audio_type=all/speech_only/sound-effect_only；off_peak=false且仅audio=true支持 | [ViduQ3](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation) |
| `viduq3` | 3–16默认5；540p/720p/1080p | 参考图必填≤7；Subjects仅本型号：≤7主体，每主体≤3图，通过@name引用 | auto_subjects=false；voice_id；其它同Q3；payload≤1048576字符；闲时任务可能48小时，不能直接沿用短超时 | 同上 |
| `viduq3-pro-fast` | duration默认5但范围未给；720P/1080P默认720P | 首帧必传1图，多图上游只取1张 | audio默认true；watermark=false；seed；metadata.input.{first_frame_url,last_frame_url,audio_url}，parameters.{audio,watermark} | [Vidu Ali](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation) |
| `viduq3-ad`、`viduq3-drama`、`viduq3-mix` | duration默认5但范围未给；720P/1080P，drama建议1080P | 不能纯prompt；推荐视频+参考图；可能强制subjects结构但文档未给完整schema | ad/mix默认audio=true；drama须显式audio；metadata透传不等于严格schema已知，须补齐后启用相应模式 | 同上 |
| `viduq4-preview` | 3–16默认5；540p/720p/1080p/2K/4K默认720p；参考比例16:9/9:16/4:3/3:4/1:1 | 1首帧或reference_images_to_video参考≤15图；参考模式可sounds≤3；多图／参考音自动走参考 | audio=true、watermark=false；is_rec默认true；seed；wm_position整数1–4、wm_url仅水印开启；meta_data为JSON字符串 | [ViduQ4](https://docs.toapis.com/docs/cn/api-reference/videos/viduq4-preview/generation) |
| `grok-video-1.0` | 1–15默认8；480p/720p默认720p；16:9/9:16/3:2/2:3/1:1 | image主图与reference_images合计≤8 | prompt必填；生成与独立remix/extend分离 | [Grok1](https://docs.toapis.com/docs/cn/api-reference/videos/grok-video/generation) |
| `grok-video-1.5` | 1–15；参考仅480p/720p，文生/首帧渠道可1080p；常用1:1/16:9/9:16/3:2/2:3依渠道 | video_generation_mode明确text_to_video/first_frame_image_to_video/reference_images_to_video；首帧恰1、参考1–7；1张参考必须显式reference模式 | prompt在文生/参考必填，部分旧首帧可省；image/input_reference别名；部分通道不能严格保证首帧控制，需要候选能力验证 | [Grok1.5](https://docs.toapis.com/docs/cn/api-reference/videos/grok-video-1.5/generation) |
| `wan2.6` | 5/10/15默认5；720p/1080p默认1080p；16:9/9:16/1:1/4:3/3:4仅文生/视频参考 | 0图文生、1图首图生成，或metadata.reference_urls视频参考，二者互斥；图生不支持aspect_ratio | prompt文生必填、图生/参考可省；negative_prompt、seed、prompt_extend=true、audio默认有声、shot_type=single/multi、watermark | [Wan2.6](https://docs.toapis.com/docs/cn/api-reference/videos/wan2.6/generation) |
| `wan2.6-flash` | 5/10/15默认5；720p/1080p默认1080p | 必须1图片或reference_urls视频，互斥；不支持纯文；prompt可省 | audio=true；watermark | [WanFlash](https://docs.toapis.com/docs/cn/api-reference/videos/wan2.6-flash/generation) |
| `wan3.0-video` | 2–30或-1智能，默认5；480p/720p/1080p默认1080p；adaptive/16:9/9:16/1:1/4:3/3:4默认adaptive | prompt或素材至少一项；首尾与参考模式互斥，尾必须首；参考图≤10、视频≤5总≤15秒、音频≤5总≤15秒；可文件/网页参考最多1项且互斥 | audio=true、watermark=false、seed；metadata.media[]=`{type:file|link,url}`；不能与首尾混用 | [Wan3](https://docs.toapis.com/docs/cn/api-reference/videos/wan3.0/generation) |

### 4.1 视频目录待核实项与文档额外型号

| ID／能力 | 证据与边界 |
| --- | --- |
| `gemini_omni` | 视频目录存在；本轮没有直接定义其参数的生成页。不能等同`gemini_omni_flash` |
| `wan3.0-video-prime` | 视频目录存在；Wan3文档明确model固定`wan3.0-video`，没有prime参数说明，不能自行复制 |
| `sora-2-vvip` | [生成文档](https://docs.toapis.com/docs/cn/api-reference/videos/sora2/generation)存在，目录视频区未列。4/8/12秒、16:9/9:16，image_urls参考；metadata.n=1–4、watermark=false、private=false、style=thanksgiving/comic/news/selfie/nostalgic/anime、storyboard:boolean；角色url+timestamps的2秒片段会让输出少1秒，自动创建／提取角色应单独定范围 |
| `gemini-omni-flash` | 旧Omni生成页给出的主名，目录使用其别名`gemini_omni_flash`，见主表 |
| `midjourney` 的视频能力 | [MJ Video](https://docs.toapis.com/docs/cn/api-reference/images/midjourney/video)：传一张image_urls可直接生成，或引用成功父任务task_id+index；batch_size仅1/2/4，返回video_urls。公开页未给时长、分辨率、比例限制；同一model可以登记image/video能力，与PRD不冲突。父任务依赖和结果批量收费需按实际操作范围确认 |

49条视频目录ID已全部覆盖于主表与4.1节，其中47条有明确生成页或官方明确别名，2条仍缺参数证据。不能把“已登记47条”描述为“47条已在账号生成验证”。

### 4.2 角色、可选字段与输出语义

1. **prompt不能一刀切强制非空**：Wan3允许纯参考素材；Wan2.6/Flash、HappyHorse首帧、Hailuo02图生、Seedance等有可省prompt的生成模式。这些是普通图／参考驱动生成，不是编辑。若PRD要求“所有视频prompt全局必填”，会缩小已确认的全部模型生成能力，应在能力schema按模式决定必填，而不是以通用handler先拒绝。模型仅“建议填写”不可擅自升级成上游硬限制。
2. **duration不能先注入通用默认**：Omni1.1VIP参考视频要求不传duration；Seedance和Wan有自动时长0/-1；实际输出时长可能决定收费。允许省略、允许特殊值、预留上限与最终规格需要按已配置schema和本地计价规则定义。
3. **aspect_ratio不是所有输入都支持**：Wan2.6图生明确不支持，H3首尾会忽略具体比例，HappyHorse/KlingTurbo图生按图决定。为了满足PRD禁止静默丢参数，若调用者显式传入一个无法保证的比例，应在创建前拒绝该模式／候选，不能直接删掉字段。adaptive是否可对外暴露应按逐模型schema，不能改size的图片比例定义。
4. 普通reference图片与first_frame/last_frame在多平台语义不同。统一image_urls:string[]本身不能携带角色；metadata可承载已声明的角色／mode／视频音频结构，adapter保留顺序和用途。**具体统一metadata命名尚属方案，不将上游字段直接认定为下游定稿**。
5. 文件／网页、asset://、原厂角色／主体ID有账号归属和上游语义。尤其asset://不应在换上游账号时当成通用URL；是否引入这些素材库操作与普通可访问URL参考应分别处理。

## 5. 统一字段映射评估（方案候选，尚未定稿）

| 统一字段／能力 | 可行映射 | 不能省略的限制 |
| --- | --- | --- |
| 图片model/prompt/size/n | 原名映射；public_model与upstream_model分离 | 每个模型／候选各自schema；上游n未公开上限时不能虚构10/15，可用单张拆分覆盖已确认多张语义 |
| 图片image_urls:string[] | ToAPIs字符串字段直接映射；旧Gemini对象字段包成`[{url:...}]`；GPT参考图转reference_images | 图片顺序、数量、类型和参考用途保留；禁止同时发送不同内容的别名列表 |
| 图片metadata.resolution | 映射顶层resolution或上游metadata.resolution | 每型号保留0.5K/1K/2K/3K/4K可用集合；这些是档位，不保证统一精确像素。额外枚举可由已确认的模型schema表达，无需每项重新产品定稿 |
| 精确像素输出 | 可评估metadata.output_size，或metadata.width/height，显式表达上游尺寸 | 必须检查与公共size比例相容；GPT2.5VIP/Official不能直接把1:1发为上游size，也不能凭1K/2K/4K随便推算任意像素；先登记具体像素模板可覆盖基础规格 |
| 视频duration/aspect_ratio | 映射duration/seconds、aspect_ratio/size/ratio | 自动时长、缺省时长、输入决定比例均是能力约束；显式值与上游不兼容应400，不可静默删除 |
| 普通图片／首尾帧角色 | metadata可有模式与角色结构，adapter转换到image_with_roles/image_list/lastFrame/last_frame_image等 | 需有统一的语义说明，尤其1张参考图≠严格首帧；与image_urls的重复／互斥关系明确后才可维护schema |
| 视频／音频／主体／网页参考 | metadata可装已声明对象数组和ID | 完整保留URL/role/顺序；不能开放任意metadata透传并称为严格校验；文档未提供subjects完整结构的候选应待核实 |
| 图像／视频普通结果 | 把result.data[].url映射为S3转存后的string[]；MJ四宫格取grid_image_url | MJ四宫格计1结果；普通数组按实际成功数量；不得把图片／视频辅助附件算成同类收费结果 |
| upstream callbacks/client_business_id/trace_id | Sub2API内部关联字段，可自行生成／保留 | 下游未承诺回调，不应把上游回调地址或账号标识当自由metadata直接透传；client_business_id不证明幂等 |

上游成功结果可能有附带的格式、尾帧、grounding引用、图层信息；统一响应目前没有可承载这些结构的位置。是否支持这些输出选项与“能调用该模型的普通生成”是两件事。

## 6. 必须区分的普通生成和特殊操作

- **已可沿普通生成路径表达**：文生图、参考图生图；文生视频、首帧/首尾帧、普通参考图／视频／音频生成。模型页是否将参考生成称为“编辑”不能单独决定排除，例如GPT参考图既可同JSON生成也有独立edits路径，需按实际输入／输出目的判断。
- **独立非生成操作**：Midjourney describe、upscale、variation、high/low-variation、reroll、zoom、pan、inpaint/modal、remix；Sora/Grok独立remix与extend；角色创建／查询、Seedance私有素材入库和真人认证；Seedance2.5 video_operation=edit/extend及draft→成稿工作流；GeminiOmni task=edit、HappyHorse video-edit、Kling参考视频base编辑。这些不应因共用生成URL就全部首期承诺。
- **生成时可选辅助输出**：Seedance return_last_frame、Sora thumbnail、Gemini grounding_metadata、Seedream layer_decomposition。若不支持，应在本地schema明确拒绝对应请求，不能受理后悄悄丢弃成果。
- Midjourney视频可只用一张URL生成，不必强制先做父任务；若支持父任务方式，则需本地任务与上游账号关联，不能暴露可跨用户任意引用的供应商task_id。

## 7. 实际需要进一步定稿的范围差异

1. **结构化辅助结果**：`layer_decomposition=true`会返回底图及PNG图层，每项有z_index，图层另有bounding_box/name/description；`return_last_frame=true`会同时产生视频尾帧图片。既定`result={type,data:string[]}`无法无损表达这些结构。建议首期保持普通媒体URL交付，对上述输出选项返回创建前400；若用户要求覆盖这些特殊能力，应另定响应扩展，不能直接塞入同一data混合交付。普通Seedream5Pro和Seedance生成仍可支持。
2. **精确像素尺寸是否作为首期高级输入**：这是GPT2.5VIP/Official的必要上游size形式。技术上可按已确定的比例+分辨率模板生成上游合法像素，也可通过metadata明确像素再校验比例。模型基础接入无需因此暂停，但要避免把“覆盖全部型号”写成“所有自定义像素规格均已覆盖”。如要对外承诺任意像素支持，需先确认metadata字段和与size冲突时的400规则。
3. **账号绑定素材／父任务操作的范围**：素材库asset://、原厂persona、Midjourney父task_id等无法天然跨账号共用。普通公开URL参考可先提供；素材库管理和依赖已有任务的高级操作按用户原定“模型特有操作再聊”单列，不作为P0所有普通生成的隐含前提。

不必为每个resolution枚举、seed字段或普通参考图限制再次提产品问题；这些已被“模型特定字段放metadata、数据库schema+后台维护、完整能力匹配、严格校验”覆盖。priority、轮询退避、对象深度、文档同步方式属于技术方案细化。

## 8. 上游文档矛盾、缺口与启用前核实

| 问题 | 对实施的影响 |
| --- | --- |
| 图片目录标题38 vs 实际41 | 使用机械清单作统计；未来应记录目录快照和来源，不把标题写入自动验收 |
| 图像带openai-video、文字带image-generation | 能力登记不能只靠目录端点标签自动决定media_type |
| GPT2Official n页首最多4 vs 参数1–10 | 原生多张上限未证实；可先n=1并拆分，不能宣称原生10张已确认 |
| Gemini3.1普通n未写上限；GPT2/VIP部分参考图上限未写 | 不从别的模型猜；基础模板可选已证实单张，但上限数据需官方或获授权的账号验证 |
| Hailuo02正文512P“图生可用”而组合表文生也勾选 | 先将512P文生组合列待验证，不能使用表中一个勾覆盖明确差异 |
| Seedance1.0默认resolution参数页720p，1.5对比页1080p | 模板应显式发送已选规格，不依赖冲突的上游缺省 |
| Omni1.1普通/VIP正文及示例均为GET /v1/videos/{id}，通用视频任务页为GET /v1/videos/generations/{id} | 按具体模型页记录查询差异，账号接入验证时核实两者是否为同任务查询别名；不把通用路径直接当作该型号已证实的唯一路径 |
| GPT2.5VIP本页生成JSON允许参考图，但迁移清单又要求参考编辑走multipart edits | 普通图生图选明确JSON生成例，不为此引入下游multipart；独立编辑能力另定范围 |
| 未公开完整枚举／范围的MJ Imagine、Vidu Ali subjects；目录有名无参数的7条 | 在schema证据不足时按PRD创建前4xx，不能为了覆盖率把metadata改成不校验透传 |
| VeoOfficial介绍Lite仅720p，共用resolution栏还有1080p/4k | Lite应使用单独候选能力；不把共用栏全量复制 |
| 私有素材审核准备最多20分钟、Vidu闲时任务可能48小时 | 普通生成worker超时要按模式配置；超时前先确认上游受理，不能换账号重复生成 |

本研究未对官方文档未公开的值做生产尝试；这些证据缺口需要后续渠道／账号接入验证或上游说明解决，不等同必须改变下游公开协议。

## 9. 与 PRD 已收敛的边界

根据本轮用户定稿，以下规则用于收敛前文的方案候选，不改变供应商事实表：

1. `prompt` 是否必填由具体模型及输入模式的能力 schema 决定；不在统一请求入口全局强制必填。
2. 图片继续使用 `size` 表达比例，使用 `metadata.resolution` 表达分辨率档位，由 adapter 按有明确依据的模型映射转换为上游像素尺寸。**不新增 `output_size`、`width`、`height` 等下游任意像素输入字段**；仅开放具备明确映射依据的比例与分辨率组合。前文精确像素内容保留为上游能力事实，不构成下游开放承诺。
3. Seedream 5.0 Pro 首期仅支持普通图片生成。`layer_decomposition` 图层拆分模式在创建任务前返回 400，后续另议，不改变既定 `result.data: string[]`。

查询协议差异仍按第 2 节与第 8 节明确保留：普通视频通用查询为 `GET /v1/videos/generations/{task_id}`；Omni 1.1 普通／VIP 的独立官方页面正文及示例均使用 `GET /v1/videos/{id}`。两者是否为同任务查询别名仍须账号接入验证，不能将尚未验证的路径一致性当作事实。
