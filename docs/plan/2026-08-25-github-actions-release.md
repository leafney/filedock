Related discussion: [docs/discuss/2026-08-25-github-actions-release.md](../discuss/2026-08-25-github-actions-release.md)

# GitHub Actions 多平台二进制自动发布 PRD

状态：已批准

日期：2026-08-25

## Problem Statement

FileDock 当前只能通过开发者本机的 Makefile 构建当前平台或两个 Linux 平台的二进制，仓库没有 GitHub Actions 发布流程。维护者每次创建版本 Tag 后，仍需自行准备前端静态资源、设置 Go 目标平台、注入版本信息、压缩文件、计算校验和、创建 GitHub Release 并上传资产。该过程重复、容易漏步骤，也无法保证四个平台使用相同源码、前端产物、依赖锁和构建时间。

FileDock 的前端会通过 Go embed 进入最终二进制。发布流程不能只运行 Go 交叉编译，还必须先完成 Bun 依赖安装、前端测试和生产构建，再把同一份前端产物交给四个平台。GitHub Actions 的不同 Job 没有共享磁盘，因此还要清楚定义中间 Artifact 的上传、下载、目录位置和保留期限。

版本信息也存在一致性问题。当前 Makefile 的构建时间使用本机默认时区和带空格格式；GitHub Ubuntu Runner 通常使用 UTC。如果直接复用默认 `date` 输出，不同环境会产生时区不明确或数值不同的 BuildTime。发布产物需要统一使用带时区的东八区 RFC 3339 时间，并确保同一批四个平台二进制写入完全相同的值。

本 PRD 会交给低智能编码模型实施。实现不能依赖模型自行推测 Job 顺序、环境变量、产物路径、失败条件或 Release 行为，必须按本文任务和验收条件执行。

## Solution

增加仅由稳定语义版本 Tag 触发的 GitHub Actions Release 流程。Workflow 分为五层：准备发布元数据、执行质量门禁并生成一次前端产物、并行交叉编译四个平台、汇总压缩包并发布正式 GitHub Release、发送不影响发布结果的 Bark 通知。

准备阶段验证 Tag 格式，并生成一次固定为 `Asia/Shanghai` 的 RFC 3339 BuildTime。质量门禁使用 Go module 指定的 Go 版本和固定 Bun `1.3.5`，运行禁用 CGO 的全部 Go 测试、冻结锁文件安装、前端测试与前端生产构建。前端产物上传到 GitHub Actions Artifact，四个平台构建 Job 各自下载到 Go embed 所需目录。

构建矩阵覆盖 `darwin/arm64`、`windows/amd64`、`linux/amd64`、`linux/arm64`。各矩阵在 Ubuntu Runner 上以 `CGO_ENABLED=0` 交叉编译，注入同一版本、Tag、提交 SHA 和构建时间。Windows 生成 `filedock.exe` 并打包为 ZIP；其余平台生成 `filedock` 并打包为 TAR.GZ。每个压缩包根目录只有一个二进制文件。

全部矩阵成功后，Release 阶段下载四个压缩包，计算 SHA-256，创建或更新 Tag 对应的正式 Release，上传四个压缩包和 `checksums.txt`，并让 GitHub 自动生成 Release notes。中间 Artifact 保留 1 天，Release 资产长期保留。Bark 根据完整链路结果发送成功或失败通知，但通知失败不会把成功发布改成失败。

根 Makefile 只同步 BuildTime 的东八区 RFC 3339 生成逻辑，不新增或调整平台发布目标。根 README 增加自动发布说明。

## User Stories

