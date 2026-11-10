---
description: gpt-image-2.5 图片生成 API 参数、输入模式、调用示例与结果查询。
---

# gpt-image-2.5

图片生成模型，使用[媒体 API 异步协议](/media-api)。参数核对日期：2026-11-10。

**模型 ID：`gpt-image-2.5`**。请原样填写，大小写不可替换。

## 接口与认证

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/v1/images/generations` | 创建任务，成功返回 HTTP 202 |
| GET | `/v1/images/generations/{task_id}` | 使用创建响应的 id 查询状态和结果 |

根地址为 `https://www.gkotta.bid`；请求头使用 `Authorization: Bearer sk-YOUR_API_KEY`。API Key 必须关联已开放本模型的媒体 API 分组。可用性与实际价格以[可用渠道](https://www.gkotta.bid/available-channels)为准。

## 请求参数

以下为本模型推荐使用的完整字段；[旧参数别名](/media-api#旧参数兼容)也继续兼容。除此之外，顶层或嵌套对象中未列出的字段均不能传入；类型、枚举、范围或组合不符时返回 `400`，不创建任务、不预留额度。

<div class="media-parameters">

| 字段 | 类型 | 必填性 | 默认值 | 用途 | 约束 |
| --- | --- | --- | --- | --- | --- |
| `model` | `string` | 必填 | 无 | 模型 ID，区分大小写，必须与本页完全一致。 | 仅限 `"gpt-image-2.5"` |
| `prompt` | `string` | 必填 | 未声明 | 描述要生成的内容或对参考图的修改要求。 | 最少字符 1；最多字符 32000；不接受 null |
| `images` | `array` | 可选 | 未声明 | 参考图 HTTP(S) URL 列表，按输入顺序处理。 | 最少元素 1；最多元素 16；不接受 null |
| `images[]` | `string` | 每个数组元素 | 未声明 | 单个参考图片 URL。 | 无用户名或密码的绝对 HTTP(S) URL；不接受 null |
| `aspectRatio` | `string` | 可选，可省略并采用默认 | `"1:1"` | 输出宽高比；允许值和 auto 支持情况见本字段约束。 | 仅限 `"auto"`、`"1:1"`、`"16:9"`、`"9:16"`、`"4:3"`、`"3:4"`、`"3:2"`、`"2:3"`、`"5:4"`、`"4:5"`、`"21:9"`、`"9:21"`、`"1:2"`、`"2:1"`；不接受 null |
| `quality` | `string` | 可选，可省略并采用默认 | `"auto"` | 生成质量档位，仅列出的值可用。 | 仅限 `"auto"`；不接受 null |
| `n` | `integer` | 可选，可省略并采用默认 | `1` | 请求生成的图片结果数量。 | 最小值 1；最大值 1；不接受 null |
| `replyType` | `string` | 可选，可省略并采用默认 | `"async"` | 目前仅支持 async：创建任务后轮询查询。 | 仅限 `"async"`；不接受 null |

</div>

参数较多时可在表格内左右滚动查看。表中必填性按默认值填充后的请求判断。模式附加的必填和禁止字段见下节；嵌套字段的必填性仅在父对象存在时判断。默认值不代表任意扩展参数都会自动补齐。

参考图必须替换为生成服务能够下载的 HTTP(S) 图片直链；示例 `example.com` 仅占位，不能直接用来生成。不支持在 `images` 中放入 Base64 字符串或本地路径。带签名的链接应在任务处理期间保持有效。

## 输入模式与组合限制

本模型无额外的模式匹配条件，按上表字段约束校验。参考图是否必填、可传数量以 `images` 行为准。

本模型固定为 **1K**，使用 `aspectRatio` 选择下表目标尺寸。`auto` 固定按 `1:1` 处理，正方形目标尺寸为 1024×1024；`quality:auto` 表示自动质量。

不接受 `imageSize`、`resolution`、`metadata.resolution` 或像素尺寸写法。质量仅支持 `quality:auto`。

实际交付保留生成服务原图，像素尺寸可能存在差异，不因该差异自动缩放、裁剪、重试或改变计费。

### 比例与输出尺寸

下表为固定档位的像素映射，分辨率无需通过请求参数指定。

| aspectRatio | 像素尺寸 |
| --- | --- |
| `1:1` | 1024 × 1024 |
| `16:9` | 1280 × 720 |
| `9:16` | 720 × 1280 |
| `4:3` | 1152 × 864 |
| `3:4` | 864 × 1152 |
| `3:2` | 1248 × 832 |
| `2:3` | 832 × 1248 |
| `5:4` | 1120 × 896 |
| `4:5` | 896 × 1120 |
| `21:9` | 1456 × 624 |
| `9:21` | 624 × 1456 |
| `1:2` | 720 × 1440 |
| `2:1` | 1440 × 720 |

## 创建任务示例

以下 cURL 使用 Bash。创建请求只发送一次，拿到 id 后改用 GET 查询：

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="sk-YOUR_API_KEY"
```

### 文生图（固定 1K）

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "gpt-image-2.5",
  "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
  "aspectRatio": "1:1",
  "quality": "auto",
  "replyType": "async",
  "n": 1
}'
```

### 参考图生图（固定 1K）

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "gpt-image-2.5",
  "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
  "aspectRatio": "1:1",
  "quality": "auto",
  "replyType": "async",
  "n": 1,
  "images": [
    "https://example.com/REPLACE_WITH_YOUR_REFERENCE.png"
  ]
}'
```

成功响应（HTTP 202）：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.5",
  "status": "queued",
  "progress": 0,
  "created_at": 1792108800
}
```

## 查询状态与获取结果

保留创建响应的 `id`，使用创建时的同一个 API Key 查询，建议间隔 5–10 秒：

```bash
export TASK_ID="media_EXAMPLE_TASK_ID"
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

`queued/0` 为排队，`in_progress/50` 为处理中，`completed/100` 为已结束。只有 `completed` 才读取最终结果；完成并不表示成功。

成功结果示例（HTTP 200）：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.5",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792109100,
  "expires_at": 1792195500,
  "result": {
    "type": "image",
    "data": [
      "https://files.example.com/media/result.png"
    ]
  },
  "error": ""
}
```

`result.data` 为文件 URL 字符串数组；无成功结果时为空，有错误时由 `error` 字符串说明。所有终态返回 `completed_at`，仅有成功结果时返回 `expires_at`，时间均为 Unix 秒。请在过期前下载自己的实际结果 URL。

完整响应规则、部分成功、错误码与轮询程序见[媒体 API 接入指南](/media-api#查询任务与下载结果)。模型或媒体类型不匹配、字段非法、未开放能力会在创建前拒绝；创建成功后的执行失败通过查询响应的 `completed + error` 返回。

## 计费与交付

按成功交付的图片结果数计费。创建任务时预留额度，全部文件保存及任务处理完成后结算；失败部分释放预留额度。

核对时标价：**0.08 元/张**。实际价格及分组倍率以[可用渠道](https://www.gkotta.bid/available-channels)为准。

当前每次请求仅生成一张图片，`n` 只能为 1。

旧字段兼容关系见[参数兼容说明](/media-api#旧参数兼容)。新旧别名同时传入时必须一致；`replyType` 当前仅支持 `async`。

[返回媒体模型目录](/media-models/) · [统一协议与错误处理](/media-api)
