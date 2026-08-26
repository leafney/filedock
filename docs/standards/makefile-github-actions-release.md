# Go 项目 Makefile 与 GitHub Actions 构建发布开发规范

适用范围：Go 单二进制项目、Go 内嵌 Bun 前端项目，以及前后端独立部署项目的后端部分。

交付类型：跨平台二进制 GitHub Release、`linux/amd64` 与 `linux/arm64` 多架构 GHCR 镜像。项目可以只启用一种，也可以同时启用两种。

相关讨论：[docs/discuss/2026-08-26-build-release-standard.md](../discuss/2026-08-26-build-release-standard.md)

## 1. 规范目标

本规范用于指导开发者或 AI 编码模型为新的 Go 项目建立一致、可追踪、可验证的本地构建与自动发布能力。实施结果必须满足以下目标：

1. 本地 Makefile 与 CI 对 Version、GitBranch、GitCommit、BuildTime 使用相同语义。
2. 正式发布只由合法稳定版本 Tag 触发，不依赖手工输入版本号。
3. 测试失败、前端构建失败、任一目标架构构建失败时，不发布不完整产物。
4. 二进制 Release 和 Docker 镜像发布互相独立，可以单独启用、单独失败、单独重跑。
5. 所有产物都能追溯到源码 Tag、短 Commit 和带时区构建时间。
6. 模板参数明确，不把某个项目的名称、路径、端口和 CGO 策略误带到另一个项目。
7. 发布后有明确验收方法，失败时有明确定位路径。

## 2. 约束等级

本文使用三个等级：

- **必须**：正确性、安全性、版本一致性或发布完整性要求。除非先更新项目规范并获得批准，否则不得省略。
- **建议**：默认最佳实践。项目可以调整，但必须记录原因，并执行对应验证。
- **可选**：按项目需求启用的扩展能力，例如 Bark、漏洞扫描或独立前端发布。

带尖括号的内容都是占位符，例如 `<APP_NAME>`、`<GO_VERSION>`。实施前必须替换或删除所有占位符。禁止把未解析占位符提交到正式 Workflow、Makefile、Dockerfile 或 Compose 文件。

## 3. 场景决策树

实施前必须先走完以下决策树，不得直接复制模板：

```text
需要交付什么？
├── 只发布跨平台二进制
│   └── 使用：Makefile + 可复用质量门禁 + binary-release.yml
├── 只发布 Docker 镜像
│   └── 使用：Makefile + 可复用质量门禁 + docker-release.yml + Dockerfile
└── 两者都发布
    └── 使用：两套独立发布 Workflow，共用质量门禁定义

前端属于哪种类型？
├── 无前端
│   └── has_frontend=false，删除全部 Bun 与前端 Artifact 步骤
├── Go 内嵌前端
│   ├── 质量门禁用 Bun 构建前端
│   ├── 二进制发布通过 Artifact 把产物放入 go:embed 目录
│   └── Dockerfile 使用独立 Bun 构建阶段
└── 前后端独立部署
    ├── 后端构建不复制前端产物
    └── 前端另建独立 Workflow；不在本规范模板中混合发布

项目是否需要 CGO？
├── 不需要
│   ├── CGO_ENABLED=0
│   ├── 二进制可从 Ubuntu Runner 直接交叉编译
│   └── Docker Go builder 可在 BUILDPLATFORM 上交叉编译
└── 需要
    ├── 明确 C 编译器、开发库、运行库和 libc
    ├── 验证每个目标架构
    └── 不得直接照抄 CGO_ENABLED=0 的交叉编译模板
```

## 4. 实施前项目参数表

实施者必须先复制并填写此表。任何标记为“必须确认”的项目如果仍为空，必须停止生成代码并向项目负责人确认。

| 参数 | 示例默认值 | 是否必须确认 | 说明 |
| --- | --- | --- | --- |
| `APP_NAME` | `filedock` | 是 | 二进制名、压缩包名前缀；使用小写安全字符 |
| `DISPLAY_NAME` | `FileDock` | 是 | Release 标题、通知标题 |
| `GO_MODULE` | `github.com/org/repo` | 是 | 从 `go.mod` 读取，不得猜测 |
| `GO_VERSION` | `1.25.12` | 是 | 从 `go.mod` 读取；Docker builder 使用相同版本 |
| `MAIN_PACKAGE` | `.` | 是 | `go build` 的入口包 |
| `VERSION_PACKAGE` | `main` | 是 | `go build -X` 注入变量所在包；可能是 `internal/version` 的完整 import path |
| `CGO_ENABLED` | `0` | 是 | 按依赖和目标平台决定，不得机械复制 |
| `BUILD_TIMEZONE` | `Asia/Shanghai` | 是 | IANA 时区名；生成结果必须带偏移 |
| `HAS_FRONTEND` | `true` | 是 | 纯 Go 为 false，内嵌前端为 true |
| `FRONTEND_DIRECTORY` | `frontend` | 前端项目必填 | 前端根目录 |
| `FRONTEND_OUTPUT_DIRECTORY` | `dist` | 前端项目必填 | 相对前端根目录的构建输出 |
| `EMBEDDED_STATIC_DIRECTORY` | `static/dist` | 内嵌前端必填 | Go `go:embed` 实际读取目录 |
| `BUN_VERSION` | `1.4.0` | 前端项目必填 | Workflow 与 Dockerfile 必须一致 |
| `BINARY_TARGETS` | 见二进制矩阵 | 二进制发布必填 | 明确每个 GOOS、GOARCH、格式和扩展名 |
| `DOCKER_CONTEXT` | `.` | Docker 发布必填 | Buildx context |
| `DOCKERFILE_PATH` | `Dockerfile` | Docker 发布必填 | 相对仓库根目录路径 |
| `IMAGE_NAME` | 仓库名 | Docker 发布必填 | GHCR 镜像名，必须转小写 |
| `RUNTIME_PORT` | `8085` | HTTP 服务必填 | Dockerfile EXPOSE、Compose 映射和健康检查统一 |
| `HEALTH_PATH` | `/health` | 有健康接口时必填 | 必须使用实际无需鉴权的健康接口 |
| `BARK_ENABLED` | `true` | 是 | 启用时必须配置仓库 Secret `BARK_KEY` |

实施者还必须回答：

- 版本变量是否确实存在且可被 linker `-X` 修改？
- Go 内嵌前端是否使用 `//go:embed all:<directory>`？
- 前端锁文件是否为 Bun 锁文件，`bun install --frozen-lockfile` 是否通过？
- 项目是否有 SQLite、图像处理、系统驱动或其他 CGO 依赖？
- 二进制是否能在 Ubuntu Runner 交叉编译全部目标？
- 容器内哪些目录需要非 root 用户写入？
- 健康接口是否检查真实服务状态，是否无需鉴权？
- GHCR Package 应为 Public 还是 Private？

## 5. 文件职责与推荐布局

```text
Makefile                              # 本地开发、测试、单平台验证构建
.github/workflows/quality-gate.yml   # 可复用质量门禁
.github/workflows/binary-release.yml # 可选：跨平台二进制与 GitHub Release
.github/workflows/docker-release.yml # 可选：多架构 GHCR 镜像
Dockerfile                            # 可选：多阶段镜像构建
compose.yml                           # 可选：部署参数与健康检查
.dockerignore                         # Docker 构建上下文排除规则
```

职责必须分离：

- **Makefile**：本地开发、测试、当前平台或少量开发验证平台构建、构建信息注入。
- **质量门禁**：测试、静态检查、前端生产构建、可选前端 Artifact。
- **二进制发布**：目标矩阵、压缩、校验和、GitHub Release、Commit 列表。
- **Docker 发布**：Buildx、QEMU、GHCR、多架构 Manifest、OCI 标签、SBOM、Provenance。
- **Dockerfile**：可重复构建程序与运行镜像，不依赖开发机预编译文件。
- **Compose**：运行配置、端口、挂载、健康检查；Dockerfile 不写 `HEALTHCHECK`。

