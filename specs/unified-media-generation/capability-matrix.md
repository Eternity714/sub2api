# GRS.AI / ToAPIs 统一媒体生成能力对照表

核对日期：2026-10-15（Asia/Shanghai）。状态：官方公开文档与代码事实基线，尚未生成可上线的能力配置、未调用真实生成接口、未核查生产账号。

需求依据：[PRD](prd.md)。执行切片：[P0 特性拆分](feature-breakdown.md)。详细参数和逐条官方来源：[GRS.AI 研究](research/grsai.md)、[ToAPIs 研究](research/toapis.md)。机器可读目录：[GRS JSON](research/grsai-models.json)、[ToAPIs JSON](research/toapis-models.json)。

## 1. 覆盖口径与结论

| 平台 | 公开目录图片条目 | 公开目录视频条目 | 证据边界 |
| --- | ---: | ---: | --- |
| GRS.AI | 16 | 2 | 当前官网共33条，媒体18条；4个图片条目标维护；4个原生生成族文档可以交叉核对，但部分型号/输入约束仍缺详细说明 |
| ToAPIs | 41 | 49 | 目录页面自身快照日期2026-09-30；图片标题写38，实际表格41条；其中35图片+47视频有生成文档，5图片+2视频缺能力证据，另1图片为编辑专用 |
| 合计 | 57 | 51 | 108条“平台 + 媒体类型 + 上游ID”目录记录，不是108个已验证可调用的不同模型 |

