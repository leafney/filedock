# GitHub Actions 多平台二进制发布讨论

## 原始需求

用户希望为 FileDock 增加 GitHub Actions 自动发布能力。每次推送版本 Tag 时，自动构建并发布以下四种目标平台的二进制文件：

- macOS ARM64。
- Windows AMD64。
- Linux AMD64。
- Linux ARM64。

构建完成后，需要把压缩产物上传到对应 GitHub Release。方案应分析 FileDock 当前项目与 Makefile，并参考另一个工作区中的 `dogit/ai-factory` 项目。讨论和 PRD 必须足够详细，后续会交给低智能编码模型实施。

## 已检查的项目现状

- FileDock 是 Go 单体程序，Go module 为 `github.com/leafney/filedock`，Go 版本为 `1.25.12`。
- 前端使用 React、TypeScript、Vite 与 Bun。当前开发机 Bun 版本为 `1.3.5`。
- 前端通过 `static/embed.go` 嵌入 `static/dist`，因此发布二进制前必须先生成前端产物。
- 根 Makefile 的 `web` 目标负责构建前端并同步到 `static/dist`。
- 根 Makefile 已有当前平台、Linux AMD64、Linux ARM64 开发构建目标，没有完整四平台发布目标。
- Makefile 已通过 Go linker flags 注入 `main.Version`、`main.GitBranch`、`main.GitCommit`、`main.BuildTime`。
- 所有目标二进制都可以使用 `CGO_ENABLED=0` 交叉编译。
- 仓库当前没有 GitHub Actions workflow。
- 当前已有 Tag `v0.1.0`、`v0.1.1`；`v0.1.1` 位于功能分支提交。
- 参考项目使用 `test -> build matrix -> release -> notify` 结构，包含 Tag 校验、四平台矩阵、Artifact 汇总、SHA-256 校验、GitHub Release 与 Bark 通知。

## 讨论问答

### 1. 哪些 Tag 触发发布？

- 推荐：仅 `v*` Tag 触发，避免其他临时 Tag 意外创建 Release。
- 用户确认：仅 `v*` 格式。

### 2. Release 创建后采用什么状态？

- 推荐：所有门禁和四平台构建成功后自动创建正式 Release；任一环节失败时不创建不完整 Release。
- 用户确认：自动正式发布。

### 3. Release 产物如何打包？

- 推荐：macOS、Linux 使用 `.tar.gz`，Windows 使用 `.zip`，同时发布 SHA-256 校验文件。
- 用户确认，并补充：每个压缩包内只放编译后的单个二进制文件，不包含 README、示例配置或其他文件。

### 4. Release 说明如何生成？

- 推荐：使用 GitHub 自动生成的 Release notes，汇总上一个 Tag 到当前 Tag 的变更。
- 用户确认。

### 5. 四个平台最初考虑怎样编译？

- 初始推荐：在单个 Ubuntu Job 中完成全部交叉编译，减少重复依赖安装。
- 用户随后提供 `dogit/ai-factory` 参考项目，要求先参考其现有 GitHub Actions 实现。
- 检查参考项目后更新推荐：前端只构建一次，再使用四个平台矩阵并行交叉编译；各矩阵 Job 通过 Artifact 获取前端产物。
- 用户确认更新后的并行矩阵方案。

### 6. 是否使用 UPX 压缩二进制？

- 参考项目对 Linux、Windows 使用 UPX，对 macOS 跳过。
- 推荐：FileDock 不使用 UPX。已有 linker 瘦身和压缩包，UPX 会增加 Windows 杀毒误报概率和发布失败点。
- 用户确认不使用 UPX。

### 7. 版本号、Release 标题与产物怎样命名？

- 推荐：完整保留 Tag 的 `v` 前缀。以 `v0.2.0` 为例，二进制版本为 `v0.2.0`，Release 标题为 `FileDock v0.2.0`，产物名包含 `filedock_v0.2.0`、Go 目标系统和架构。
- 用户确认，并强调 Release 中的平台产物必须全部是压缩文件，不上传裸二进制。
- 确认的示例资产：
  - `filedock_v0.2.0_darwin_arm64.tar.gz`
  - `filedock_v0.2.0_windows_amd64.zip`
  - `filedock_v0.2.0_linux_amd64.tar.gz`
  - `filedock_v0.2.0_linux_arm64.tar.gz`
  - `checksums.txt`

### 8. 是否严格校验 Tag？

- 推荐：触发器使用 `v*`，Workflow 内再严格校验 `^v[0-9]+\.[0-9]+\.[0-9]+$`。`v0.2.0` 合法；`v-test`、`v1`、`v0.2.0-beta.1` 不合法。
- 用户确认：当前只支持稳定语义版本。