## 6. 公共发布基线

### 6.1 Tag 规则

**必须**：两个发布 Workflow 都只监听：

```yaml
on:
  push:
    tags:
      - "v*"
```

**必须**：Workflow 内再次严格校验：

```bash
if [[ ! "${GITHUB_REF_NAME}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Invalid release tag: ${GITHUB_REF_NAME}. Expected format: v1.2.3"
  exit 1
fi
```

合法：`v0.1.0`、`v1.2.3`、`v10.20.30`。

默认非法：`v1`、`v1.2`、`v1.2.3-rc.1`、`v1.2.3+build.1`、`release-1.2.3`。

如果未来项目需要预发布版本，必须单独设计规则；预发布镜像不得更新 `latest`。

### 6.2 构建信息契约

程序建议提供以下 linker 可注入变量：

```go
var (
	Version   = "dev"
	GitBranch = "unknown"
	GitCommit = "unknown"
	BuildTime = "unknown"
)
```

含义固定如下：

| 字段 | 本地 Makefile | Tag 发布 |
| --- | --- | --- |
| `Version` | 精确 Tag；否则短 Commit；否则 `dev` | 完整 Tag，例如 `v1.2.3` |
| `GitBranch` | 当前分支；否则 `unknown` | Tag 名，例如 `v1.2.3` |
| `GitCommit` | `git rev-parse --short HEAD` | 同样执行 `git rev-parse --short HEAD` |
| `BuildTime` | 显式项目时区、RFC 3339 | prepare Job 只生成一次，全部架构复用 |

**必须**：GitHub Actions 不得直接把完整 `${GITHUB_SHA}` 注入 `GitCommit`，也不得硬编码截取 7 位。使用 Git 自身的短哈希计算，才能与 Makefile 保持一致。

### 6.3 BuildTime 规则

**必须**：BuildTime 携带时区。推荐格式：

```text
2026-08-26T10:30:00+08:00
2026-08-26T02:30:00Z
```

禁止格式：

```text
2026-08-26 10:30:00
```

该字符串没有时区，无法判断真实时刻。

**必须**：显式设置 `TZ=<BUILD_TIMEZONE>`，不能依赖开发机或 Runner 默认时区。禁止先格式化 UTC 文本再手工加八小时。

GNU/BSD `date` 的 `%z` 常输出 `+0800`，RFC 3339 要求 `+08:00`。兼容写法：

```bash
raw="$(TZ="${BUILD_TIMEZONE}" date +"%Y-%m-%dT%H:%M:%S%z")"
build_time="$(printf '%s\n' "${raw}" | sed -E 's/([+-][0-9]{2})([0-9]{2})$/\1:\2/')"
```

不要依赖并非所有 GNU/BSD `date` 都支持的 `%:z`。

### 6.4 Shell 规则

所有 GitHub Actions 多行 Bash 步骤必须：

```bash
set -euo pipefail
```

并遵守：

- 变量引用使用双引号，例如 `"${value}"`。
- GitHub step 输出写入 `"${GITHUB_OUTPUT}"`，不要继续使用已废弃的 `set-output`。
- 失败使用 `echo "::error::<message>"` 并 `exit 1`。
- 临时目录使用 `mktemp -d`，需要清理时使用 `trap`。
- 关键输入文件和输出文件使用 `-s`、文件数量或归档内容再次验证。
- 不在日志输出 Secret、Registry token、签名材料或完整环境变量。
- 不把不可信用户输入直接拼成 Shell 命令。

### 6.5 权限和并发

**必须**：使用最小权限。二进制 Workflow 全局 `contents: read`，只有 Release Job 使用 `contents: write`。

Docker 发布 Job 默认：

```yaml
permissions:
  contents: read
  packages: write
  attestations: write
  id-token: write
```

关闭 Provenance 时可以移除 `attestations: write` 与 `id-token: write`。

**必须**：同一 Tag、同一发布类型串行；不同发布类型互不阻断：

```yaml
concurrency:
  group: binary-release-${{ github.ref }}
  cancel-in-progress: false
```

Docker Workflow 使用 `docker-release-${{ github.ref }}`。不得设置 `cancel-in-progress: true`，避免中途取消 Release 上传或 Manifest 推送。

## 7. Makefile 规范

### 7.1 通用模板

以下模板用于本地 Go 构建。替换 `<APP_NAME>`、`<VERSION_PACKAGE>`、`<MAIN_PACKAGE>`；按项目确认 CGO。

```makefile
.DEFAULT_GOAL := help

APP ?= <APP_NAME>
BIN_DIR ?= bin
MAIN_PACKAGE ?= <MAIN_PACKAGE>
VERSION_PACKAGE ?= <VERSION_PACKAGE>
CGO_ENABLED ?= 0
BUILD_TIMEZONE ?= Asia/Shanghai

VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo "dev")
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell TZ=$(BUILD_TIMEZONE) date +"%Y-%m-%dT%H:%M:%S%z" | sed -E 's/([+-][0-9]{2})([0-9]{2})$$/\1:\2/')

LDFLAGS := -s -w \
	-X '$(VERSION_PACKAGE).Version=$(VERSION)' \
	-X '$(VERSION_PACKAGE).GitBranch=$(GIT_BRANCH)' \
	-X '$(VERSION_PACKAGE).GitCommit=$(GIT_COMMIT)' \
	-X '$(VERSION_PACKAGE).BuildTime=$(BUILD_TIME)'

.PHONY: help clean build test

help:
	@echo "Commands:"
	@echo "  make build  构建当前平台二进制"
	@echo "  make test   运行 Go 测试"
	@echo "  make clean  清理构建产物"

clean:
	@rm -rf $(BIN_DIR)

build:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) go build \
		-trimpath \
		-ldflags="$(LDFLAGS)" \
		-o ./$(BIN_DIR)/$(APP) \
		$(MAIN_PACKAGE)

test:
	@CGO_ENABLED=$(CGO_ENABLED) go test ./...
```

注意：

- Makefile 中 sed 正则的行尾 `$` 必须写成 `$$`，否则 Make 会先消费 `$`。
- 保留 `?=`，使 CI 或调用者可显式覆盖构建信息。
- `<VERSION_PACKAGE>` 必须是变量实际所在包。变量在 `main` 包时填 `main`；在内部包时填完整 Go import path。
- 不假设开发机使用最新版 GNU Make；模板必须可在 macOS 自带 Make 中运行。
- 构建值包含空格时容易破坏 linker 参数，因此 BuildTime 使用无空格 RFC 3339。
- Makefile 以开发体验为目标，不强制添加 Release、GHCR 推送或 Bark 目标。

### 7.2 Go 内嵌 Bun 前端扩展

内嵌前端项目增加。下面的 `clean` 目标用于**替换**通用模板中的 `clean`，不能把两个带 recipe 的同名 `clean` 目标同时保留，否则 Make 会出现覆盖警告：

```makefile
FRONTEND_DIR ?= frontend
FRONTEND_OUTPUT_DIR ?= $(FRONTEND_DIR)/dist
EMBEDDED_STATIC_DIR ?= static/dist

.PHONY: deps web

deps:
	@cd $(FRONTEND_DIR) && bun install --frozen-lockfile

web:
	@cd $(FRONTEND_DIR) && bun run build
	@rm -rf $(EMBEDDED_STATIC_DIR)
	@mkdir -p $(EMBEDDED_STATIC_DIR)
	@cp -R $(FRONTEND_OUTPUT_DIR)/. $(EMBEDDED_STATIC_DIR)/
	@touch $(EMBEDDED_STATIC_DIR)/.placeholder

build: web

clean:
	@rm -rf $(BIN_DIR) $(FRONTEND_OUTPUT_DIR) $(EMBEDDED_STATIC_DIR)
	@mkdir -p $(EMBEDDED_STATIC_DIR)
	@touch $(EMBEDDED_STATIC_DIR)/.placeholder
```

