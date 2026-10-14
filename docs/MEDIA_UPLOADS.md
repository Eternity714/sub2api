# 文件上传接口

Sub2API 提供与 ToAPIs 文件上传接口一致的三个入口，将客户端文件保存到管理员配置的 S3 兼容对象存储，并返回可用于后续生成请求的 URL。

## 接口与格式

| 用途 | 接口 | 支持格式 | 默认单文件上限 |
| --- | --- | --- | --- |
| 图片 | `POST /v1/uploads/images` | JPEG、PNG、WebP、GIF | 10 MiB |
| 视频 | `POST /v1/uploads/videos` | MP4、WebM、MOV | 50 MiB |
| 音频 | `POST /v1/uploads/audios` | MP3、WAV、M4A | 50 MiB |

请求使用 `multipart/form-data`，每次上传一个文件：

| 表单字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | 文件内容；类型须与接口相符 |
| `purpose` | 否 | 兼容字段，默认 `generation`；不影响存储位置，也不写入数据库 |

使用 `Authorization: Bearer <API Key>` 鉴权。上传沿用网关的 API Key 准入检查，包括用户状态、Key 有效期、IP 限制、Key 配额和账号余额或订阅状态。上传本身不扣除余额，也不创建模型消费记录。

服务器根据文件内容核对类型，修改扩展名或声明的 `Content-Type` 不能使不支持的文件通过校验。单文件上限按文件实际字节数计算，不包含 multipart 的边界和表单开销。

```bash
curl https://api.example.com/v1/uploads/images \
  -H 'Authorization: Bearer sk-...' \
  -F 'file=@./reference.png' \
  -F 'purpose=generation'

curl https://api.example.com/v1/uploads/videos \
  -H 'Authorization: Bearer sk-...' \
  -F 'file=@./reference.mp4'

curl https://api.example.com/v1/uploads/audios \
  -H 'Authorization: Bearer sk-...' \
  -F 'file=@./reference.mp3'
```

请让 HTTP 客户端自动生成 multipart 的 `Content-Type` 与 boundary；不要手工设置缺少 boundary 的请求头。

成功响应使用 `200 OK`：

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": "f6c8fae4-25d2-4a36-8197-8a93c784c718",
    "url": "https://media.example.com/uploads/images/.../file.png",
    "mime_type": "image/png",
    "size": 12345
  }
}
```

`id` 是标识本次上传的 UUID，`size` 是文件字节数，`mime_type` 是服务器确认的媒体类型。后续请求使用 `data.url` 作为文件地址。

## 独立对象存储配置

在 **管理员 → 数据库备份** 页的 **图片上传对象存储**、**视频上传对象存储**、**音频上传对象存储** 三张卡片中分别设置。三类配置互相独立，均默认关闭；开启前需配置对应的对象存储。

每类均可设置：

- 启用开关与「复用上方备份 S3 配置」开关；复用默认关闭。
- S3 endpoint、region、Access Key ID、Secret Access Key 和 path-style 选项。
- bucket、对象前缀、公开访问域名、预签名有效期和最大上传大小。

预签名有效期默认 24 小时，管理界面可设置为 1–168 小时。

默认对象前缀分别为 `uploads/images/`、`uploads/videos/`、`uploads/audios/`，对象键为「前缀 + 用户 ID + UUID 文件名」。管理界面以 1–1024 MiB 的整数分别配置文件大小；管理接口使用 `max_upload_bytes`，单位为字节，支持不超过 1 GiB 的正数，`0` 表示使用该类默认值（1 MiB = 1,048,576 字节）。

只有显式打开某类的 **复用上方备份 S3 配置**，该类才使用备份配置的 endpoint、region、凭证和 path-style 选项。该类仍使用自己的前缀、访问域名、有效期和上传大小限制；bucket 留空时使用备份 bucket，也可填写独立 bucket。关闭复用时必须提供独立 S3 配置。

保存后配置在后续请求中生效，无需重启。保存沿用管理端对象存储配置的 step-up 验证流程。修改配置不会搬迁此前已经上传的对象。

管理接口如下，`kind` 为 `images`、`videos` 或 `audios`：

```text
GET  /api/v1/admin/backups/media-upload-storage/{kind}
PUT  /api/v1/admin/backups/media-upload-storage/{kind}
POST /api/v1/admin/backups/media-upload-storage/{kind}/test
```

GET 返回 `config` 和 `secret_configured`，不返回已保存的 Secret Access Key；`secret_configured` 表示该类独立密钥是否已保存。PUT 和连接测试提交完整配置，Secret Access Key 留空可沿用该类已保存的密钥。复用开关只改变当前使用的配置，已保存的独立配置保留，可随时关闭复用并切回独立存储。连接测试通过 `HeadBucket` 检查 bucket 访问，不保存表单；上传还需要写入对象的权限，预签名访问需要读取对象的权限。

示例配置：

```json
{
  "enabled": true,
  "reuse_backup_s3": false,
  "endpoint": "https://s3.example.com",
  "region": "us-east-1",
  "bucket": "image-uploads",
  "access_key_id": "...",
  "secret_access_key": "...",
  "force_path_style": false,
  "prefix": "uploads/images/",
  "public_base_url": "https://media.example.com",
  "presign_expiry_hours": 24,
  "max_upload_bytes": 10485760
}
```

## URL 与错误处理

设置 `public_base_url` 时，接口返回「公开访问域名 + 对象键」组成的 URL。管理员需保证该域名能够读取 bucket 中的对象；填写域名不会自动修改 bucket 的公开访问权限。

`public_base_url` 留空时，接口返回 S3 预签名 URL，其有效期由该类的 `presign_expiry_hours` 控制。URL 到期不代表对象已删除；需要长期可用的地址时，应配置公开访问域名或由业务自行管理访问链接。当前上传接口不提供上传记录查询、链接刷新或删除接口，对象保留与清理可通过 bucket 的生命周期规则管理。

| HTTP 状态 | 典型原因 |
| --- | --- |
| `400` | multipart 格式错误、缺少文件、文件为空、媒体类型不支持或超过上传限制 |
| `401` / `403` / `429` | API Key 准入检查失败 |
| `503` | 对应媒体上传功能未启用或对象存储配置不可用 |
| `500` | 对象存储上传失败 |

上传处理阶段的失败响应为 `{"success":false,"message":"..."}`；API Key 鉴权失败沿用网关的错误格式。

若请求尚未到达 Sub2API 就收到 `413`，应核对反向代理或 CDN 的请求体限制，例如 Nginx 的 `client_max_body_size`；它也需要容纳文件上限以及 multipart 开销。
