# Task 2 报告：兼容迁移与 v3 任务领取边界

日期：2026-09-28

## 已完成

- 新增 `245_grsai_video_tasks.sql`，为旧图片记录提供 `image`、`0` 和空分辨率默认值。
- Ent schema 与生成代码新增 `media_kind`、`video_duration_seconds`、`video_resolution`。
- 创建任务时支持图片 v2 与视频 v3；视频强制 duration 大于 0，并将 `requested_image_count` 固定为 1。
- 新 worker 的 v2 仓储操作支持 v2/v3 共用领取、租约、容量锁、状态转换及 owner 查询；查询仍限定 `user_id + api_key_id`。
- 旧 v1 记录不会进入新 v2/v3 worker 查询。

## 验证

- `go generate ./ent`：通过。
- `go test ./internal/repository ./internal/service -run 'TestGrsai' -count=1`：通过（当前未启用 integration tag）。
- `go test ./internal/repository ./internal/service -run TestDoesNotExist -count=0`：通过编译。

PostgreSQL integration test 需要本地 integration 数据库环境，未在本次离线验证中宣称通过。