1. 作为项目维护者，我希望推送合法版本 Tag 后自动开始发布，以便不再手工运行多个平台构建命令。
2. 作为项目维护者，我希望只有 `v数字.数字.数字` 格式的稳定版本 Tag 可以发布，以便临时 Tag 不会产生正式 Release。
3. 作为项目维护者，我希望 Tag 可以指向任意分支提交，以便保留当前版本发布习惯。
4. 作为项目维护者，我希望发布流程不提供手动版本输入入口，以便二进制版本始终与 Git Tag 一致。
5. 作为项目维护者，我希望发布前运行全部 Go 测试，以便后端回归不会进入 Release。
6. 作为项目维护者，我希望 Go 测试也禁用 CGO，以便测试环境与正式二进制的关键构建约束一致。
7. 作为项目维护者，我希望发布前运行前端测试，以便前端回归不会被嵌入二进制。
8. 作为项目维护者，我希望发布前完成 TypeScript 与 Vite 生产构建，以便只发布可生成生产静态资源的版本。
9. 作为项目维护者，我希望前端只构建一次，以便四个平台嵌入完全相同的页面并减少重复耗时。
10. 作为项目维护者，我希望依赖安装严格使用 Bun 锁文件，以便 CI 不会静默解析出不同依赖。
11. 作为项目维护者，我希望 CI 固定使用 Bun `1.3.5`，以便与当前开发环境保持一致。
12. 作为 macOS Apple Silicon 用户，我希望下载原生 ARM64 二进制压缩包，以便无需本地安装 Go 和 Bun。
13. 作为 Windows 用户，我希望下载 AMD64 ZIP 压缩包，以便直接解压得到 `.exe` 程序。
14. 作为 Linux x86-64 用户，我希望下载 AMD64 TAR.GZ 压缩包，以便在常见服务器或桌面环境运行。
15. 作为 Linux ARM64 用户，我希望下载 ARM64 TAR.GZ 压缩包，以便在 ARM 服务器或开发板运行。
16. 作为下载用户，我希望每个平台压缩包内只有一个二进制文件，以便解压后无需寻找入口文件。
17. 作为下载用户，我希望压缩包没有多余目录、README 或示例配置，以便资产保持最小且结构一致。
18. 作为下载用户，我希望 Release 不上传裸二进制，以便所有平台资产都使用明确的压缩格式。
19. 作为下载用户，我希望平台资产名包含应用名、完整版本、目标系统和架构，以便下载前即可判断用途。
20. 作为安全意识较强的用户，我希望 Release 提供 SHA-256 校验文件，以便验证下载完整性。
21. 作为问题排查人员，我希望二进制版本信息包含完整 Tag，以便确认正在运行的发布版本。
22. 作为问题排查人员，我希望二进制包含与 Makefile 一致的 Git 短哈希，以便显示紧凑并保持本地与发布构建一致。
23. 作为问题排查人员，我希望 Tag 构建的分支字段明确写入 Tag 名，以便不猜测提交属于哪个分支。
24. 作为问题排查人员，我希望构建时间带有明确 `+08:00` 时区，以便不同地区查看时不会误解。
25. 作为问题排查人员，我希望同次发布的四个平台具有完全相同的 BuildTime，以便确认它们属于同一构建批次。
26. 作为本地开发者，我希望 Makefile 的 BuildTime 与 CI 使用相同格式和时区语义，以便本地构建和发布构建信息一致。
27. 作为项目维护者，我希望任一测试、前端构建或平台构建失败时不创建不完整 Release。
28. 作为项目维护者，我希望 Release notes 由 GitHub 自动生成，以便无需维护额外变更日志脚本。
29. 作为项目维护者，我希望重新运行同一 Tag 的 Workflow 时更新已有 Release 并覆盖同名资产，以便恢复失败发布而不产生重复记录。
30. 作为项目维护者，我希望同一 Tag 的多个发布运行不会同时写入 Release，以便避免资产上传竞态。
31. 作为项目维护者，我希望不同 Tag 可以并行发布，以便不造成不必要的全仓库串行等待。
32. 作为项目维护者，我希望只有 Release 阶段获得内容写权限，以便其他 Job 不持有多余仓库权限。
33. 作为项目维护者，我希望中间 Artifact 只保留 1 天，以便完成 Job 间传递且减少 Actions 存储占用。
34. 作为项目维护者，我希望最终 Release 资产不受中间 Artifact 保留期限影响，以便用户长期下载。
35. 作为项目维护者，我希望完整发布成功或失败后收到 Bark 通知，以便及时知道结果。
36. 作为项目维护者，我希望 Bark 消息包含版本号，以便不打开 Actions 页面也能识别发布版本。
37. 作为项目维护者，我希望 Bark 服务或 Secret 异常不影响已经成功的 Release，以便通知能力不会成为发布阻塞点。
38. 作为后续开发者，我希望 README 记录触发方式、产物、校验和、Bark 配置和 BuildTime 语义，以便维护流程不依赖口头知识。
39. 作为后续编码模型，我希望每个 Job 的输入、输出、依赖、目录和失败行为都有明确说明，以便不自行发明发布架构。
40. 作为代码审查者，我希望此次变更不修改业务代码、前端文案或 API，以便发布基础设施变更保持聚焦。

