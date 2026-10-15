# 统一媒体模型参数模板

本文对应仓库内置的逐模型能力模板，核实日期为 2026-10-15。管理员复制模板后配置公开模型别名、定价和 Composite 媒体候选，并显式启用。模板不会自动写入数据库或证明账号具备可用生成渠道。下游实际允许参数以管理员发布的公开模型文档为准。

**图片入口**：`POST /v1/images/generations`；**视频入口**：`POST /v1/videos/generations`。查询使用同路径追加 `/{task_id}`。统一协议仅供启用媒体路由的 Composite 分组。

图片顶层仅允许 `model,prompt,size,n,image_urls,metadata`；视频顶层仅允许 `model,prompt,duration,aspect_ratio,image_urls,metadata`。`size` 和 `aspect_ratio` 是正整数宽高比；图片像素档位使用 `metadata.resolution`，不接受 `metadata.output_size`。所有未声明的字段均返回 400，不创建任务或预留额度。

图片 `n` 的本地整项上限为 8；模板采用单结果上游请求，可独立拆分多张。它不是上游单次张数承诺。有关联的连续组图、编辑、变体、父任务引用、角色/素材库管理及混合辅助结果不在这些基础模板内。Midjourney 图片四宫格作为 1 个图片结果交付。

状态统一为 `queued=0`、`in_progress=50`、`completed=100`，`completed` 只表示整项处理完成。结果统一为 S3 URL 数组 `result.data:string[]`，失败说明在顶层 `error:string`。未终结的子项与转存结果不会提前返回。

## 模板覆盖

目录登记 115 条，含 103 条基础生成模板和 12 条未发布 schema 的待核实条目。统计包含同名模型的不同媒体能力，不能当作不同模型数量或账号实测数量。

| 平台 | 图片模板 | 视频模板 | 待核实/独立操作 |
| --- | ---: | ---: | ---: |
| grsai | 16 | 2 | 0 |
| toapis | 39 | 46 | 12 |

## 阅读参数表

必填列表示全局必填；选定模式的额外必填和禁用字段见对应模式表。缺省值仅表示模板明确注入的值；“—”表示模板不注入，管理员应确认相应上游缺省行为与本地定价。数组中 URL 必须是 HTTP(S)，禁止本地路径和 Base64。这里只做协议与字段校验，不读取输入文件来保证其格式、大小或尺寸。

## toapis / doubao-seedream-4-0 / image

官方来源：[doubao-seedream-4-0](https://docs.toapis.com/docs/cn/api-reference/images/seedream-4.0/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedream-4-0`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.optimize_prompt_options` | object | 否 | — | — |
| `metadata.optimize_prompt_options.mode` | string | 否 | — | 枚举 `["standard","fast"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","21:9","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/doubao-seedream-4-0",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / doubao-seedream-4-5 / image

官方来源：[doubao-seedream-4-5](https://docs.toapis.com/docs/cn/api-reference/images/seedream-4.5/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedream-4-5`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.optimize_prompt_options` | object | 否 | — | — |
| `metadata.optimize_prompt_options.mode` | string | 否 | — | 枚举 `["standard","fast"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","21:9","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/doubao-seedream-4-5",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "2K"
  }
}
```


## toapis / doubao-seedream-5-0 / image

官方来源：[doubao-seedream-5-0](https://docs.toapis.com/docs/cn/api-reference/images/seedream-5.0/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedream-5-0`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["2K","3K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","21:9","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/doubao-seedream-5-0",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "2K"
  }
}
```


## toapis / doubao-seedream-5-0-pro / image

官方来源：[doubao-seedream-5-0-pro](https://docs.toapis.com/docs/cn/api-reference/images/seedream-5.0-pro/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedream-5-0-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","21:9","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/doubao-seedream-5-0-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / flux-2-flex / image

官方来源：[flux-2-flex](https://docs.toapis.com/docs/cn/api-reference/images/flux-2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/flux-2-flex`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 8 |

请求示例：

