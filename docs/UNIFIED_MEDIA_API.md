# 统一图片与视频生成 API

本文面向管理员与接口维护者，说明配置、校验、路由和计费实现。下游用户教程统一维护在 `frontend/docs/`：

| 用户入口 | 源文件 | 内容 |
| --- | --- | --- |
| `/docs/media-api` | `frontend/docs/media-api.md` | 地址、鉴权、异步查询、结果、错误与计费 |
| `/docs/media-images` | `frontend/docs/media-images.md` | 文生图、参考图生图与统一图片参数 |
| `/docs/media-videos` | `frontend/docs/media-videos.md` | 视频协议、参考素材及当前开放状态 |
| `/docs/media-models/` | 由 `docs/media-api-catalog.json` 生成 | 实际已开放模型、精确限制与售价快照 |

接口或公开能力变更时，同步更新这些页面和公开快照；生成与验证流程见 `frontend/docs/README.md`。本文件及后台模板目录描述可配置能力，不代表当前下游已开放型号。

本功能仅对配置了 GRS.AI／ToAPIs／快跑 媒体路由的 Composite 分组启用。其它分组继续使用原有平台协议。所有请求使用 Sub2API API Key：`Authorization: Bearer <key>`。

## 管理员启用步骤

1. 在账号管理添加 GRS.AI／ToAPIs／快跑 API Key 账号，明确配置 Base URL、密钥、可调度状态及共享总／日／周额度。ToAPIs 的“测试连接”只查询模型目录，不触发生图或视频消费。
2. 创建或选择 Composite 分组并关联账号，开启现有媒体生成许可。
3. 在“媒体模型能力”中选择模型基础模板，复制为对外 `public_model`，核对参数后启用。模板默认不创建配置、不自动启用；模板可用不代表账号已实测成功。
4. 在分组模型路由中添加 `images` 或 `videos` 路由：公开模型、目标平台、上游模型及优先级。相同公开模型可有多个候选。原生媒体路由的参数由 Provider 自动适配，`capability_model` 留空；下游每个公开模型只维护一份能力记录。Provider 按上游模型的内置规则筛选候选，并冻结参数及接口映射规则供任务恢复使用。没有已核实规则的原生上游会在创建前拒绝。旧的显式 `capability_model` 仍兼容，仅用于旧配置及通用 OpenAI 图片接入。
5. 在“渠道管理”创建并启用渠道、关联该 Composite 分组，按公开模型配置图片／视频／按次价格。统一媒体只使用关联有效渠道价，不读取分组模型价；缺价在创建前返回 `400 MEDIA_PRICING_MISSING`。规格分层价格的标签必须精确匹配 `imageSize`（图片）或 `resolution`（视频）。视频按秒价格需显式正数时长；按次价格按成功结果计费。本站余额数值 1 对应人民币 1 元（界面仍显示 `$`），ToAPIs 的 USD 参考价在配置时乘以 7，运行时不重复换算。调价仅影响新任务，已创建任务按原快照结算。账号倍率不改变下游已确定的价格。
6. 在后台对象存储配置中启用 S3 并确认可用。图片及视频都先转存 S3，再完成结算与对外交付。
7. 用测试 Key 创建任务并查询，随后在“媒体任务”核对账号、执行单元、冻结金额和实际扣费。

模板逐模型参数见 [模型参数表](UNIFIED_MEDIA_MODEL_PARAMETERS.md)。没有能力配置、缺少价格或不支持的参数会在创建前拒绝。

