---
description: 媒体 API 的统一图片与视频生成协议：创建异步任务、查询结果、参数校验、错误处理与轮询示例。
---

# 媒体 API 接入指南

媒体 API 使用统一的异步协议生成图片和视频。选择模型、创建任务并保存 `id`，再使用同一个 API Key 查询，任务完成后从 `result.data` 获取文件 URL。

每个模型支持的比例、分辨率、时长、参考图及其他参数见[媒体模型目录](/media-models/)。例如 [GPT Image 2](/media-models/gpt-image-2) 和 [Nano Banana 2.1](/media-models/nano-banana-2.1)。模型及价格是否对当前密钥开放，以[可用渠道](https://www.gkotta.bid/available-channels)为准。

::: warning 先确认密钥使用媒体 API 分组
本页适用于标记为“媒体 API”的分组。创建接口固定返回异步任务，图片 `size` 表示宽高比，查询路径包含 `/generations/`。其它协议分组可能使用同名创建路径，但请求字段与响应格式不同，请使用对应协议的文档。
:::

## 地址与认证

API 根地址为 `https://www.gkotta.bid`，下列路径已包含 `/v1`。所有创建和查询请求均使用：

```http
Authorization: Bearer YOUR_SUB2API_KEY
```

以下 cURL 示例使用 Bash，可在 macOS、Linux 或 Windows Git Bash 中运行。先设置变量，根地址末尾不加 `/`：

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="YOUR_SUB2API_KEY"
```

Windows PowerShell 中设置变量：

```powershell
$env:SUB2API_BASE_URL = "https://www.gkotta.bid"
$env:SUB2API_KEY = "YOUR_SUB2API_KEY"
```

| 方法 | 路径 | 成功 HTTP 状态 | 用途 |
| --- | --- | --- | --- |
| `POST` | `/v1/images/generations` | `202` | 创建图片任务 |
| `GET` | `/v1/images/generations/{task_id}` | `200` | 查询图片任务 |
| `POST` | `/v1/videos/generations` | `202` | 创建视频任务 |
| `GET` | `/v1/videos/generations/{task_id}` | `200` | 查询视频任务 |

`task_id` 使用创建响应中的 `id`，不是模型名。任务归属于创建它的用户及 API Key；同一账户下换一个 Key 查询也会返回 `404`。查询已有任务不要求 Key 剩余额度足够再次生成，但 Key 必须仍可正常鉴权。

## 创建图片任务

图片接口只接受以下顶层字段。表格列出协议的字段范围，不表示每个模型都支持全部字段；模型页没有声明的字段也不能传入。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | `string` | 必填，当前分组开放的图片模型 ID |
| `prompt` | `string` | 提示词；是否必填由模型及输入模式决定 |
| `size` | `string` | 图片宽高比，例如 `1:1`、`16:9`；允许值见模型页 |
| `n` | `integer` | 希望生成的图片结果数；默认值、上限及批量限制见模型页 |
| `image_urls` | `string[]` | 模型支持的参考图 URL；数量、顺序和输入要求见模型页 |
| `metadata` | `object` | 模型特有参数，只接受模型页声明的字段 |

分辨率放在 `metadata.resolution`，例如某些模型支持 `1K`、`2K`、`4K`。`size` 不接受 `1024x1024` 等像素尺寸，也不支持 `metadata.output_size`。比例与分辨率必须是该模型支持的组合。

省略字段时，仅应用该模型明确声明的默认值。如果 `metadata` 本身声明了默认 `{}`，省略时会先创建对象，再应用子字段默认值；如果父对象没有默认值，子字段默认值不会凭空创建父对象。每个模型页均说明实际规则，需要特定输出规格时建议明确传入对应字段。

下面使用 `gpt-image-2` 创建一张图片；可选值及参考图用法见[模型文档](/media-models/gpt-image-2)：

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "gpt-image-2",
    "prompt": "一只坐在窗边的橘猫，柔和自然光，细节清晰",
    "size": "1:1",
    "n": 1,
    "metadata": {"resolution": "1K"}
  }'
```

创建成功返回 **HTTP `202 Accepted`**，响应只包含以下五个字段：

```json
{
  "id": "media_4b0d7db4-8d24-4b67-afb9-f3364a0c7db3",
  "model": "gpt-image-2",
  "status": "queued",
  "progress": 0,
  "created_at": 1792108800
}
```

`202` 表示任务已受理。保存 `id` 后开始查询，不能把创建成功当作图片已生成。

## 创建视频任务

视频接口只接受以下顶层字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | `string` | 必填，当前分组开放的视频模型 ID |
| `prompt` | `string` | 提示词；是否必填由模型及输入模式决定 |
| `duration` | `number` | 视频时长，单位秒；是否要求整数、范围及默认值见模型页 |
| `aspect_ratio` | `string` | 视频宽高比，例如 `16:9`、`9:16` |
| `image_urls` | `string[]` | 模型支持的参考图 URL；数量、顺序和输入要求见模型页 |
| `metadata` | `object` | 模型特有参数，包括该模型支持的 `resolution` 等字段 |

视频没有顶层 `size` 或 `n` 字段。分辨率同样放在 `metadata.resolution`，具体值由模型决定，例如 `480p`、`720p`、`1080p`，不能直接照搬图片模型的分辨率。

下面是文生视频示例。使用前在[模型目录](/media-models/)确认密钥可用的模型及参数：

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "doubao-seedance-1-0-pro-fast",
    "prompt": "海边日出，海浪轻柔拍岸，镜头缓慢前移",
    "duration": 5,
    "aspect_ratio": "16:9",
    "metadata": {"resolution": "480p"}
  }'