## Implementation Decisions

### 1. 变更模块与边界

- 新增一条 GitHub Actions Release workflow，作为真实发布产物的唯一自动构建入口。
- 修改根 Makefile 的 BuildTime 默认值生成逻辑。不得修改已有 `build`、`build-lin`、`build-lin-arm`、`web` 等目标的依赖关系、输出文件名或职责；不得新增 macOS、Windows或聚合发布目标。
- 修改根 README，增加面向维护者的自动发布说明。
- 不修改 Go 业务代码、前端源码、翻译词典、配置结构、数据库或 API。
- 本功能不新增运行时用户文案，因此项目的中英文 i18n 词典不需要变化。Workflow 日志与 Bark 运维通知不属于 React 页面或后端 API 用户响应。

### 2. Workflow 触发、Tag 校验与并发

- Workflow 只监听 Tag push，触发模式为 `v*`。不得增加分支 push、pull request、schedule 或手动 dispatch 触发器。
- 触发模式只负责启动 Workflow；准备阶段必须再次严格校验完整 Tag 是否匹配 `^v[0-9]+\.[0-9]+\.[0-9]+$`。
- 合法示例：`v0.1.0`、`v1.2.3`、`v10.20.30`。
- 非法示例：`v1`、`v1.2`、`v1.2.3-beta.1`、`v1.2.3+build.1`、`version-1.2.3`、`test`。
- 非法 Tag 触发后，准备阶段必须输出明确 GitHub Actions error annotation，并以非零状态结束。不得创建 Release 或构建平台资产。
- 不验证 Tag 所指提交是否属于默认分支；任意分支提交都可发布。
- 并发组必须基于完整 Git ref，使相同 Tag 的运行属于同一组，不同 Tag 属于不同组。
- `cancel-in-progress` 必须关闭。相同 Tag 的后续运行等待当前运行完成，不能在 Release 上传过程中取消前一个运行。

### 3. 权限与凭据

- Workflow 全局权限只授予仓库内容读取权限。
- 准备、质量门禁、构建与通知 Job 不得获得仓库内容写权限。
- 只有 Release Job 单独授予仓库内容写权限，用默认 `GITHUB_TOKEN` 创建或更新 Release。
- 不新增个人访问令牌或自定义 Release Token。
- Bark 只读取仓库 Secret `BARK_KEY`。不得把 key 写入环境日志、Release notes、Artifact 或仓库文件。

### 4. Job 拓扑与阻断规则

Workflow 固定采用以下逻辑依赖：

1. 准备 Job 无前置依赖，校验 Tag 并生成共享元数据。
2. 质量门禁 Job 依赖准备 Job，执行 Go/前端验证并上传一次前端产物。
3. 四平台构建矩阵依赖质量门禁 Job，下载同一前端产物并输出四个压缩包。
4. Release Job 依赖完整构建矩阵，只有四个平台全部成功才执行。
5. 通知 Job 依赖以上所有阶段，并使用始终执行条件，因此成功、失败、取消或前置 Job 被跳过时都能尝试通知。

默认失败传播必须保留：准备失败时不执行质量门禁、构建和 Release；质量门禁失败时不执行构建和 Release；任一矩阵失败时不执行 Release。不得使用允许测试或构建失败后继续发布的配置。

构建矩阵关闭快速失败。某个平台失败时，其他已经开始的平台仍运行完毕，以便 Actions 页面完整显示各平台结果；但 Release 仍必须等待并要求四个平台全部成功。

### 5. 准备 Job 输出

准备 Job 使用 Ubuntu 最新 Runner，至少产生以下可供下游读取的 Job Outputs：