### 9. 发布前执行哪些门禁？

- 推荐：运行全部 Go 测试、前端 Bun 测试、TypeScript/Vite 前端生产构建。任一失败时停止编译和发布。
- 用户确认。

### 10. 是否同步补齐 Makefile 的四平台构建目标？

- 初始推荐：补充 macOS、Windows 和四平台聚合构建目标，使本地与 CI 构建逻辑一致。
- 用户拒绝：Makefile 仅用于开发验证，不用于真实部署文件生成，暂不改变其平台构建目标。
- 最终结论：发布脚本独立实现交叉编译；Makefile 不新增或整理发布目标。

### 11. 是否支持手动触发 Workflow？

- 推荐：不支持手动触发，防止手动输入版本与 Git Tag 不一致。
- 用户确认：仅由合法 Tag 推送触发。

### 12. 是否增加外部发布结果通知？

- 初始推荐：不增加，避免引入 Secret 和额外失败点。
- 用户要求：增加 Bark 通知，并会手动配置 Secret。

### 13. Bark 如何配置？

- 推荐：沿用参考项目，使用 `Crownor/bark-action`，从 `BARK_KEY` Secret 获取 key，使用默认 Bark 服务地址。
- 推荐标题：`GitHub Action for FileDock`。
- 推荐正文：成功为“编译成功 v0.2.0”，失败为“编译失败 v0.2.0”。
- 成功必须表示测试、前端构建、四平台构建和 Release 发布全部成功。
- 用户全部确认。

### 14. Bark 通知失败是否令发布流程失败？

- 推荐：不影响发布结果。通知步骤允许失败，错误只保留在 Actions 日志中。
- 用户确认。

### 15. 同一 Tag 重新运行时怎样处理已有 Release？

- 推荐：幂等更新已有 Release，覆盖同名平台资产与 `checksums.txt`，不创建重复 Release，也不因资产已存在而失败。
- 用户确认。

### 16. 二进制写入哪些构建信息？

- 推荐：
  - `Version` 写完整 Tag，例如 `v0.2.0`。
  - `GitCommit` 写完整 40 位提交 SHA。
  - `GitBranch` 在 Tag 构建中写 Tag 名，避免猜测提交所属分支。
  - Go 构建增加 `-trimpath`，linker flags 保留 `-s -w`。
- 用户确认这些字段，随后对 `BuildTime` 提出新的格式和时区要求。

### 17. `BuildTime` 使用什么时区和格式？

- 用户最初希望使用东八区的 `2026-08-25 16:16:45`，并要求 Makefile 与 GitHub Actions 同步修改。
- 讨论确认：单独执行 `date '+%Y-%m-%d %H:%M:%S'` 会使用 Runner 当前时区；GitHub Ubuntu Runner 通常为 UTC，因此不能单独使用。
- 讨论确认：`TZ=Asia/Shanghai` 会让 `date` 先把当前时刻转换到上海时区，再格式化，时间值不会因为 Runner 使用 UTC 而错误。
- 用户最终选择携带时区且无空格的 RFC 3339 格式，例如 `2026-08-25T16:16:45+08:00`。
- 最终结论：Makefile 与 GitHub Actions 都固定使用 `Asia/Shanghai`，输出秒精度 RFC 3339 字符串。不能只输出无时区字符串，也不能手工对 UTC 字符串加八小时。
- 兼容性结论：常见 `date` 的 `%z` 可能生成 `+0800`，必须转换为 `+08:00`；不要依赖并非所有 GNU/BSD `date` 都支持的 `%:z`。

### 18. 四个平台是否共享同一个构建时间？

- 推荐：由前置准备 Job 生成一次，再通过 Job Output 传给四个平台，避免矩阵启动时间差造成二进制信息不一致。
- 用户确认。

### 19. GitHub Actions 使用哪个 Bun 版本？

- 代码检查结果：当前开发机 Bun 为 `1.3.5`，仓库存在 `frontend/bun.lock`。
- 推荐：固定 Bun `1.3.5`，使用冻结锁文件安装，依赖或锁文件不一致时立即失败。
- 用户确认。

### 20. Workflow 权限如何设置？

- 推荐：全局只给 `contents: read`；只有 Release Job 给 `contents: write`；使用默认 `GITHUB_TOKEN`，不配置额外 Release Token。
- 用户确认。

### 21. 同一 Tag 多次运行时是否限制并发？

- 推荐：按 Git ref 设置并发组，同一 Tag 的后续运行排队，不取消正在上传 Release 的运行；不同 Tag 可并行。
- 用户确认。

### 22. 第三方 Action 使用什么版本？