Go 嵌入指令使用：

```go
//go:embed all:dist
var dist embed.FS
```

原因：普通 `//go:embed dist` 会忽略 `.placeholder`。干净 checkout 中如果目录只有隐藏占位文件，`go test ./...` 会报：

```text
pattern dist: cannot embed directory dist: contains no embeddable files
```

`all:dist` 只解决干净仓库可编译问题。正式发布仍必须检查真实前端输出存在且非空，不能把 `.placeholder` 当成成功构建产物。

## 8. 可复用质量门禁 Workflow

推荐文件：`.github/workflows/quality-gate.yml`。

模板默认 Bun `1.4.0`。实施前必须验证该版本真实存在，并与项目锁文件兼容。

```yaml
name: Reusable Quality Gate

on:
  workflow_call:
    inputs:
      go_version_file:
        type: string
        required: false
        default: go.mod
      cgo_enabled:
        type: string
        required: false
        default: "0"
      has_frontend:
        type: boolean
        required: false
        default: false
      frontend_directory:
        type: string
        required: false
        default: frontend
      frontend_output_directory:
        type: string
        required: false
        default: dist
      embedded_static_directory:
        type: string
        required: false
        default: static/dist
      frontend_artifact_name:
        type: string
        required: false
        default: frontend-dist
      bun_version:
        type: string
        required: false
        default: "1.4.0"
      frontend_install_command:
        type: string
        required: false
        default: bun install --frozen-lockfile
      frontend_test_command:
        type: string
        required: false
        default: bun test
      frontend_build_command:
        type: string
        required: false
        default: bun run build

permissions:
  contents: read

jobs:
  verify:
    name: Verify source
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: ${{ inputs.go_version_file }}
          cache: true

      - name: Set up Bun
        if: ${{ inputs.has_frontend }}
        uses: oven-sh/setup-bun@v2
        with:
          bun-version: ${{ inputs.bun_version }}

      - name: Install frontend dependencies
        if: ${{ inputs.has_frontend }}
        working-directory: ${{ inputs.frontend_directory }}
        env:
          FRONTEND_COMMAND: ${{ inputs.frontend_install_command }}
        run: bash -euo pipefail -c "${FRONTEND_COMMAND}"

      - name: Run frontend tests
        if: ${{ inputs.has_frontend }}
        working-directory: ${{ inputs.frontend_directory }}
        env:
          FRONTEND_COMMAND: ${{ inputs.frontend_test_command }}
        run: bash -euo pipefail -c "${FRONTEND_COMMAND}"

      - name: Build frontend
        if: ${{ inputs.has_frontend }}
        working-directory: ${{ inputs.frontend_directory }}
        env:
          FRONTEND_COMMAND: ${{ inputs.frontend_build_command }}
        run: bash -euo pipefail -c "${FRONTEND_COMMAND}"

      - name: Prepare embedded frontend
        if: ${{ inputs.has_frontend }}
        shell: bash
        env:
          FRONTEND_SOURCE: ${{ inputs.frontend_directory }}/${{ inputs.frontend_output_directory }}
          EMBEDDED_TARGET: ${{ inputs.embedded_static_directory }}
        run: |
          set -euo pipefail

          if [[ ! -d "${FRONTEND_SOURCE}" ]] || ! find "${FRONTEND_SOURCE}" -type f -print -quit | grep -q .; then
            echo "::error::Frontend build output is missing or empty: ${FRONTEND_SOURCE}"
            exit 1
          fi

          rm -rf "${EMBEDDED_TARGET}"
          mkdir -p "${EMBEDDED_TARGET}"
          cp -R "${FRONTEND_SOURCE}/." "${EMBEDDED_TARGET}/"

      - name: Run Go tests
        env:
          CGO_ENABLED: ${{ inputs.cgo_enabled }}
        run: go test ./...

      - name: Upload frontend artifact
        if: ${{ inputs.has_frontend }}
        uses: actions/upload-artifact@v7
        with:
          name: ${{ inputs.frontend_artifact_name }}
          path: ${{ inputs.frontend_directory }}/${{ inputs.frontend_output_directory }}
          if-no-files-found: error
          retention-days: 1
```

安全说明：前端命令输入只能来自仓库内受审查的调用 Workflow，不能直接绑定 `workflow_dispatch` 用户输入、Issue 内容或 PR 标题。

两个发布 Workflow 各自调用一次质量门禁是预期行为。它们共享定义，但不共享某次运行结果，保证可以独立重跑。

## 9. 跨平台二进制 Release 规范

### 9.1 默认产物契约

目标矩阵必须由项目负责人确认。以下只是常见示例：

| GOOS | GOARCH | 包内文件 | 格式 | 资产名示例 |
| --- | --- | --- | --- | --- |
| `darwin` | `arm64` | `<APP_NAME>` | `tar.gz` | `<APP_NAME>_v1.2.3_darwin_arm64.tar.gz` |
| `windows` | `amd64` | `<APP_NAME>.exe` | `zip` | `<APP_NAME>_v1.2.3_windows_amd64.zip` |
| `linux` | `amd64` | `<APP_NAME>` | `tar.gz` | `<APP_NAME>_v1.2.3_linux_amd64.tar.gz` |
| `linux` | `arm64` | `<APP_NAME>` | `tar.gz` | `<APP_NAME>_v1.2.3_linux_arm64.tar.gz` |

**必须**：

- Release 只上传压缩包和 `checksums.txt`，不上传裸二进制。
- 每个压缩包根目录默认只有一个二进制，不包含二级目录、README 或配置文件。
- Windows 使用 `.zip`；Linux、macOS 使用 `.tar.gz`。
- `checksums.txt` 使用 SHA-256，且只覆盖压缩包。
- 中间 Artifact 保留 1 天；最终 Release 资产不受此期限影响。

### 9.2 二进制 Workflow 模板

推荐文件：`.github/workflows/binary-release.yml`。

下面模板按“Go 内嵌前端”展示。纯 Go 项目必须把 `HAS_FRONTEND` 改成 `false`，在质量门禁调用中设置 `has_frontend: false`，并删除或保留带条件的 Artifact 下载步骤。