```

成功响应同样为 HTTP `202`：

```json
{
  "id": "media_2f45139b-fcbe-4463-b4af-ac40319be4c0",
  "model": "doubao-seedance-1-0-pro-fast",
  "status": "queued",
  "progress": 0,
  "created_at": 1792108800
}
```

参考图、首尾帧或其它模式的参数各不相同，请直接使用对应模型页的示例。是否允许省略 `prompt` 也由该模型的输入模式决定。

## 查询任务与下载结果

图片查询：

```bash
export TASK_ID="media_4b0d7db4-8d24-4b67-afb9-f3364a0c7db3"

curl --fail-with-body \
  "$SUB2API_BASE_URL/v1/images/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

视频查询：

```bash
export TASK_ID="media_2f45139b-fcbe-4463-b4af-ac40319be4c0"

curl --fail-with-body \
  "$SUB2API_BASE_URL/v1/videos/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

建议每隔 **5–10 秒**查询一次；视频生成可能需要更长时间。GET 查询只读取状态，不会重复创建任务。图片任务不能用视频路径查询，反之亦然。

### 状态与进度

| `status` | `progress` | 含义 |
| --- | --- | --- |
| `queued` | `0` | 任务已受理，等待处理 |
| `in_progress` | `50` | 任务处理中，继续查询 |
| `completed` | `100` | 整项任务已结束，检查结果与错误 |

`progress` 是固定状态值，不是实时生成百分比。**`completed` 只表示处理结束，成功与否由 `result.data` 和 `error` 判断。**

| 查询响应字段 | 类型 | 返回规则 |
| --- | --- | --- |
| `id` | `string` | 任务 ID |
| `model` | `string` | 创建任务时使用的模型 ID |
| `status` | `string` | `queued`、`in_progress` 或 `completed` |
| `progress` | `integer` | 分别为 `0`、`50`、`100` |
| `created_at` | `integer` | 创建时间，Unix 秒 |
| `completed_at` | `integer` | 仅任务完成时返回，Unix 秒 |
| `expires_at` | `integer` | 有成功结果时返回可获取截止时间，Unix 秒 |
| `result` | `object` | 包含 `type` 和 `data` |
| `result.type` | `string` | 图片为 `image`，视频为 `video` |
| `result.data` | `string[]` | 可下载的结果 URL；处理中为空数组 |
| `error` | `string` | 处理中或无错误时为 `""`，完成后有错误时为摘要字符串 |

处理中响应示例：

```json
{
  "id": "media_4b0d7db4-8d24-4b67-afb9-f3364a0c7db3",
  "model": "gpt-image-2",
  "status": "in_progress",
  "progress": 50,
  "created_at": 1792108800,
  "result": {"type": "image", "data": []},
  "error": ""
}
```

服务会在全部生成、文件保存和费用结算处理结束后，一次性返回可交付结果。处理中不会提前暴露部分文件。

### 全部成功

图片示例：

```json
{
  "id": "media_4b0d7db4-8d24-4b67-afb9-f3364a0c7db3",
  "model": "gpt-image-2",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792108860,
  "expires_at": 1792195260,
  "result": {
    "type": "image",
    "data": ["https://files.example.com/media/image-1.png"]
  },
  "error": ""
}
```

视频示例：

```json
{
  "id": "media_2f45139b-fcbe-4463-b4af-ac40319be4c0",
  "model": "doubao-seedance-1-0-pro-fast",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792109100,
  "expires_at": 1792195500,
  "result": {
    "type": "video",
    "data": ["https://files.example.com/media/video-1.mp4"]
  },
  "error": ""
}
```

示例 URL 仅用于说明格式。实际文件 URL 来自查询响应，下载时直接使用完整 URL，通常无需附带 API Key。URL 可能包含临时访问凭证，应按结果文件的访问链接妥善保管。

### 部分成功

例如创建图片任务时请求 `n=2`，完成后只交付一张图片：

```json
{
  "id": "media_a51b008b-e709-426b-b7a6-6c8d2c696b1c",
  "model": "gpt-image-2",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792108920,
  "expires_at": 1792195320,
  "result": {
    "type": "image",
    "data": ["https://files.example.com/media/image-1.png"]
  },
  "error": "media_result_storage_failed"
}
```

`result.data` 保留成功的文件，`error` 汇总失败原因，没有 `result.errors` 或逐项 `index`。只收取实际成功交付部分的费用。普通单视频请求只请求一个结果，按同样的 `result.data` 和 `error` 规则判断交付情况。

### 全部失败

已创建任务的最终失败仍通过查询返回 **HTTP `200`**，任务状态为 `completed`，结果数组为空：

```json
{
  "id": "media_a51b008b-e709-426b-b7a6-6c8d2c696b1c",
  "model": "gpt-image-2",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792108920,
  "result": {"type": "image", "data": []},
  "error": "media_result_storage_failed"
}
```

视频失败示例：

```json
{
  "id": "media_2f45139b-fcbe-4463-b4af-ac40319be4c0",
  "model": "doubao-seedance-1-0-pro-fast",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792109100,
  "result": {"type": "video", "data": []},
  "error": "media_task_expired"
}
```

全部失败不返回 `expires_at`，预留费用全部释放。客户端应保留 `id` 和错误摘要方便排查，不要无限重试生成。

## 参数校验与 HTTP 错误

参数按模型严格校验。图片接口传视频模型、图片请求传 `duration`、视频请求传 `size`，以及未知字段、错误类型、不支持的枚举或组合、重复 JSON 键等，都会在创建前拒绝。`metadata` 中的字段及嵌套字段也必须在该模型文档中声明；不能把任意自定义数据放入 `metadata`。

未通过参数校验的请求不会创建任务或预留额度。不能用其它协议的字段替代，例如把 `resolution` 放在顶层、用 `messages` 代替 `prompt`，或者把 `response_format` 放在任意模型中。

HTTP 错误中的 `error` 是**对象**；查询已创建任务时的 `error` 才是**字符串**。客户端先检查 HTTP 状态，再按对应响应结构读取。

例如图片请求误传 `duration`，返回 HTTP `400`：

```json
{
  "error": {
    "type": "INVALID_MEDIA_CAPABILITY",
    "message": "invalid media capability configuration or request",
    "fields": [
      {
        "path": "duration",
        "code": "unknown_field",
        "message": "field is not declared for this media capability"
      }
    ]
  }
}
```

`fields` 仅在有字段详情时出现。`path` 指出字段，例如 `metadata.resolution`，`code` 指出校验类型，`message` 说明问题；不能假定每个 HTTP 错误都有 `fields`。

| HTTP 状态 | 常见原因 | 客户端处理 |
| --- | --- | --- |
| `400` | 参数或模型能力不匹配、规格不支持、余额不足等 | 读取 `error.type`、`message` 和可选 `fields`，修正后再创建 |
| `401` | API Key 无效或鉴权失败 | 检查 Bearer Key 与密钥状态 |
| `403` | 密钥或分组没有生成权限 | 检查可用渠道与密钥权限 |
| `404` | 任务不存在、Key 或查询路径不匹配、分组不提供该接口 | 核对分组、原始 Key、任务 ID 和图片/视频查询路径 |
| `410` | 任务结果已过期 | 使用已保存的文件；需要重新生成时再主动创建 |
| `429` | Key 额度或请求频率不足以受理任务 | 检查额度和限流；查询临时受限时延后重试 |
| `5xx` | 服务暂时不可用 | 查询可延后重试；创建请求按下方重复提交规则处理 |

错误消息及摘要可能扩展，业务逻辑应主要依赖 HTTP 状态、任务 `status`、`result.data` 和 `error` 是否为空，不要将全部错误文本写死为固定枚举。

## 费用、有效期与重复提交

- 创建成功时预留任务所需额度；生成和文件保存完成后，按已创建任务的价格正式结算。图片按成功结果、视频按对应模型的时长或按次规则计费，具体价格见可用渠道。
- 批量图片可自动拆分处理，最终等待整项任务结束后统一交付；部分成功只收成功部分，失败部分释放预留。
- 文件保存暂时失败会继续显示 `in_progress` 并重试。无法完成交付时任务结束，释放未交付部分的预留费用。
- 在 `expires_at` 前下载并保存文件。时间戳统一为 Unix **秒**，在 JavaScript 中转换日期需用 `new Date(expires_at * 1000)`；不要把秒直接当毫秒。查询过期结果返回 `410`。
- 创建接口不支持 `Idempotency-Key` 幂等去重。**重复 POST 会创建独立任务并可能重复收费。**拿到 `id` 后只重试 GET；创建请求超时且未拿到 `id` 时，不要自动重新 POST，应先核查任务是否已受理。

## JavaScript 创建与轮询示例

下面脚本适用于 Node.js 18 及以上。使用前设置 `SUB2API_BASE_URL` 和 `SUB2API_KEY`，将代码保存为 `media-example.mjs` 后运行 `node media-example.mjs`。轮询函数同样适用于图片与视频；生产应用应将创建返回的 `id` 持久保存，以便页面关闭或进程重启后继续 GET 查询。

```js
const baseURL = process.env.SUB2API_BASE_URL?.replace(/\/+$/, '');
const apiKey = process.env.SUB2API_KEY;
if (!baseURL || !apiKey) {
  throw new Error('请设置 SUB2API_BASE_URL 和 SUB2API_KEY');
}

