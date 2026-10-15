# F01 验证

前提：Docker Linux引擎可用；能力迁移应用到临时测试库，使用现管理员登录。没有真实上游账号也能验证本切片。全部测试均在Compose容器内执行。

1. 核心：`docker compose -f deploy/docker-compose.test.yml run --rm backend-test go test -tags=unit ./internal/pkg/mediacapability -count=1`。
2. 管理服务与handler：`docker compose -f deploy/docker-compose.test.yml run --rm backend-test go test -tags=unit ./internal/service ./internal/handler/admin -run MediaCapability -count=1`。
3. 仓储：`docker compose -f deploy/docker-compose.test.yml run --rm --no-deps backend-integration go test -tags=integration ./internal/repository -run MediaCapability -count=1`；仅Testcontainers临时库。
4. 前端：`docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test`。

人工链路：管理员→媒体模型能力→选图片模板→输入公开模型→调整字段/模式→预览合法请求→保存→刷新回读；同名视频另建成功，重复同媒体409；旧revision更新409，普通用户403。非法顶层/metadata、缺schema、prompt按模式、size+resolution映射、n拆分匹配及图层分解拒绝都由行为用例验证。预览过程中检查任务/费用/上游调用为0。

本文件不是测试通过声明；实际结果记录到validation.md。生成主链路在后续切片完成后再做真实模型测试。
