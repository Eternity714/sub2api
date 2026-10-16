---
description: MiniMax-H3-Max-Turbo 视频生成 API 参数、输入模式、调用示例与结果查询。
---

# MiniMax-H3-Max-Turbo

视频生成模型，使用[媒体 API 异步协议](/media-api)。参数核对日期：2026-10-16。

**模型 ID：`MiniMax-H3-Max-Turbo`**。请原样填写，大小写不可替换。

## 接口与认证

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | `/v1/videos/generations` | 创建任务，成功返回 HTTP 202 |
| GET | `/v1/videos/generations/{task_id}` | 使用创建响应的 id 查询状态和结果 |

根地址为 `https://www.gkotta.bid`；请求头使用 `Authorization: Bearer sk-YOUR_API_KEY`。API Key 必须关联已开放本模型的媒体 API 分组。可用性与实际价格以[可用渠道](https://www.gkotta.bid/available-channels)为准。

## 请求参数

以下为本模型当前开放的完整字段。顶层或嵌套对象中未列出的字段均不能传入；类型、枚举、范围或组合不符时返回 `400`，不创建任务、不预留额度。

<div class="media-parameters">

| 字段 | 类型 | 必填性 | 默认值 | 用途 | 约束 |
| --- | --- | --- | --- | --- | --- |
| `model` | `string` | 必填 | 无 | 模型 ID，区分大小写，必须与本页完全一致。 | 仅限 `"MiniMax-H3-Max-Turbo"` |
| `prompt` | `string` | 必填 | 未声明 | 描述要生成的内容或对参考图的修改要求。 | 最少字符 1；最多字符 7000；不接受 null |
| `duration` | `integer` | 可选，可省略并采用默认 | `5` | 视频时长，单位为秒。 | 最小值 5；最大值 15；不接受 null |
| `aspect_ratio` | `string` | 可选 | 未声明 | 视频宽高比，部分参考图模式禁止显式传入。 | 仅限 `"21:9"`、`"16:9"`、`"4:3"`、`"1:1"`、`"3:4"`、`"9:16"`；不接受 null |
| `image_urls` | `array` | 可选 | 未声明 | 参考图 URL 列表。传入前请确认素材可公开读取。 | 最少元素 1；最多元素 2；不接受 null |
| `image_urls[]` | `string` | 每个数组元素 | 未声明 | 单个参考图片 URL。 | 无用户名或密码的绝对 HTTP(S) URL；不接受 null |
| `metadata` | `object` | 可选，可省略并采用默认 | `{}` | 模型扩展参数对象，仅接受下列已声明的字段。 | 不接受 null |
| `metadata.resolution` | `string` | 可选，可省略并采用默认 | `"480P"` | 输出分辨率档位，严格使用所列字符串及大小写。 | 仅限 `"480P"`、`"768P"`；不接受 null |
| `metadata.watermark` | `boolean` | 可选 | 未声明 | 是否启用模型水印。 | 不接受 null |

</div>

参数较多时可在表格内左右滚动查看。表中必填性按默认值填充后的请求判断。模式附加的必填和禁止字段见下节；嵌套字段的必填性仅在父对象存在时判断。默认值不代表任意扩展参数都会自动补齐。

省略 `metadata` 时，本模型先采用默认 `{}`，再补入其已声明的子字段默认值。

参考图必须替换为调用方实际可公开访问的 HTTP(S) 文件 URL；示例 `example.com` 仅占位，不能直接用来生成。不支持在 `image_urls` 中放入 Base64 字符串或本地文件路径。

## 输入模式与组合限制

补入默认值后，请求必须且只能匹配下表的一种模式。未传与 `null` 不等价，仅当字段明确允许时才能传 `null`。

| 模式 | 同时满足的条件 | 附加必填 | 禁止传入 |
| --- | --- | --- | --- |
| `text` | `image_urls` 未传或为空 | `prompt` | 无 |
| `reference` | `image_urls` 已传且非空 | `prompt` | `aspect_ratio` |

视频请求不接受顶层 `size` 或 `n`。分辨率字符串严格区分大小写，例如 `720p` 与 `720P` 不可自行替换。

## 创建任务示例

以下 cURL 使用 Bash。设置当前密钥，示例不会自动提交或重试：

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="sk-YOUR_API_KEY"
```

### 基础请求

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "MiniMax-H3-Max-Turbo",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480P"
  },
  "duration": 5
}'
```

### 文生视频

对应输入模式：`text`。

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "MiniMax-H3-Max-Turbo",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480P"
  },
  "duration": 5
}'
```

### 参考图生视频

对应输入模式：`reference`。

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "MiniMax-H3-Max-Turbo",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480P"
  },
  "duration": 5,
  "image_urls": [
    "https://example.com/reference.png"
  ]
}'
```

成功响应（HTTP 202）：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "MiniMax-H3-Max-Turbo",
  "status": "queued",
  "progress": 0,
  "created_at": 1792108800
}
```

## 查询状态与获取结果

保留创建响应的 `id`，使用创建时的同一个 API Key 查询，建议间隔 5–10 秒：

```bash
export TASK_ID="media_EXAMPLE_TASK_ID"
curl --fail-with-body "$SUB2API_BASE_URL/v1/videos/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

`queued/0` 为排队，`in_progress/50` 为处理中，`completed/100` 为已结束。只有 `completed` 才读取最终结果；完成并不表示成功。

成功结果示例（HTTP 200）：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "MiniMax-H3-Max-Turbo",
  "status": "completed",
  "progress": 100,
  "created_at": 1792108800,
  "completed_at": 1792109100,
  "expires_at": 1792195500,
  "result": {
    "type": "video",
    "data": [
      "https://files.example.com/media/result.mp4"
    ]
  },
  "error": ""
}
```

`result.data` 为文件 URL 字符串数组；无成功结果时为空，有错误时由 `error` 字符串说明。所有终态返回 `completed_at`，仅有成功结果时返回 `expires_at`，时间均为 Unix 秒。请在过期前下载自己的实际结果 URL。

完整响应规则、部分成功、错误码与轮询程序见[媒体 API 接入指南](/media-api#查询任务与下载结果)。模型或媒体类型不匹配、字段非法、未开放能力会在创建前拒绝；创建成功后的执行失败通过查询响应的 `completed + error` 返回。

## 计费与交付

按成功交付的视频时长（秒）计费。创建任务时预留额度，全部文件保存及任务处理完成后结算；失败部分释放预留额度。

当前价目档位：`480P`、`768P`。价目档位不扩展可用参数范围；请求仍须满足本页字段及模式限制。

实际单价及分组倍率以[可用渠道](https://www.gkotta.bid/available-channels)显示为准。

[返回媒体模型目录](/media-models/) · [统一协议与错误处理](/media-api)