```yaml
name: Binary Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

concurrency:
  group: binary-release-${{ github.ref }}
  cancel-in-progress: false

env:
  APP_NAME: <APP_NAME>
  DISPLAY_NAME: <DISPLAY_NAME>
  MAIN_PACKAGE: <MAIN_PACKAGE>
  VERSION_PACKAGE: <VERSION_PACKAGE>
  BUILD_TIMEZONE: Asia/Shanghai
  HAS_FRONTEND: "true"
  FRONTEND_ARTIFACT_NAME: frontend-dist
  EMBEDDED_STATIC_DIR: <EMBEDDED_STATIC_DIRECTORY>

jobs:
  prepare:
    name: Prepare release metadata
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.metadata.outputs.version }}
      release_name: ${{ steps.metadata.outputs.release_name }}
      build_time: ${{ steps.metadata.outputs.build_time }}
    steps:
      - name: Validate tag and prepare metadata
        id: metadata
        shell: bash
        run: |
          set -euo pipefail

          tag="${GITHUB_REF_NAME}"
          if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "::error::Invalid release tag: ${tag}. Expected format: v1.2.3"
            exit 1
          fi

          raw="$(TZ="${BUILD_TIMEZONE}" date +"%Y-%m-%dT%H:%M:%S%z")"
          build_time="$(printf '%s\n' "${raw}" | sed -E 's/([+-][0-9]{2})([0-9]{2})$/\1:\2/')"
          if [[ ! "${build_time}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[+-][0-9]{2}:[0-9]{2}$ ]]; then
            echo "::error::Invalid RFC 3339 build time: ${build_time}"
            exit 1
          fi

          echo "version=${tag}" >> "${GITHUB_OUTPUT}"
          echo "release_name=${DISPLAY_NAME} ${tag}" >> "${GITHUB_OUTPUT}"
          echo "build_time=${build_time}" >> "${GITHUB_OUTPUT}"

  quality:
    name: Quality gate
    needs: prepare
    uses: ./.github/workflows/quality-gate.yml
    with:
      go_version_file: go.mod
      cgo_enabled: "<CGO_ENABLED>"
      has_frontend: true
      frontend_directory: <FRONTEND_DIRECTORY>
      frontend_output_directory: <FRONTEND_OUTPUT_DIRECTORY>
      embedded_static_directory: <EMBEDDED_STATIC_DIRECTORY>
      frontend_artifact_name: frontend-dist
      bun_version: "1.4.0"

  build:
    name: Build ${{ matrix.goos }}/${{ matrix.goarch }}
    runs-on: ubuntu-latest
    needs:
      - prepare
      - quality
    strategy:
      fail-fast: false
      matrix:
        include:
          - goos: darwin
            goarch: arm64
            archive: tar.gz
            binary_ext: ""
          - goos: windows
            goarch: amd64
            archive: zip
            binary_ext: .exe
          - goos: linux
            goarch: amd64
            archive: tar.gz
            binary_ext: ""
          - goos: linux
            goarch: arm64
            archive: tar.gz
            binary_ext: ""
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - name: Download frontend artifact
        if: ${{ env.HAS_FRONTEND == 'true' }}
        uses: actions/download-artifact@v8
        with:
          name: ${{ env.FRONTEND_ARTIFACT_NAME }}
          path: ${{ env.EMBEDDED_STATIC_DIR }}

      - name: Verify embedded frontend
        if: ${{ env.HAS_FRONTEND == 'true' }}
        shell: bash
        run: |
          set -euo pipefail
          if [[ ! -d "${EMBEDDED_STATIC_DIR}" ]] || ! find "${EMBEDDED_STATIC_DIR}" -type f -print -quit | grep -q .; then
            echo "::error::Embedded frontend files are missing or empty: ${EMBEDDED_STATIC_DIR}"
            exit 1
          fi

      - name: Build and package binary
        shell: bash
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: "<CGO_ENABLED>"
          VERSION: ${{ needs.prepare.outputs.version }}
          BUILD_TIME: ${{ needs.prepare.outputs.build_time }}
        run: |
          set -euo pipefail

          binary="${APP_NAME}${{ matrix.binary_ext }}"
          asset="${APP_NAME}_${VERSION}_${GOOS}_${GOARCH}.${{ matrix.archive }}"
          git_commit="$(git rev-parse --short HEAD)"
          if [[ ! "${git_commit}" =~ ^[0-9a-f]+$ ]]; then
            echo "::error::Invalid Git short commit: ${git_commit}"
            exit 1
          fi

          mkdir -p dist artifacts
          go build \
            -trimpath \
            -ldflags="-s -w -X '${VERSION_PACKAGE}.Version=${VERSION}' -X '${VERSION_PACKAGE}.GitBranch=${GITHUB_REF_NAME}' -X '${VERSION_PACKAGE}.GitCommit=${git_commit}' -X '${VERSION_PACKAGE}.BuildTime=${BUILD_TIME}'" \
            -o "dist/${binary}" \
            "${MAIN_PACKAGE}"

          if [[ ! -s "dist/${binary}" ]]; then
            echo "::error::Binary is missing or empty: dist/${binary}"
            exit 1
          fi

          if [[ "${{ matrix.archive }}" == "zip" ]]; then
            (cd dist && zip -q "../artifacts/${asset}" "${binary}")
            archive_entries="$(unzip -Z1 "artifacts/${asset}")"
          else
            tar -C dist -czf "artifacts/${asset}" "${binary}"
            archive_entries="$(tar -tzf "artifacts/${asset}")"
          fi

          if [[ ! -s "artifacts/${asset}" ]]; then
            echo "::error::Release archive is missing or empty: ${asset}"
            exit 1
          fi
          if [[ "${archive_entries}" != "${binary}" ]]; then
            echo "::error::Archive must contain only ${binary}; found:"
            printf '%s\n' "${archive_entries}"
            exit 1
          fi

      - name: Upload release artifact
        uses: actions/upload-artifact@v7
        with:
          name: release-${{ matrix.goos }}-${{ matrix.goarch }}
          path: artifacts/*
          if-no-files-found: error
          retention-days: 1

  release:
    name: Publish GitHub Release
    runs-on: ubuntu-latest
    needs:
      - prepare
      - build
    permissions:
      contents: write
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Download release artifacts
        uses: actions/download-artifact@v8
        with:
          pattern: release-*
          path: release
          merge-multiple: true

      - name: Verify assets and generate checksums
        shell: bash
        env:
          VERSION: ${{ needs.prepare.outputs.version }}
        run: |
          set -euo pipefail

          expected_assets=(
            "${APP_NAME}_${VERSION}_darwin_arm64.tar.gz"
            "${APP_NAME}_${VERSION}_windows_amd64.zip"
            "${APP_NAME}_${VERSION}_linux_amd64.tar.gz"
            "${APP_NAME}_${VERSION}_linux_arm64.tar.gz"
          )

          file_count="$(find release -maxdepth 1 -type f | wc -l | tr -d '[:space:]')"
          if [[ "${file_count}" != "${#expected_assets[@]}" ]]; then
            echo "::error::Expected ${#expected_assets[@]} release archives, found ${file_count}."
            exit 1
          fi

          for asset in "${expected_assets[@]}"; do
            if [[ ! -s "release/${asset}" ]]; then
              echo "::error::Expected release archive is missing or empty: ${asset}"
              exit 1
            fi
          done

          (
            cd release
            printf '%s\n' "${expected_assets[@]}" | sort | while IFS= read -r asset; do
              sha256sum "${asset}"
            done > checksums.txt
          )

          checksum_count="$(wc -l < release/checksums.txt | tr -d '[:space:]')"
          if [[ "${checksum_count}" != "${#expected_assets[@]}" ]]; then
            echo "::error::Unexpected checksum entry count: ${checksum_count}"
            exit 1
          fi

      - name: Generate release notes
        shell: bash
        run: |
          set -euo pipefail

          previous_tag=""
          previous_ref="${GITHUB_REF_NAME}^"
          if git rev-parse --verify "${previous_ref}" >/dev/null 2>&1; then
            while IFS= read -r candidate; do
              if [[ "${candidate}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                previous_tag="${candidate}"
                break
              fi
            done < <(git tag --merged "${previous_ref}" --sort=-version:refname)
          fi
          if [[ -n "${previous_tag}" ]]; then
            commit_range="${previous_tag}..${GITHUB_REF_NAME}"
          else
            commit_range="${GITHUB_REF_NAME}"
          fi

          commits="$(git log --pretty=format:'- `%h` %s' "${commit_range}")"
          {
            echo "## 提交记录"
            echo
            if [[ -n "${commits}" ]]; then
              printf '%s\n' "${commits}"
            else
              echo "- 无新增提交"
            fi
          } > release-notes.md

      - name: Publish release
        uses: softprops/action-gh-release@v3
        with:
          tag_name: ${{ needs.prepare.outputs.version }}
          name: ${{ needs.prepare.outputs.release_name }}
          draft: false
          prerelease: false
          body_path: release-notes.md
          overwrite_files: true
          fail_on_unmatched_files: true
          files: |
            release/*.tar.gz
            release/*.zip
            release/checksums.txt

  notify:
    name: Notify binary release result
    runs-on: ubuntu-latest
    needs:
      - prepare
      - quality
      - build
      - release
    if: ${{ always() }}
    steps:
      - name: Notify success
        if: ${{ needs.prepare.result == 'success' && needs.quality.result == 'success' && needs.build.result == 'success' && needs.release.result == 'success' }}
        continue-on-error: true
        uses: Crownor/bark-action@V3.0
        with:
          key: ${{ secrets.BARK_KEY }}
          title: GitHub Action for <DISPLAY_NAME>
          body: 二进制发布成功 ${{ github.ref_name }}

      - name: Notify failure
        if: ${{ needs.prepare.result != 'success' || needs.quality.result != 'success' || needs.build.result != 'success' || needs.release.result != 'success' }}
        continue-on-error: true
        uses: Crownor/bark-action@V3.0
        with:
          key: ${{ secrets.BARK_KEY }}
          title: GitHub Action for <DISPLAY_NAME>
          body: 二进制发布失败 ${{ github.ref_name }}
```

