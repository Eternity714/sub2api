# GitLab 镜像构建

主仓库为 <https://gitlab.gkotta.cn/yeshunfa/sub2api>，默认分支为 `main`。
本地 `origin` 指向 GitLab，`github` 保留原 GitHub 仓库，`upstream` 保留上游仓库。

`.gitlab-ci.yml` 使用标签为 `linux`、`podman` 的共享 Runner（Docker executor）。
分支和合并请求运行后端单元/集成测试、golangci-lint、govulncheck、
`make test-frontend` 中的静态检查与关键回归测试，以及发布脚本检查、生产依赖审计。
审计沿用 `.github/audit-exceptions.yml` 中有期限的例外，审计服务异常会阻止构建。

受保护的默认分支在检查通过后构建 Linux amd64 镜像并推送至
`ghcr.io/eternity714/sub2api:sha-<完整 commit SHA>`，不生成 `latest` 或 `main` 标签。
构建使用 Go 1.27.4，避开 Go 1.27.3 的 GO-2026-6859。
后端测试使用含编译工具的 Debian Go 镜像；golangci-lint 明确选择 Go 1.27.4 工具链。
GitLab 任务与镜像构建通过 `APK_MIRROR` 使用阿里云 Alpine 镜像源；
可在 CI 变量中调整。直接使用 Dockerfile 时该参数默认空，沿用原软件源。
已存在的 SHA 标签不会被覆盖；仅在拉取后 revision、平台一致时复用。
发布任务的 `image.env` 和 `image-metadata.json`
记录镜像地址与 digest。

在 GitLab 项目 CI/CD Variables 中配置以下变量：

| 变量 | 配置 |
| --- | --- |
| `GHCR_USERNAME` | 有镜像发布权限的 GitHub 用户名；Protected |
| `GHCR_TOKEN` | 对该镜像有 `write:packages` 权限的令牌；Masked、Protected，关闭变量展开 |

`main` 必须保持受保护。发布任务需要 Runner 允许 Docker-in-Docker；
Docker daemon 只在任务内部网络提供服务，不向宿主机发布端口。
DinD 与 BuildKit 均通过 `docker.m.daocloud.io` 拉取 Docker Hub 镜像，
适配 Runner 直连 Docker Hub 超时的网络环境；GHCR 发布地址保持不变。
镜像导出/推送前的暂时性下载错误最多重试三次，并保留同一构建器的缓存；
一旦进入发布阶段便不自动重建，以免改写已经发布的 SHA 标签。
流水线不持有生产 SSH 凭据，也不自动部署、切流或晋升。

镜像就绪后的部署继续遵循 `AGENTS.md`：先在服务器运行 `gray-status.sh`，
再通过现有 `gray-deploy.sh <候选槽位> sha-<commit>` 部署到 0% 候选槽位。
放量和晋升通过既有灰度脚本单独执行。

需要先验收 0% 候选时，可在受保护 `main` 上手动运行流水线（网页或 API），
设置 `CANDIDATE_ONLY=true` 和 `VALIDATED_SOURCE_SHA=<已验收提交的完整 SHA>`。
该模式单独构建候选镜像；脚本要求该提交为当前提交的祖先，且两者只能存在
上述 GitLab CI 配置、构建脚本、BuildKit 配置和本文档的差异，应用及 Dockerfile
必须完全相同。普通推送和普通手动流水线仍运行所有检查。
候选模式用于服务器 0% 验收；必须在完整流水线与人工验收均通过后才允许放量或晋升。
