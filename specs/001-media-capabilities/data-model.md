# F01 数据模型

## MediaModelCapability

- id: bigint标识；public_model:非空、最多200字符；media_type:image/video。
- schema_version:1；revision:正整数、每次更新/删除递增；enabled:默认false。
- schema:完整typed schema JSONB；template_id/template_version:可空来源；notes:可空文本。
- created_at/updated_at/deleted_at；软删除。仅deleted_at为空的(public_model,media_type)有唯一约束。
- 创建/更新先编译完整schema；更新和删除用id+当前revision+未删除状态做原子CAS，旧版本409；删除后可重建同模型能力。
- 不存账号、密钥、费用或上游任务。F02维护实际候选；F03持久化已接受快照。

## Immutable CapabilitySnapshot

能力id、revision、schema_version、完整schema与规范JSON摘要、媒体类型和public_model。返回时深复制；停用/更新不修改已有对象。摘要用于检测内容变化，不承载鉴权或计费权限。

## NormalizedRequest

Values:含声明默认值后的参数对象；Mode:唯一匹配模式；ProvidedFields:显式输入路径；ImageSpec:可选像素映射；RequestedCount:图片n，视频默认1。F03应连同能力快照、候选与价格依据一起固定。F01不创建真实任务。

## CandidateMatch

Supported、NeedsSplit、NativeBatchSize和FieldError[]。保留整项数量与请求语义；不保存账号状态。
