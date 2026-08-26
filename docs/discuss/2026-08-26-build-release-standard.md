# Makefile 与 GitHub Actions 构建发布规范讨论

## 原始需求

用户希望整理当前 FileDock 项目中 Makefile 与 GitHub Actions 的实现经验，输出为可供后续类似项目参考的 Markdown 开发规范。规范不应只描述 FileDock，而应支持两类常见交付场景：

1. 通过 GitHub Actions 交叉编译跨平台二进制，并发布到 GitHub Release。
2. 通过 GitHub Actions 构建 `linux/amd64`、`linux/arm64` 多架构 Docker 镜像，并推送到容器 Registry。

规范可能直接交给低智能 AI 模型分析和实施，因此必须明确场景选择、项目参数、文件职责、执行顺序、完整模板、失败行为、验证方法和注意事项，尽量消除需要实施者自行猜测的部分。

## 已分析的现有实现与经验

- FileDock Makefile 已实现本地前端构建、Go 构建、版本信息注入、短 Git Commit、带时区 RFC 3339 BuildTime。
- FileDock GitHub Actions 已实现稳定版本 Tag 校验、共享发布元数据、质量门禁、四平台二进制矩阵、Artifact、压缩包结构校验、SHA-256、Release Commit 列表和 Bark 通知。
- 已处理干净 Runner 中 `go:embed` 只有隐藏占位文件导致编译失败的问题，采用 `//go:embed all:dist`。
- 已处理 Linux CI 中 Zap 对 `/dev/stdout` 执行 `Sync()` 返回 `EINVAL` 的平台差异。
- 已处理 GitHub Actions 使用完整 SHA 而 Makefile 使用短哈希的不一致，统一通过 `git rev-parse --short HEAD` 计算。
- 已处理 GitHub 自动 Release notes 只显示 `Full Changelog` 链接的问题，改为列出两个 Tag 之间的 Commit 主题。

## 讨论问答

### 1. 规范适用范围是什么？

- 初始推荐：面向“Go 单二进制项目，可选内嵌前端”的通用规范。
- 用户补充：还必须覆盖 `linux/amd64`、`linux/arm64` Docker 镜像发布。
- 结论：规范同时覆盖跨平台二进制与多架构 Docker 镜像。

### 2. 两类发布流程如何组织？

- 推荐：同一份规范分为公共基线、二进制发布、Docker 发布三个模块；项目可任选一种或同时启用。
- 推荐：两类发布使用独立 Workflow，避免一种发布失败直接阻断另一种。
- 用户确认。

### 3. Docker 镜像默认发布到哪里？

- 推荐：默认使用 GitHub Container Registry，镜像地址为 `ghcr.io/<owner>/<repo>`。
- 用户确认。

### 4. Docker 镜像 Tag 如何映射？

- 初始推荐：从 `v1.2.3` 生成 `1.2.3`、`1.2`、`1`、`latest`。
- 用户要求：只保留 `1.2.3` 与 `latest`。
- 结论：Git Tag 保留 `v`；镜像版本 Tag 去掉 `v`；不生成主版本或次版本浮动 Tag。

### 5. 同版本镜像是否允许覆盖？

- 初始推荐：版本 Tag 不可覆盖，`latest` 可更新。
- 用户后续修订：一般不主动覆盖版本，但特殊情况下允许覆盖，不需要额外 API 检查强制阻止。
- 结论：规范提示覆盖风险，但不实现版本 Tag 存在性检查。

### 6. Docker 多架构如何构建？

- 推荐：单个 Job 使用 QEMU 与 Docker Buildx，一次构建并推送 `linux/amd64,linux/arm64` Manifest。
- 用户确认。

### 7. Docker 镜像内二进制如何生成？

- 推荐：使用多阶段 Dockerfile，根据 Buildx 目标参数构建，不复制开发机预编译文件。
- 用户确认，并指出 `CGO_ENABLED` 必须按项目实际情况确定。
- 结论：纯 Go 默认可用 `CGO_ENABLED=0`；启用 CGO 的项目必须处理编译器、libc、动态库与目标架构兼容性，禁止机械照抄。

### 8. Docker 是否默认使用非 root 用户？

- 推荐：默认非 root；特殊权限需求必须说明原因。
- 用户确认。