### 9.3 CGO 注意事项

上面矩阵默认最适合 `CGO_ENABLED=0`。若项目启用 CGO：

- 不得只把 `<CGO_ENABLED>` 改成 `1` 就认为完成。
- 必须确认 Ubuntu Runner 是否具备目标架构 C 工具链。
- macOS CGO 交叉编译通常需要 Apple SDK，普通 Ubuntu Runner 不具备。
- Windows CGO 需要对应 MinGW 工具链。
- Linux ARM64 CGO 需要 ARM64 交叉编译器和目标开发库。
- 更稳妥的方案可能是按目标系统选择原生 Runner，或使用 GoReleaser/专用构建镜像；这需要单独设计，不属于默认模板。

## 10. 多架构 Docker 发布规范

### 10.1 镜像契约

Git Tag `v1.2.3` 只生成：

```text
ghcr.io/<owner>/<image>:1.2.3
ghcr.io/<owner>/<image>:latest
```

两个 Tag 指向同一个包含以下平台的 Manifest：

```text
linux/amd64
linux/arm64
```

不生成 `1.2`、`1` 或分支 Tag。

默认允许特殊情况下覆盖同名版本 Tag。必须理解：覆盖会改变该 Tag 对应 Digest，可能影响审计、缓存与回滚。日常发布仍建议创建新版本。

### 10.2 Docker Workflow 模板

推荐文件：`.github/workflows/docker-release.yml`。

```yaml
name: Docker Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

concurrency:
  group: docker-release-${{ github.ref }}
  cancel-in-progress: false

env:
  DISPLAY_NAME: <DISPLAY_NAME>
  BUILD_TIMEZONE: Asia/Shanghai
  DOCKER_CONTEXT: <DOCKER_CONTEXT>
  DOCKERFILE_PATH: <DOCKERFILE_PATH>

jobs:
  prepare:
    name: Prepare image metadata
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.metadata.outputs.version }}
      image_version: ${{ steps.metadata.outputs.image_version }}
      image: ${{ steps.metadata.outputs.image }}
      git_commit: ${{ steps.metadata.outputs.git_commit }}
      build_time: ${{ steps.metadata.outputs.build_time }}
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Validate tag and prepare metadata
        id: metadata
        shell: bash
        run: |
          set -euo pipefail

          tag="${GITHUB_REF_NAME}"
          if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "::error::Invalid release tag: ${tag}. Expected format: v1.2.3"
            exit 1
          fi

          image="ghcr.io/${GITHUB_REPOSITORY,,}"
          image_version="${tag#v}"
          git_commit="$(git rev-parse --short HEAD)"
          raw="$(TZ="${BUILD_TIMEZONE}" date +"%Y-%m-%dT%H:%M:%S%z")"
          build_time="$(printf '%s\n' "${raw}" | sed -E 's/([+-][0-9]{2})([0-9]{2})$/\1:\2/')"

          if [[ ! "${git_commit}" =~ ^[0-9a-f]+$ ]]; then
            echo "::error::Invalid Git short commit: ${git_commit}"
            exit 1
          fi
          if [[ ! "${build_time}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[+-][0-9]{2}:[0-9]{2}$ ]]; then
            echo "::error::Invalid RFC 3339 build time: ${build_time}"
            exit 1
          fi

          echo "version=${tag}" >> "${GITHUB_OUTPUT}"
          echo "image_version=${image_version}" >> "${GITHUB_OUTPUT}"
          echo "image=${image}" >> "${GITHUB_OUTPUT}"
          echo "git_commit=${git_commit}" >> "${GITHUB_OUTPUT}"
          echo "build_time=${build_time}" >> "${GITHUB_OUTPUT}"

  quality:
    name: Quality gate
    needs: prepare
    uses: ./.github/workflows/quality-gate.yml
    with:
      go_version_file: go.mod
      cgo_enabled: "<CGO_ENABLED>"
      has_frontend: <HAS_FRONTEND_BOOLEAN>
      frontend_directory: <FRONTEND_DIRECTORY>
      frontend_output_directory: <FRONTEND_OUTPUT_DIRECTORY>
      embedded_static_directory: <EMBEDDED_STATIC_DIRECTORY>
      bun_version: "1.4.0"

  publish:
    name: Publish multi-platform image
    runs-on: ubuntu-latest
    needs:
      - prepare
      - quality
    permissions:
      contents: read
      packages: write
      attestations: write
      id-token: write
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push image
        uses: docker/build-push-action@v6
        with:
          context: ${{ env.DOCKER_CONTEXT }}
          file: ${{ env.DOCKERFILE_PATH }}
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ${{ needs.prepare.outputs.image }}:${{ needs.prepare.outputs.image_version }}
            ${{ needs.prepare.outputs.image }}:latest
          labels: |
            org.opencontainers.image.source=${{ github.server_url }}/${{ github.repository }}
            org.opencontainers.image.version=${{ needs.prepare.outputs.version }}
            org.opencontainers.image.revision=${{ needs.prepare.outputs.git_commit }}
            org.opencontainers.image.created=${{ needs.prepare.outputs.build_time }}
          build-args: |
            VERSION=${{ needs.prepare.outputs.version }}
            GIT_BRANCH=${{ needs.prepare.outputs.version }}
            GIT_COMMIT=${{ needs.prepare.outputs.git_commit }}
            BUILD_TIME=${{ needs.prepare.outputs.build_time }}
            CGO_ENABLED=<CGO_ENABLED>
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: mode=max
          sbom: true

  notify:
    name: Notify Docker release result
    runs-on: ubuntu-latest
    needs:
      - prepare
      - quality
      - publish
    if: ${{ always() }}
    steps:
      - name: Notify success
        if: ${{ needs.prepare.result == 'success' && needs.quality.result == 'success' && needs.publish.result == 'success' }}
        continue-on-error: true
        uses: Crownor/bark-action@V3.0
        with:
          key: ${{ secrets.BARK_KEY }}
          title: GitHub Action for <DISPLAY_NAME>
          body: 容器镜像发布成功 ${{ github.ref_name }}

      - name: Notify failure
        if: ${{ needs.prepare.result != 'success' || needs.quality.result != 'success' || needs.publish.result != 'success' }}
        continue-on-error: true
        uses: Crownor/bark-action@V3.0
        with:
          key: ${{ secrets.BARK_KEY }}
          title: GitHub Action for <DISPLAY_NAME>
          body: 容器镜像发布失败 ${{ github.ref_name }}
```

Docker Workflow 不创建、不修改 GitHub Release。这样它不会与二进制 Workflow 竞争同一 Release。

模板默认用仓库名作为镜像名。Monorepo 或镜像名与仓库名不同的项目，必须把 `image` 改为 `ghcr.io/${GITHUB_REPOSITORY_OWNER,,}/<IMAGE_NAME>`，并确保 `<IMAGE_NAME>` 已转小写。

## 11. Dockerfile 规范

