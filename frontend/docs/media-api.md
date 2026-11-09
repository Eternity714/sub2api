---
description: 媒体 API 的地址与认证、异步任务查询、结果下载、错误处理和计费规则。
---

# 媒体 API 接入总览

媒体 API 使用统一参数创建图片、视频任务。客户端只填写公开模型名，提交后保存任务 `id`，使用同一个 API Key 查询，完成后下载 `result.data` 中的文件。

| 需要做什么 | 阅读文档 |
| --- | --- |
| 文生图、参考图生图 | [图片生成 API](/media-images) |
| 文生视频、图片或音频参考 | [视频生成 API](/media-videos) |
| 确认当前模型、参数和售价 | [当前模型目录](/media-models/) |
| 查询状态、处理失败、下载文件 | [任务查询与结果](#查询任务与下载结果) |

本套文档适用于标记为**媒体 API**的分组。OpenAI/Gemini 图片、Grok 视频和 Seedance 原生任务使用各自协议，见[其他分组的接口](#其他分组的接口)。

## 地址与认证

API 根地址为 `https://www.gkotta.bid`，本文的完整路径已包含 `/v1`。在[API 密钥](https://www.gkotta.bid/keys)选择媒体 API 分组的 Key，并在[可用渠道](https://www.gkotta.bid/available-channels)确认模型和价格。

所有创建、查询请求都带：

```http
Authorization: Bearer YOUR_SUB2API_KEY
```

Bash / Git Bash 环境变量：

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="YOUR_SUB2API_KEY"
```

Windows PowerShell：

```powershell
$env:SUB2API_BASE_URL = "https://www.gkotta.bid"
$env:SUB2API_KEY = "YOUR_SUB2API_KEY"
```

### 接口清单

| 方法 | 路径 | 成功响应 |
| --- | --- | --- |
| GET | `/v1/models` | 当前 Key 可见的公开模型列表 |
| POST | `/v1/images/generations` | `202`，创建图片任务 |
| GET | `/v1/images/generations/{task_id}` | `200`，查询图片任务 |
| POST | `/v1/videos/generations` | `202`，创建视频任务 |
| GET | `/v1/videos/generations/{task_id}` | `200`，查询视频任务 |

模型列表可用于确认名称；具体媒体类型、参数、权限和价格仍须满足对应模型文档。接口存在不表示分组已开放该类模型。

## 统一参数约定

图片、视频各自的完整字段表见对应指南。常用字段统一如下：

| 含义 | 字段 | 约定 |
| --- | --- | --- |
| 公开模型 | `model` | 使用目录中的完整 ID，区分大小写 |
| 提示词 | `prompt` | 必填性和长度按模型规则 |
| 参考图片 | `images` | HTTP(S) 图片直链数组，保持顺序 |
| 比例 | `aspectRatio` | 如 `3:2`；图片的受限像素写法须另经支持检查 |
| 图片分辨率 | `imageSize` | 仅在模型声明时传入；固定 `-1k/-2k` 型号不接受此字段及其别名 |
| 视频分辨率 | `resolution` | 如 `720p`，按模型页填写 |
| 质量 | `quality` | 仅传模型允许的值 |
| 回复类型 | `replyType` | 目前仅 `async`，创建后轮询 |

未列出的字段不自动透传。模型扩展字段仅能按该模型的公开规则传入 `metadata`。目前不接受 Base64 参考图、本地文件路径、`replyType:json/stream` 或 `metadata.output_size`。

### 旧参数兼容

已有客户端仅在模型声明了对应参数时可以使用旧名字；新接入统一使用下表左列。新旧别名同时传入时，值必须一致。`gpt-image-2.6-1k`、`gpt-image-2.5-1k`、`gpt-image-2.6-2k` 和 `gpt-image-2.5-2k` 不接受 `imageSize`、`resolution` 或 `metadata.resolution`。

| 推荐字段 | 兼容字段 |
| --- | --- |
| 图片 `aspectRatio` | `size` |
| 视频 `aspectRatio` | `aspect_ratio` |
| `images` | `image_urls` |
| `quality` | `metadata.quality` |
| 图片 `imageSize` / 视频 `resolution` | `metadata.resolution` |
| 视频 `audios`（模型支持时） | `metadata.audio_urls` |

兼容字段不会扩大模型能力。例如只开放 `1K` 的模型仍不能请求 `2K`，当前只允许 `n=1` 的模型也不能生成多张。

## 创建响应

图片、视频创建成功均返回 **HTTP 202**，响应只有以下五个字段：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "YOUR_PUBLIC_MODEL_ID",
  "status": "queued",
  "progress": 0,
  "created_at": 1794182400
}
```

`202` 表示任务已受理。立即保存 `id`，之后只查询这个任务。任务属于创建时的用户及 API Key；同一账户下换一个 Key 查询也会返回 `404`。

## 查询任务与下载结果

每隔 **5–10 秒**查询一次。图片、视频使用各自的查询路径，不能互换：

```bash
# 替换为创建响应中的真实 id
export TASK_ID="media_YOUR_TASK_ID"

curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"

# 视频任务改用 /v1/videos/generations/$TASK_ID
```

GET 查询不会再次生成。Key 额度不足以创建新任务时，仍可查询自己已经创建的任务，但 Key 必须能够正常鉴权。

### 状态与进度

| `status` | `progress` | 客户端处理 |
| --- | --- | --- |
| `queued` | `0` | 等待后继续查询 |
| `in_progress` | `50` | 继续查询，尚未交付文件 |
| `completed` | `100` | 停止轮询，检查结果与错误 |

进度值固定为 0 / 50 / 100。**`completed` 只表示处理结束，成功与否由 `result.data` 和 `error` 决定。**

| 完成后的内容 | 含义 |
| --- | --- |
| `result.data` 有 URL，`error` 为空 | 成功交付 |
| `result.data` 有 URL，`error` 非空 | 部分结果成功；适用于支持多结果的任务 |
| `result.data` 为空，`error` 非空 | 未交付结果，按失败处理 |

当前开放模型的数量上限见模型页；不要为演示部分成功而向只支持一张的模型发送 `n=2`。

### 查询响应

图片成功示例：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.6-1k",
  "status": "completed",
  "progress": 100,
  "created_at": 1794182400,
  "completed_at": 1794182460,
  "expires_at": 1794268860,
  "result": {
    "type": "image",
    "data": ["https://files.example.com/result.png"]
  },
  "error": ""
}
```

视频使用相同结构，`result.type` 为 `video`，`result.data` 中是视频 URL。处理中也返回 `result`，但 `data` 为空数组；文件保存和结算完成后才统一交付。

| 字段 | 返回规则 |
| --- | --- |
| `id`、`model` | 当前任务与公开模型 |
| `status`、`progress` | 上表定义的状态和固定进度 |
| `created_at` | 创建时间，Unix 秒 |
| `completed_at` | 仅终态返回，Unix 秒 |
| `expires_at` | 有成功结果时返回，Unix 秒 |
| `result.type` | `image` 或 `video` |
| `result.data` | 文件 URL 字符串数组 |
| `error` | 错误摘要字符串，无错误时为 `""` |

完成后的生成失败仍返回 HTTP 200，例如 `status:"completed"`、`result.data:[]`、`error:"media_task_expired"`。HTTP 成功不等于生成成功；没有 `result.errors` 或逐项 `index` 字段。

### 下载文件

用查询响应中的完整 URL 下载，**不向文件地址附加 Gkotta API Key**：

```bash
curl --fail-with-body "YOUR_RETURNED_RESULT_URL" -o result.png
# 视频可保存为 result.mp4，扩展名按实际文件格式选择
```

结果 URL 可能含临时访问凭证。在 `expires_at` 前下载自己的文件；结果过期后查询返回 `410`。时间戳单位是秒，JavaScript 使用 `new Date(expires_at * 1000)` 转换。

## 错误处理

创建前会校验模型、媒体类型、字段、必填项、枚举、范围、别名冲突和字段组合。参数错误不创建任务、不预留额度。无法找到支持该组参数的生成服务时，也会在创建前拒绝。

HTTP 错误的 `error` 为对象；已创建任务查询响应的 `error` 为字符串：

```json
{
  "error": {
    "type": "INVALID_MEDIA_CAPABILITY",
    "message": "invalid media capability configuration or request",
    "fields": [
      {"path":"duration","code":"unknown_field","message":"field is not declared for this media capability"}
    ]
  }
}
```

`fields` 只在有字段详情时返回。`path` 可能采用上表中的兼容字段名；按对应的统一字段修正参数即可。错误文本可能扩展，程序不要依赖全部消息的精确拼写。

| HTTP | 常见原因 | 处理方式 |
| --- | --- | --- |
| `400` | 模型或参数不支持、冲突、余额不足 | 阅读 `error` 后修正，不盲目重试 |
| `401` | Key 无效、过期或鉴权失败 | 检查 Bearer Key 及状态 |
| `403` | 分组或 Key 没有生成权限 | 核对分组与可用渠道 |
| `404` | 任务不存在、Key/路径不匹配、分组未提供接口 | 使用原 Key、真实 id 和对应媒体路径 |
| `410` | 结果已过期 | 使用此前保存的文件 |
| `429` | Key 额度、速率或请求频率受限 | 核对额度，查询可延后重试 |
| `5xx` | 服务暂时不可用 | GET 可延后重试；POST 按下节核查受理情况 |

## 计费与重复提交

创建成功时预留所需额度，完成生成和文件保存后按任务创建时的价格结算。图片按成功交付的张数，视频按对应模型的秒数或次数计费；失败部分释放预留。价格变更只影响新任务。

参考图下载或生成服务请求失败时，系统可能选择其他支持本次参数的服务。客户端继续查询原任务即可，不需要自行再创建一个任务；不会因内部切换重复按同一结果收费。受理结果不明时不会盲目重新生成，最终以查询结果为准。

目前创建接口不支持 `Idempotency-Key` 去重。**重复 POST 会创建独立任务，可能重复收费。**拿到 `id` 后仅重试 GET；POST 超时且未拿到 `id` 时，先核查[使用记录](https://www.gkotta.bid/usage)或联系管理员确认受理情况。

## Node.js 轮询示例

下面使用 Node.js 18+ 的 `fetch`。创建只发一次，GET 临时失败才重试。设置 `MEDIA_TASK_ID` 可恢复查询已有任务；图片创建参数见[生图指南](/media-images)，视频创建参数见[生视频指南](/media-videos)。

```js
const base = (process.env.SUB2API_BASE_URL || 'https://www.gkotta.bid').replace(/\/+$/, '');
const key = process.env.SUB2API_KEY;
const kind = process.env.MEDIA_KIND || 'image';
const routes = {image: '/v1/images/generations', video: '/v1/videos/generations'};
if (!key || !routes[kind]) throw new Error('请设置 Key，并选择 image 或 video');
const headers = {Authorization: `Bearer ${key}`, 'Content-Type': 'application/json'};
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

async function createImage() {
  if (kind !== 'image') throw new Error('视频请先按视频指南创建，再设置 MEDIA_TASK_ID 查询');
  const response = await fetch(base + routes.image, {
    method: 'POST', headers,
    body: JSON.stringify({
      model: 'gpt-image-2.6-1k', prompt: '白色桌面上的蓝色陶瓷杯',
      aspectRatio: '3:2', quality: 'auto', replyType: 'async', n: 1,
    }),
    signal: AbortSignal.timeout(60000),
  });
  const task = await response.json();
  if (response.status !== 202 || typeof task.id !== 'string') {
    throw new Error(`创建失败 HTTP ${response.status}: ${JSON.stringify(task)}`);
  }
  console.log('保存任务 ID：', task.id);
  return task.id;
}

async function waitForTask(id) {
  const deadline = Date.now() + 30 * 60 * 1000;
  while (Date.now() < deadline) {
    let response;
    try {
      response = await fetch(`${base}${routes[kind]}/${encodeURIComponent(id)}`, {
        headers, signal: AbortSignal.timeout(30000),
      });
    } catch {
      await pause(5000); // 只重试 GET
      continue;
    }
    if (response.status === 429 || response.status >= 500) {
      await pause(10000);
      continue;
    }
    const task = await response.json();
    if (!response.ok) throw new Error(`查询失败 HTTP ${response.status}: ${JSON.stringify(task)}`);
    if (task.status === 'completed') return task;
    if (!['queued', 'in_progress'].includes(task.status)) throw new Error('未知任务状态');
    await pause(5000);
  }
  throw new Error(`等待超时，请保留 ${id} 稍后继续查询，不要重新 POST`);
}

const id = process.env.MEDIA_TASK_ID || await createImage();
const task = await waitForTask(id);
console.log('可交付文件：', task.result.data);
if (task.error) console.error('任务错误摘要：', task.error);
```

30 分钟是示例客户端的等待上限，不代表服务端任务已失败。所有文件链接都需要在过期前另行下载保存。

## 其他分组的接口

| 分组协议 | 说明 |
| --- | --- |
| OpenAI Images / Gemini 原生图片 | [独立图片协议](/image-generate)，同步响应或原生内容格式 |
| Grok 视频 | [Grok 视频 API](/video-generation)，使用 `request_id` 和 `/v1/videos/{request_id}` |
| Seedance 原生任务 | [Seedance API](/seedance-api)，使用 `/api/v3/contents/generations/tasks` |

按 Key 所属分组选择一整套请求、查询与响应格式。独立协议里的模型示例不表示已在统一媒体 API 中开放。