- 发布版本：完整 Tag，不去掉 `v`，例如 `v0.2.0`。
- Release 标题：`FileDock` 加一个空格再加完整 Tag，例如 `FileDock v0.2.0`。
- BuildTime：固定东八区、秒精度、RFC 3339 格式，例如 `2026-08-25T16:16:45+08:00`。

BuildTime 的生成必须遵守以下规则：

- 在执行 `date` 的同一个进程环境中设置 `TZ=Asia/Shanghai`；不能依赖 Runner 默认时区。
- 日期时间部分使用年、月、日、字母 `T`、时、分、秒。
- 时区部分必须最终包含冒号，输出 `+08:00`，不能保留常见 `%z` 产生的 `+0800`。
- 为兼容 GNU 与 BSD/macOS `date`，不要依赖只有部分实现支持的 `%:z`。应先取得基本数字偏移，再把末尾的小时和分钟偏移转换为带冒号形式。
- 不得先生成 UTC 文本后再手工增加八小时。
- 只生成一次并通过 Job Output 传递；构建矩阵中不得再次调用 `date` 计算 BuildTime。
- 输出必须匹配 `YYYY-MM-DDTHH:mm:ss+08:00`，不得包含空格、纳秒、时区缩写或换行。

### 6. 质量门禁与前端产物

质量门禁 Job 使用 Ubuntu 最新 Runner，并按以下职责执行：

1. 检出触发 Tag 对应的源码。
2. 根据 Go module 声明安装 Go `1.25.12`，启用 Go 依赖缓存。
3. 安装 Bun `1.3.5`。
4. 在项目根目录以 `CGO_ENABLED=0` 运行全部 Go 测试。
5. 在前端工作目录使用冻结锁文件模式安装依赖。锁文件缺失、与依赖声明不匹配或需要更新时必须失败，不能自动改写锁文件后继续。
6. 在前端工作目录运行现有前端测试脚本。
7. 在前端工作目录运行现有生产构建脚本。该脚本同时完成 TypeScript build mode 检查和 Vite production build。
8. 确认前端输出目录存在且至少有一个构建文件。
9. 把前端输出目录作为一个命名清晰的中间 Artifact 上传，保留 1 天，缺少文件时失败。

质量门禁不需要调用 Makefile。不得为四个平台重复运行 Bun 安装、前端测试或前端构建。

前端 Artifact 下载后必须落到 Go embed 声明的 `static/dist` 目录，使 `static/embed.go` 的 `//go:embed dist` 能嵌入生产页面。不能下载到 `frontend/dist` 后直接运行 Go build，因为 Go 编译只读取 `static/dist`。

### 7. 四平台构建矩阵

构建矩阵固定包含四项，不能增加或遗漏：

| Go 目标系统 | Go 目标架构 | 包内二进制 | 压缩格式 | Release 压缩包示例 |
|---|---|---|---|---|
| `darwin` | `arm64` | `filedock` | `tar.gz` | `filedock_v0.2.0_darwin_arm64.tar.gz` |
| `windows` | `amd64` | `filedock.exe` | `zip` | `filedock_v0.2.0_windows_amd64.zip` |
| `linux` | `amd64` | `filedock` | `tar.gz` | `filedock_v0.2.0_linux_amd64.tar.gz` |
| `linux` | `arm64` | `filedock` | `tar.gz` | `filedock_v0.2.0_linux_arm64.tar.gz` |

每个矩阵 Job 使用 Ubuntu 最新 Runner，并执行以下职责：

1. 以完整 Git 历史检出触发 Tag 对应源码，保证 Git 元数据可用。
2. 根据 Go module 安装 Go 并启用缓存。
3. 下载质量门禁产生的前端 Artifact 到 `static/dist`。
4. 明确设置该矩阵项的 `GOOS`、`GOARCH` 与 `CGO_ENABLED=0`。
5. 创建隔离的临时二进制输出目录和压缩包输出目录。
6. 使用项目根包作为 Go build 入口，不调用 Makefile。
7. 构建后确认期望二进制文件存在且非空。
8. 按矩阵定义压缩，压缩包根目录只包含一个二进制文件。
9. 确认最终压缩包存在且非空。
10. 上传该矩阵唯一压缩包为中间 Artifact，保留 1 天，缺少文件时失败。

