# 统一图片与视频生成 API

本功能仅对配置了 GRS.AI／ToAPIs 媒体路由的 Composite 分组启用。其它分组继续使用原有平台协议。所有请求使用 Sub2API API Key：`Authorization: Bearer <key>`。

## 管理员启用步骤

1. 在账号管理添加 GRS.AI／ToAPIs API Key 账号，明确配置 Base URL、密钥、可调度状态及共享总／日／周额度。ToAPIs 的“测试连接”只查询模型目录，不触发生图或视频消费。
2. 创建或选择 Composite 分组并关联账号，开启现有媒体生成许可。
3. 在“媒体模型能力”中选择模型基础模板，复制为对外 `public_model`，核对参数后启用。模板默认不创建配置、不自动启用；模板可用不代表账号已实测成功。
4. 在分组模型路由中添加 `images` 或 `videos` 路由：公开模型、目标平台、上游模型及优先级。相同公开模型可有多个候选。候选规格不同则分别建立能力记录，通过 `capability_model` 引用；为空使用公开模型能力。
5. 按公开模型配置图片／视频／按次价格。规格分层价格的标签必须精确匹配 `metadata.resolution`。视频按秒价格需显式正数时长；按次价格按成功结果计费。账号倍率不改变下游已确定的价格。
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
  -d '{"model":"your-image-model","prompt":"一只站在窗边的猫","size":"16:9","n":2,"metadata":{"resolution":"2K"}}'
```

顶层仅接受：

| 字段 | 类型 | 说明 |
|---|---|---|
| model | string | 必填，公开模型名 |
| prompt | string | 必填性由模型及输入模式决定 |
| size | string | 宽高比，例如 `16:9`；枚举由 schema 定义 |
| n | integer | 所需结果数；省略值及上限由 schema 定义 |
| image_urls | string[] | 按模型支持的参考图 URL，顺序保留 |
| metadata | object | schema 声明的模型特定参数；分辨率使用 `resolution` |

`size` 不接受像素字符串。需要具体像素的上游使用能力配置中的比例／分辨率映射；客户端无需传像素字段。不支持 `metadata.output_size`。

## 创建视频

```bash
curl "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"your-video-model","prompt":"镜头缓慢向前移动","duration":5,"aspect_ratio":"16:9","metadata":{"resolution":"720p"}}'
```

顶层仅接受 `model`、`prompt`、`duration`、`aspect_ratio`、`image_urls`、`metadata`。`duration` 单位秒；类型、枚举、必填性及默认值由模型 schema 定义。没有视频顶层 `n`。首期不开放自动时长与未确定收费上限的模式。

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

自动化采用假上游验证 HTTP 转换、真 PostgreSQL 验证事务及账务，不消耗真实平台余额。发布前仍应由管理员使用明确授权的测试账号分别验证 GRS.AI、ToAPIs 与实际 S3 可访问性。