- 推荐并确认：
  - `actions/checkout@v6`
  - `actions/setup-go@v6`
  - `actions/upload-artifact@v7`
  - `actions/download-artifact@v8`
  - `oven-sh/setup-bun@v2`
  - `softprops/action-gh-release@v3`
  - `Crownor/bark-action@V3.0`
- 当前选择可读版本标签，不固定到完整 Commit SHA。

### 23. 为什么需要中间 Artifact，保留多久？

- 用户询问为什么生成文件需要保留，以及 GitHub Actions 是否提供公共存储空间。
- 解释：每个矩阵 Job 运行在独立 Runner，Job 之间不共享本地磁盘；必须通过 GitHub Actions Artifact 服务上传、下载中间产物。无需自行提供对象存储、服务器或额外 Secret。
- 中间 Artifact 只负责把前端产物传给构建 Job、把四个平台压缩包传给 Release Job；它不同于长期保留的 Release 资产。
- 推荐：中间 Artifact 保留 1 天，减少 Actions 存储占用；Release 资产不受该期限影响。
- 用户确认保留 1 天。

### 24. 是否为 macOS、Windows 二进制代码签名？

- 推荐：暂不签名。签名需要 Apple Developer 与 Windows 代码签名证书及额外 Secret；未签名产物可能触发 macOS 开发者验证提示或 Windows SmartScreen 提示。
- 用户确认不需要签名。

### 25. Tag 是否必须指向默认分支提交？

- 推荐：不限制分支。现有 `v0.1.1` 已位于功能分支提交，强制默认分支会改变当前发布习惯。
- 用户确认：合法 Tag 可以指向任意分支提交。

### 26. Go 测试是否使用 `CGO_ENABLED=0`？

- 推荐：使用。正式二进制全部禁用 CGO，测试应使用相同模式，提前发现依赖误用 CGO。
- 用户确认。

### 27. 是否更新 README 发布说明？

- 推荐：更新，记录合法 Tag、自动发布行为、目标平台、资产命名、校验文件、Bark Secret 与构建时间语义。
- 用户确认。

### 28. PRD 需要达到什么详细程度？

- 用户强调：PRD 将交给低智能模型编码，必须补充充分细节，避免模型自行猜测实现意图。
- 最终结论：PRD 必须明确 Job 依赖、变量来源、命令职责、目录流转、产物清单、时间转换、权限、失败行为、禁止事项、实施顺序和逐项验收标准。

## 最终共识

新增一条仅由稳定语义版本 Tag 触发的 GitHub Actions 发布流程。流程使用前置准备、质量门禁、四平台构建矩阵、Release 发布和 Bark 通知五个阶段。前端只构建一次，并通过 GitHub Actions Artifact 传给四个独立构建 Job；所有矩阵产物再通过 Artifact 汇总到 Release Job。中间 Artifact 保留 1 天，最终 Release 资产长期保留。

四个平台均在 Ubuntu Runner 上使用 `CGO_ENABLED=0` 交叉编译。构建不使用 UPX，不执行代码签名。Go linker flags 写入完整 Tag、Tag 名、完整提交 SHA 和同一份东八区 RFC 3339 构建时间。BuildTime 示例为 `2026-08-25T16:16:45+08:00`，由前置 Job 生成一次；Makefile 的开发构建时间同步采用相同语义和格式，但不调整现有 Makefile 平台构建目标。

Release 只上传四个压缩包与 `checksums.txt`，不上传裸二进制。Linux 与 macOS 使用 `.tar.gz`，Windows 使用 `.zip`；每个包内只有平台二进制且无二级目录。全部测试、前端构建和四平台构建成功后才发布正式 Release。Release notes 由 GitHub 自动生成，重复运行覆盖同名资产。

Bark 使用 `BARK_KEY` Secret。只有完整发布链路成功才发送成功通知，其他状态发送失败通知；Bark 自身失败不改变 Release 结果。Workflow 使用最小权限、固定 Action 大版本、固定 Bun `1.3.5`、冻结锁文件和同 Tag 串行策略。README 同步记录发布方法和约束。

## 实施后需求变更

### 2026-08-26：Git Commit 改用短哈希

- 用户发现 GitHub Actions 发布的二进制写入了完整 40 位 Commit ID，与 Makefile 的本地构建结果不一致。
- 最终决定：GitHub Actions 不再直接使用完整 `GITHUB_SHA`，而是在已检出源码的构建 Job 中执行 `git rev-parse --short HEAD`。
- 四个平台都从同一 Tag 提交计算短哈希，因此得到相同的 `GitCommit`。
- 该方式与 Makefile 的现有 `GIT_COMMIT` 默认计算方式完全一致，不硬编码固定截取长度，并遵循 Git 自身的短哈希规则。