### 11.1 Go 内嵌 Bun 前端、CGO 关闭模板

默认示例：Bun `1.4.0`、Go `1.25.12`、Debian Bookworm。实施时 Go 版本必须改成项目 `go.mod` 声明的真实版本。

```dockerfile
# syntax=docker/dockerfile:1.7

ARG BUN_VERSION=1.4.0
ARG GO_VERSION=1.25.12

FROM --platform=$BUILDPLATFORM oven/bun:${BUN_VERSION}-slim AS frontend-builder
WORKDIR /src/frontend

COPY frontend/package.json frontend/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

COPY frontend/ ./
RUN bun test && bun run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS go-builder

ARG TARGETOS
ARG TARGETARCH
ARG CGO_ENABLED=0
ARG VERSION=dev
ARG GIT_BRANCH=unknown
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
COPY --from=frontend-builder /src/frontend/dist ./<EMBEDDED_STATIC_DIRECTORY>

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED="${CGO_ENABLED}" GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build \
      -trimpath \
      -ldflags="-s -w -X '<VERSION_PACKAGE>.Version=${VERSION}' -X '<VERSION_PACKAGE>.GitBranch=${GIT_BRANCH}' -X '<VERSION_PACKAGE>.GitCommit=${GIT_COMMIT}' -X '<VERSION_PACKAGE>.BuildTime=${BUILD_TIME}'" \
      -o /out/<APP_NAME> \
      <MAIN_PACKAGE>

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 app \
    && useradd --system --uid 10001 --gid app --home-dir /nonexistent --shell /usr/sbin/nologin app \
    && mkdir -p /app <WRITABLE_DIRECTORIES> \
    && chown -R app:app /app <WRITABLE_DIRECTORIES>

WORKDIR /app
COPY --from=go-builder --chown=app:app /out/<APP_NAME> /app/<APP_NAME>

USER 10001:10001
EXPOSE <RUNTIME_PORT>

ENTRYPOINT ["/app/<APP_NAME>"]
```

必须替换：

- `<EMBEDDED_STATIC_DIRECTORY>`：例如 `static/dist`。
- `<VERSION_PACKAGE>`：例如 `main` 或完整 import path。
- `<APP_NAME>`、`<MAIN_PACKAGE>`、`<WRITABLE_DIRECTORIES>`、`<RUNTIME_PORT>`。
- 若没有可写目录，把 `<WRITABLE_DIRECTORIES>` 从 mkdir/chown 两处完整删除，不能留下空占位符。

Dockerfile 不配置 `HEALTHCHECK`。健康检查只放在 Compose 或 Kubernetes 部署配置中。

### 11.2 纯 Go 项目调整

纯 Go 项目：

1. 删除整个 `frontend-builder` 阶段。
2. 删除从 `frontend-builder` 复制静态产物的 `COPY`。
3. 保留 Go module 缓存、Go build、Debian runtime、非 root 用户和版本注入。

### 11.3 CGO 开启项目调整

启用 CGO 时不能使用上面的纯交叉编译方式直接结束实施。建议用目标平台原生 builder 阶段，让 Buildx/QEMU 在目标架构中执行：

```dockerfile
FROM --platform=$TARGETPLATFORM golang:${GO_VERSION}-bookworm AS go-builder
```

同时：

- `CGO_ENABLED=1`。
- Builder 安装项目需要的 `build-essential`、`pkg-config` 和开发库。
- Runtime 安装对应动态运行库。
- Builder 与 Runtime 均使用 Debian Bookworm 系列，避免 glibc/musl 不一致。
- 对 `linux/amd64`、`linux/arm64` 分别验证启动和核心功能。
- ARM64 在 QEMU 下构建可能较慢，不能把耗时误判为死锁。
- 如果依赖不支持某架构，必须从平台列表移除并更新项目规范，不能发布表面存在但无法启动的 Manifest。

默认不使用 Alpine。原因是 Alpine 使用 musl，容易与 glibc 编译产物、CGO 动态库和第三方二进制不兼容。

## 12. `.dockerignore` 模板

```dockerignore
.git
.github
.gocache
.gomodcache
.idea
.vscode
.DS_Store

bin
dist
artifacts
data
tmp
coverage*

frontend/node_modules
frontend/dist

static/dist/*
!static/dist/.placeholder

docs
```

注意：

- Dockerfile 需要的源文件不得被忽略。
- Go 内嵌前端由 Docker Bun 阶段生成，因此可以忽略开发机的 `frontend/dist` 和 `static/dist` 真实产物。
- 如果构建过程需要 docs、迁移文件、模板或配置默认文件，必须从忽略表移除。
- `.git` 被忽略后，Dockerfile 内不能执行 `git rev-parse`；版本信息必须由 Workflow 通过 build args 传入。

## 13. Docker Compose 健康检查

Dockerfile 不写 `HEALTHCHECK`。Compose 按部署环境定义：

```yaml
services:
  app:
    image: ghcr.io/<owner>/<image>:${APP_VERSION:-latest}
    ports:
      - "8085:8085"
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://localhost:8085/health || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s
```

必须按项目修改：

- `8085`：容器实际监听端口。
- `/health`：项目真实健康接口。
- `start_period`：应用迁移、缓存预热或依赖连接所需时间。
- 健康接口应无需鉴权，并返回非 2xx 表示不健康。
- 如果接口只代表进程存活，必须明确它不是完整就绪检查。

运行镜像安装 `curl` 是为了让 Compose 探测命令可执行。Kubernetes 项目仍需在 Deployment 中单独配置 liveness/readiness probe。

## 14. Bark 通知规范

Bark 是可选模块。启用时：

1. 在 GitHub 仓库 `Settings -> Secrets and variables -> Actions` 中创建 Repository Secret：`BARK_KEY`。
2. Workflow 只能通过 `${{ secrets.BARK_KEY }}` 读取，不得写入仓库文件或日志。
3. 二进制与 Docker 使用独立消息，例如：

```text
二进制发布成功 v1.2.3
容器镜像发布成功 v1.2.3
```

4. 成功必须表示该发布类型的质量门禁和全部发布步骤都成功。
5. 任一前置阶段失败、取消或跳过时走失败通知。
6. 通知 step 必须 `continue-on-error: true`。Bark 配置错误或服务不可用不得改变 Release 或镜像发布结果。
7. 未配置 `BARK_KEY` 时 Bark step 可能失败，但因非阻断配置，发布结果保持不变。

如果项目决定不启用 Bark，必须从两个发布 Workflow 中删除整个 `notify` Job，而不是保留空 Secret 或伪造占位 key。

可替换为飞书、Slack、邮件等渠道，但仍必须遵守“通知与发布解耦”。

## 15. GHCR 与供应链元数据

### 15.1 GHCR 注意事项

- 使用 `GITHUB_TOKEN` 登录，不需要另建 Registry 密码。
- Docker 发布 Job 必须拥有 `packages: write`。
- owner 与镜像名统一转小写。
- 公共仓库建议把 Package 设置为 Public；私有仓库保持 Private。
- 首次推送后，人工进入 Package Settings 检查可见性和源码仓库关联。
- Workflow 不自动修改 Package 可见性，避免增加 API 权限。
- `1.2.3` 与 `latest` 默认都允许特殊情况下覆盖；覆盖会改变 Digest，应谨慎操作。

### 15.2 OCI 标签

以下标签必须存在：

```text
org.opencontainers.image.source
org.opencontainers.image.version
org.opencontainers.image.revision
org.opencontainers.image.created
```

其中 `version` 使用完整 Git Tag `v1.2.3`，`revision` 使用短 Commit，`created` 使用带时区 RFC 3339。

### 15.3 Provenance 与 SBOM

默认启用：

```yaml
provenance: mode=max
sbom: true
```

