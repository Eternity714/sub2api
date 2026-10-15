# 公共媒体契约（F01定义，F07挂载）

仅配置媒体路由的Composite使用：POST /v1/images/generations、POST /v1/videos/generations，以及各自GET /{task_id}。其它平台入口保持。

创建202只含id/model/status=queued/progress=0/created_at。查询字段id/model/status/progress/created_at/result/error；completed_at仅终态，expires_at仅有成功结果；时间Unix秒。result={type:image|video,data:string[]}，error为字符串；处理中不提前公开部分结果。completed含全部成功、部分成功及全部失败，progress固定0/50/100。无request查询参数、无响应metadata、无result.errors或index。

校验失败HTTP400，返回可定位字段的错误，不创建任务/冻结额度/调用上游。未知模型/错误媒体/无schema为明确4xx；内部存储错误不是400。参数能力匹配但账号不可用由F02/F03仍创建任务，后续completed+error并释放；F01不能提前拒绝为参数错误。

F01不模拟真实202任务、不挂公开路由，也不新增下游动态能力目录。正式公开前需F02—F07贯通资金预留、执行、S3与结算。
