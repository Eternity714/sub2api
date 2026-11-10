---
description: gpt-image-2.6-flare 图片生成 API 参数、输入模式、调用示例与结果查询。
---

# gpt-image-2.6-flare

图片生成模型，使用[媒体 API 异步协议](/media-api)。参数核对日期：2026-11-10。

**模型 ID：`gpt-image-2.6-flare`**。请原样填写，大小写不可替换。

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
| `model` | `string` | 必填 | 无 | 模型 ID，区分大小写，必须与本页完全一致。 | 仅限 `"gpt-image-2.6-flare"` |
| `prompt` | `string` | 必填 | 未声明 | 描述要生成的内容或对参考图的修改要求。 | 最少字符 1；最多字符 32000；不接受 null |
| `images` | `array` | 可选 | 未声明 | 参考图 HTTP(S) URL 列表，按输入顺序处理。 | 最少元素 1；最多元素 16；不接受 null |
| `images[]` | `string` | 每个数组元素 | 未声明 | 单个参考图片 URL。 | 无用户名或密码的绝对 HTTP(S) URL；不接受 null |
| `aspectRatio` | `string` | 可选，可省略并采用默认 | `"1:1"` | 输出宽高比；允许值和 auto 支持情况见本字段约束。 | 仅限 `"auto"`、`"1:1"`、`"16:9"`、`"9:16"`、`"4:3"`、`"3:4"`、`"3:2"`、`"2:3"`、`"5:4"`、`"4:5"`、`"21:9"`、`"9:21"`、`"1:2"`、`"2:1"`、`"1:3"`、`"3:1"`；不接受 null |
| `imageSize` | `string` | 可选，可省略并采用默认 | `"1K"` | 图片分辨率档位，也接受 resolution 别名。 | 仅限 `"1K"`、`"2K"`、`"4K"`；不接受 null |
| `quality` | `string` | 可选，可省略并采用默认 | `"auto"` | 生成质量档位，仅列出的值可用。 | 仅限 `"auto"`、`"low"`、`"medium"`、`"high"`；不接受 null |
| `n` | `integer` | 可选，可省略并采用默认 | `1` | 请求生成的图片结果数量。 | 最小值 1；最大值 1；不接受 null |
| `replyType` | `string` | 可选，可省略并采用默认 | `"async"` | 目前仅支持 async：创建任务后轮询查询。 | 仅限 `"async"`；不接受 null |

</div>

参数较多时可在表格内左右滚动查看。表中必填性按默认值填充后的请求判断。模式附加的必填和禁止字段见下节；嵌套字段的必填性仅在父对象存在时判断。默认值不代表任意扩展参数都会自动补齐。

参考图必须替换为生成服务能够下载的 HTTP(S) 图片直链；示例 `example.com` 仅占位，不能直接用来生成。不支持在 `images` 中放入 Base64 字符串或本地路径。带签名的链接应在任务处理期间保持有效。

## 参数使用说明

### model 与 prompt

本页只接受 `model: "gpt-image-2.6-flare"`。`prompt` 为必填字符串，长度 1–32000 个字符，描述主体、场景、构图、风格或参考图修改要求。

### images：参考图

文生图时省略 `images`。参考图生图时传入 1–16 个 HTTP(S) 图片直链，按数组顺序描述各图用途。没有参考图时不要传空数组 `[]` 或 `null`；不接受 Base64、data URL、本地路径和带用户名密码的 URL。链接应无需网页登录，并在任务处理期间有效。

### aspectRatio：画面比例

省略时为 `1:1`；`auto` 也按正方形构造请求，不会根据参考图自动继承比例。完整允许值见上方参数表。仅接受 `16:9` 这样的比例字符串，不接受 `1024x1024` 或 `2048x2048` 等像素尺寸。实际原图可能与下表目标尺寸存在差异。

### imageSize：分辨率档位