Go build 必须加入 `-trimpath`，linker flags 必须保留 `-s -w` 并注入以下四个 `main` 包变量：

- `Version`：准备 Job 的完整版本 Tag。
- `GitBranch`：完整 Tag 名。不要从远程分支列表猜测分支，因为一个提交可能被多个分支包含或没有远程分支包含。
- `GitCommit`：在已检出源码的构建 Job 中执行 `git rev-parse --short HEAD` 得到的 Git 短哈希。不得直接注入完整 `GITHUB_SHA`，也不得硬编码字符串截取长度；该计算方式必须与 Makefile 的 `GIT_COMMIT` 默认值保持一致。
- `BuildTime`：准备 Job 生成的同一个带 `+08:00` BuildTime。

不得启用 UPX，不得安装 UPX，不得执行 macOS notarization、Apple 签名、Windows Authenticode 签名或其他二进制后处理。

### 8. 压缩包结构与命名

- 资产名固定为：应用名、小写下划线连接完整 Tag、Go 目标系统、Go 目标架构，再加平台压缩扩展名。
- 应用名固定小写 `filedock`。
- 完整 Tag 保留 `v`，不得把 `v0.2.0` 转成 `0.2.0`。
- macOS 系统名使用 Go 的 `darwin`，不改写成 `macos`。
- Windows 压缩包扩展名必须为 `.zip`，包内文件必须为 `filedock.exe`。
- macOS 与 Linux 压缩包扩展名必须为 `.tar.gz`，包内文件必须为 `filedock`。
- 压缩包内不得包含上级目录、平台目录、版本目录、README、LICENSE、配置示例、校验和、前端源文件或裸静态目录。
- Release 不得额外上传矩阵中的裸 `filedock` 或 `filedock.exe`。

### 9. 中间 Artifact 传递与保留

- 不同 Job 之间不得假设共享工作目录或本地磁盘。
- 前端输出与四个平台压缩包必须通过 GitHub Actions Artifact 服务传递。
- 每个平台的 Artifact 名必须包含目标系统和架构，避免矩阵上传时互相覆盖。
- 所有中间 Artifact 的 `retention-days` 显式设置为 `1`。
- 上传步骤必须设置找不到文件即失败。
- Release Job 下载四个平台 Artifact 时，应使用平台 Artifact 的统一名称模式并合并到单一 Release 工作目录。
- 中间 Artifact 是 Actions 工作流内部数据，不是用户最终下载入口；其自动删除不得删除或影响 GitHub Release 资产。
- 不得用 Actions Cache 替代 Artifact 传递发布文件。Cache 只用于依赖加速，不适合作为可靠的 Job 输出接口。

### 10. SHA-256 校验文件

- Release Job 在四个压缩包全部下载完成后生成 `checksums.txt`。
- 校验文件只覆盖四个平台压缩包，不包含裸二进制，也不递归包含 `checksums.txt` 自身。
- 输入文件名应按稳定字典顺序排列，使校验文件顺序固定。
- 每行使用常见 `sha256sum` 输出格式：十六进制 SHA-256、分隔空格、压缩包文件名。
- 生成后必须确认 `checksums.txt` 恰好包含四条非空校验记录，且四个期望压缩包各出现一次。
- `checksums.txt` 与四个平台压缩包一起上传到 GitHub Release。

### 11. Release 创建、说明与重复运行

- Release Job 只能在四个平台矩阵全部成功后执行。
- Release 使用触发 Tag 作为 Release tag，不创建新 Tag。
- Release 标题使用准备 Job 输出，例如 `FileDock v0.2.0`。
- `draft` 固定为 false，`prerelease` 固定为 false。
- 开启 GitHub 自动生成 Release notes，不自行拼接 `git log`，不维护独立 release-notes 文件。
- 上传文件列表只能包含四个平台压缩包与 `checksums.txt`。
- 同一 Tag 重新运行时应定位并更新已有 Release，而不是创建第二条 Release。
- 同名资产必须允许覆盖，使失败修复后的重新运行可以替换旧压缩包和校验文件。
- 任何 Release API 或资产上传失败都必须让 Release Job 失败，并进入失败 Bark 通知分支。

