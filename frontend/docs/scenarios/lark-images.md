---
description: 用自建服务连接飞书多维表格与 Gkotta 图片 API，将生成图片上传为表格附件。
---

# 飞书多维表格图片生成

可以将多维表格中的提示词交给你自己的服务，由服务调用 Gkotta 图片 API，再把生成的图片上传到飞书并回写附件。本文介绍这套自建集成流程，需要你部署服务并配置飞书应用；Gkotta 提供图片接口，当前没有内置的飞书多维表格插件。

## 流程

```mermaid
flowchart LR
  A[多维表格中的待生成记录] --> B[自建服务读取并领取任务]
  B --> C[Gkotta 图片 API]
  C --> D[保存图片字节]
  D --> E[上传飞书素材并获取 file_token]
  E --> F[更新原记录的附件和状态]
```

先用一条记录验证完整流程，再增加自动触发、并发和批量任务。即使表格有很多行，也不要一次同时发送全部生成请求。

## 准备条件

### Gkotta

1. 在[API 密钥](https://www.gkotta.bid/keys)创建专用密钥，设置适当的额度上限。
2. 在[可用渠道](https://www.gkotta.bid/available-channels)确认开放的图片模型和协议。
3. 对 OpenAI Images/Grok 图片接口，确认分组已开启图片生成权限。
4. 在自建服务中先完成[图片生成与编辑](/image-generate)的最小请求，确认能保存图片文件。

`YOUR_IMAGE_MODEL_ID` 必须替换为当前密钥分组开放的图片模型 ID，可使用 `/v1/models` 查询。使用 Gemini 原生分组时，按图片教程的 `/v1beta` 请求和图片返回格式接入。

### 飞书

在[飞书开放平台](https://open.feishu.cn/)创建企业自建应用，按控制台和对应接口文档申请读取、更新多维表格记录及上传素材的权限，完成应用发布和管理员授权。还需要为应用提供目标多维表格的访问权限。

记录以下标识：

| 标识 | 用途 |
| --- | --- |
| `app_id`、`app_secret` | 获取飞书应用的访问令牌，保存在服务端 |
| `app_token` | 目标多维表格应用的标识 |
| `table_id` | 目标数据表的标识 |
| `record_id` | 需要读取和回写的单条记录 |
| 字段名称或字段 ID | 确认提示词、状态、结果附件等字段 |

多维表格的 `app_token` 用于定位数据，并不等于 HTTP 鉴权用的 `tenant_access_token`。Gkotta API Key 和飞书访问令牌也分别只用于各自的接口。

## 建议的字段设计

下面是你可以创建的业务字段，名称和状态值由自建服务约定：

| 字段 | 类型 | 内容 |
| --- | --- | --- |
| 提示词 | 多行文本 | 图片主体、风格与场景描述 |
| 模型 ID | 文本 | 已确认可用的图片模型 ID，可为空并采用服务端默认值 |
| 任务状态 | 单选 | 待生成、处理中、上传中、已完成、失败 |
| 生成图片 | 附件 | 上传飞书后得到的图片附件 |
| 任务编号 | 文本 | 自建服务的任务标识，用于排错和防止重复处理 |
| 错误信息 | 多行文本 | 可公开给表格用户的错误摘要 |
| 重试次数 | 数字 | 记录本条任务的重试次数 |
| 输入版本 | 文本或数字 | 识别提示词或模型是否已经修改 |

附件字段用于展示图片；普通文本字段中的图片 URL 不会自动变成附件。把任务编号、输入版本和执行阶段同时保存在服务自己的任务库中，有助于服务重启后继续处理。

## 1. 获取飞书访问令牌

自建应用通过 `POST /open-apis/auth/v3/tenant_access_token/internal` 获取 `tenant_access_token`。请求体、应用类型和令牌有效期按[自建应用获取访问令牌](https://open.feishu.cn/document/server-docs/authentication-management/access-token/tenant_access_token_internal)设置。

后续飞书 API 使用 `Authorization: Bearer <tenant_access_token>`。在有效期内复用令牌并在过期前更新；不要为每一条图片记录重复申请，也不要把 `app_secret` 写入表格字段。

## 2. 读取并领取任务

通过[列出记录](https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/list)读取表格：

```http
GET /open-apis/bitable/v1/apps/{app_token}/tables/{table_id}/records
Authorization: Bearer YOUR_TENANT_ACCESS_TOKEN
```

根据实际字段选出“待生成”的记录。列表有分页时，处理响应中的 `has_more` 与 `page_token`，直到取完所需记录。

领取任务时，先在自己的任务库记录 `table_id`、`record_id`、输入版本和执行阶段，再将表格状态改为“处理中”。使用任务库的唯一约束或锁，保证同一条记录的同一输入版本只有一个执行者；表格中的状态字段本身不承担并发锁的作用。

最初可以定时读取待生成记录。需要事件或自动化触发时，再根据已配置的飞书能力把 `record_id` 传给自建服务；服务收到触发后仍需读取和校验原记录。

## 3. 调用 Gkotta 并保存图片

OpenAI Images 协议的最小请求体示例：

```json
{
  "model": "YOUR_IMAGE_MODEL_ID",
  "prompt": "表格中本条记录的提示词",
  "n": 1
}
```

向 `https://www.gkotta.bid/v1/images/generations` 发送该 JSON，并使用服务端保存的 Gkotta 密钥鉴权。使用程序的 JSON 序列化器构建请求，避免提示词中的引号、换行或特殊字符破坏请求体。

生成成功后，将 `data[].b64_json` 解码为图片字节，或从响应 `data[].url` 下载图片。核对图片实际格式、大小和内容后保存到服务管理的临时文件或存储中，再进入“上传中”阶段。URL 可能过期，不能只把它当作永久结果保存。

如果当前 Gkotta 部署开放了异步图片接口，也可以提交任务后持久化返回的 `task_id`，按间隔使用创建时的同一密钥查询，再处理完成结果。异步接口的分组和存储条件见[图片生成与编辑](/image-generate)中的异步图片任务说明。

## 4. 上传飞书素材

调用[上传素材](https://open.feishu.cn/document/server-docs/docs/drive-v1/media/upload_all)接口，将图片文件上传到目标多维表格需要的素材上下文：

```http
POST /open-apis/drive/v1/medias/upload_all
Authorization: Bearer YOUR_TENANT_ACCESS_TOKEN
```

按官方文档或官方 SDK 构造 multipart 上传请求，设置文件名、文件字节、大小及目标父节点相关参数。`parent_type` 和 `parent_node` 必须与多维表格附件的使用场景匹配；它们和可能需要的附加参数应根据官方文档填写，不能套用云盘普通文件或消息图片的上传方式。

上传成功后保存返回的 `file_token`。Gkotta 的图片 URL、图片 Base64、普通消息图片的 `image_key`，都不能替代这里的素材 `file_token`。

检查飞书响应中的业务 `code` 与消息，确认上传成功后再回写记录。HTTP 请求返回 `200` 不一定代表业务操作成功。超出接口支持的大小时，按官方说明处理图片或选择适用的上传流程。

## 5. 回写原记录

使用[更新记录](https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/update)，向原来的 `record_id` 回写结果：

```http
PUT /open-apis/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}
Authorization: Bearer YOUR_TENANT_ACCESS_TOKEN
Content-Type: application/json
```

请求的 `fields` 中更新“生成图片”附件字段，使用官方附件字段格式和刚上传成功的 `file_token`；同时更新“任务状态”为“已完成”，清理旧错误信息，并保留任务编号。

如果附件字段已有文件，先决定本次是替换还是追加。追加时保留现有附件引用并合并本次结果，避免更新字段时意外覆盖旧附件。修改记录前检查输入版本：如果用户已改写提示词，可以将旧结果作为历史结果保存，或按自己的业务规则取消回写。

## 状态、重试与费用

把生成、下载、上传和回写分成可以恢复的阶段：

| 失败阶段 | 建议处理 |
| --- | --- |
| 读取记录或领取任务 | 按飞书限流提示重试；检查表格访问权限和字段类型 |
| Gkotta 认证、权限或模型错误 | 写入错误摘要，修正密钥、分组或模型后再重试 |
| 图片生成超时 | 先确认任务是否已经接收；已有异步任务 ID 时查询原任务，避免重复生成 |
| 图片下载失败 | 重试仍有效的下载地址；若已失效，先保留错误并确认是否需要重新生成 |
| 飞书上传失败 | 已保存图片时只重试上传，不再调用图片生成接口 |
| 飞书记录回写失败 | 已保存 `file_token` 时只重试更新记录，避免重复上传和重复生成 |

对限流和临时服务错误采用有上限的间隔重试；认证或字段错误先修复配置。服务崩溃后根据任务库恢复当前阶段，不能直接把所有“处理中”记录当成待生成重新提交。

图片生成可能产生费用，重复生成也可能再次计费。设置任务重试上限和每日预算，并在[Gkotta 使用记录](https://www.gkotta.bid/usage)核对消耗。表格的错误信息中保留请求时间、任务编号及可公开的错误原因，密钥和访问令牌保存在服务端。

## 验证一条记录

先选择一条短提示词记录，确认以下结果：应用能读取记录、Gkotta 能返回一张有效图片、飞书上传返回素材 `file_token`、原记录附件能打开，状态变为“已完成”。再用同一条记录验证重复触发不会重复生成，并验证上传或回写失败时能从对应阶段重试。

官方接口资料：[访问令牌](https://open.feishu.cn/document/server-docs/authentication-management/access-token/tenant_access_token_internal)、[列出记录](https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/list)、[上传素材](https://open.feishu.cn/document/server-docs/docs/drive-v1/media/upload_all)、[更新记录](https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/update)。Gkotta 的请求与结果保存代码见[图片生成与编辑](/image-generate)。
