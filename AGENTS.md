# AGENTS.md

提交日志需要使用中文。

## 本机测试

本机测试必须通过 Docker Compose 容器执行，不得在宿主机直接运行测试命令
（包括 `go test`、前端类型检查和前端测试）。
后端单元测试使用 `docker compose -f deploy/docker-compose.test.yml run --rm backend-test`。
前端检查与关键回归使用 `docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test`。
后端集成测试使用 `docker compose -f deploy/docker-compose.test.yml run --rm --no-deps backend-integration`，
该服务仅通过 Testcontainers 创建临时测试数据库。

## 生产灰度发布

生产环境通过 SSH 别名 `tenxunyun.guigu` 管理，Sub2API 部署目录为
`/opt/sub2api`。灰度发布只允许使用服务器中的灰度脚本和不可变 GHCR
镜像标签（`sha-<commit>`）；不得使用 `latest`。

灰度已初始化完成。不得再次执行 `gray-bootstrap.sh`，也不得启动旧的
`sub2api` 容器；每次操作前先运行 `gray-status.sh`，以其中记录的稳定和候选
槽位为准。正常操作顺序为：

1. `gray-deploy.sh <候选槽位> sha-<commit>`：部署到 0% 流量候选槽位；
2. 完成健康检查和人工验证后，用 `gray-set-traffic.sh <0..100>` 逐步扩大；
3. 候选达到 100% 并经过观察期后，用 `gray-promote.sh` 确认晋升；
4. 用 `gray-status.sh` 核对槽位、镜像和健康状态。

- 不得执行 `podman compose down`、删除数据卷、清理正在运行的 blue/green
  槽位，或在灰度回滚时恢复数据库备份。
- 先向无流量的候选槽位部署，确认容器健康、`/health`、关键人工测试与
  Nginx 配置检查均通过，才可以扩大流量。
- 流量切换必须通过灰度脚本完成；每次变更后检查 Nginx 配置、容器健康、
  最近错误日志与槽位标记。
- 如果发现候选槽位不健康、Nginx 校验失败、5xx/超时显著升高、鉴权或额度
  异常、账务/数据一致性风险，必须先立即将新请求切回稳定槽位，再保留候选
  容器、日志和备份现场，并向用户报告问题与已采取的回滚动作。
- 自动回滚仅切换流量，不销毁候选容器，不删除镜像，不恢复数据库；后续处理
  必须等待用户明确指示。

### 密码重置 Token 格式升级

从明文 Token 旧版首次升级到哈希存储版时，先在 0% 候选槽位通过管理员设置
接口启用 `password_reset_token_legacy_compat`，确认旧、新槽位的链接均可验证，
再逐步放量。晋升后确认旧槽位的在途邮件已处理完，再关闭兼容开关，恢复仅
保存哈希；新版本仍可消费有效期内的旧链接。该开关默认关闭，后续同格式版本
之间的发布无需启用。设置变更前后核对其它功能开关，避免误改生产配置。