### 9. Docker 是否生成供应链元数据？

- 用户先询问元数据含义。
- 解释：OCI 标签记录源码、版本、Commit、创建时间；Provenance 记录构建来源与参数；SBOM 记录镜像软件包和依赖。
- 结论：OCI 标签必须；Provenance、SBOM 默认启用，兼容性例外需说明。

### 10. 同时启用两类发布时如何通知？

- 推荐：二进制与 Docker Workflow 独立通知，默认支持 Bark，也允许替换为其他渠道；通知失败不得影响发布。
- 用户确认。

### 11. 两类发布是否复用质量门禁？

- 推荐：抽取 `workflow_call` 可复用 Workflow，统一维护 Go、前端及项目特定检查。
- 用户确认。

### 12. 同一 Tag 是否接受重复执行两次质量门禁？

- 说明：两类独立 Workflow 各自调用一次质量门禁，因此测试实际执行两次。
- 推荐：接受，以换取独立重跑、独立失败和低耦合。
- 用户确认。

### 13. Makefile 的职责边界是什么？

- 推荐：负责本地开发、测试、单平台构建和构建信息注入；GitHub Actions 负责正式多平台产物、Release、镜像推送和通知。
- Makefile 不负责创建 Release、推送镜像或通知；Makefile 与 CI 必须统一构建信息语义，但目标集合无需完全相同。
- 用户确认。

### 14. BuildTime 使用什么规则？

- 推荐：格式必须为带时区 RFC 3339；时区作为项目参数，默认 `Asia/Shanghai`。
- 用户补充：Runner 可能处于 UTC、东八区或其他时区，生成值必须显式携带时区，不能依赖环境默认时区。
- 结论：Makefile、二进制 Workflow、Docker Workflow 显式设置项目时区；同类发布的全部架构复用同一个 BuildTime；禁止无时区字符串和手工加减小时。

### 15. 发布 Tag 支持哪些格式？

- 推荐：默认只支持稳定语义版本 `v数字.数字.数字`。
- 二进制版本保留 `v`；Docker 镜像 Tag 去掉 `v`；预发布扩展不得更新 `latest`。
- 用户确认默认不支持 alpha、beta、rc。

### 16. 二进制产物结构是否统一？

- 推荐：Linux、macOS 使用 `.tar.gz`；Windows 使用 `.zip`；包内默认只有一个二进制；不上传裸二进制；发布 `checksums.txt`。
- 目标矩阵必须由项目显式确认，禁止无脑复制 FileDock 平台列表。
- 用户确认。

### 17. 容器镜像是否包含配置文件？

- 推荐：不包含运行配置和密钥；通过环境变量、参数、挂载文件或部署平台 Secret 注入。
- 用户确认。

### 18. Docker 基础镜像如何选择？

- 初始推荐：按 CGO 与运行能力选择 scratch、distroless、Debian 或 Alpine。
- 用户明确：Go 构建一般使用 `golang:1.25.12-bookworm`，运行使用 `debian:bookworm-slim`，不要使用 Alpine。
- 结论：规范默认 Debian Bookworm 系列；Go 版本必须与项目 `go.mod` 一致，`1.25.12` 是示例而不是永久固定值；不使用 Alpine，避免 musl 与 CGO 兼容问题。

### 19. Docker 构建缓存如何处理？

- 推荐：GitHub Actions 使用 `cache-from: type=gha`、`cache-to: type=gha,mode=max`；Dockerfile 使用 Go module、Go build、Bun 安装缓存挂载。
- 用户确认。

### 20. 规范文档输出在哪里？

- 推荐讨论纪要：`docs/discuss/2026-08-26-build-release-standard.md`。
- 推荐规范：`docs/standards/makefile-github-actions-release.md`。
- 本次是开发规范，不是待实施业务功能，不生成 PRD，也不修改现有脚本。
- 用户确认。

### 21. 是否包含可复制模板？

- 推荐：包含 Makefile、可复用质量门禁、二进制 Release、Docker 发布、多阶段 Dockerfile、`.dockerignore`、参数表、验收清单和故障表。
- 模板使用明显占位符，实施前必须全部解析。
- 用户确认。

### 22. 是否增加必配项和易遗漏事项？