它们作为 Registry 关联证明保存，不是容器内业务文件。项目因工具或 Registry 兼容性需要关闭时，必须记录原因，并同步移除不再需要的 `attestations`、`id-token` 权限。

### 15.4 漏洞扫描

漏洞扫描不是默认发布阻断项。建议增加独立 Trivy Workflow：

- 初期只报告。
- 项目明确严重级别阈值后再阻断。
- 不得因为未定义风险接受标准而直接让所有历史基础镜像漏洞阻止发布。

## 16. Action 与工具版本策略

模板中使用的版本在实施时必须重新确认真实存在：

```text
actions/checkout@v6
actions/setup-go@v6
actions/upload-artifact@v7
actions/download-artifact@v8
oven-sh/setup-bun@v2
softprops/action-gh-release@v3
Crownor/bark-action@V3.0
docker/setup-qemu-action@v3
docker/setup-buildx-action@v3
docker/login-action@v3
docker/build-push-action@v6
```

规则：

- 禁止 `@main`、`@master`、`@latest`。
- 普通项目允许明确主版本，降低维护成本。
- 高安全项目固定完整 Commit SHA，并使用 Dependabot/Renovate 更新。
- Go 版本从 `go.mod` 读取。
- Bun Workflow 与 Dockerfile 使用同一明确版本，当前规范示例为 `1.4.0`。
- Go builder 使用 `golang:<go.mod版本>-bookworm`。
- Runtime 使用 `debian:bookworm-slim`。
- 禁止假设文档示例永远有效；实施时查官方 Action、Docker Hub 或 GHCR。

## 17. 实施步骤

低智能模型必须严格按顺序执行：

### 阶段一：发现项目

1. 确认当前目录在 Git 仓库内。
2. 读取项目级 `AGENTS.md` 或贡献规范。
3. 读取 `go.mod`、Makefile、主入口、版本变量、Dockerfile、Compose、现有 Workflow。
4. 判断交付类型和前端类型。
5. 搜索 CGO：检查依赖、`import "C"`、SQLite/图像/系统库。
6. 填写第 4 节参数表。
7. 发现无法从代码确定的参数时，逐个询问用户；不要猜测。

### 阶段二：统一构建信息

1. 确认四个版本变量可用 linker `-X` 注入。
2. 修改 Makefile，统一 Version、Branch、短 Commit、带时区 BuildTime。
3. 验证默认 BuildTime 格式。
4. 验证显式 `BUILD_TIME=...` 仍可覆盖。
5. 构建当前平台并执行版本命令检查实际输出。

### 阶段三：处理内嵌前端

仅内嵌前端项目执行：

1. 固定 Bun 版本并验证锁文件。
2. 确认前端输出目录。
3. 确认 Go 嵌入目录。
4. 提交 `.placeholder`。
5. 使用 `//go:embed all:<directory>`。
6. 在只有占位文件的干净场景运行 `go test ./...`。

### 阶段四：建立质量门禁

1. 新增可复用 Workflow。
2. 纯 Go关闭前端输入。
3. 内嵌前端执行冻结安装、测试、生产构建、嵌入复制和 Artifact 上传。
4. 使用与正式发布相同的 CGO 策略运行 Go 测试。
5. 任一命令失败必须阻断调用方发布。

### 阶段五：建立二进制发布

项目需要二进制时执行：

1. 确认目标矩阵。
2. 添加 prepare、quality、build、release、notify Job。
3. 前端只构建一次；矩阵下载同一 Artifact。
4. 四个平台使用相同 Version、Commit、BuildTime。
5. 验证每个压缩包只有一个二进制。
6. 汇总 Artifact、生成 SHA-256、生成 Commit 列表。
7. Release 只上传期望文件。

### 阶段六：建立 Docker 发布

项目需要镜像时执行：

1. 新增 Dockerfile 与 `.dockerignore`。
2. 内嵌前端增加 Bun `1.4.0` 阶段。
3. Go builder 版本与 `go.mod` 一致。
4. Runtime 使用 Debian Bookworm slim，安装 CA、curl、时区数据。
5. 创建非 root 用户并授予必要目录权限。
6. 按 CGO 策略决定 BUILDPLATFORM 交叉编译或 TARGETPLATFORM 原生构建。
7. 新增 Docker Workflow，推送 `1.2.3` 与 `latest` 双架构 Manifest。
8. 启用 OCI 标签、GHA 缓存、Provenance、SBOM。
9. Docker Workflow 不修改 GitHub Release。

### 阶段七：部署与通知

1. Compose 增加健康检查，不修改 Dockerfile 添加 HEALTHCHECK。
2. 配置仓库 Secret `BARK_KEY`。
3. 检查 GHCR Package 可见性。
4. 确认通知失败不阻断发布。

### 阶段八：验证与交付

1. 完成第 19 节全部适用项。
2. 搜索并清除所有 `<PLACEHOLDER>`。
3. 检查 Git diff 和无关文件。
4. 提交后创建新 Tag；不得假设旧 Tag 重跑会读取新提交。

## 18. 必配项与易遗漏事项

### 18.1 GitHub 配置

- [ ] 启用 Bark 时已配置 Repository Secret `BARK_KEY`。
- [ ] Release Job 有 `contents: write`。
- [ ] Docker Job 有 `packages: write`。
- [ ] Provenance 启用时有 `attestations: write`、`id-token: write`。
- [ ] 仓库组织策略允许 GitHub Actions 创建 Release 和 Package。
- [ ] 首次 GHCR 推送后已检查 Package Public/Private 可见性。

### 18.2 Tag 与 Workflow

- [ ] Tag 匹配 `^v[0-9]+\.[0-9]+\.[0-9]+$`。
- [ ] Tag 指向包含最新 Workflow 修改的提交。
- [ ] 修复 Workflow 后创建了新 Tag。

重要：GitHub Actions 对 Tag push 检出 Tag 指向的提交。旧 Tag 指向旧提交，重跑旧 Workflow 不会自动读取分支上的新修复。公开 Tag 不建议强制移动；应创建新补丁版本。

### 18.3 `go:embed`

- [ ] 干净仓库嵌入目录存在 `.placeholder`。
- [ ] 使用 `all:` 前缀嵌入隐藏占位文件。
- [ ] 正式构建前真实前端产物存在且非空。
- [ ] Artifact 下载路径正是 Go 嵌入目录，不是仅下载到前端输出目录。

### 18.4 日志关闭平台差异

Zap 等日志库对 stdout/stderr 调用 `Sync()` 时：

- macOS/终端可能返回 `ENOTTY`、`EBADF`。
- Linux GitHub Runner 的管道 stdout 可能返回 `EINVAL`。

如果应用关闭逻辑把这些标准输出同步错误当成致命错误，测试可能出现：

```text
sync /dev/stdout: invalid argument
```

建议只对 stdout/stderr 的已知非致命同步错误忽略，真实文件日志的其他同步错误仍返回。不得简单删除 `Close()` 测试或让 Workflow 忽略测试失败。

### 18.5 Artifact 生命周期

- Artifact 是 Job 间临时传递存储，不同 Job 不共享本地磁盘。
- `retention-days: 1` 只删除 Actions 中间 Artifact。
- GitHub Release 资产和 GHCR 镜像不会因此删除。
- Cache 用于加速依赖，不得替代 Artifact 传递正式产物。

### 18.6 非 root 权限

- [ ] 应用写入的数据、日志、临时目录已创建并 `chown`。
- [ ] 挂载 volume 后部署环境仍给 UID/GID 正确权限。
- [ ] 容器不依赖特权端口或 root 才能访问的设备。
- [ ] 特殊 root 需求已在项目文档说明。

## 19. 验收清单

### 19.1 静态检查