来源：[GRS 官网目录](https://grsai.com/zh/dashboard/models)、[ToAPIs 公开目录](https://docs.toapis.com/docs/cn/api-reference/models)。完整ID清单在第7节；文档额外发现的ID独立列于第8节，不混进108条目录分母。

几个概念需要分开：

1. **目录登记**：官方公开页面列出该ID。
2. **生成协议有据**：能定位该型号的普通生成文档；有文档仍可能缺部分范围、默认值和组合约束。
3. **能力schema可发布**：已明确拟开放的全部字段、组合和输出语义，且有对应转换与定价依据。
4. **账号可调度**：管理员已配置账号、路由、模型与计费，有能力匹配的可用账号。
5. **生成验证通过**：使用授权测试账号完成实际提交、轮询、S3与结算验证。本轮未执行。

“覆盖全部可用生成模型”继续作为目标；文档中的证据缺口需要补齐，不因缺文档就默认为从首期永久删去。也不能为了声称全覆盖，给未知模型复制相近型号的schema、开放任意metadata或启用维护中的候选。只配置已证实的模式；未知部分按既定规则拒绝并给出清楚原因。

## 2. 上游提交 / 查询 / 结果差异

| 平台与协议 | 上游创建 | 上游查询 | 归一化要点 |
| --- | --- | --- | --- |
| GRS 新原生四族 | POST `/v1/api/generate`；服务端固定 `replyType=async` | GET `/v1/api/result?id={upstream_id}` | 原生结果为 `results[].url`；状态有running/violation/succeeded/failed；非2xx也可能带任务ID，不能一概重发 |
| ToAPIs 普通图片 | POST `/v1/images/generations` | GET `/v1/images/generations/{id}` | 异步；结果data是含url的对象数组，需转存并提取string[] |
| ToAPIs 普通视频 | POST `/v1/videos/generations` | GET `/v1/videos/generations/{id}` | 异步；上游completed表示成功，与Sub2API“处理结束”不同；可附带视频格式/尾帧等额外字段 |
| ToAPIs Omni 1.1 普通/VIP | 同普通视频创建 | 专属页面写GET `/v1/videos/{id}`，与通用查询页存在路径差异 | 不能假定所有视频型号只有一个查询模板；适配前需核实具体路径及别名是否同时有效 |
| ToAPIs Midjourney Imagine | POST `/v1/midjourney/generations/imagine`，默认generations入口等价 | GET `/v1/midjourney/{id}`；另有tasks别名 | 四宫格grid_image_url作为1张结果；按钮、增强/变体等后续操作不纳入本期 |
| ToAPIs Midjourney Video | POST `/v1/midjourney/generations/video` | 同MJ查询 | video_urls；支持单图或引用上游父任务，两种输入要分开核实账号/归属与schema |

这里只列上游协议，**下游始终使用PRD的四个接口**。下游GET只读本地状态，不直接把上游查询响应返回，也不触发新生成。GRS新原生与本地旧GRS入口恰好同名，关闭本地旧路由不代表停止服务端请求上游同名路径。

GRS与ToAPIs均使用管理员配置账号的Bearer凭证；不能透传用户Key给上游。官方页面存在不同Base URL写法，账号配置时必须明确并验证，不静默更换主机。上游临时结果期限不能作为Sub2API S3 expires_at；本期保留独立S3转存与worker轮询。

## 3. 统一字段与逐模型转换

| 下游字段 | GRS 新原生 | ToAPIs | 配置与校验要求 |
| --- | --- | --- | --- |
| model | model | model | 由管理员public_model路由到精确上游ID；同名、大小写或连字符/下划线相近不代表同一模型；同一public_model可登记双媒体能力 |
| prompt | prompt | prompt | model始终必填；prompt按具体模型和输入模式决定必填性；不伪造提示词满足旧全局校验 |
| 图片size | aspectRatio；GPT部分型号实际要求像素字符串 | 通常size为比例；GPT2.5 VIP/Official要求像素size | 下游保持比例；按size + metadata.resolution使用具体模型的规格映射。无映射依据不猜，不新增output_size |
| metadata.resolution | Nano:imageSize；视频:resolution；GPT可能需合成为像素aspectRatio | 多数:resolution；像素型size需由适配合成 | 图片常见1K/2K/4K，也有模型0.5K/3K；视频有480p/512P/540p/720p/768p/1080p/2K/4K等，按型号枚举及大小写映射，不跨媒体等同 |
| 图片n | 原生四族无n，用本地拆分 | 有的固定1，有的支持多图，有的文档上限冲突 | 统一n是整项结果数；允许拆分但不能拆散“语义相关的一组图”后宣称等价；逐实际子请求检查素材+结果数量限制 |
| image_urls | images，URL数组 | 字符串数组、对象数组、reference_images或首帧字段 | 转换只改变结构，不能把普通参考图自动当首/尾帧；保留顺序和明确角色；超过上游只取首图的限制时拒绝，不能截断 |
| 视频duration | 整数duration，按resolution联合限制 | 不同型号范围不同，部分有自动时长，部分参考视频要求省略 | 不注入一个全局默认；按模式schema校验。自动/输出决定时长必须有本地可预留与结算规则后才开放 |
| 视频aspect_ratio | aspectRatio；MiniMax只写portrait/landscape | aspect_ratio/size；部分图生模式禁止或无法控制 | 只开放有明确语义映射的值；用户传入无法保证的比例时拒绝该候选，不删除字段继续生成 |
| metadata模型参数 | quality/background/mask/seed/audios等 | quality/grounding/seed/audio/首尾帧/主体/视频参考等 | 每个参数有schema白名单、类型、枚举、范围和组合约束；不能无条件透传整个metadata到上游 |
| replyType/回调/凭证/受理控制 | 内部replyType | 原生callback_url等 | 不能仅因上游支持就自动成为下游可配置字段；本期没有下游回调承诺，执行控制由服务端管理 |

图片分辨率档位是规格标签，不意味着“1K永远1024×1024、4K永远4096×4096”。例如GRS GPT官方规格表列4K方图2880×2880，且某些比例尺寸是近似值；映射与具体输出尺寸应写入逐模型文档，不能将所有平台套同一计算公式。详细表见GRS研究§4.2。

## 4. 图片族的主要差异

| 平台/族 | 已核实差异 | 统一适配重点 |
| --- | --- | --- |
| GRS Nano | 11个目录ID；CL/VIP各型号分辨率不同；fast/lite缺逐项限制 | 不能全族继承1K/2K/4K；比例/imageSize映射，n拆分 |
| GRS GPT | 5个目录ID；quality按型号不同，部分像素输入；4个型号维护 | 比例×档位表转像素；mask/background逐型号证据；维护状态不等于可调度 |
| ToAPIs Gemini/Nano | preview/VIP/Official的参考图对象形式、张数、比例、默认分辨率、grounding与Vertex参数不同 | 不把VIP字段给普通候选；多图/搜索/思考等嵌套字段逐型号schema化 |
| ToAPIs GPT | GPT2、VIP、Official、2.5各版规格/quality不同；部分文档n范围冲突 | 规格映射；不接受上游会忽略的quality；不依赖上游价格决定下游费用 |
| ToAPIs Seedream | 4.0/4.5/5.0/5Pro各有不同分辨率、n与参考图联合上限 | 拆分后保留顺序生成语义；5Pro普通生成纳入，图层分解创建前400 |
| ToAPIs Flux | Kontext最多1参考图；Flux2可多图；默认画幅随模式变化 | defaults按模式声明；像素规格表不能套GPT |
| ToAPIs Qwen | prompt扩展agent模式仅纯文，有图会被上游降为direct | 统一层禁止静默改模式；冲突创建前400 |
| ToAPIs Grok | Imagine生成与edit专用型号分开；其它图片型号来自额外文档 | 不把edit专用ID当普通生成；保留已确认的参考图输入能力 |
| ToAPIs Vidu图片 | 同族部分只支持1K，其它可2K/4K | 按型号筛候选；同名在视频中出现时按请求路径能力区分 |
| ToAPIs Midjourney | 目录外独立协议；Imagine四宫格、后续操作、Video分别有语义 | 仅生成协议；四宫格作为1结果/1次生成；不混入增强/变体按钮 |

## 5. 视频族的主要差异

| 平台/族 | 已核实差异 | 统一适配重点 |
| --- | --- | --- |
| GRS MiniMax | 480p/768p最长15秒，1080p最长10秒；参考图片/音频 | 比例portrait/landscape的实际比例需证据；不与ToAPIs同名H3默认等价 |
| GRS Agnes | 4–12秒；720p/1080p/2k；图≤5、音频≤3 | 官网宣传首尾帧但原生schema未说明角色，先补证据 |
| ToAPIs Seedance | 普通/fast/mini/2.5/ext等时长/分辨率/角色限制不同；ext不等于extend | 分别配置首尾/参考/音频联合约束；2.5的edit/extend不是普通generate |
| ToAPIs Gemini Omni | 多版模式/时长不同；VIP参考视频必须不传duration；专属查询有差异 | 按模式控制字段与查询；不套Veo字段；edit模式单列 |
| ToAPIs Veo | 普通固定8秒；Official部分可4/6/8；quality参考限制与Lite分辨率不同 | 首图与多参考互斥；不能忽略多余图或降分辨率 |
| ToAPIs Sora | 普通生成与remix、角色创建有不同语义；额外文档有vvip | generation本体与角色资源操作分开；某模式实际时长不同需计价依据 |
| ToAPIs HappyHorse | 文生/首图/参考/编辑多模式；首图prompt可省 | 仅生成模式；模式决定必填及比例是否有效 |
| ToAPIs MiniMax | Hailuo02、2.3/Fast、H3/Max/Turbo的分辨率、参考与尾帧规则不同 | Fast需要图；Turbo不支持reference；不照抄同名族全部能力 |
| ToAPIs Kling | std/pro与分辨率、音频联动；Omni/O1有主体占位符和视频base/feature | base编辑与feature参考生成分开；角色与顺序不能被普通URL数组吞掉 |
| ToAPIs Vidu | Q3不同型号文生/首尾/subjects能力不同，Ali部分schema不完整；Q4多模态参考 | 补齐subjects结构；闲时可能48小时，worker窗口按模型配置；上游多余图不截断 |
| ToAPIs Grok | 首图与reference模式不同，部分渠道首帧控制不保证 | 候选能力必须细到实际渠道能保证的语义，不能靠型号名声称满足 |
| ToAPIs Wan | 部分仅图/视频参考；Wan3允许纯素材、智能时长和file/link参考 | prompt按模式；素材互斥、返回时长与费用规则需明确；账户绑定资源另行核实 |

完整枚举/范围/默认值/组合约束与官方链接均在两份research中；这两张族表用于阅读和拆分，不替代逐模型schema。

## 6. 已确认与待收敛项

### 6.1 本轮用户已确认

- prompt是否必填由模型和输入模式schema决定，model始终必填。
- 图片继续由size比例与metadata.resolution确定上游像素；本期不新增metadata.output_size，不开放未经证实的尺寸组合。
- Seedream5.0 Pro普通生图保留；layer_decomposition图层分解模式创建前400，后续另议。不得丢掉图层位置/名称后冒充完整支持该模式。

### 6.2 上游证据缺口（由工程核实，不让用户替供应商定义事实）

| 缺口 | 处理与影响 |
| --- | --- |
| ToAPIs 7个目录ID缺明确schema | 5个图片与2个视频已在清单标明；核实正式别名、模式和字段后才能发布该能力，不复制相邻型号 |
| GRS目录与新文档有出入 | nano-banana已公告下架；pro-vt仅在文档；2-lite目录有但接口枚举漏列；需区分目录/历史/文档并集 |
| 规格/默认值冲突 | 如ToAPIs GPT2Official的n上限4/10冲突、Seedance1.0默认分辨率冲突、VeoLite范围不完整；未解决组合不开放 |
| 特殊输入角色未说明 | GRS Agnes首尾帧、Vidu Ali subjects等需完整官方字段定义；没有定义不能把它们做成“任意metadata” |
| 型号特定查询路径 | Omni1.1专属与通用视频查询路径不同，需要验证可用路径，不能多次创建试探 |
| 任意像素输入与档位映射 | 供应商接受像素不等于已有全档位映射；先核实可开放的比例/档位组合，不扩大下游字段 |

### 6.3 逐特性方案继续收敛，暂不视作用户已定稿

- **视频附带尾帧等辅助结果**：上游return_last_frame之类参数可能要求额外图片；当前result.type=video、data:string[]不能表达混合类型和角色。仅生成视频URL的基础能力不受影响；该辅助模式是否首期开放及如何交付，进入相关spec时单独确认，不能先接受字段再静默丢结果。
- **已有上游素材/角色/父任务ID**：普通可访问图片/视频/音频URL参考属于生成输入；创建角色资源、依赖供应商asset://或引用上游父任务涉及账号归属与新操作，需单独核实/定范围。不能暴露上游task_id让下游自行传回。
- **自动/输出决定的视频时长**：不改变“按Sub2API规格收费”；技术方案需明确预留上界、正式扣费依据及失败释放，涉及收费规则变更再提请确认。
- metadata字段命名、输入对象结构、缺省值与安全限制由F01方案确定；字段组可复用但每个型号最终允许集必须明确。S3保留策略按PRD分阶段处理。

## 7. 公开目录的完整ID清单

下表每行对应一个平台+媒体类型+上游ID；“有生成文档”不表示已满足所有参数证据或已经实测。具体字段请查上述对应research。

### 7.1 GRS.AI 当前目录（18条）

| 上游精确ID | 媒体 | 文档族 | 目录状态 |
| --- | --- | --- | --- |
| `nano-banana-2.1` | image | Nano | 未标维护；未实测 |
| `nano-banana-pro` | image | Nano | 未标维护；未实测 |
| `nano-banana-2` | image | Nano | 未标维护；未实测 |
| `nano-banana-2-lite` | image | Nano | 未标维护；未实测 |
| `nano-banana-fast` | image | Nano | 未标维护；未实测 |
| `nano-banana-pro-cl` | image | Nano | 未标维护；未实测 |
| `nano-banana-2-cl` | image | Nano | 未标维护；未实测 |
| `nano-banana-2-2k-cl` | image | Nano | 未标维护；未实测 |
| `nano-banana-pro-vip` | image | Nano | 未标维护；未实测 |
| `nano-banana-pro-4k-vip` | image | Nano | 未标维护；未实测 |
| `nano-banana-2-4k-cl` | image | Nano | 未标维护；未实测 |
| `gpt-image-2` | image | GPT | 维护中；不据此自动启用 |
| `gpt-image-2-vip` | image | GPT | 未标维护；未实测 |
| `gpt-image-2.5` | image | GPT | 维护中；不据此自动启用 |
| `gpt-image-2.5-flare` | image | GPT | 维护中；不据此自动启用 |
| `gpt-image-2.5-sunburst` | image | GPT | 维护中；不据此自动启用 |
| `minimax-h3` | video | MiniMax | 未标维护；未实测 |
| `agnes-video-2.5` | video | Agnes | 未标维护；未实测 |

### 7.2 ToAPIs 公开目录（90条）

| 上游精确ID | 媒体 | 证据状态 | 逐型号文档 |
| --- | --- | --- | --- |
| `doubao-seedream-4-0` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/seedream-4.0/generation) |
| `doubao-seedream-4-5` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/seedream-4.5/generation) |
| `doubao-seedream-5-0` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/seedream-5.0/generation) |
| `doubao-seedream-5-0-pro` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/seedream-5.0-pro/generation) |
| `flux-2-flex` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/flux-2/generation) |
| `flux-2-pro` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/flux-2/generation) |
| `flux-kontext-max` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/flux-kontext/generation) |
| `flux-kontext-pro` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/flux-kontext/generation) |
| `gemini-2.5-flash-image-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-2.5-flash-official/generation) |
| `gemini-2.5-flash-image-preview` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-2.5-flash/generation) |
| `gemini-2.5-flash-image-preview-vip` | image | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |
| `gemini-3-pro-image-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image-official/generation) |
| `gemini-3-pro-image-preview` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image/generation) |
| `gemini-3-pro-image-preview-vip` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image-vip/generation) |
| `gemini-3.1-flash-image-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash-official/generation) |
| `gemini-3.1-flash-image-preview` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash/generation) |
| `gemini-3.1-flash-image-preview-vip` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash-vip/generation) |
| `gemini-3.1-flash-lite-image-official` | image | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |
| `gemini-nano-banana-2.1` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1/generation) |
| `gemini-nano-banana-2.1-vip` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1/generation) |
| `gemini-nano-banana-2.1-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1-official/generation) |
| `gpt-image-2` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2/generation) |
| `gpt-image-2-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2/official) |
| `gpt-image-2-vip` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2-vip/generation) |
| `gpt-image-2.5-flare` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5/generation) |
| `gpt-image-2.5-flare-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-official/generation) |
| `gpt-image-2.5-flare-vip` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-vip/generation) |
| `gpt-image-2.5-sunburst` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5/generation) |
| `gpt-image-2.5-sunburst-official` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-official/generation) |
| `gpt-image-2.5-sunburst-vip` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-vip/generation) |
| `grok-imagine-1.0` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/grok-imagine-1.0/generation) |
| `grok-imagine-1.0-edit` | image | 编辑专用；本期非普通生成 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/grok-imagine-1.0/edits) |
| `nano_banana` | image | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |
| `nano_banana_2` | image | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |
| `nano_banana_pro` | image | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |
| `qwen-image-3.0` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/qwen-image-3.0/generation) |
| `qwen-image-3.0-pro` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/qwen-image-3.0/generation) |
| `vidu-image` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation) |
| `viduq2-fast` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation) |
| `viduq2-pro` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation) |
| `viduq3-fast` | image | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation) |
| `doubao-seedance-1-0-pro-fast` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance/generation) |
| `doubao-seedance-1-0-pro-quality` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance/generation) |
| `doubao-seedance-1-5-pro` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance-1-5/generation) |
| `gemini_omni` | video | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |
| `gemini_omni_flash` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation) |
| `gemini-omni-1.1-flash-preview` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation-1-1) |
| `gemini-omni-1.1-flash-preview-official` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-1.1-flash-preview-official/generation) |
| `gemini-omni-1.1-flash-preview-vip` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation-1-1-vip) |
| `gemini-omni-flash-preview-official` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash-preview-official/generation) |
| `grok-video-1.0` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/grok-video/generation) |
| `grok-video-1.5` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/grok-video-1.5/generation) |
| `happyhorse-1.1` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/happyhorse/generation) |
| `kling-3.0-turbo` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/kling-3.0-turbo/generation) |
| `kling-v2-6` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/kling-2-6/generation) |
| `kling-v3` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/kling-v3/generation) |
| `kling-v3-omni` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/kling-v3-omni/generation) |
| `kling-video-o1` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/kling-video-o1/generation) |
| `MiniMax-H3` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation) |
| `MiniMax-H3-Max` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation) |
| `MiniMax-H3-Max-Turbo` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation) |
| `MiniMax-Hailuo-02` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo/generation) |
| `MiniMax-Hailuo-2.3` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo-2.3/generation) |
| `MiniMax-Hailuo-2.3-Fast` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo-2.3/generation) |
| `seedance-2` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation) |
| `seedance-2-5` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-5/generation) |
| `seedance-2-ext` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-ext/generation) |
| `seedance-2-fast` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation) |
| `seedance-2-fast-ext` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-fast-ext/generation) |
| `seedance-2-mini` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation) |
| `seedance-2.5-ext` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-5-ext/generation) |
| `sora-2-official` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/sora-2-official/generation) |
| `veo3.1-fast` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation) |
| `Veo3.1-fast-official` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation) |
| `veo3.1-lite` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation) |
| `Veo3.1-lite-official` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation) |
| `veo3.1-quality` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation) |
| `Veo3.1-quality-official` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation) |
| `viduq3` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation) |
| `viduq3-ad` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation) |
| `viduq3-drama` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation) |
| `viduq3-mix` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation) |
| `viduq3-pro` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation) |
| `viduq3-pro-fast` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation) |
| `viduq3-turbo` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation) |
| `viduq4-preview` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/viduq4-preview/generation) |
| `wan2.6` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/wan2.6/generation) |
| `wan2.6-flash` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/wan2.6-flash/generation) |
| `wan3.0-video` | video | 有生成文档；参数仍需逐项核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/videos/wan3.0/generation) |
| `wan3.0-video-prime` | video | schema待核实 | [官方文档](https://docs.toapis.com/docs/cn/api-reference/models) |

## 8. 目录媒体区之外的文档线索

| 平台 | 文档ID/族 | 处理 |
| --- | --- | --- |
| GRS | nano-banana、nano-banana-pro-vt | 前者有下架公告，后者目录无登记；不作为当前可用清单 |
| GRS | 旧Veo、Sora、Imagen及角色接口 | 旧页仍存在，当前目录未列；不能据历史页面承诺可调用，更不把独立角色操作纳入普通生成 |
| ToAPIs | gpt-4o-image、grok-4-1-image、grok-4-2-image | 有专门图片生成文档，目录图片区未列；补进能力候选研究，账号可用性待验证 |
| ToAPIs | midjourney 图片与视频能力 | 有独立生成文档；同名可分媒体登记，不与目录90条混算；四宫格规则已定，后续动作另议 |
| ToAPIs | sora-2-vvip、gemini-omni-flash | 前者目录未列，后者有官方明确目录别名gemini_omni_flash；别名不重复计数 |
| ToAPIs | 文本区Gemini 3.1 Pro/3.5 Flash/3.5 Thinking的image-generation标签 | 只有标签、未找到专属图片参数页；不能仅凭标签发布图片schema |

## 9. 交付与后续使用

本轮完成公开文档与源码基线、108条目录枚举及额外型号线索、逐族字段约束、必要的产品澄清和P0拆分。没有生成调用、账号实测、业务实现、提交或部署。

下一步从F01开始编写独立spec，再由plan确定能力schema、基础模板、管理配置和校验契约；F04/F05把本清单逐项变为可验证适配与模型配置。上线覆盖率以“已配置且证据充分、账号可用的生成能力”为依据，同时保留缺口清单，不用目录条数冒充实际支持率。逐模型对外接入文档属于F07，不新增下游动态能力查询。