- 用户明确要求提示 `BARK_KEY` 等 GitHub Secrets 配置。
- 结论：规范集中列出 Secrets、权限、Package 可见性、Tag 指向、旧 Tag、`go:embed`、stdout Sync、完整 Git 历史、Artifact 生命周期、非 root 权限和 Bark 非阻断等注意事项。

### 23. 是否强制阻止 Docker 版本 Tag 覆盖？

- 初始推荐：发布前查询 GHCR，已存在则失败。
- 用户否决：一般不会覆盖，特殊情况下允许，不需强调或强制。
- 结论：不增加 Registry API 检查，只说明覆盖会改变 Digest。

### 24. 如何覆盖不同前端场景？

- 推荐三类：纯 Go、Go 内嵌前端、前后端独立部署。
- 用户确认前两类最常见；第三类只做附加说明；内嵌前端一般使用 Bun。

### 25. 内嵌前端的 Docker 构建位置？

- 推荐：Dockerfile 使用独立 Bun 阶段，产物复制到 Go builder 的嵌入目录；不在 Go 镜像临时安装 Bun，也不依赖 CI 工作区预生成文件。
- 用户确认。

### 26. Bun 镜像版本是什么？

- 用户先提供 `oven/bun:1.2.22-slim` 作为旧示例，随后明确改为 `oven/bun:1.4.0-slim`。
- 结论：规范默认示例使用 Bun `1.4.0`；Workflow、Dockerfile、开发环境保持一致；禁止浮动 `latest`。

### 27. Docker 内程序版本如何表示？

- 推荐：Git Tag `v1.2.3`，镜像 Tag `1.2.3` 和 `latest`，镜像内二进制 `Version=v1.2.3`、`GitCommit=短哈希`、`GitBranch=v1.2.3`、`BuildTime=带时区 RFC 3339`。
- 用户确认。

### 28. 两类发布是否使用相同 Tag 触发？

- 推荐：都监听 `v*`，内部严格校验稳定版本；两个 Workflow 独立运行和通知。
- 用户确认。

### 29. Docker Workflow 是否创建 GitHub Release？

- 推荐：不创建、不修改。二进制 Workflow 独占 GitHub Release；Docker Workflow 只推送 GHCR。
- 用户确认。

### 30. 是否提供本地 Docker 验证命令？

- 初始推荐：提供单架构 `buildx --load` 示例。
- 用户明确不需要。
- 结论：规范不提供本地 Docker 构建命令，也不要求 Makefile 增加 Docker 目标；仍提供 CI 和发布后验收方法。

### 31. Docker 是否配置健康检查？

- 初始推荐：Dockerfile 可选 `HEALTHCHECK`。
- 用户建议一般提供健康检查，并给出 Docker Compose `curl -fsS http://localhost:8085/health` 示例。
- 用户最终纠正：Dockerfile 不提供 `HEALTHCHECK`，只在 Docker Compose 中配置。
- 结论：运行镜像安装 `curl`；Compose 使用项目实际端口和健康路径；Kubernetes 在部署清单另配探针。

### 32. 是否同时给 Dockerfile 与 Compose 健康检查？

- 初始推荐两者都给。
- 用户纠正：只给 Docker Compose。
- 结论：Dockerfile 明确不写 `HEALTHCHECK`。

### 33. GHCR 默认可见性是什么？

- 推荐：公共仓库建议 Public，私有仓库保持 Private；首次发布后人工检查 Package Settings 与仓库关联；Workflow 不自动修改。
- 用户确认。

### 34. GitHub Actions 版本如何锁定？

- 推荐：普通项目使用明确主版本；禁止 `@main`、`@master`、`@latest`；高安全项目固定完整 SHA 并自动更新。
- 用户确认。

### 35. 是否验证工具和 Action 版本真实存在？

- 推荐：实施前验证 Action、Go、Bun、Builder、Debian 和 Artifact Action 版本，禁止机械复制或编造。
- 用户确认。

### 36. 可复用质量门禁如何适配项目？

- 推荐参数化 Go 版本文件、CGO、前端开关、前端目录、Bun 版本、前端命令和嵌入目录。
- 低智能模型必须先填写项目参数表，再生成 Workflow。
- 用户确认。

### 37. 是否加入决策流程图？

- 推荐：先判断交付物，再判断前端类型，最后填写参数表。
- 用户确认。