### 12. Bark 通知

- 通知 Job 使用始终执行条件，确保前置 Job 失败或被取消时仍能尝试执行。
- 通知使用 `Crownor/bark-action@V3.0`。
- key 只来自 `BARK_KEY` Secret，host 使用 Action 默认值。
- 标题固定为 `GitHub Action for FileDock`。
- 版本从触发 Tag 直接取得并保留 `v`，避免依赖可能失败的准备 Job Output。
- 只有准备、质量门禁、构建矩阵和 Release 四个阶段全部为 success 时，正文为“编译成功 <完整Tag>”。
- 任一阶段为 failure、cancelled、skipped 或其他非 success 状态时，正文为“编译失败 <完整Tag>”。
- 成功和失败通知必须互斥，一次 Workflow 最多尝试发送一条 Bark 消息。
- 两个 Bark 发送步骤都必须允许步骤失败。Bark 缺少 Secret、网络异常或服务端错误只记录日志，不得改变完整发布链路的结果。
- 不得把通知 Job 的成功当成 Release 成功条件。

### 13. Action 与工具版本

实现使用以下已确认版本，不自行换成旧版或其他同类 Action：

- 源码检出：`actions/checkout@v6`。
- Go 安装：`actions/setup-go@v6`，版本读取 Go module。
- Bun 安装：`oven-sh/setup-bun@v2`，明确指定 Bun `1.3.5`。
- Artifact 上传：`actions/upload-artifact@v7`。
- Artifact 下载：`actions/download-artifact@v8`。
- GitHub Release：`softprops/action-gh-release@v3`。
- Bark：`Crownor/bark-action@V3.0`。

本次按用户确认锁定 Action 版本标签，不要求固定完整 Commit SHA。不得引入 GoReleaser、release-please 或其他发布框架。

### 14. Makefile BuildTime

- Makefile 只修改 `BUILD_TIME` 的默认生成表达式，其他变量和目标保持原样。
- 默认值必须在 `Asia/Shanghai` 时区生成当前时刻。
- 输出必须为秒精度 RFC 3339，例如 `2026-08-25T16:16:45+08:00`。
- 实现应使用 GNU/Linux 与 BSD/macOS 都支持的基础 `date` 时区偏移输出，然后把末尾 `+0800` 规范化成 `+08:00`。不要依赖 `%:z`。
- Makefile 仍保留 `BUILD_TIME ?=`，允许调用者显式覆盖 BuildTime。
- 新格式没有空格，可继续安全传给现有 linker flags。
- 不改变 `VERSION`、`GIT_BRANCH`、`GIT_COMMIT` 的本地默认计算逻辑。
- 不新增 `build-mac`、`build-win`、`build-all` 或 Release 目标。

### 15. README 发布说明

README 的构建区域附近增加独立自动发布说明，至少写清：

- 自动发布只由 `v数字.数字.数字` Tag push 触发。
- 创建与推送 Tag 的最小命令示例。
- Tag 可以指向任意分支提交，但维护者应自行确认目标提交正确。
- 发布前会执行 Go 测试、前端测试和生产构建。
- 四个目标平台及各自压缩格式。
- 完整资产命名示例，以及压缩包内只有单个二进制。
- Release 同时提供 `checksums.txt`。
- Release notes 由 GitHub 自动生成。
- Bark 需要仓库 Secret `BARK_KEY`；Bark 失败不影响 Release。
- BuildTime 固定为 `Asia/Shanghai` 的带偏移 RFC 3339 字符串。
- 发布二进制未进行 macOS 或 Windows 代码签名，用户可能看到系统安全提示。

README 是开发/运维文档，不需要复制一份英文版；不得因此修改应用 i18n 词典。

### 16. 实施顺序与停顿点

编码模型必须按以下顺序实施，禁止一次性修改全部文件：