const routes = {
  image: '/v1/images/generations',
  video: '/v1/videos/generations',
};
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

async function createTask(kind, body) {
  if (!routes[kind]) throw new Error('kind 必须为 image 或 video');
  // 创建请求仅发一次；网络异常不自动重发 POST。
  const response = await fetch(`${baseURL}${routes[kind]}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(60000),
  });
  const task = await response.json();
  if (response.status !== 202) {
    throw new Error(`创建失败 HTTP ${response.status}: ${JSON.stringify(task)}`);
  }
  if (typeof task.id !== 'string') throw new Error('创建响应缺少任务 id');
  console.log('请保存任务 ID：', task.id);
  return task;
}

async function waitForTask(kind, id, maxWaitMs = 30 * 60 * 1000) {
  if (!routes[kind]) throw new Error('kind 必须为 image 或 video');
  const deadline = Date.now() + maxWaitMs;
  while (Date.now() < deadline) {
    let response;
    try {
      response = await fetch(
        `${baseURL}${routes[kind]}/${encodeURIComponent(id)}`,
        {
          headers: {Authorization: `Bearer ${apiKey}`},
          signal: AbortSignal.timeout(30000),
        },
      );
    } catch (error) {
      // GET 超时或短暂网络异常，仅重试当前任务的查询。
      console.warn('查询暂时失败，将继续查询同一任务：', error.message);
      await pause(5000);
      continue;
    }
    if (response.status === 429 || response.status >= 500) {
      await pause(10000);
      continue;
    }
    const task = await response.json();
    if (!response.ok) {
      throw new Error(`查询失败 HTTP ${response.status}: ${JSON.stringify(task)}`);
    }
    if (task.status === 'completed') return task;
    if (!['queued', 'in_progress'].includes(task.status)) {
      throw new Error(`无法识别的任务状态：${task.status}`);
    }
    await pause(5000);
  }
  throw new Error(`本次等待超时，保留任务 ${id} 稍后继续查询，不要重新创建`);
}

const created = await createTask('image', {
  model: 'gpt-image-2',
  prompt: '一只坐在窗边的橘猫，柔和自然光',
  size: '1:1',
  n: 1,
  metadata: {resolution: '1K'},
});
const task = await waitForTask('image', created.id);
const urls = task.result.data;
if (urls.length === 0) {
  console.error('任务完成但无成功结果：', task.error);
} else {
  console.log('结果 URL：', urls);
  if (task.error) console.warn('存在未成功交付的部分：', task.error);
  console.log('请在此时间前下载：', new Date(task.expires_at * 1000).toISOString());
}

// 视频使用相同函数，传入对应模型文档中允许的参数：
// const createdVideo = await createTask('video', {
//   model: 'doubao-seedance-1-0-pro-fast',
//   prompt: '海边日出，海浪轻柔拍岸，镜头缓慢前移',
//   duration: 5,
//   aspect_ratio: '16:9',
//   metadata: {resolution: '480p'},
// });
// const videoTask = await waitForTask('video', createdVideo.id);
```

这段代码的一次轮询等待上限为 30 分钟，只是客户端等待设置，不代表服务端任务已失败。等待超时后保留任务 ID，稍后继续查询原任务。

## 继续接入

在[媒体模型目录](/media-models/)选择模型，按该页的字段表、输入条件和示例构造请求。不同模型即使使用同一创建接口，也可能有不同的参考图数量、比例、分辨率和特有参数；切换模型时应重新核对这些限制。