## 端点

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/v1/images/generations` | 创建图片任务，202 |
| GET | `/v1/images/generations/{task_id}` | 查询当前 Key 的图片任务 |
| POST | `/v1/videos/generations` | 创建视频任务，202 |
| GET | `/v1/videos/generations/{task_id}` | 查询当前 Key 的视频任务 |

只对上述两个 POST 路径启用统一协议，不给其它原有入口增加别名。原 `POST /v1/api/generate`、`GET /v1/api/result`、`GET /v1/api/tasks` 已移除。

## 创建图片

```bash
curl "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"your-image-model","prompt":"一只站在窗边的猫","aspectRatio":"16:9","imageSize":"2K","quality":"auto","replyType":"async","n":2}'
```

下游采用 GRS 风格参数，具体支持范围由公开模型能力定义：

| 字段 | 类型 | 说明 |
|---|---|---|
| model | string | 必填，公开模型名，例如 `gpt-image-2.6` |
| prompt | string | 提示词，必填性由模型及输入模式决定 |
| images | string[] | 参考图 HTTP(S) URL，按输入顺序传递 |
| aspectRatio | string | 宽高比，例如 `3:2`；图片也支持 `1536x1024` 像素写法，由公开规则和 Provider 校验 |
| imageSize / resolution | string | 分辨率档位，例如 `1K`；两者为同一参数的别名，不能传入冲突值 |
| quality | string | 质量，例如 `auto`，具体枚举由公开规则和 Provider 支持范围决定 |
| replyType | string | 目前只支持 `async`，返回 202 后轮询任务 |
| n | integer | 所需图片数；默认值、上限和是否允许独立拆分由公开规则定义 |
| metadata | object | 兼容已有模型扩展参数，仅接受能力 schema 声明的属性 |

旧请求 `size`、`image_urls`、`metadata.quality` 和 `metadata.resolution` 继续兼容；与新名字同时传入时必须相同。GRS 风格和旧风格 schema 都可用于公开能力配置。内部归一化后交给 Provider；GRS 转为 `images/aspectRatio`，快跑转为 `image/aspect_ratio/resolution`，ToAPIs 按对应模型接口映射。快跑 `gpt-image-2.6-1k` 固定 1K，不会把 2K/4K 请求降档。比例和分辨率档位不承诺跨上游完全相同的输出像素。显式像素只选择能保留该尺寸的候选，例如 `1536x1024` 可交给 GRS，快跑固定 1K 的 3:2 网格为 `1248x832`，不会静默替换。

公开规则确定下游接受范围，Provider 在其中筛选实际支持的上游。只会补充有文档依据的默认值；`quality:auto` 允许选择上游默认质量，显式质量和分辨率不做降级。全部候选都不支持时在创建任务、冻结余额和调用上游前返回参数错误。`replyType:json/stream`、Base64 参考图及 `metadata.output_size` 暂未开放。

## 创建视频

```bash
curl "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"your-video-model","prompt":"镜头缓慢向前移动","duration":5,"aspectRatio":"16:9","resolution":"720p","replyType":"async"}'
```

视频同样使用 `images/aspectRatio/resolution/replyType`；已有 `aspect_ratio/image_urls/metadata` 写法继续兼容。`audios` 仅在模型规则声明音频参考时开放。`duration` 单位秒；类型、枚举、必填性及默认值由模型 schema 定义。没有视频顶层 `n`。首期不开放自动时长与未确定收费上限的模式。

未知顶层字段、未知或嵌套 metadata 字段、重复 JSON 键、错误类型、范围、枚举、必填和组合错误均返回 400，响应指出字段；不建任务、不冻结、不请求生成。图片路径传视频模型或视频字段同样拒绝。公开同名模型可以分别登记图片和视频能力，由路径选择。

## 创建响应

```json
{"id":"media_4b0d7db4-8d24-4b67-afb9-f3364a0c7db3","model":"your-image-model","status":"queued","progress":0,"created_at":1792036800}
```

202 响应仅包含上述五个字段。保留任务 ID 后轮询对应 GET，建议 5–10 秒一次。创建不是成功承诺；账号不可用也会创建本地任务，最终返回错误并释放预留。

## 查询与结果

```bash
curl "$SUB2API_BASE_URL/v1/images/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

处理中：

```json
{"id":"media_example","model":"your-image-model","status":"in_progress","progress":50,"created_at":1792036800,"result":{"type":"image","data":[]},"error":""}
```

全部成功：

```json
{"id":"media_example","model":"your-image-model","status":"completed","progress":100,"created_at":1792036800,"completed_at":1792036860,"expires_at":1792123260,"result":{"type":"image","data":["https://your-s3.example/result-1.png","https://your-s3.example/result-2.png"]},"error":""}
```

部分成功：

```json
{"id":"media_example","model":"your-image-model","status":"completed","progress":100,"created_at":1792036800,"completed_at":1792036860,"expires_at":1792123260,"result":{"type":"image","data":["https://your-s3.example/result-1.png"]},"error":"some_media_results_failed"}
```

全部失败：

```json
{"id":"media_example","model":"your-image-model","status":"completed","progress":100,"created_at":1792036800,"completed_at":1792036860,"result":{"type":"image","data":[]},"error":"no_available_media_account"}
```

- 状态仅 `queued`、`in_progress`、`completed`，进度固定 0／50／100。`completed` 表示整项处理结束，成功与否看 `result.data` 和 `error`。
- 所有时间为 Unix 秒。终态有 `completed_at`，成功结果才有 `expires_at`。过期结果返回 410。
- `result.data` 固定为 URL 字符串数组，仅包含 S3 结果，不暴露上游 URL、账号凭据或原始请求。
- 所有执行单元结束、S3 转存和结算完成后一次性返回可交付结果。GET 只读，不触发生成或转存。
- 任务必须同时属于同一用户及同一 API Key；另一 Key 或错误媒体路径返回 404。Key 额度耗尽仍可读取自己已创建的任务。
- S3 转存暂时失败继续 `in_progress` 并重试，耗尽后终结并释放未交付部分。部分成功只收实际交付数量的费用。
- 无客户端 `Idempotency-Key`；内部持久提交意图防止 worker 恢复时重复生成，受理不明不重新提交。
- Midjourney 四宫格作为一个图片结果计费；增强、变体、图层分解、辅助尾帧与账号绑定素材等特殊操作需另行支持。

## 管理诊断

管理员可使用 `GET /api/v1/admin/media-tasks` 和 `GET /api/v1/admin/media-tasks/{id}` 查看任务、账号绑定、上游任务 ID、执行状态、冻结金额与扣费。诊断接口不返回解密后的提示词和密钥，也不提供重新生成操作。

## 本机验证

所有测试从项目 worktree 通过 Docker Compose 执行：

```powershell
docker compose -f deploy/docker-compose.test.yml run --rm backend-test
docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test
docker compose -f deploy/docker-compose.test.yml run --rm --no-deps backend-integration
```

自动化采用假上游验证 HTTP 转换、真 PostgreSQL 验证事务及账务，不消耗真实平台余额。发布前仍应由管理员使用明确授权的测试账号分别验证 GRS.AI、ToAPIs、快跑与实际 S3 可访问性。