允许 `1K`、`2K`、`4K`，省略时为 `1K`，严格区分大小写。分辨率决定价格，`quality` 不改变本次计费档位。4K 的正方形目标为 **2880×2880**，不是 4096×4096；其他比例的目标尺寸见下表。

### quality：生成质量

允许 `auto`、`low`、`medium`、`high`，省略时为 `auto`（自动质量）；后三项分别请求低、中、高质量。同一 `imageSize` 下四种质量同价，改变质量不等于改变分辨率，也不承诺某个固定耗时或文件大小。`xhigh` 和 `max` 未开放。

### n 与 replyType

`n` 默认为 `1`，且只能为 `1`。`replyType` 默认为 `async`，且只能为 `async`：先接收任务 ID，再轮询结果；不支持 `json` 同步返回或 `stream` 流式返回。

### 未开放的参数

`background`、`outputFormat`、`response_format`、`mask`、`webhook` 等未列出的字段不能传入。当前没有通过参数指定透明背景、GIF/SVG 输出或蒙版编辑的能力。字段名、字符串枚举、大小写或别名冲突不符时，创建前返回 400，不创建任务、不预留额度。

六个型号的对照、Python / JavaScript 示例及 OpenAPI 下载见[GPT Image 系列调用指南](/gpt-image)。

## 输入模式与组合限制

本模型无额外的模式匹配条件，按上表字段约束校验。参考图是否必填、可传数量以 `images` 行为准。

使用 `imageSize` 选择 **1K、2K、4K**，省略时默认 **1K**；`aspectRatio:auto` 按 `1:1` 构造请求，三个档位的正方形目标尺寸分别为 1024×1024、2048×2048、2880×2880。

下表为请求的目标尺寸映射。实际交付保留生成服务原图，像素尺寸可能存在差异，不因该差异自动缩放、裁剪或重试。按请求的 `imageSize` 档位计费，不根据结果像素重新分档。

仅接受比例写法，不接受 `aspectRatio:1024x1024` 等像素尺寸。四种质量在同一分辨率下同价。

### 有效比例与分辨率组合

本模型列出的比例支持全部三个分辨率档位。下表是请求的目标像素映射；auto使用对应档位的1:1映射，实际交付像素允许存在差异。

| aspectRatio | imageSize | 目标像素尺寸 |
| --- | --- | --- |
| `1:1` | `1K` | 1024 × 1024 |
| `16:9` | `1K` | 1280 × 720 |
| `9:16` | `1K` | 720 × 1280 |
| `4:3` | `1K` | 1152 × 864 |
| `3:4` | `1K` | 864 × 1152 |
| `3:2` | `1K` | 1248 × 832 |
| `2:3` | `1K` | 832 × 1248 |
| `5:4` | `1K` | 1120 × 896 |
| `4:5` | `1K` | 896 × 1120 |
| `21:9` | `1K` | 1456 × 624 |
| `9:21` | `1K` | 624 × 1456 |
| `1:2` | `1K` | 720 × 1440 |
| `2:1` | `1K` | 1440 × 720 |
| `1:3` | `1K` | 576 × 1728 |
| `3:1` | `1K` | 1728 × 576 |
| `1:1` | `2K` | 2048 × 2048 |
| `16:9` | `2K` | 2560 × 1440 |
| `9:16` | `2K` | 1440 × 2560 |
| `4:3` | `2K` | 2304 × 1728 |
| `3:4` | `2K` | 1728 × 2304 |
| `3:2` | `2K` | 2496 × 1664 |
| `2:3` | `2K` | 1664 × 2496 |
| `5:4` | `2K` | 2240 × 1792 |
| `4:5` | `2K` | 1792 × 2240 |
| `21:9` | `2K` | 3024 × 1296 |
| `9:21` | `2K` | 1296 × 3024 |
| `1:2` | `2K` | 1440 × 2880 |
| `2:1` | `2K` | 2880 × 1440 |
| `1:3` | `2K` | 1168 × 3504 |
| `3:1` | `2K` | 3504 × 1168 |
| `1:1` | `4K` | 2880 × 2880 |
| `16:9` | `4K` | 3840 × 2160 |
| `9:16` | `4K` | 2160 × 3840 |
| `4:3` | `4K` | 3264 × 2448 |
| `3:4` | `4K` | 2448 × 3264 |
| `3:2` | `4K` | 3504 × 2336 |
| `2:3` | `4K` | 2336 × 3504 |
| `5:4` | `4K` | 3200 × 2560 |
| `4:5` | `4K` | 2560 × 3200 |
| `21:9` | `4K` | 3808 × 1632 |
| `9:21` | `4K` | 1632 × 3808 |
| `1:2` | `4K` | 1920 × 3840 |
| `2:1` | `4K` | 3840 × 1920 |
| `1:3` | `4K` | 1280 × 3840 |
| `3:1` | `4K` | 3840 × 1280 |