1. 先修改 Makefile BuildTime，验证本机输出格式与时区偏移。完成这一个重大文件变更后停止，向用户展示差异和验证结果，等待确认。
2. 用户确认后新增 Release workflow。先完成静态检查和逻辑审查，展示完整 Job 图、矩阵、权限与关键输出，等待确认。
3. 用户确认后更新 README，展示新增发布说明，等待确认。
4. 用户确认后执行最终综合验证，检查 Git diff、空白错误、配置一致性和现有测试。

如果实施中需要改变本 PRD 已确认的 Tag、产物、时间、Job、权限、通知或 Makefile 范围，必须先更新本 PRD，获得用户批准后才能继续。

### 17. Git 提交要求

- 用户批准 PRD 并完成全部实施与验证前，不得创建实现提交。
- 提交标题必须遵守 Conventional Commits；type 与 scope 使用英文，description 只使用简体中文。
- 建议提交标题：`feat(release): 增加多平台自动发布能力`。
- PRD 路径引用放在提交正文，不混入中文 description，避免违反 description 禁止中英混合的规则。
- 不得把无关工作区改动加入提交。

## Testing Decisions

### 1. 自动门禁

- 后端外部行为门禁使用现有完整 Go 测试集，并明确设置 `CGO_ENABLED=0`。测试成功意味着当前业务代码在正式发布所用 CGO 模式下通过。
- 前端外部行为门禁使用现有 Bun 测试脚本，不新增与发布基础设施无关的组件测试。
- 前端生产构建是必要门禁，用于验证 TypeScript、Vite 和 Go embed 输入能生成，不以单元测试替代。
- 本功能只改基础设施、Makefile 和文档，不新增业务逻辑，因此不新增 Go 或前端测试文件。

### 2. Makefile 验证

- 在未传入覆盖值时，展开后的 BuildTime 必须匹配秒精度 RFC 3339，并以 `+08:00` 结尾。
- 输出不得包含空格，不得以 `Z`、`UTC`、`+0800` 或本机其他时区结尾。
- 显式传入 `BUILD_TIME` 时必须继续覆盖默认值，确认 `?=` 语义未破坏。
- 验证只检查变量和构建信息，不要求新增 Makefile 平台构建。

### 3. Workflow 静态验证

- YAML 必须能被标准解析器读取，不得有缩进、表达式或多行字符串语法错误。
- 检查触发器只有 Tag push `v*`。
- 检查严格 Tag 校验存在且预发布 Tag 会失败。
- 检查 Job 依赖形成准备、质量门禁、矩阵、Release、通知的固定链路。
- 检查 BuildTime 只在准备阶段生成一次，矩阵只读取 Output。
- 检查矩阵恰好四项，系统、架构、扩展名和压缩格式与资产表一致。
- 检查所有 Artifact 显式保留 1 天，缺少文件时失败。
- 检查全局最小权限与 Release Job 独立写权限。
- 检查 Release 只上传四个压缩包与校验文件。
- 检查 Bark 成功/失败条件互斥，且发送步骤允许失败。

### 4. 本地可执行验证

- 运行现有 Go 测试，并使用 `CGO_ENABLED=0`。
- 在前端目录使用 Bun `1.3.5` 和冻结锁文件安装，运行前端测试与生产构建。
- 在本机至少验证当前平台 Go 构建仍可使用修改后的 Makefile BuildTime 注入。
- 不要求本机执行 Windows AMD64 或 Linux AMD64 二进制；它们无法在 macOS ARM64 主机直接运行。
- 可在本机对交叉编译输出使用文件类型检查，但最终四平台真实工作流效果仍需通过推送测试 Tag 或正式 Tag 在 GitHub Actions 验证。

### 5. 首次真实发布验收

实施完成并由用户决定推送合法 Tag 后，手工确认：