```json
{
  "model": "toapis/flux-2-flex",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / flux-2-pro / image

官方来源：[flux-2-pro](https://docs.toapis.com/docs/cn/api-reference/images/flux-2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/flux-2-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 8 |

请求示例：

```json
{
  "model": "toapis/flux-2-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / flux-kontext-max / image

官方来源：[flux-kontext-max](https://docs.toapis.com/docs/cn/api-reference/images/flux-kontext/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/flux-kontext-max`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.response_format` | string | 否 | — | 枚举 `["png","jpg"]` |
| `metadata.safety_tolerance` | integer | 否 | — | 最小 0；最大 6 |
| `metadata.prompt_upsampling` | boolean | 否 | — | — |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","21:9","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/flux-kontext-max",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / flux-kontext-pro / image

官方来源：[flux-kontext-pro](https://docs.toapis.com/docs/cn/api-reference/images/flux-kontext/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/flux-kontext-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.response_format` | string | 否 | — | 枚举 `["png","jpg"]` |
| `metadata.safety_tolerance` | integer | 否 | — | 最小 0；最大 6 |
| `metadata.prompt_upsampling` | boolean | 否 | — | — |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","21:9","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/flux-kontext-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / gemini-2.5-flash-image-official / image

官方来源：[gemini-2.5-flash-image-official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-2.5-flash-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-2.5-flash-image-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.temperature` | number | 否 | — | 最小 0；最大 2 |
| `metadata.topP` | number | 否 | — | 最小 0；最大 1 |
| `metadata.maxOutputTokens` | integer | 否 | — | 最小 1 |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["ALLOW_ALL","ALLOW_ADULT","ALLOW_NONE"]` |
| `metadata.imageOutputOptions` | object | 否 | — | — |
| `metadata.imageOutputOptions.mimeType` | string | 否 | — | 枚举 `["image/png","image/jpeg","image/webp"]` |
| `metadata.imageOutputOptions.compressionQuality` | integer | 否 | — | 最小 0；最大 100 |
| `metadata.safetySettings` | array<object> | 否 | — | 最多项 4 |
| `metadata.safetySettings[].category` | string | 否 | — | 枚举 `["HARM_CATEGORY_HATE_SPEECH","HARM_CATEGORY_DANGEROUS_CONTENT","HARM_CATEGORY_SEXUALLY_EXPLICIT","HARM_CATEGORY_HARASSMENT"]` |
| `metadata.safetySettings[].threshold` | string | 否 | — | 枚举 `["OFF","BLOCK_LOW_AND_ABOVE","BLOCK_MEDIUM_AND_ABOVE","BLOCK_ONLY_HIGH"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 3 |

请求示例：

```json
{
  "model": "toapis/gemini-2.5-flash-image-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / gemini-2.5-flash-image-preview / image

官方来源：[gemini-2.5-flash-image-preview](https://docs.toapis.com/docs/cn/api-reference/images/gemini-2.5-flash/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-2.5-flash-image-preview`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.orientation` | string | 否 | — | 枚举 `["landscape","portrait"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

请求示例：

```json
{
  "model": "toapis/gemini-2.5-flash-image-preview",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-2.5-flash-image-preview-vip / image

官方来源：[gemini-2.5-flash-image-preview-vip](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / gemini-3-pro-image-official / image

官方来源：[gemini-3-pro-image-official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-3-pro-image-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.grounding` | string | 否 | — | 枚举 `["none","web"]` |
| `metadata.temperature` | number | 否 | — | 最小 0；最大 2 |
| `metadata.topP` | number | 否 | — | 最小 0；最大 1 |
| `metadata.maxOutputTokens` | integer | 否 | — | 最小 1 |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["ALLOW_ALL","ALLOW_ADULT","ALLOW_NONE"]` |
| `metadata.imageOutputOptions` | object | 否 | — | — |
| `metadata.imageOutputOptions.mimeType` | string | 否 | — | 枚举 `["image/png","image/jpeg","image/webp"]` |
| `metadata.imageOutputOptions.compressionQuality` | integer | 否 | — | 最小 0；最大 100 |
| `metadata.thinkingConfig` | object | 否 | — | — |
| `metadata.thinkingConfig.thinkingBudget` | integer | 否 | — | 最小 0；最大 24576 |
| `metadata.thinkingConfig.thinkingLevel` | string | 否 | — | 枚举 `["LOW","MEDIUM","HIGH","MINIMAL"]` |
| `metadata.safetySettings` | array<object> | 否 | — | 最多项 4 |
| `metadata.safetySettings[].category` | string | 否 | — | 枚举 `["HARM_CATEGORY_HATE_SPEECH","HARM_CATEGORY_DANGEROUS_CONTENT","HARM_CATEGORY_SEXUALLY_EXPLICIT","HARM_CATEGORY_HARASSMENT"]` |
| `metadata.safetySettings[].threshold` | string | 否 | — | 枚举 `["OFF","BLOCK_LOW_AND_ABOVE","BLOCK_MEDIUM_AND_ABOVE","BLOCK_ONLY_HIGH"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-3-pro-image-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-3-pro-image-preview / image

官方来源：[gemini-3-pro-image-preview](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-3-pro-image-preview`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.orientation` | string | 否 | — | 枚举 `["landscape","portrait"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 6 |

请求示例：

```json
{
  "model": "toapis/gemini-3-pro-image-preview",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-3-pro-image-preview-vip / image

官方来源：[gemini-3-pro-image-preview-vip](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3-pro-image-vip/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-3-pro-image-preview-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.grounding` | string | 否 | — | 枚举 `["none","web"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-3-pro-image-preview-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-3.1-flash-image-official / image

官方来源：[gemini-3.1-flash-image-official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-3.1-flash-image-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.grounding` | string | 否 | — | 枚举 `["none","web","image","web_image"]` |
| `metadata.temperature` | number | 否 | — | 最小 0；最大 2 |
| `metadata.topP` | number | 否 | — | 最小 0；最大 1 |
| `metadata.maxOutputTokens` | integer | 否 | — | 最小 1 |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["ALLOW_ALL","ALLOW_ADULT","ALLOW_NONE"]` |
| `metadata.imageOutputOptions` | object | 否 | — | — |
| `metadata.imageOutputOptions.mimeType` | string | 否 | — | 枚举 `["image/png","image/jpeg","image/webp"]` |
| `metadata.imageOutputOptions.compressionQuality` | integer | 否 | — | 最小 0；最大 100 |
| `metadata.thinkingConfig` | object | 否 | — | — |
| `metadata.thinkingConfig.thinkingBudget` | integer | 否 | — | 最小 0；最大 24576 |
| `metadata.thinkingConfig.thinkingLevel` | string | 否 | — | 枚举 `["LOW","MEDIUM","HIGH","MINIMAL"]` |
| `metadata.safetySettings` | array<object> | 否 | — | 最多项 4 |
| `metadata.safetySettings[].category` | string | 否 | — | 枚举 `["HARM_CATEGORY_HATE_SPEECH","HARM_CATEGORY_DANGEROUS_CONTENT","HARM_CATEGORY_SEXUALLY_EXPLICIT","HARM_CATEGORY_HARASSMENT"]` |
| `metadata.safetySettings[].threshold` | string | 否 | — | 枚举 `["OFF","BLOCK_LOW_AND_ABOVE","BLOCK_MEDIUM_AND_ABOVE","BLOCK_ONLY_HIGH"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","4:1","8:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-3.1-flash-image-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-3.1-flash-image-preview / image

官方来源：[gemini-3.1-flash-image-preview](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-3.1-flash-image-preview`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | `{}` | — |
| `metadata.google_search` | boolean | 否 | `false` | — |
| `metadata.google_image_search` | boolean | 否 | `false` | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["0.5K","1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","1:4","4:1","1:8","8:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 6 |

模式条件（必须恰好命中一种）：

- `image_search_off`：`metadata.google_image_search equals false`。
- `image_search_on`：`metadata.google_image_search equals true` 且 `metadata.google_search equals true`。

请求示例：

```json
{
  "model": "toapis/gemini-3.1-flash-image-preview",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "0.5K"
  }
}
```


## toapis / gemini-3.1-flash-image-preview-vip / image

官方来源：[gemini-3.1-flash-image-preview-vip](https://docs.toapis.com/docs/cn/api-reference/images/gemini-3.1-flash-vip/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-3.1-flash-image-preview-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.grounding` | string | 否 | — | 枚举 `["none","web","image","web_image"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-3.1-flash-image-preview-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-3.1-flash-lite-image-official / image

官方来源：[gemini-3.1-flash-lite-image-official](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / gemini-nano-banana-2.1 / image

官方来源：[gemini-nano-banana-2.1](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-nano-banana-2.1`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","1:4","4:1","1:8","8:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-nano-banana-2.1",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-nano-banana-2.1-vip / image

官方来源：[gemini-nano-banana-2.1-vip](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-nano-banana-2.1-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.grounding` | string | 否 | — | 枚举 `["none","web","image","web_image"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","1:4","4:1","1:8","8:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-nano-banana-2.1-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gemini-nano-banana-2.1-official / image

官方来源：[gemini-nano-banana-2.1-official](https://docs.toapis.com/docs/cn/api-reference/images/gemini-nano-banana-2.1-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-nano-banana-2.1-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.grounding` | string | 否 | — | 枚举 `["none","web","image","web_image"]` |
| `metadata.temperature` | number | 否 | — | 最小 0；最大 2 |
| `metadata.topP` | number | 否 | — | 最小 0；最大 1 |
| `metadata.maxOutputTokens` | integer | 否 | — | 最小 1 |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["ALLOW_ALL","ALLOW_ADULT","ALLOW_NONE"]` |
| `metadata.imageOutputOptions` | object | 否 | — | — |
| `metadata.imageOutputOptions.mimeType` | string | 否 | — | 枚举 `["image/png","image/jpeg","image/webp"]` |
| `metadata.imageOutputOptions.compressionQuality` | integer | 否 | — | 最小 0；最大 100 |
| `metadata.thinkingConfig` | object | 否 | — | — |
| `metadata.thinkingConfig.thinkingBudget` | integer | 否 | — | 最小 0；最大 24576 |
| `metadata.thinkingConfig.thinkingLevel` | string | 否 | — | 枚举 `["LOW","MEDIUM","HIGH","MINIMAL"]` |
| `metadata.safetySettings` | array<object> | 否 | — | 最多项 4 |
| `metadata.safetySettings[].category` | string | 否 | — | 枚举 `["HARM_CATEGORY_HATE_SPEECH","HARM_CATEGORY_DANGEROUS_CONTENT","HARM_CATEGORY_SEXUALLY_EXPLICIT","HARM_CATEGORY_HARASSMENT"]` |
| `metadata.safetySettings[].threshold` | string | 否 | — | 枚举 `["OFF","BLOCK_LOW_AND_ABOVE","BLOCK_MEDIUM_AND_ABOVE","BLOCK_ONLY_HIGH"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","1:4","4:1","1:8","8:1","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gemini-nano-banana-2.1-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2 / image

官方来源：[gpt-image-2](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gpt-image-2`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 32000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 14 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2-official / image

官方来源：[gpt-image-2-official](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2/official)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gpt-image-2-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.quality` | string | 否 | — | 枚举 `["low","medium","high"]` |
| `metadata.background` | string | 否 | — | 枚举 `["auto","transparent","opaque"]` |
| `metadata.output_format` | string | 否 | — | 枚举 `["png","jpeg"]` |
| `metadata.output_compression` | integer | 否 | — | 最小 0；最大 100 |
| `metadata.mask_url` | string | 否 | — | 格式 http_url |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 16 |

组合约束：

- `requires`：`metadata.mask_url`、`image_urls`。

请求示例：

```json
{
  "model": "toapis/gpt-image-2-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2-vip / image

官方来源：[gpt-image-2-vip](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2-vip/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gpt-image-2-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.quality` | string | 否 | — | 枚举 `["low","medium","high"]` |
| `metadata.background` | string | 否 | — | 枚举 `["auto","transparent","opaque"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2.5-flare / image

官方来源：[gpt-image-2.5-flare](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gpt-image-2.5-flare`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.quality` | string | 否 | — | 枚举 `["high"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2.5-flare",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2.5-flare-official / image

官方来源：[gpt-image-2.5-flare-official](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前只登记有明确像素依据的1:1 + 1K = 1024x1024；其他比例与档位须补像素映射后开放。

建议公开别名：`toapis/gpt-image-2.5-flare-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | `{}` | — |
| `metadata.quality` | string | 否 | — | 枚举 `["auto","low","medium","high","xhigh","max"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | `"1K"` | 枚举 `["1K"]` |
| `size` | string | 否 | `"1:1"` | 枚举 `["1:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

像素映射只允许 schema 中列出的 1 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2.5-flare-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2.5-flare-vip / image

官方来源：[gpt-image-2.5-flare-vip](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-vip/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前只登记有明确像素依据的1:1 + 1K = 1024x1024；其他比例与档位须补像素映射后开放。

建议公开别名：`toapis/gpt-image-2.5-flare-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | `{}` | — |
| `metadata.quality` | string | 否 | — | 枚举 `["auto","low","medium","high","xhigh","max"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | `"1K"` | 枚举 `["1K"]` |
| `size` | string | 否 | `"1:1"` | 枚举 `["1:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

像素映射只允许 schema 中列出的 1 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2.5-flare-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2.5-sunburst / image

官方来源：[gpt-image-2.5-sunburst](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gpt-image-2.5-sunburst`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.quality` | string | 否 | — | 枚举 `["high"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2.5-sunburst",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2.5-sunburst-official / image

官方来源：[gpt-image-2.5-sunburst-official](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前只登记有明确像素依据的1:1 + 1K = 1024x1024；其他比例与档位须补像素映射后开放。

建议公开别名：`toapis/gpt-image-2.5-sunburst-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | `{}` | — |
| `metadata.quality` | string | 否 | — | 枚举 `["auto","low","medium","high","xhigh","max"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | `"1K"` | 枚举 `["1K"]` |
| `size` | string | 否 | `"1:1"` | 枚举 `["1:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

像素映射只允许 schema 中列出的 1 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2.5-sunburst-official",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / gpt-image-2.5-sunburst-vip / image

官方来源：[gpt-image-2.5-sunburst-vip](https://docs.toapis.com/docs/cn/api-reference/images/gpt-image-2.5-vip/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前只登记有明确像素依据的1:1 + 1K = 1024x1024；其他比例与档位须补像素映射后开放。

建议公开别名：`toapis/gpt-image-2.5-sunburst-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | `{}` | — |
| `metadata.quality` | string | 否 | — | 枚举 `["auto","low","medium","high","xhigh","max"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |
| `metadata.resolution` | string | 否 | `"1K"` | 枚举 `["1K"]` |
| `size` | string | 否 | `"1:1"` | 枚举 `["1:1"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

像素映射只允许 schema 中列出的 1 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |

请求示例：

```json
{
  "model": "toapis/gpt-image-2.5-sunburst-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / grok-imagine-1.0 / image

官方来源：[grok-imagine-1.0](https://docs.toapis.com/docs/cn/api-reference/images/grok-imagine-1.0/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/grok-imagine-1.0`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","16:9","9:16","3:2","2:3"]` |

请求示例：

```json
{
  "model": "toapis/grok-imagine-1.0",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / grok-imagine-1.0-edit / image

官方来源：[grok-imagine-1.0-edit](https://docs.toapis.com/docs/cn/api-reference/images/grok-imagine-1.0/edits)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / nano_banana / image

官方来源：[nano_banana](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / nano_banana_2 / image

官方来源：[nano_banana_2](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / nano_banana_pro / image

官方来源：[nano_banana_pro](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / qwen-image-3.0 / image

官方来源：[qwen-image-3.0](https://docs.toapis.com/docs/cn/api-reference/images/qwen-image-3.0/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/qwen-image-3.0`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.prompt_extend` | boolean | 否 | — | — |
| `metadata.prompt_extend_mode` | string | 否 | — | 枚举 `["direct","agent"]` |
| `metadata.negative_prompt` | string | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 3 |

组合约束：

- `mutually_exclusive`：`image_urls`、`metadata.prompt_extend_mode`，适用条件 `[{"path":"metadata.prompt_extend_mode","op":"equals","value":"agent"}]`。

请求示例：

```json
{
  "model": "toapis/qwen-image-3.0",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / qwen-image-3.0-pro / image

官方来源：[qwen-image-3.0-pro](https://docs.toapis.com/docs/cn/api-reference/images/qwen-image-3.0/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/qwen-image-3.0-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.prompt_extend` | boolean | 否 | — | — |
| `metadata.prompt_extend_mode` | string | 否 | — | 枚举 `["direct","agent"]` |
| `metadata.negative_prompt` | string | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 3 |

组合约束：

- `mutually_exclusive`：`image_urls`、`metadata.prompt_extend_mode`，适用条件 `[{"path":"metadata.prompt_extend_mode","op":"equals","value":"agent"}]`。

请求示例：

```json
{
  "model": "toapis/qwen-image-3.0-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / vidu-image / image

官方来源：[vidu-image](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/vidu-image`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 7 |

请求示例：

```json
{
  "model": "toapis/vidu-image",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / viduq2-fast / image

官方来源：[viduq2-fast](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/viduq2-fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 7 |

请求示例：

```json
{
  "model": "toapis/viduq2-fast",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / viduq2-pro / image

官方来源：[viduq2-pro](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/viduq2-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 7 |

请求示例：

```json
{
  "model": "toapis/viduq2-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / viduq3-fast / image

官方来源：[viduq3-fast](https://docs.toapis.com/docs/cn/api-reference/images/vidu-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/viduq3-fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 7 |

请求示例：

```json
{
  "model": "toapis/viduq3-fast",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## toapis / doubao-seedance-1-0-pro-fast / video

官方来源：[doubao-seedance-1-0-pro-fast](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedance-1-0-pro-fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 2；最大 12 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p"]` |
| `metadata.seed` | integer | 否 | — | 最小 -1；最大 4294967295 |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/doubao-seedance-1-0-pro-fast",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / doubao-seedance-1-0-pro-quality / video

官方来源：[doubao-seedance-1-0-pro-quality](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedance-1-0-pro-quality`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 2；最大 12 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p"]` |
| `metadata.seed` | integer | 否 | — | 最小 -1；最大 4294967295 |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/doubao-seedance-1-0-pro-quality",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / doubao-seedance-1-5-pro / video

官方来源：[doubao-seedance-1-5-pro](https://docs.toapis.com/docs/cn/api-reference/videos/doubao-seedance-1-5/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/doubao-seedance-1-5-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 4；最大 12 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p"]` |
| `metadata.seed` | integer | 否 | — | 最小 -1；最大 4294967295 |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.camerafixed` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 2 |

请求示例：

```json
{
  "model": "toapis/doubao-seedance-1-5-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / gemini_omni / video

官方来源：[gemini_omni](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / gemini_omni_flash / video

官方来源：[gemini_omni_flash](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前基础模板仅开放720p，1080p横屏组合另行配置。

建议公开别名：`toapis/gemini_omni_flash`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `6` | 枚举 `[4,6,10]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 3 |

请求示例：

```json
{
  "model": "toapis/gemini_omni_flash",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 6
}
```


## toapis / gemini-omni-1.1-flash-preview / video

官方来源：[gemini-omni-1.1-flash-preview](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation-1-1)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-omni-1.1-flash-preview`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `10` | 枚举 `[10]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 5 |

请求示例：

```json
{
  "model": "toapis/gemini-omni-1.1-flash-preview",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 10
}
```


## toapis / gemini-omni-1.1-flash-preview-official / video

官方来源：[gemini-omni-1.1-flash-preview-official](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-1.1-flash-preview-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-omni-1.1-flash-preview-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `6` | 最小 3；最大 10 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["360p","720p","1080p","4K"]` |
| `metadata.task` | string | 否 | — | 枚举 `["text_to_video","image_to_video","reference_to_video"]` |
| `metadata.temperature` | number | 否 | — | 最小 0；最大 2 |
| `metadata.topP` | number | 否 | — | 最小 0；最大 1 |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/gemini-omni-1.1-flash-preview-official",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "360p"
  },
  "duration": 6
}
```


## toapis / gemini-omni-1.1-flash-preview-vip / video

官方来源：[gemini-omni-1.1-flash-preview-vip](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation-1-1-vip)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-omni-1.1-flash-preview-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `6` | 枚举 `[4,6,8,10]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["360p","720p","1080p","4k"]` |
| `metadata.generation_type` | string | 否 | — | 枚举 `["frame"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty` 且 `metadata.generation_type absent`。
- `frame_auto`：`image_urls not_empty` 且 `metadata.generation_type absent`。
- `frame_explicit`：`image_urls not_empty` 且 `metadata.generation_type equals "frame"`。

请求示例：

```json
{
  "model": "toapis/gemini-omni-1.1-flash-preview-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "360p"
  },
  "duration": 6
}
```


## toapis / gemini-omni-flash-preview-official / video

官方来源：[gemini-omni-flash-preview-official](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash-preview-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gemini-omni-flash-preview-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `6` | 最小 3；最大 10 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `metadata.task` | string | 否 | — | 枚举 `["text_to_video","image_to_video","reference_to_video"]` |
| `metadata.temperature` | number | 否 | — | 最小 0；最大 2 |
| `metadata.topP` | number | 否 | — | 最小 0；最大 1 |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/gemini-omni-flash-preview-official",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 6
}
```


## toapis / grok-video-1.0 / video

官方来源：[grok-video-1.0](https://docs.toapis.com/docs/cn/api-reference/videos/grok-video/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/grok-video-1.0`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 最小 1；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","3:2","2:3","1:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 8 |

请求示例：

```json
{
  "model": "toapis/grok-video-1.0",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 8
}
```


## toapis / grok-video-1.5 / video

官方来源：[grok-video-1.5](https://docs.toapis.com/docs/cn/api-reference/videos/grok-video-1.5/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/grok-video-1.5`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 最小 1；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","3:2","2:3","1:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p"]` |
| `metadata.video_generation_mode` | string | 否 | — | 枚举 `["text_to_video","reference_images_to_video"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 7 |

模式条件（必须恰好命中一种）：

- `text_auto`：`image_urls empty` 且 `metadata.video_generation_mode absent`。
- `text`：`image_urls empty` 且 `metadata.video_generation_mode equals "text_to_video"`。
- `reference`：`image_urls not_empty` 且 `metadata.video_generation_mode equals "reference_images_to_video"`。

请求示例：

```json
{
  "model": "toapis/grok-video-1.5",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 8
}
```


## toapis / happyhorse-1.1 / video

官方来源：[happyhorse-1.1](https://docs.toapis.com/docs/cn/api-reference/videos/happyhorse/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/happyhorse-1.1`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 否 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 3；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720P","1080P"]` |
| `metadata.action` | string | 否 | — | 枚举 `["text-to-video","image-to-video"]` |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

模式条件（必须恰好命中一种）：

- `text_false`：`image_urls empty` 且 `metadata.action absent`；必填 `prompt`。
- `first_frame_false`：`image_urls not_empty` 且 `metadata.action absent`；禁止 `aspect_ratio`。
- `text_true`：`image_urls empty` 且 `metadata.action equals "text-to-video"`；必填 `prompt`。
- `first_frame_true`：`image_urls not_empty` 且 `metadata.action equals "image-to-video"`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/happyhorse-1.1",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720P",
    "action": "text-to-video"
  },
  "duration": 5
}
```


## toapis / kling-3.0-turbo / video

官方来源：[kling-3.0-turbo](https://docs.toapis.com/docs/cn/api-reference/videos/kling-3.0-turbo/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/kling-3.0-turbo`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2500 |
| `duration` | integer | 否 | `5` | 最小 3；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p"]` |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；必填 `prompt`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/kling-3.0-turbo",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 5
}
```


## toapis / kling-v2-6 / video

官方来源：[kling-v2-6](https://docs.toapis.com/docs/cn/api-reference/videos/kling-2-6/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/kling-v2-6`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2500 |
| `duration` | integer | 否 | `5` | 枚举 `[5,10]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1"]` |
| `metadata` | object | 否 | `{}` | — |
| `metadata.mode` | string | 否 | `"std"` | 枚举 `["std","pro"]` |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.audio` | boolean | 否 | `false` | — |
| `metadata.negative_prompt` | string | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

模式条件（必须恰好命中一种）：

- `silent`：`metadata.audio equals false`。
- `sound_pro`：`metadata.audio equals true` 且 `metadata.mode equals "pro"`。

请求示例：

```json
{
  "model": "toapis/kling-v2-6",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 5
}
```


## toapis / kling-v3 / video

官方来源：[kling-v3](https://docs.toapis.com/docs/cn/api-reference/videos/kling-v3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/kling-v3`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2500 |
| `duration` | integer | 否 | `5` | 最小 3；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.mode` | string | 否 | — | 枚举 `["std","pro"]` |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.negative_prompt` | string | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

请求示例：

```json
{
  "model": "toapis/kling-v3",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 5
}
```


## toapis / kling-v3-omni / video

官方来源：[kling-v3-omni](https://docs.toapis.com/docs/cn/api-reference/videos/kling-v3-omni/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/kling-v3-omni`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2500 |
| `duration` | integer | 否 | `5` | 最小 3；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.mode` | string | 否 | — | 枚举 `["std","pro"]` |
| `metadata.watermark` | boolean | 否 | — | — |

请求示例：

```json
{
  "model": "toapis/kling-v3-omni",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 5
}
```


## toapis / kling-video-o1 / video

官方来源：[kling-video-o1](https://docs.toapis.com/docs/cn/api-reference/videos/kling-video-o1/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/kling-video-o1`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2500 |
| `duration` | integer | 否 | `5` | 最小 3；最大 10 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1"]` |
| `metadata` | object | 否 | — | — |
| `metadata.mode` | string | 否 | — | 枚举 `["std","pro"]` |
| `metadata.watermark` | boolean | 否 | — | — |

请求示例：

```json
{
  "model": "toapis/kling-video-o1",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 5
}
```


## toapis / MiniMax-H3 / video

官方来源：[MiniMax-H3](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 首尾/图片输入模式不接受显式aspect_ratio，避免上游忽略参数；参考素材角色扩展另行配置。

建议公开别名：`toapis/MiniMax-H3`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 7000 |
| `duration` | integer | 否 | `5` | 最小 4；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["2K","768p"]` |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；必填 `prompt`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/MiniMax-H3",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "2K"
  },
  "duration": 5
}
```


## toapis / MiniMax-H3-Max / video

官方来源：[MiniMax-H3-Max](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 首尾/图片输入模式不接受显式aspect_ratio，避免上游忽略参数；参考素材角色扩展另行配置。

建议公开别名：`toapis/MiniMax-H3-Max`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 7000 |
| `duration` | integer | 否 | `5` | 最小 5；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480P","768P"]` |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；必填 `prompt`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/MiniMax-H3-Max",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480P"
  },
  "duration": 5
}
```


## toapis / MiniMax-H3-Max-Turbo / video

官方来源：[MiniMax-H3-Max-Turbo](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-h3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 首尾/图片输入模式不接受显式aspect_ratio，避免上游忽略参数；参考素材角色扩展另行配置。

建议公开别名：`toapis/MiniMax-H3-Max-Turbo`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 7000 |
| `duration` | integer | 否 | `5` | 最小 5；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480P","768P"]` |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 2 |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；必填 `prompt`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/MiniMax-H3-Max-Turbo",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480P"
  },
  "duration": 5
}
```


## toapis / MiniMax-Hailuo-02 / video

官方来源：[MiniMax-Hailuo-02](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前基础模板仅开放768P，1080P的6秒组合另行配置。

建议公开别名：`toapis/MiniMax-Hailuo-02`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 否 | — | 最短字符 1；最长字符 2000 |
| `duration` | integer | 否 | `6` | 枚举 `[6,10]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["768P"]` |
| `metadata.prompt_optimizer` | boolean | 否 | — | — |
| `metadata.fast_pretreatment` | boolean | 否 | — | — |
| `metadata.aigc_watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`。

请求示例：

```json
{
  "model": "toapis/MiniMax-Hailuo-02",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "768P"
  },
  "duration": 6
}
```


## toapis / MiniMax-Hailuo-2.3 / video

官方来源：[MiniMax-Hailuo-2.3](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo-2.3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前基础模板仅开放768P，1080P的6秒组合另行配置。

建议公开别名：`toapis/MiniMax-Hailuo-2.3`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2000 |
| `duration` | integer | 否 | `6` | 枚举 `[6,10]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["768P"]` |
| `metadata.prompt_optimizer` | boolean | 否 | — | — |
| `metadata.fast_pretreatment` | boolean | 否 | — | — |
| `metadata.aigc_watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/MiniMax-Hailuo-2.3",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "768P"
  },
  "duration": 6
}
```


## toapis / MiniMax-Hailuo-2.3-Fast / video

官方来源：[MiniMax-Hailuo-2.3-Fast](https://docs.toapis.com/docs/cn/api-reference/videos/minimax-hailuo-2.3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前基础模板仅开放768P，1080P的6秒组合另行配置。

建议公开别名：`toapis/MiniMax-Hailuo-2.3-Fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 2000 |
| `duration` | integer | 否 | `6` | 枚举 `[6,10]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["768P"]` |
| `metadata.prompt_optimizer` | boolean | 否 | — | — |
| `metadata.fast_pretreatment` | boolean | 否 | — | — |
| `metadata.aigc_watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/MiniMax-Hailuo-2.3-Fast",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "768P"
  },
  "duration": 6
}
```


## toapis / seedance-2 / video

官方来源：[seedance-2](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 4；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p","4k"]` |
| `metadata.generate_audio` | boolean | 否 | — | — |
| `metadata.return_last_frame` | boolean | 否 | — | 枚举 `[false]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

请求示例：

```json
{
  "model": "toapis/seedance-2",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / seedance-2-5 / video

官方来源：[seedance-2-5](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-5/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2-5`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 4；最大 30 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p"]` |
| `metadata.generate_audio` | boolean | 否 | — | — |
| `metadata.return_last_frame` | boolean | 否 | — | 枚举 `[false]` |
| `metadata.video_operation` | string | 否 | — | 枚举 `["generate"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 30 |

请求示例：

```json
{
  "model": "toapis/seedance-2-5",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / seedance-2-ext / video

官方来源：[seedance-2-ext](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-ext/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2-ext`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 4；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

请求示例：

```json
{
  "model": "toapis/seedance-2-ext",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 5
}
```


## toapis / seedance-2-fast / video

官方来源：[seedance-2-fast](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2-fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 4；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p"]` |
| `metadata.generate_audio` | boolean | 否 | — | — |
| `metadata.return_last_frame` | boolean | 否 | — | 枚举 `[false]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

请求示例：

```json
{
  "model": "toapis/seedance-2-fast",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / seedance-2-fast-ext / video

官方来源：[seedance-2-fast-ext](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-fast-ext/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2-fast-ext`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 枚举 `[5,10,15]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

请求示例：

```json
{
  "model": "toapis/seedance-2-fast-ext",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / seedance-2-mini / video

官方来源：[seedance-2-mini](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2-mini`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 4；最大 15 |
| `aspect_ratio` | string | 否 | — | 枚举 `["21:9","16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p"]` |
| `metadata.generate_audio` | boolean | 否 | — | — |
| `metadata.return_last_frame` | boolean | 否 | — | 枚举 `[false]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 9 |

请求示例：

```json
{
  "model": "toapis/seedance-2-mini",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / seedance-2.5-ext / video

官方来源：[seedance-2.5-ext](https://docs.toapis.com/docs/cn/api-reference/videos/seedance-2-5-ext/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/seedance-2.5-ext`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `10` | 最小 10；最大 30 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","4:3","1:1","3:4","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 10 |

请求示例：

```json
{
  "model": "toapis/seedance-2.5-ext",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 10
}
```


## toapis / sora-2-official / video

官方来源：[sora-2-official](https://docs.toapis.com/docs/cn/api-reference/videos/sora-2-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/sora-2-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `4` | 枚举 `[4,8,12]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/sora-2-official",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 4
}
```


## toapis / veo3.1-fast / video

官方来源：[veo3.1-fast](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/veo3.1-fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[8]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p","4k"]` |
| `metadata.generation_type` | string | 否 | — | 枚举 `["frame"]` |
| `metadata.enable_gif` | boolean | 否 | — | 枚举 `[false]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/veo3.1-fast",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 8
}
```


## toapis / Veo3.1-fast-official / video

官方来源：[Veo3.1-fast-official](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/Veo3.1-fast-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[4,6,8]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p","4k"]` |
| `metadata.generateAudio` | boolean | 否 | — | — |
| `metadata.negativePrompt` | string | 否 | — | — |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["allow_adult","dont_allow"]` |
| `metadata.compressionQuality` | string | 否 | — | 枚举 `["optimized","lossless"]` |
| `metadata.resizeMode` | string | 否 | — | 枚举 `["pad","crop"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/Veo3.1-fast-official",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 8
}
```


## toapis / veo3.1-lite / video

官方来源：[veo3.1-lite](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/veo3.1-lite`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[8]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `metadata.generation_type` | string | 否 | — | 枚举 `["frame"]` |
| `metadata.enable_gif` | boolean | 否 | — | 枚举 `[false]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/veo3.1-lite",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 8
}
```


## toapis / Veo3.1-lite-official / video

官方来源：[Veo3.1-lite-official](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/Veo3.1-lite-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[4,6,8]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `metadata.generateAudio` | boolean | 否 | — | — |
| `metadata.negativePrompt` | string | 否 | — | — |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["allow_adult","dont_allow"]` |
| `metadata.compressionQuality` | string | 否 | — | 枚举 `["optimized","lossless"]` |
| `metadata.resizeMode` | string | 否 | — | 枚举 `["pad","crop"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/Veo3.1-lite-official",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 8
}
```


## toapis / veo3.1-quality / video

官方来源：[veo3.1-quality](https://docs.toapis.com/docs/cn/api-reference/videos/veo3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/veo3.1-quality`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[8]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p","4k"]` |
| `metadata.generation_type` | string | 否 | — | 枚举 `["frame"]` |
| `metadata.enable_gif` | boolean | 否 | — | 枚举 `[false]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/veo3.1-quality",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 8
}
```


## toapis / Veo3.1-quality-official / video

官方来源：[Veo3.1-quality-official](https://docs.toapis.com/docs/cn/api-reference/videos/veo3-official/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/Veo3.1-quality-official`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[4,6,8]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p","4k"]` |
| `metadata.generateAudio` | boolean | 否 | — | — |
| `metadata.negativePrompt` | string | 否 | — | — |
| `metadata.personGeneration` | string | 否 | — | 枚举 `["allow_adult","dont_allow"]` |
| `metadata.compressionQuality` | string | 否 | — | 枚举 `["optimized","lossless"]` |
| `metadata.resizeMode` | string | 否 | — | 枚举 `["pad","crop"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/Veo3.1-quality-official",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 8
}
```


## toapis / viduq3 / video

官方来源：[viduq3](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/viduq3`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 5000 |
| `duration` | integer | 否 | `5` | 最小 3；最大 16 |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["540p","720p","1080p"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.audio_type` | string | 否 | — | 枚举 `["all","speech_only","sound-effect_only"]` |
| `image_urls` | array<string> | 是 | — | 最少项 1；最多项 7 |

请求示例：

```json
{
  "model": "toapis/viduq3",
  "prompt": "A quiet mountain lake at sunrise",
  "image_urls": [
    "https://example.com/reference.png"
  ],
  "metadata": {
    "resolution": "540p"
  },
  "duration": 5
}
```


## toapis / viduq3-ad / video

官方来源：[viduq3-ad](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / viduq3-drama / video

官方来源：[viduq3-drama](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / viduq3-mix / video

官方来源：[viduq3-mix](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / viduq3-pro / video

官方来源：[viduq3-pro](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/viduq3-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 5000 |
| `duration` | integer | 否 | `5` | 最小 1；最大 16 |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["540p","720p","1080p"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.audio_type` | string | 否 | — | 枚举 `["all","speech_only","sound-effect_only"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 2 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4"]` |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；必填 `prompt`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/viduq3-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "540p"
  },
  "duration": 5
}
```


## toapis / viduq3-pro-fast / video

官方来源：[viduq3-pro-fast](https://docs.toapis.com/docs/cn/api-reference/videos/vidu-ali/generation)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / viduq3-turbo / video

官方来源：[viduq3-turbo](https://docs.toapis.com/docs/cn/api-reference/videos/viduq3/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/viduq3-turbo`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 5000 |
| `duration` | integer | 否 | `5` | 最小 1；最大 16 |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["540p","720p","1080p"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.audio_type` | string | 否 | — | 枚举 `["all","speech_only","sound-effect_only"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 2 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4"]` |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；必填 `prompt`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/viduq3-turbo",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "540p"
  },
  "duration": 5
}
```


## toapis / viduq4-preview / video

官方来源：[viduq4-preview](https://docs.toapis.com/docs/cn/api-reference/videos/viduq4-preview/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前基础模板仅开放单图首帧，不接受video_generation_mode或aspect_ratio；参考模式另行配置。

建议公开别名：`toapis/viduq4-preview`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 3；最大 16 |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["540p","720p","1080p","2K","4K"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 是 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/viduq4-preview",
  "prompt": "A quiet mountain lake at sunrise",
  "image_urls": [
    "https://example.com/reference.png"
  ],
  "metadata": {
    "resolution": "540p"
  },
  "duration": 5
}
```


## toapis / wan2.6 / video

官方来源：[wan2.6](https://docs.toapis.com/docs/cn/api-reference/videos/wan2.6/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/wan2.6`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 否 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 枚举 `[5,10,15]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.negative_prompt` | string | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.prompt_extend` | boolean | 否 | — | — |
| `metadata.shot_type` | string | 否 | — | 枚举 `["single","multi"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 1 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4"]` |

模式条件（必须恰好命中一种）：

- `text`：`image_urls empty`；必填 `prompt`。
- `reference`：`image_urls not_empty`；禁止 `aspect_ratio`。

请求示例：

```json
{
  "model": "toapis/wan2.6",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 5
}
```


## toapis / wan2.6-flash / video

官方来源：[wan2.6-flash](https://docs.toapis.com/docs/cn/api-reference/videos/wan2.6-flash/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/wan2.6-flash`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 否 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 枚举 `[5,10,15]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p","1080p"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `image_urls` | array<string> | 是 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/wan2.6-flash",
  "prompt": "A quiet mountain lake at sunrise",
  "image_urls": [
    "https://example.com/reference.png"
  ],
  "metadata": {
    "resolution": "720p"
  },
  "duration": 5
}
```


## toapis / wan3.0-video / video

官方来源：[wan3.0-video](https://docs.toapis.com/docs/cn/api-reference/videos/wan3.0/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/wan3.0-video`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `5` | 最小 2；最大 30 |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16","1:1","4:3","3:4"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["480p","720p","1080p"]` |
| `metadata.audio` | boolean | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.seed` | integer | 否 | — | — |

请求示例：

```json
{
  "model": "toapis/wan3.0-video",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "480p"
  },
  "duration": 5
}
```


## toapis / wan3.0-video-prime / video

官方来源：[wan3.0-video-prime](https://docs.toapis.com/docs/cn/api-reference/models)

文档缺少完整模式/字段约束或属于独立编辑，不能据相邻型号自动启用。

**未提供可启用 schema。** 保留目录线索，当前不会出现在可复制模板中。


## toapis / gpt-4o-image / image

官方来源：[gpt-4o-image](https://docs.toapis.com/docs/cn/api-reference/images/gpt-4o/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/gpt-4o-image`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","2:3","3:2"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 5 |

请求示例：

```json
{
  "model": "toapis/gpt-4o-image",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / grok-4-1-image / image

官方来源：[grok-4-1-image](https://docs.toapis.com/docs/cn/api-reference/images/grok-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/grok-4-1-image`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","2:3","3:2","16:9","9:16"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 5 |

请求示例：

```json
{
  "model": "toapis/grok-4-1-image",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / grok-4-2-image / image

官方来源：[grok-4-2-image](https://docs.toapis.com/docs/cn/api-reference/images/grok-image/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/grok-4-2-image`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1；最长字符 1000 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `size` | string | 否 | — | 枚举 `["1:1","2:3","3:2","16:9","9:16"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 5 |

请求示例：

```json
{
  "model": "toapis/grok-4-2-image",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## toapis / midjourney / image

官方来源：[midjourney](https://docs.toapis.com/docs/cn/api-reference/images/midjourney/imagine)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/midjourney`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `metadata` | object | 否 | — | — |
| `metadata.speed` | string | 否 | — | 枚举 `["relax","fast","turbo"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |

请求示例：

```json
{
  "model": "toapis/midjourney",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1
}
```


## toapis / sora-2-vvip / video

官方来源：[sora-2-vvip](https://docs.toapis.com/docs/cn/api-reference/videos/sora2/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/sora-2-vvip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `8` | 枚举 `[4,8,12]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `metadata` | object | 否 | — | — |
| `metadata.watermark` | boolean | 否 | — | — |
| `metadata.private` | boolean | 否 | — | — |
| `metadata.style` | string | 否 | — | 枚举 `["thanksgiving","comic","news","selfie","nostalgic","anime"]` |

请求示例：

```json
{
  "model": "toapis/sora-2-vvip",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 8
}
```


## toapis / gemini-omni-flash / video

官方来源：[gemini-omni-flash](https://docs.toapis.com/docs/cn/api-reference/videos/gemini-omni-flash/generation)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。 当前基础模板仅开放720p，1080p横屏组合另行配置。

建议公开别名：`toapis/gemini-omni-flash`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 否 | `6` | 枚举 `[4,6,10]` |
| `aspect_ratio` | string | 否 | — | 枚举 `["16:9","9:16"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["720p"]` |
| `image_urls` | array<string> | 否 | — | 最少项 1；最多项 3 |

请求示例：

```json
{
  "model": "toapis/gemini-omni-flash",
  "prompt": "A quiet mountain lake at sunrise",
  "metadata": {
    "resolution": "720p"
  },
  "duration": 6
}
```


## toapis / midjourney / video

官方来源：[midjourney](https://docs.toapis.com/docs/cn/api-reference/images/midjourney/video)

基础普通生成模板；未覆盖的高级模式不在白名单内。复制后核对账号、定价和渠道能力再启用。

建议公开别名：`toapis/midjourney`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `image_urls` | array<string> | 是 | — | 最少项 1；最多项 1 |

请求示例：

```json
{
  "model": "toapis/midjourney",
  "image_urls": [
    "https://example.com/reference.png"
  ]
}
```


## grsai / nano-banana-2.1 / image

官方来源：[nano-banana-2.1](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-2.1`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-2.1",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / nano-banana-pro / image

官方来源：[nano-banana-pro](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-pro`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-pro",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / nano-banana-2 / image

官方来源：[nano-banana-2](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-2`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K","4K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-2",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / nano-banana-2-lite / image

官方来源：[nano-banana-2-lite](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。 分辨率缺逐型号证据，因此不接受resolution。

建议公开别名：`grsai/nano-banana-2-lite`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-2-lite",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## grsai / nano-banana-fast / image

官方来源：[nano-banana-fast](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。 分辨率缺逐型号证据，因此不接受resolution。

建议公开别名：`grsai/nano-banana-fast`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-fast",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1"
}
```


## grsai / nano-banana-pro-cl / image

官方来源：[nano-banana-pro-cl](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-pro-cl`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-pro-cl",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / nano-banana-2-cl / image

官方来源：[nano-banana-2-cl](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-2-cl`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-2-cl",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / nano-banana-2-2k-cl / image

官方来源：[nano-banana-2-2k-cl](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-2-2k-cl`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["2K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-2-2k-cl",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "2K"
  }
}
```


## grsai / nano-banana-pro-vip / image

官方来源：[nano-banana-pro-vip](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-pro-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K","2K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-pro-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / nano-banana-pro-4k-vip / image

官方来源：[nano-banana-pro-4k-vip](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-pro-4k-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["4K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-pro-4k-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "4K"
  }
}
```


## grsai / nano-banana-2-4k-cl / image

官方来源：[nano-banana-2-4k-cl](https://qmy27nhsd9.apifox.cn/452392911e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/nano-banana-2-4k-cl`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["4K"]` |

请求示例：

```json
{
  "model": "grsai/nano-banana-2-4k-cl",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "4K"
  }
}
```


## grsai / gpt-image-2 / image

官方来源：[gpt-image-2](https://qmy27nhsd9.apifox.cn/452409160e0.md)

当前官方目录登记，标记维护中；不据此启用调度；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/gpt-image-2`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K"]` |
| `metadata.quality` | string | 否 | — | 枚举 `["auto"]` |

请求示例：

```json
{
  "model": "grsai/gpt-image-2",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / gpt-image-2-vip / image

官方来源：[gpt-image-2-vip](https://qmy27nhsd9.apifox.cn/452409160e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/gpt-image-2-vip`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 是 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `metadata` | object | 是 | — | — |
| `metadata.resolution` | string | 是 | — | 枚举 `["1K","2K","4K"]` |
| `metadata.quality` | string | 否 | — | 枚举 `["medium"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |

像素映射只允许 schema 中列出的 39 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |
| `1:1` | `2K` | 2048 | 2048 |
| `1:1` | `4K` | 2880 | 2880 |
| `16:9` | `1K` | 1280 | 720 |
| `16:9` | `2K` | 2048 | 1152 |
| `16:9` | `4K` | 3840 | 2160 |
| `9:16` | `1K` | 720 | 1280 |
| `9:16` | `2K` | 1152 | 2048 |
| `9:16` | `4K` | 2160 | 3840 |
| `4:3` | `1K` | 1152 | 864 |
| `4:3` | `2K` | 2304 | 1728 |
| `4:3` | `4K` | 3264 | 2448 |
| `3:4` | `1K` | 864 | 1152 |
| `3:4` | `2K` | 1728 | 2304 |
| `3:4` | `4K` | 2448 | 3264 |
| `3:2` | `1K` | 1536 | 1024 |
| `3:2` | `2K` | 2048 | 1360 |
| `3:2` | `4K` | 3504 | 2336 |
| `2:3` | `1K` | 1024 | 1536 |
| `2:3` | `2K` | 1360 | 2048 |
| `2:3` | `4K` | 2336 | 3504 |
| `5:4` | `1K` | 1120 | 896 |
| `5:4` | `2K` | 2240 | 1792 |
| `5:4` | `4K` | 3200 | 2560 |
| `4:5` | `1K` | 896 | 1120 |
| `4:5` | `2K` | 1792 | 2240 |
| `4:5` | `4K` | 2560 | 3200 |
| `21:9` | `1K` | 1456 | 624 |
| `21:9` | `2K` | 2912 | 1248 |
| `21:9` | `4K` | 3840 | 1648 |
| `9:21` | `1K` | 624 | 1456 |
| `9:21` | `2K` | 1248 | 2912 |
| `9:21` | `4K` | 1648 | 3840 |
| `2:1` | `1K` | 1536 | 768 |
| `2:1` | `2K` | 3072 | 1536 |
| `2:1` | `4K` | 3840 | 1920 |
| `1:2` | `1K` | 768 | 1536 |
| `1:2` | `2K` | 1536 | 3072 |
| `1:2` | `4K` | 1920 | 3840 |

请求示例：

```json
{
  "model": "grsai/gpt-image-2-vip",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / gpt-image-2.5 / image

官方来源：[gpt-image-2.5](https://qmy27nhsd9.apifox.cn/452409160e0.md)

当前官方目录登记，标记维护中；不据此启用调度；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/gpt-image-2.5`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 否 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `metadata` | object | 否 | — | — |
| `metadata.resolution` | string | 否 | — | 枚举 `["1K"]` |
| `metadata.quality` | string | 否 | — | 枚举 `["auto"]` |

请求示例：

```json
{
  "model": "grsai/gpt-image-2.5",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / gpt-image-2.5-flare / image

官方来源：[gpt-image-2.5-flare](https://qmy27nhsd9.apifox.cn/452409160e0.md)

当前官方目录登记，标记维护中；不据此启用调度；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/gpt-image-2.5-flare`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 是 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `metadata` | object | 是 | — | — |
| `metadata.resolution` | string | 是 | — | 枚举 `["1K","2K","4K"]` |
| `metadata.quality` | string | 否 | — | 枚举 `["low","medium","high"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |

像素映射只允许 schema 中列出的 39 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |
| `1:1` | `2K` | 2048 | 2048 |
| `1:1` | `4K` | 2880 | 2880 |
| `16:9` | `1K` | 1280 | 720 |
| `16:9` | `2K` | 2048 | 1152 |
| `16:9` | `4K` | 3840 | 2160 |
| `9:16` | `1K` | 720 | 1280 |
| `9:16` | `2K` | 1152 | 2048 |
| `9:16` | `4K` | 2160 | 3840 |
| `4:3` | `1K` | 1152 | 864 |
| `4:3` | `2K` | 2304 | 1728 |
| `4:3` | `4K` | 3264 | 2448 |
| `3:4` | `1K` | 864 | 1152 |
| `3:4` | `2K` | 1728 | 2304 |
| `3:4` | `4K` | 2448 | 3264 |
| `3:2` | `1K` | 1536 | 1024 |
| `3:2` | `2K` | 2048 | 1360 |
| `3:2` | `4K` | 3504 | 2336 |
| `2:3` | `1K` | 1024 | 1536 |
| `2:3` | `2K` | 1360 | 2048 |
| `2:3` | `4K` | 2336 | 3504 |
| `5:4` | `1K` | 1120 | 896 |
| `5:4` | `2K` | 2240 | 1792 |
| `5:4` | `4K` | 3200 | 2560 |
| `4:5` | `1K` | 896 | 1120 |
| `4:5` | `2K` | 1792 | 2240 |
| `4:5` | `4K` | 2560 | 3200 |
| `21:9` | `1K` | 1456 | 624 |
| `21:9` | `2K` | 2912 | 1248 |
| `21:9` | `4K` | 3840 | 1648 |
| `9:21` | `1K` | 624 | 1456 |
| `9:21` | `2K` | 1248 | 2912 |
| `9:21` | `4K` | 1648 | 3840 |
| `2:1` | `1K` | 1536 | 768 |
| `2:1` | `2K` | 3072 | 1536 |
| `2:1` | `4K` | 3840 | 1920 |
| `1:2` | `1K` | 768 | 1536 |
| `1:2` | `2K` | 1536 | 3072 |
| `1:2` | `4K` | 1920 | 3840 |

请求示例：

```json
{
  "model": "grsai/gpt-image-2.5-flare",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / gpt-image-2.5-sunburst / image

官方来源：[gpt-image-2.5-sunburst](https://qmy27nhsd9.apifox.cn/452409160e0.md)

当前官方目录登记，标记维护中；不据此启用调度；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/gpt-image-2.5-sunburst`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `n` | integer | 否 | `1` | 最小 1；最大 8 |
| `image_urls` | array<string> | 否 | — | 最少项 1 |
| `size` | string | 是 | — | 枚举 `["1:1","4:3","3:4","16:9","9:16","3:2","2:3","5:4","4:5","21:9","2:1","1:2","9:21"]` |
| `metadata` | object | 是 | — | — |
| `metadata.resolution` | string | 是 | — | 枚举 `["1K","2K","4K"]` |
| `metadata.quality` | string | 否 | — | 枚举 `["low","medium","high","xhigh","max"]` |
| `metadata.background` | string | 否 | — | 枚举 `["transparent"]` |

像素映射只允许 schema 中列出的 39 个 `size + metadata.resolution` 组合；未列组合返回 400，不能按档位猜像素。

| size | metadata.resolution | width | height |
| --- | --- | ---: | ---: |
| `1:1` | `1K` | 1024 | 1024 |
| `1:1` | `2K` | 2048 | 2048 |
| `1:1` | `4K` | 2880 | 2880 |
| `16:9` | `1K` | 1280 | 720 |
| `16:9` | `2K` | 2048 | 1152 |
| `16:9` | `4K` | 3840 | 2160 |
| `9:16` | `1K` | 720 | 1280 |
| `9:16` | `2K` | 1152 | 2048 |
| `9:16` | `4K` | 2160 | 3840 |
| `4:3` | `1K` | 1152 | 864 |
| `4:3` | `2K` | 2304 | 1728 |
| `4:3` | `4K` | 3264 | 2448 |
| `3:4` | `1K` | 864 | 1152 |
| `3:4` | `2K` | 1728 | 2304 |
| `3:4` | `4K` | 2448 | 3264 |
| `3:2` | `1K` | 1536 | 1024 |
| `3:2` | `2K` | 2048 | 1360 |
| `3:2` | `4K` | 3504 | 2336 |
| `2:3` | `1K` | 1024 | 1536 |
| `2:3` | `2K` | 1360 | 2048 |
| `2:3` | `4K` | 2336 | 3504 |
| `5:4` | `1K` | 1120 | 896 |
| `5:4` | `2K` | 2240 | 1792 |
| `5:4` | `4K` | 3200 | 2560 |
| `4:5` | `1K` | 896 | 1120 |
| `4:5` | `2K` | 1792 | 2240 |
| `4:5` | `4K` | 2560 | 3200 |
| `21:9` | `1K` | 1456 | 624 |
| `21:9` | `2K` | 2912 | 1248 |
| `21:9` | `4K` | 3840 | 1648 |
| `9:21` | `1K` | 624 | 1456 |
| `9:21` | `2K` | 1248 | 2912 |
| `9:21` | `4K` | 1648 | 3840 |
| `2:1` | `1K` | 1536 | 768 |
| `2:1` | `2K` | 3072 | 1536 |
| `2:1` | `4K` | 3840 | 1920 |
| `1:2` | `1K` | 768 | 1536 |
| `1:2` | `2K` | 1536 | 3072 |
| `1:2` | `4K` | 1920 | 3840 |

请求示例：

```json
{
  "model": "grsai/gpt-image-2.5-sunburst",
  "prompt": "A quiet mountain lake at sunrise",
  "n": 1,
  "size": "1:1",
  "metadata": {
    "resolution": "1K"
  }
}
```


## grsai / minimax-h3 / video

官方来源：[minimax-h3](https://qmy27nhsd9.apifox.cn/514679297e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/minimax-h3`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 是 | — | 最小 1；最大 10 |
| `aspect_ratio` | string | 是 | — | 枚举 `["16:9","9:16"]` |
| `image_urls` | array<string> | 否 | — | 最多项 9 |
| `metadata` | object | 是 | — | — |
| `metadata.resolution` | string | 是 | — | 枚举 `["480p","768p","1080p"]` |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.audio_urls` | array<string> | 否 | — | 最多项 3 |

请求示例：

```json
{
  "model": "grsai/minimax-h3",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 5,
  "aspect_ratio": "16:9",
  "metadata": {
    "resolution": "768p"
  }
}
```


## grsai / agnes-video-2.5 / video

官方来源：[agnes-video-2.5](https://qmy27nhsd9.apifox.cn/523334530e0.md)

当前官方目录登记，未标维护；未实测；基础模板不自动启用，未证实的蒙版/指定首尾帧不开放。

建议公开别名：`grsai/agnes-video-2.5`。管理员可以另设统一别名，并用候选能力配置区分渠道限制。

| 字段 | 类型 | 全局必填 | 默认 | 允许值/范围 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 已配置的公开模型别名 |
| `prompt` | string | 是 | — | 最短字符 1 |
| `duration` | integer | 是 | — | 最小 4；最大 12 |
| `aspect_ratio` | string | 是 | — | 枚举 `["16:9","9:16","21:9","4:3","1:1","3:4"]` |
| `image_urls` | array<string> | 否 | — | 最多项 5 |
| `metadata` | object | 是 | — | — |
| `metadata.resolution` | string | 是 | — | 枚举 `["720p","1080p","2k"]` |
| `metadata.seed` | integer | 否 | — | — |
| `metadata.audio_urls` | array<string> | 否 | — | 最多项 3 |

请求示例：

```json
{
  "model": "grsai/agnes-video-2.5",
  "prompt": "A quiet mountain lake at sunrise",
  "duration": 5,
  "aspect_ratio": "16:9",
  "metadata": {
    "resolution": "720p"
  }
}
```