- [ ] YAML 可被解析。
- [ ] Workflow Job 依赖无循环，质量门禁位于发布之前。
- [ ] Dockerfile 可解析，Compose 配置可解析。
- [ ] Makefile `make -n build` 能展开正确命令。
- [ ] 仓库不存在未解析的 `<...>` 模板占位符。
- [ ] Action 与基础镜像版本真实存在。
- [ ] 没有 `@main`、`@master`、`@latest` Action。
- [ ] 没有 Secret 输出到日志。

### 19.2 本地检查

- [ ] `go test ./...` 通过；CGO 模式与发布一致。
- [ ] 内嵌前端 `bun install --frozen-lockfile` 通过。
- [ ] 前端测试通过。
- [ ] 前端生产构建通过。
- [ ] 干净嵌入目录只有 `.placeholder` 时 Go 测试仍通过。
- [ ] 当前平台 Makefile 构建成功。
- [ ] 版本命令显示正确 Version、Branch、短 Commit、带时区 BuildTime。

### 19.3 二进制 CI 检查

- [ ] 非法 `v*` Tag 在 prepare 阶段失败。
- [ ] 矩阵恰好包含已批准平台。
- [ ] 所有架构共享同一 BuildTime。
- [ ] GitCommit 与 `git rev-parse --short HEAD` 一致。
- [ ] Windows ZIP 只有 `<APP_NAME>.exe`。
- [ ] Linux/macOS TAR.GZ 只有 `<APP_NAME>`。
- [ ] Release 不包含裸二进制。
- [ ] `checksums.txt` 行数与压缩包数量一致且验证成功。
- [ ] Release 说明列出上一个 Tag 以来的全部 Commit 主题。
- [ ] 中间 Artifact 保留 1 天。

### 19.4 Docker CI 与发布后检查

- [ ] GHCR 同时存在 `1.2.3` 和 `latest`。
- [ ] 两个 Tag 指向期望 Manifest。
- [ ] Manifest 包含 `linux/amd64`、`linux/arm64`。
- [ ] 两个架构镜像都能启动。
- [ ] 容器内程序版本保留 `v1.2.3`。
- [ ] GitCommit 为短哈希。
- [ ] BuildTime 带明确时区。
- [ ] OCI source/version/revision/created 标签正确。
- [ ] Provenance 与 SBOM 已生成，或例外原因已记录。
- [ ] 进程以非 root 用户运行。
- [ ] 必要目录可写，其他目录没有多余写权限。
- [ ] Compose 健康检查进入 healthy。
- [ ] GHCR 可见性符合项目要求。

### 19.5 通知检查

- [ ] `BARK_KEY` 已配置。
- [ ] 二进制与 Docker 使用不同通知文案。
- [ ] 成功通知包含 Tag。
- [ ] 前置失败会发送失败通知。
- [ ] Bark 失败不会使成功发布变红。

## 20. 常见错误与处理

| 现象 | 根因 | 正确处理 |
| --- | --- | --- |
| `contains no embeddable files` | 嵌入目录只有被忽略的 `.placeholder` | 使用 `//go:embed all:dist`，并保留占位文件 |
| Go 测试找不到真实前端 | 测试前没有构建/复制前端 | 质量门禁先构建前端并复制到嵌入目录 |
| 发布二进制打开页面为空 | Artifact 下载到错误目录 | 下载到 `go:embed` 实际目录，并验证非空 |
| CI `sync /dev/stdout: invalid argument` | Linux pipe 不支持日志 fsync | 忽略 stdout/stderr 的已知 `EINVAL` 等非致命错误，不跳过测试 |
| GitCommit 是 40 位 | 直接注入 `GITHUB_SHA` | checkout 后执行 `git rev-parse --short HEAD` |
| BuildTime 少 8 小时 | Runner 默认 UTC且未设置 TZ | 显式 `TZ=<BUILD_TIMEZONE>` 后格式化 |
| BuildTime 是 `+0800` | `%z` 不带冒号 | 用 sed 规范成 `+08:00` |
| Makefile sed 行尾匹配失效 | `$` 被 Make 消费 | Makefile 正则中使用 `$$` |
| Release 只有 Full Changelog 链接 | 使用 GitHub 自动 Release notes | checkout 全历史，自行生成 Tag 间 Commit 列表并使用 `body_path` |
| Release Job 找不到矩阵文件 | Job 文件系统互不共享 | build 上传 Artifact，release 下载并合并 |
| 重新运行旧 Tag 仍失败 | Tag 仍指向旧 Workflow 提交 | 推送修复提交后创建新补丁 Tag |
| GHCR 推送 403 | 缺少 `packages: write` 或组织限制 | 检查 Job 权限和组织 Actions/Package 策略 |
| GHCR 镜像名被拒绝 | owner/image 含大写 | 在 Bash 中转换 `${GITHUB_REPOSITORY,,}` |
| ARM64 镜像构建很慢 | QEMU 模拟运行 | 使用缓存，评估原生 ARM Runner；不要误判为死锁 |
| CGO 镜像启动缺库 | Builder 与 Runtime 动态库不一致 | Debian 系列保持一致，安装匹配运行库，逐架构验证 |
| Alpine 中运行失败 | musl 与 glibc/CGO 不兼容 | 使用 Debian Bookworm builder 与 slim runtime |
| Compose 一直 unhealthy | 端口、路径、curl 或启动时间错误 | 检查实际监听地址、健康路径、curl 安装和 `start_period` |
| Bark step 失败导致发布失败 | 未设置非阻断 | 通知 step 增加 `continue-on-error: true` |

## 21. 禁止事项

- 禁止未填写参数表就直接复制模板。
- 禁止把 FileDock 的应用名、端口、路径、平台矩阵直接套到其他项目。
- 禁止假定所有 Go 项目都能 `CGO_ENABLED=0`。
- 禁止对 CGO 项目只修改一个环境变量而不验证工具链与动态库。
- 禁止使用 Alpine 作为本规范默认 Go 构建或运行镜像。
- 禁止使用浮动 Bun `latest` 或 Action `@main`。
- 禁止在 Docker 镜像中写入 Secret、生产配置或 Registry 凭据。
- 禁止上传裸二进制与未校验产物。
- 禁止让通知失败改变正式发布结果。
- 禁止用 Cache 替代 Artifact。
- 禁止让 Docker Workflow 与二进制 Workflow 同时修改同一 GitHub Release。
- 禁止在 Dockerfile 添加健康检查；本规范统一由 Compose 或部署平台配置。
- 禁止重写公开 Tag 作为常规修复方式；优先发布新补丁版本。

## 22. 最终交付说明模板

实施完成后，向项目负责人至少报告：

```markdown
构建发布能力已完成。

- 交付类型：<二进制 / Docker / 两者>
- Tag 规则：v数字.数字.数字
- Go 版本：<GO_VERSION>
- CGO：<0/1及原因>
- 前端类型：<纯Go/内嵌Bun/独立部署>
- Bun 版本：<版本或不适用>
- 二进制平台：<列表或不适用>
- Docker 平台：linux/amd64、linux/arm64 或不适用
- Registry：<GHCR地址或不适用>
- BuildTime：<时区与示例>
- 通知：<Bark/其他/未启用>
- 必配 Secret：<BARK_KEY或无>

验证结果：
- Go 测试：<通过/失败>
- 前端测试与构建：<通过/不适用/失败>
- 二进制矩阵：<通过/不适用/失败>
- 压缩包与校验和：<通过/不适用/失败>
- Docker Manifest：<通过/不适用/失败>
- 非 root 与健康检查：<通过/不适用/失败>

仍需人工操作：
1. <配置 Secret、检查 GHCR 可见性等>
2. 推送包含 Workflow 的提交。
3. 创建并推送新的稳定版本 Tag。
```

规范实施的成功标准不是“Workflow 文件存在”，而是目标产物可下载或可拉取、版本信息正确、目标架构真实可用、失败路径能阻断发布、部署健康检查有效，并且维护者清楚所有外部配置要求。