### 38. 多行 Shell 是否强制严格模式？

- 推荐：使用 `set -euo pipefail`；变量双引号；临时目录使用 `mktemp -d`；验证文件；使用 `$GITHUB_OUTPUT`；失败输出 `::error::`；不打印 Secret。
- 用户确认。

### 39. GHCR 镜像名如何生成？

- 推荐：默认 `ghcr.io/${{ github.repository_owner }}/<IMAGE_NAME>`，owner 和镜像名转小写；Monorepo 显式配置；Docker context 和 Dockerfile 路径参数化；模板不硬编码组织名。
- 用户确认。

### 40. Docker Workflow 需要哪些权限？

- 推荐：`contents: read`、`packages: write`、`attestations: write`、`id-token: write`；关闭 Provenance 时可移除后两项；使用 `GITHUB_TOKEN` 登录 GHCR。
- 用户确认。

### 41. 是否强制容器漏洞扫描？

- 推荐：SBOM 默认启用；漏洞扫描作为独立可选扩展，先报告，再由项目决定是否按阈值阻断。
- 用户确认。

### 42. 可复用质量门禁是否输出前端 Artifact？

- 推荐：纯 Go不输出；内嵌前端上传产物，保留 1 天；二进制 Workflow 下载后编译；Dockerfile 自己重新构建前端，不依赖 Artifact。
- 用户确认。

### 43. `go:embed` 占位目录如何处理？

- 推荐：提交 `.placeholder`，嵌入使用 `//go:embed all:dist`；真实发布仍验证前端产物，不能把占位文件当成成功构建。
- 用户确认。

### 44. 二进制 Release 说明如何生成？

- 推荐：完整 checkout；查找上一个可达稳定 Tag；逐行列出短哈希和 Commit 主题；首次发布列出可达历史；空范围显示“无新增提交”；不用 GitHub `Full Changelog`。
- 用户确认。

### 45. 发布并发如何隔离？

- 推荐：`<release-type>-${{ github.ref }}`，不取消进行中任务；二进制与 Docker 前缀不同，因此可并行。
- 用户确认。

### 46. 规范要求哪些验收层级？

- 推荐：静态检查、本地检查、CI 构建检查、发布后检查四层，并提供勾选表。
- 用户确认。

### 47. Makefile 是否强调跨平台兼容？

- 推荐：说明 `?=` 覆盖、显式 `TZ`、RFC 3339 时区冒号、Makefile 中 `$$`、短 Commit、linker 引用、macOS Make 兼容和 clean 后恢复占位目录。
- 用户确认。

### 48. 规范条款是否分级？

- 推荐：分为“必须”“建议”“可选”，每个变量写明默认值、调整条件和调整后验证内容。
- 用户确认。

## 最终共识

生成一份面向后续 Go 项目的详细构建发布规范。规范以稳定版本 Tag 为入口，公共基线统一 Version、GitBranch、GitCommit、BuildTime、质量门禁、权限、并发和通知；二进制与 Docker 使用独立 Workflow，可单独启用，也可并行启用。

二进制模块支持项目显式选择的平台矩阵，默认使用 Go 交叉编译、平台压缩包、SHA-256、GitHub Release 与 Tag 间 Commit 列表。Docker 模块默认使用 GHCR、Buildx、QEMU、`linux/amd64` 与 `linux/arm64` Manifest，只发布去掉 `v` 的完整版本 Tag 和 `latest`，并启用 OCI 标签、Provenance、SBOM 和 GitHub Actions 缓存。

Go Docker 默认基于与 `go.mod` 一致的 `golang:<version>-bookworm` 构建，使用 `debian:bookworm-slim` 运行，不使用 Alpine。内嵌前端项目使用固定 Bun `1.4.0` 的独立 Docker 阶段。CGO 策略必须按项目确认；容器默认非 root。Dockerfile 不写健康检查，Docker Compose 使用 `curl` 调用项目真实健康接口。

规范提供纯 Go、内嵌 Bun 前端和独立前端三类路径，重点覆盖前两类。它必须包含决策树、参数表、完整模板、逐步实施顺序、必配 Secrets、权限、注意事项、验收清单和常见故障处理，并用“必须 / 建议 / 可选”区分约束强度。内容按低智能 AI 可直接理解和执行的详细程度编写。