## 创建任务示例

以下 cURL 使用 Bash。创建请求只发送一次，拿到 id 后改用 GET 查询：

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="sk-YOUR_API_KEY"
```

### 文生图（1K）

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "gpt-image-2.6-flare",
  "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
  "aspectRatio": "1:1",
  "imageSize": "1K",
  "quality": "auto",
  "replyType": "async",
  "n": 1
}'
```

### 文生图（2K）

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "gpt-image-2.6-flare",
  "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
  "aspectRatio": "16:9",
  "imageSize": "2K",
  "quality": "medium",
  "replyType": "async",
  "n": 1
}'
```

### 文生图（4K）

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "gpt-image-2.6-flare",
  "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
  "aspectRatio": "21:9",
  "imageSize": "4K",
  "quality": "high",
  "replyType": "async",
  "n": 1
}'
```

### 参考图生图（4K）

```bash
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
  "model": "gpt-image-2.6-flare",
  "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
  "aspectRatio": "21:9",
  "imageSize": "4K",
  "quality": "high",
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
  "model": "gpt-image-2.6-flare",
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
  "model": "gpt-image-2.6-flare",
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

### 处理中与失败

处理中返回 `status:queued/in_progress`、`progress:0/50`，`result.data` 为空；此时继续 GET 查询。终态为 `completed/100`，必须同时检查 `result.data` 和 `error`。

生成失败仍可能是 HTTP 200，例如：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.6-flare",
  "status": "completed",
  "progress": 100,
  "created_at": 1794182400,
  "completed_at": 1794268800,
  "result": {
    "type": "image",
    "data": []
  },
  "error": "media_task_expired"
}
```

上例没有图片，不应按成功处理；`expires_at` 仅在有成功结果时返回。请保存任务 ID，使用原 Key 继续查询；查询不会再次生成。不要将 `completed`、HTTP 200 或仅有 `id` 当作已生成图片。

创建前的参数错误使用 HTTP 400，`error` 为包含 `type`、`message` 和可选 `fields` 的对象；任务查询中的 `error` 为字符串。完整错误码和客户端重试规则见[错误处理](/media-api#错误处理)。

## 计费与交付

按成功交付的图片结果数计费。创建任务时预留额度，全部文件保存及任务处理完成后结算；失败部分释放预留额度。

当前价目档位：`1K`、`2K`、`4K`。价目档位不扩展可用参数范围；请求仍须满足本页字段及模式限制。

| imageSize | 单价（元/张） |
| --- | ---: |
| `1K` | 0.12 |
| `2K` | 0.2 |
| `4K` | 0.35 |

当前每次请求仅生成一张图片，`n` 只能为 1。

旧字段兼容关系见[参数兼容说明](/media-api#旧参数兼容)。新旧别名同时传入时必须一致；`replyType` 当前仅支持 `async`。

[返回媒体模型目录](/media-models/) · [统一协议与错误处理](/media-api)
