# F01 管理API

实际前缀`/api/v1/admin/media-capabilities`；沿现AdminAuth、审计、合规链。标准管理envelope由现API client解包。普通用户无权限。

- GET 基础路径：page/page_size/search/media_type；返回{items,total,page,page_size}。
- POST 基础路径：{public_model,media_type,schema,enabled:false,template_id?,template_version?,notes?}；创建成功返回记录。
- GET /templates：返回{items:模板数组}，每项{id,version,name,media_type,schema}。复制后配置独立。
- POST /validate：{public_model,media_type,schema,request_json:string}；只编译/校验，返回{valid,errors:FieldError[],normalized?}。合法HTTP请求中的参数校验失败返回valid=false；管理body结构无效400。不生成任务、不冻结额度、不发上游。
- GET /{id}：返回记录。
- PUT /{id}：同创建可写字段且revision必填；原子CAS。
- DELETE /{id}?revision={revision}：软删除并校验版本。

记录：{id,public_model,media_type,schema_version,revision,enabled,schema,template_id?,template_version?,notes?,created_at,updated_at}。

错误：非法schema/字段400，未授权按现认证链，非管理员403，未知记录404，重复或旧revision409，存储故障5xx。字段错误写入metadata.fields数组，不能把内部错误当不支持的参数。

前端基础字段可编辑，复杂模式/嵌套支持高级JSON；无论前端是否通过，服务端独立验证。保存失败保留草稿，显示字段错误；不静默重试旧版本覆盖。