1. Workflow 只运行一次且同 Tag 没有并发写入。
2. 准备 Job 输出合法版本与以 `+08:00` 结尾的 BuildTime。
3. Go 测试、前端测试和前端构建全部成功。
4. 四个平台矩阵全部成功。
5. 四个平台构建日志显示复用同一个 BuildTime。
6. Release 为正式状态，不是草稿或预发布。
7. Release 标题与 Tag 一致。
8. Release notes 由 GitHub 自动生成。
9. Release 恰好包含四个平台压缩包和一个 `checksums.txt`。
10. Release 不包含裸二进制或前端 Artifact。
11. 解压每个平台资产后只有 `filedock` 或 `filedock.exe` 一个文件，无二级目录。
12. `checksums.txt` 的四条 SHA-256 可正确验证四个压缩包。
13. 运行可执行平台的二进制版本命令，确认 Version、GitBranch、GitCommit、BuildTime 注入正确。
14. BuildTime 为合法东八区 RFC 3339 字符串，四个平台完全相同。
15. 成功 Bark 正文包含完整 Tag。
16. Actions 中间 Artifact 显示保留 1 天。

### 6. 失败路径验收

- 使用非法但能匹配触发器的测试 Tag 时，准备 Job 必须失败且不创建 Release；是否实际推送该 Tag 由用户决定，实施模型不得擅自创建或推送远程 Tag。
- 模拟测试失败时，构建与 Release 必须被跳过，通知应走失败分支。
- 模拟单一矩阵失败时，其他矩阵可以完成，但 Release 必须被跳过，通知应走失败分支。
- Bark Secret 缺失或 Bark 服务失败时，通知步骤记录失败，但成功 Release 不得因此变红或回滚。
- 重新运行同一 Tag 时，不得创建重复 Release；同名资产应被替换，校验文件应与新资产一致。

## Out of Scope

- 不支持 macOS AMD64、Windows ARM64、Linux 386 或其他目标平台。
- 不支持预发布 Tag，包括 alpha、beta、rc 或带 build metadata 的版本。
- 不支持手动 Workflow dispatch、定时发布、分支 push 发布或 pull request 发布。
- 不要求 Tag 来自默认分支。
- 不引入 GoReleaser、release-please、Docker 镜像发布、包管理器发布或安装脚本。
- 不使用 UPX 或其他可执行文件压缩器。
- 不执行 Apple notarization、macOS 代码签名、Windows Authenticode 签名或证书管理。
- 不在压缩包内添加 README、LICENSE、配置示例、启动脚本、服务文件或目录层级。
- 不上传裸二进制到 Release。
- 不修改应用运行时版本 API、命令行输出格式或前后端业务逻辑。
- 不修改前端或后端 i18n 文案。
- 不给 Makefile 增加新的平台构建或 Release 目标。
- 不创建 GitHub Issue，不调用外部 issue tracker，不应用 triage 标签。
- 不自动创建、删除或推送 Git Tag；真实发布动作由用户完成。
- 不为 Bark 配置 Secret；用户在 GitHub 仓库设置中手动添加 `BARK_KEY`。

## Further Notes

- GitHub Actions Artifact 是不同 Job 之间传递文件的 GitHub 托管临时存储。它与 GitHub Release 资产是两套生命周期；1 天期限只适用于中间 Artifact。
- `Asia/Shanghai` 当前固定为 UTC+8 且无夏令时。使用 IANA 时区名称仍优于直接在 UTC 文本上加八小时，因为转换语义明确且不依赖 Runner 时区。
- RFC 3339 要求数字时区偏移包含冒号。常见 GNU/BSD `date` 的 `%z` 输出为 `+0800`，实现必须规范化为 `+08:00`。
- Go embed 在编译时读取 `static/dist`。如果构建矩阵把 Artifact 下载到其他目录，Go build 可能只嵌入仓库中的占位文件，生成表面成功但网页不可用的二进制；实施时必须重点检查下载目录。
- Release workflow 使用版本标签而不是完整 Action Commit SHA，这是用户明确接受的维护性取舍。
- 当前 PRD 已获用户批准。实施必须按本文阶段顺序完成，并在每个阶段创建独立 Git 提交。

### 2026-08-26：Git Commit 短哈希修订

- 用户明确要求 GitHub Actions 生成的 `GitCommit` 改用短哈希，与 Makefile 保持一致。
- 构建 Job 必须在 checkout 后通过 `git rev-parse --short HEAD` 计算短哈希，并把结果注入 `main.GitCommit`。
- 四个平台必须得到相同短哈希。
- 禁止继续直接注入完整 `GITHUB_SHA`，也禁止硬编码固定截取 7 个字符。
