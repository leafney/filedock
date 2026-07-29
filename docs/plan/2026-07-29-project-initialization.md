Related discussion: [docs/discuss/2026-07-29-project-initialization.md](../discuss/2026-07-29-project-initialization.md)

# FileDock 基础架构初始化 PRD

状态：待用户批准

日期：2026-07-29

## Problem Statement

FileDock 当前只有项目说明和需求文档，还没有可编译、可运行、可测试的前后端工程骨架。后续会实现匿名会话、房间、文件传输、SSE 和聊天等功能；如果直接开始业务编码，目录边界、依赖注入、数据库生命周期、配置方式、路由位置、响应结构、前端构建和静态资源发布方式容易出现多套实现。

本次工作必须先建立一个可运行的单服务架构。它应严格遵循单服务架构规范，以 `invoice-flow` 作为实现参考，同时不得复制其发票业务。初始化结果必须足够明确，使不了解上下文的低智能编码模型也能按固定任务顺序完成，不需要自行猜测技术选型、文件位置、构造顺序或验收标准。

## Solution

建立 FileDock 单仓单服务基础架构：

- 后端使用 Go、Fiber、Wire、GORM、`github.com/libtnb/sqlite` 和 `zlogx`。
- 配置使用 TOML，默认路径为 `data/config.toml`。
- SQLite 是唯一数据库，启动时完成连接、PRAGMA 验证和资源装配，失败时整个服务启动失败。
- 只提供 `/version` 服务级接口，不实现任何业务接口。
- 复制 `invoice-flow/pkg` 的全部通用包和测试，并把内部 import 修改为 FileDock 模块路径。
- 前端使用 React、TypeScript、Vite、Tailwind CSS 和已确认的完整依赖集合。
- 前端首页调用 `/version`，用于验证前后端、代理、构建、静态资源复制和 Go embed 链路。
- 生产构建把前端产物复制到 Go 嵌入目录，通过 Fiber 提供静态资源和 SPA 回退。
- 使用最小但有效的自动化测试验证配置、SQLite、版本接口、静态资源和迁入通用包。

## User Stories

1. 作为后续开发者，我希望项目具有固定的单服务目录结构，以便新增业务时能明确选择 API、业务、数据访问、模型、DTO 或基础服务层。
2. 作为后续开发者，我希望所有核心依赖由 Wire 显式装配，以便快速理解真实依赖关系并避免全局容器。
3. 作为后续开发者，我希望服务器构造完成时已经注册中间件、路由和静态资源，以便运行阶段只负责启动服务。
4. 作为后续开发者，我希望使用统一 TOML 配置，以便开发环境通过一个固定文件配置应用、HTTP、日志和 SQLite。
5. 作为开发环境使用者，我希望配置文件缺失时应用仍能使用默认值启动，以便快速运行工程骨架。
6. 作为开发环境使用者，我希望配置文件损坏时应用明确失败，以便立即发现配置问题。
7. 作为后续开发者，我希望 SQLite 路径为空时自动落到默认数据目录，以便减少重复配置。
8. 作为运维人员，我希望 SQLite 无法连接时服务拒绝启动，以便避免对外提供不可用服务。
9. 作为后续开发者，我希望 SQLite 自动启用 WAL、外键和忙等待，以便后续业务从一致的数据库基础开始。
10. 作为后续开发者，我希望数据库连接由应用统一关闭，以便测试和进程退出时不泄漏文件句柄。
11. 作为后续开发者，我希望项目统一使用 `zlogx`，以便服务日志和 GORM 日志共享同一日志能力。
12. 作为后续开发者，我希望复用参考项目的通用包，以便后续直接获得缓存、加密、错误、响应、日志、校验等公共能力。
13. 作为代码维护者，我希望迁入通用包的原有测试继续通过，以便确认模块路径修改没有破坏行为。
14. 作为监控或开发人员，我希望通过 `/version` 同时查看进程状态和构建信息，以便进行轻量存活判断和版本排查。
15. 作为安全维护者，我希望版本接口不暴露数据库路径等内部信息，以便减少无必要的信息泄露。
16. 作为前端开发者，我希望开发服务器代理后端路径，以便在不开启全局 CORS 的前提下完成本地联调。
17. 作为前端开发者，我希望所有已选前端依赖在初始化阶段安装完成，以便下一阶段直接开发业务。
18. 作为前端开发者，我希望首页能展示服务状态和版本，以便确认代理和后端连接正常。
19. 作为部署人员，我希望前端产物能被复制到后端静态目录并嵌入单个 Go 二进制，以便无须独立部署 Web 服务。
20. 作为浏览器用户，我希望刷新前端 SPA 路由仍能返回首页，以便后续直接访问房间路由不会出现静态文件 404。
21. 作为开发者，我希望通过 Makefile 使用一致命令安装依赖、启动前端、构建前端、运行后端、生成 Wire 和执行测试。
22. 作为开发者，我希望 Air 热更新配置可直接使用，以便后续后端开发无需手动重启。
23. 作为代码审查者，我希望初始化不包含任何会话、房间、文件、聊天或 SSE 业务代码，以便架构提交保持聚焦。
24. 作为代码审查者，我希望空的 DAL 和模型层不包含虚假接口或占位业务实现，以便遵守最少样板原则。
25. 作为后续编码模型，我希望每个任务有明确输入、输出、禁止事项和验收方式，以便无需猜测实现意图。
26. 作为项目负责人，我希望每次重大文件变更后暂停确认，以便及时纠正低智能编码模型的偏差。

## Implementation Decisions

### 1. 规范优先级与范围边界

实现时按以下优先级处理冲突：

1. 本 PRD 中已经确认的明确决策。
2. `docs/first/architecture-spec-single.md` 中的架构约束。
3. `docs/first/LanShare_PRD.md` 中的功能和产品约束。
4. `invoice-flow` 中可复用的具体实现方式。

本阶段只实现基础架构。禁止实现以下内容：

- 匿名用户与会话。
- Cookie 鉴权。
- 房间创建、加入、延期、离开和销毁。
- 文件上传、下载、接受、拒绝和撤回。
- 成员在线状态。
- SSE、聊天、预览、清理任务和管理后台。
- 任何业务数据表或业务数据库迁移。

### 2. 项目身份与工具版本

- 项目展示名称统一为 `FileDock`。
- 二进制名称统一为 `filedock`。
- Go module 必须为 `github.com/leafney/filedock`。
- Go 版本使用当前仓库已经声明的 `1.25.12`。
- 后端框架使用 Fiber v2，不升级到 Fiber v3。
- 依赖注入使用 Google Wire。
- 前端包管理器和脚本运行器统一使用 Bun。
- 不引入第二套日志框架、第二套 ORM 或第二个数据库。

### 3. 最终目录和文件清单

编码模型必须创建或完善以下文件。没有列出的业务文件不得自行新增。

根目录：

- `.air.toml`：Air 热更新配置。
- `.gitignore`：忽略运行数据、构建产物、缓存和依赖目录。
- `Makefile`：统一开发和构建命令。
- `README.md`：项目说明、环境要求、开发启动和构建说明。
- `go.mod`、`go.sum`：Go 模块和依赖。
- `main.go`：进程入口、参数解析、构建信息注入、Wire 初始化和运行。

配置层：

- `config/config.go`：配置结构、默认值、加载和开发期准备逻辑。
- `config/config_test.go`：配置测试。
- `config/config.example.toml`：可复制的示例配置。

核心层：

- `core/app.go`：应用生命周期和资源关闭。
- `core/build.go`：构建信息结构。
- `core/request_log.go`：Fiber 请求日志中间件。
- `core/router.go`：全部路由注册。
- `core/server.go`：Fiber 构造、错误处理、中间件、静态资源、监听和关闭。
- `core/server_test.go`：版本路由与静态回退测试。

内部层：

- `internal/api/version.api.go`：版本 HTTP handler。
- `internal/biz/version.biz.go`：组装版本响应和开发期状态。
- `internal/dto/version.dto.go`：版本响应 DTO。
- `internal/service/version.svc.go`：只读构建信息服务。
- `internal/dal/.gitkeep`：保留数据访问目录，不添加代码。
- `internal/model/.gitkeep`：保留模型目录，不添加代码。

静态资源与依赖注入：

- `static/embed.go`：嵌入静态构建目录并返回子文件系统。
- `static/dist/.placeholder`：确保未构建前 Go embed 有输入文件。
- `wire/provider.go`：所有 provider，包括通过 `pkg/gormx` 直接构造 SQLite 数据库。
- `wire/provider_test.go`：SQLite provider 与 PRAGMA 集成测试。
- `wire/wire.go`：Wire 注入声明。
- `wire/wire_gen.go`：只允许 Wire 生成，禁止手写业务逻辑。
- `wire/wire_test.go`：验证生成结果能构造最小应用。

前端：

- `frontend/package.json`、`frontend/bun.lock`：依赖与锁文件。
- `frontend/index.html`：Vite 入口。
- `frontend/tsconfig.json`、`frontend/tsconfig.app.json`、`frontend/tsconfig.node.json`：TypeScript 配置。
- `frontend/vite.config.ts`：React 插件、开发代理和构建配置。
- `frontend/tailwind.config.js`、`frontend/postcss.config.js`：Tailwind CSS v3 配置。
- `frontend/src/main.tsx`：React 根节点、QueryClient 和 Router provider。
- `frontend/src/app.tsx`：最小首页路由和状态页。
- `frontend/src/styles.css`：Tailwind 指令和最小全局样式。
- `frontend/src/lib/api-client.ts`：axios 最小客户端。
- `frontend/src/services/version.ts`：版本接口请求函数。
- `frontend/src/types/version.ts`：版本响应类型。
- `frontend/src/vite-env.d.ts`：Vite 类型声明。

通用包：

- 把 `invoice-flow/pkg` 下全部 31 个现有源码与测试文件按原目录复制到 FileDock 的 `pkg`。
- 不得遗漏测试文件。
- 不得复制 `invoice-flow` 的 `core`、`config`、`internal`、`data`、`frontend` 或任何发票业务文件。

### 4. `internal` 文件命名与方法设计

所有内部文件必须按架构规范使用职责后缀：

- API：`xx.api.go`。
- 业务：`xx.biz.go`。
- DTO：`xx.dto.go`。
- 数据访问：`xx.dal.go`。
- 模型：`xx.m.go`。
- 基础服务：`xx.svc.go`。

本阶段版本链固定为：

`VersionAPI -> VersionBiz -> VersionSvc`

职责必须严格分开：

- `VersionAPI` 只处理 Fiber 上下文并输出 JSON，不读取构建全局变量，不拼装业务数据。
- `VersionBiz` 生成最终 DTO，把开发期 `status` 固定为 `ok`。
- `VersionSvc` 只保存和返回 Wire 注入的构建信息。
- 版本链不需要 DAL，因为构建信息不来自持久化存储。
- 禁止为了填满目录而创建空 DAL、空模型、单实现接口或全局注册器。

构造函数命名固定采用 `NewXxx` 风格。核心依赖必须使用具体强类型参数，不得使用 `...interface{}`、map 容器、AppCtx 或 Service Locator。

### 5. 配置契约

默认配置路径固定为 `data/config.toml`。命令行允许通过 `-config` 指定其他路径。

配置只包含四组：

- `app`：`name`、`env`、`data_dir`。
- `http`：`addr`。
- `log`：`enable`、`level`、`output`、`file`、`caller`。
- `sqlite`：`path`。

默认值固定为：

- 应用名称：`filedock`。
- 环境：`dev`。
- 数据目录：`data`。
- HTTP 地址：`:8095`。
- 日志启用：`true`。
- 日志级别：`info`。
- 日志输出：`stdout`。
- 日志文件：`data/logs/filedock.log`。
- 显示调用位置：`true`。
- SQLite 配置路径为空时，最终路径为 `data/filedock.db`。

示例配置中的 SQLite 必须是一级 TOML 表，不能嵌套在 `storage` 下。示例语义为 `[sqlite]` 下提供空 `path`。

加载规则：

1. 传入路径为空时改用默认路径。
2. 配置文件存在时，在代码默认值之上解码 TOML。
3. TOML 无法解析时返回包含文件路径的明确错误。
4. 配置文件不存在时继续使用代码默认值，不自动生成文件。
5. 空字符串字段按上述默认值补齐。
6. 创建数据目录、SQLite 父目录和文件日志父目录。
7. 目录创建失败时返回错误，终止 Wire 初始化。

正式部署需要的环境变量覆盖、敏感配置、严格未知字段校验和真实健康检查不在本阶段，已记录到待办文档。

### 6. 通用包迁移契约

迁移步骤必须机械、可审查：

1. 复制参考项目 `pkg` 的全部目录、源码和测试。
2. 把所有 `github.com/leafney/invoice-flow/pkg/...` import 改为 `github.com/leafney/filedock/pkg/...`。
3. 不改变导出类型、函数签名和测试意图。
4. 只允许为模块路径和依赖版本兼容做必要修改。
5. 禁止把暂未使用包接入 Wire。
6. 禁止把 BadgerDB 或 Redis 变成 FileDock 运行依赖。
7. 禁止删除 `pkg/utils` 等现有目录；全量复制是本 PRD 对最少样板原则的明确例外。
8. 合并参考项目通用包所需的 Go 依赖版本，再执行 `go mod tidy`。

未使用通用包不会被 Go 链接到最终二进制，但其测试必须能通过 `go test ./...` 编译和执行。

### 7. 日志契约

- 应用核心日志使用 `pkg/zlogx.ZLogSvc`。
- Logger 由 Wire 根据配置构造并注入，不使用包级业务全局变量。
- SQLite provider 必须把同一个 logger 实例传给 `pkg/gormx`。
- Fiber 使用请求日志中间件记录方法、路径、状态码和耗时。
- 请求日志中间件不得读取尚不存在的用户或房间信息。
- 应用关闭时调用 logger 的同步方法；终端不支持同步造成的 `ENOTTY` 或 `EBADF` 不应把正常退出变成失败。
- 不引入 zerolog。

### 8. SQLite 与 `pkg/gormx` 契约

SQLite 使用 GORM 和 `github.com/libtnb/sqlite`，并直接复用迁入的 `pkg/gormx`。禁止再创建 `internal/service/sqlite.svc.go`、`SQLiteDBSvc` 或其他 SQLite 包装服务。

Wire provider 固定返回 `*gormx.GormDBSvc`。provider 负责根据配置构造 SQLite DSN，并把 `sqlite.Open(dsn)`、连接参数、PRAGMA 验证回调和 logger 传给 `gormx.NewGormDBSvc`。API、业务层和版本接口不得依赖数据库。

SQLite DSN 必须表达以下设置：

- 事务锁：`immediate`。
- journal mode：`WAL`。
- synchronous：`NORMAL`。
- foreign keys：开启。
- busy timeout：`5000ms`。

打开顺序：

1. SQLite provider 检查配置和 logger 均已注入。
2. 使用最终 SQLite 路径构建 DSN。
3. SQLite provider 构造 `gormx.Options`：Dialector 使用 `sqlite.Open(dsn)`，`OnOpen` 负责验证 PRAGMA，`AfterOpen` 负责设置连接数。
4. SQLite provider 调用 `gormx.NewGormDBSvc`。
5. `pkg/gormx` 打开 GORM，并通过 `OnOpen` 验证 WAL、外键和 busy timeout。
6. `pkg/gormx` 获取底层 `sql.DB` 并执行 Ping。
7. `pkg/gormx` 通过 `AfterOpen` 把最大打开连接数和最大空闲连接数均设为 1。
8. 任一步失败时由 `pkg/gormx` 关闭已打开资源并返回错误，provider 再补充 SQLite 上下文。

关闭规则：

- 应用直接持有 `*gormx.GormDBSvc` 并拥有其生命周期。
- 应用通过 `sync.Once` 保证关闭流程可重复调用，底层 `gormx.Close()` 只执行一次。
- 本阶段不执行 `AutoMigrate`，不建立迁移服务，不创建任何业务表。
- API 和业务层不得接触 SQLite Dialector。
- SQLite 专用 DSN 和 PRAGMA 逻辑只留在 Wire provider 内，不得散落到 API、biz、core 或版本 service。

### 9. 版本接口契约

只注册 `GET /version`，禁止注册 `/health`。

成功响应使用 HTTP 200 和直接 JSON，不使用业务响应包装。字段固定为：

- `status`：开发阶段固定字符串 `ok`。
- `version`：构建版本，开发默认 `dev`。
- `git_branch`：Git 分支，开发默认 `unknown`。
- `git_commit`：Git 提交，开发默认 `unknown`。
- `build_time`：构建时间，开发默认 `unknown`。

禁止加入 `service` 字段。禁止返回配置路径、数据库路径、日志路径、环境变量或其他运行内部信息。

本阶段不在每次请求时 Ping SQLite。理由是 SQLite 是启动必需依赖，连接失败时服务不会启动。正式部署前再扩展 `status` 的真实验证。

### 10. Fiber、路由和静态资源契约

服务器构造阶段必须完成：

- 创建 Fiber App。
- 安装全局 panic recover。
- 安装请求日志中间件。
- 安装统一错误处理器。
- 调用核心路由注册函数。
- 获取嵌入文件系统并注册静态资源。
- 注册 SPA 回退。

路由规则：

- 所有路由只在核心路由模块中注册。
- `/version` 必须在静态资源中间件之前注册。
- 不创建 `/api` 业务路由。
- 不开启全局 CORS。

SPA 回退规则：

- 静态文件正常返回对应资源。
- 非 API 的 GET 路径找不到物理文件时返回前端 `index.html`。
- `/version`、以 `/api/`、`/open/`、`/public/` 开头的路径不得被回退为 HTML。
- 静态资源或嵌入文件系统构造失败时，服务器构造失败。

`Run()` 只负责监听配置地址，不得补注册路由、中间件或静态资源。服务器提供带 5 秒超时的优雅关闭。

### 11. 应用生命周期与 Wire 契约

应用对象至少持有配置、logger、`*gormx.GormDBSvc` 和 Fiber 服务器。

Wire 构造顺序必须表达以下依赖：

1. 配置。
2. logger。
3. 由 SQLite provider 构造的 `*gormx.GormDBSvc`。
4. 版本服务、业务和 API。
5. Fiber 服务器。
6. 应用对象。

进程入口职责：

- 解析 `-config`。
- 支持 `-version` 在不启动服务时打印构建信息。
- 构造 BuildInfo 并传给 Wire。
- Wire 初始化失败时打印错误并以非零状态退出。
- 应用运行失败时打印错误并以非零状态退出。
- 不注册路由，不直接创建数据库，不手工拼复杂依赖。

应用运行职责：

- 监听 SIGINT 和 SIGTERM。
- 启动 HTTP 服务并等待退出信号或服务器错误。
- 退出时按服务器、数据库、logger 的顺序关闭。
- 合并多个关闭错误，不静默丢失错误。
- 构造完成后所有必需依赖必须非空。

### 12. 前端依赖与初始化契约

前端必须一次性引入以下依赖，虽然部分依赖将在后续功能阶段使用：

- React 18 和 React DOM 18。
- React Router v6。
- TanStack Query v5。
- Zustand。
- Tailwind CSS v3、PostCSS 和 Autoprefixer。
- axios。
- `@microsoft/fetch-event-source`。
- idb。
- Lucide React。

不得引入 Ant Design。不得使用原生 fetch 请求 `/version`。

前端启动层必须安装：

- React 严格模式。
- QueryClientProvider。
- BrowserRouter。

最小首页行为：

1. 页面加载后，通过独立版本 service 调用 axios 客户端请求 `/version`。
2. 加载期间显示明确的检查状态。
3. 成功后显示 `FileDock`、`status` 和 `version`。
4. 失败后显示服务暂不可用提示，并允许浏览器刷新重试。
5. 不实现任何业务表单、房间页面、文件选择或 SSE 连接。

TanStack Query 用于管理该版本请求。Zustand、idb、Lucide React 和 `fetch-event-source` 只完成依赖安装，不需要创建虚假 store、数据库或 SSE 管理器。

### 13. Vite、静态构建与 Go embed 契约

开发环境：

- Vite 默认端口使用 `5173`。
- `/version` 代理到 `http://127.0.0.1:8095`。
- `/api` 也预留代理到同一后端，供下一阶段使用。
- 不通过后端开放宽泛 CORS。

构建环境：

1. `bun run build` 输出到前端自身的 `dist`。
2. `make web` 删除旧的后端静态构建目录。
3. `make web` 把整个前端构建产物复制到后端静态构建目录。
4. Go embed 只嵌入后端静态构建目录，不直接嵌入前端 `dist`。
5. Go 二进制运行时不读取前端源码或外部静态目录。

为了让静态 SPA 回退可以独立测试，静态注册逻辑应把“获取生产嵌入文件系统”和“向 Fiber 注册一个已给定文件系统”分成两个小函数。生产构造使用真实 embed FS，测试使用内存文件系统提供 `index.html`。不得要求 `go test ./...` 先运行前端构建。

### 14. Makefile 与 Air 契约

Makefile 的默认目标为帮助信息。必须提供：

- `deps`：进入前端目录运行 `bun install`。
- `dev`：启动 Vite 开发服务器。
- `web`：构建前端并同步后端静态目录。
- `run`：使用默认配置路径执行 Go 服务。
- `build`：先构建前端，再构建当前平台二进制。
- `build-lin`：构建 Linux AMD64。
- `build-lin-arm`：构建 Linux ARM64。
- `air`：使用 Air 配置启动热更新。
- `wire`：运行 Wire 生成代码。
- `tidy`：运行 `go mod tidy`。
- `test`：运行 `go test ./...`。
- `clean`：清理二进制、前端 dist 和后端静态构建产物，并恢复 embed 占位文件。

本阶段不提供 `proto`、Windows 或 macOS 专用发布目标。

构建信息通过 ldflags 写入 `Version`、`GitBranch`、`GitCommit` 和 `BuildTime`。开发运行时使用默认值。

### 15. Git 忽略规则

至少忽略：

- `bin/`。
- `data/`。
- `tmp/`。
- `.gocache/`。
- `node_modules/`。
- `frontend/dist/`。
- `static/dist` 下的生成文件，但保留 embed 占位文件。
- `.playwright-cli/`。
- 常见 Go 测试产物、系统文件和环境文件。

不得提交开发数据库、日志、实际配置文件、上传文件或第三方依赖目录。

### 16. 实施任务顺序与停顿点

低智能编码模型必须严格按以下顺序执行。每个任务完成、展示变更和验证结果后停止，等待用户确认，再开始下一任务。

#### 任务 1：根工程与配置骨架

- 完善 Go module、Git 忽略、Makefile、Air 和配置层。
- 不复制通用包，不写服务器。
- 完成配置测试并执行对应测试。
- 停止并等待确认。

#### 任务 2：通用包迁移

- 全量复制 `invoice-flow/pkg`。
- 修改内部 module import。
- 合并 Go 依赖并运行通用包测试。
- 不修改通用包公共行为。
- 停止并等待确认。

#### 任务 3：SQLite provider 与数据库装配

- 在 Wire provider 中使用 `pkg/gormx.NewGormDBSvc` 直接构造数据库。
- 实现 SQLite DSN、PRAGMA 验证、连接参数和 provider 集成测试。
- 不创建模型、表或迁移服务。
- 不创建 `internal/service/sqlite.svc.go` 或任何 SQLite 包装服务。
- 执行 SQLite provider 测试。
- 停止并等待确认。

#### 任务 4：版本链、核心服务和 Wire

- 实现构建信息、版本 service、biz、DTO、API。
- 实现请求日志、路由、服务器、应用生命周期和进程入口。
- 在已有 SQLite provider 基础上补齐版本、服务器和应用 provider，完成注入声明并生成 Wire 文件。
- 实现后端版本契约测试。
- 停止并等待确认。

#### 任务 5：前端与静态嵌入

- 创建前端依赖、配置、最小首页和版本请求。
- 实现 Vite 代理、Tailwind、构建同步、embed 和 SPA 回退。
- 执行前端构建和静态回退测试。
- 停止并等待确认。

#### 任务 6：文档与全量验收

- 更新 README。
- 运行格式化、Wire、依赖整理、后端全量测试、前端构建和 Go 构建。
- 检查没有业务接口、业务模型和运行数据进入提交。
- 汇总结果并停止，等待最终确认。

### 17. 禁止实现清单

编码模型不得：

- 自行改变已确认技术栈。
- 把 `/health` 加回来。
- 在 `/version` 中查询 SQLite。
- 在 `main.go` 注册路由或创建数据库。
- 在 API 层直接访问 GORM。
- 在业务层返回 Fiber 响应。
- 创建 AppCtx、全局依赖容器、自动注册中心或单实现接口。
- 创建空迁移服务、业务表或占位模型。
- 把 Redis、BadgerDB、S3、JWT 接入运行链。
- 引入 Ant Design、原生 EventSource 或第二套 HTTP 客户端。
- 开启允许任意来源的 CORS。
- 手写带业务逻辑的 `wire_gen.go`。
- 删除或覆盖用户现有文档与无关改动。
- 未经用户确认连续完成多个重大任务。

### 18. Git 提交约束

除非用户明确要求，否则实施过程中不要自行提交。

用户要求提交时：

- 提交标题必须使用 Conventional Commits。
- `type` 和 `scope` 使用英文。
- 标题 description 只使用简体中文，不混入英文路径。
- PRD 路径放在提交正文，避免违反标题语言规则。
- 推荐标题：`chore(architecture): 初始化项目基础架构`。
- 提交正文注明：`PRD: docs/plan/2026-07-29-project-initialization.md`。

## Testing Decisions

### 测试原则

- 不采用全面 TDD；完成对应模块后补充必要测试。
- 只验证对外行为、关键配置结果、资源生命周期和协议契约。
- 不断言私有函数的具体实现步骤。
- 测试必须可重复，不依赖开发者真实 `data` 目录、现有配置或互联网。
- 文件系统和 SQLite 测试使用临时目录。
- 不设置覆盖率门槛。

### 配置测试

必须覆盖：

1. 配置文件不存在时返回默认配置。
2. 默认 HTTP 地址为 `:8095`。
3. SQLite path 为空时变为临时测试数据目录下的默认数据库名，测试不得污染仓库 `data`。
4. 有效 TOML 能覆盖应用、HTTP、日志和 SQLite 字段。
5. 非法 TOML 返回错误。
6. 必需目录无法创建时返回错误。

### SQLite provider 测试

测试放在 Wire provider 对应测试中，必须覆盖：

1. 使用临时数据库文件成功打开。
2. `PRAGMA journal_mode` 为 WAL。
3. `PRAGMA foreign_keys` 为 1。
4. `PRAGMA busy_timeout` 不小于 5000。
5. 底层最大打开连接数和最大空闲连接数均为 1。
6. 关闭成功，重复关闭不产生不可接受错误。
7. 不创建任何业务表。

### 版本接口测试

必须覆盖：

1. `GET /version` 返回 HTTP 200。
2. 返回字段恰好包含 `status`、`version`、`git_branch`、`git_commit` 和 `build_time`。
3. `status` 等于 `ok`。
4. Wire 注入的构建信息原样进入响应。
5. 响应不包含 `service`。
6. `/health` 不存在，不能返回成功。

### 静态资源测试

必须使用内存文件系统，不依赖先构建前端，并覆盖：

1. 已存在静态文件可以返回。
2. 未知普通 GET 路径回退到 `index.html`。
3. `/version` 不被 SPA 回退覆盖。
4. `/api/...` 不被 SPA 回退为 HTML。

### 通用包测试

- 保留并运行复制过来的全部原有测试。
- 模块 import 替换后测试语义不变。
- 若依赖升级造成编译问题，只做最小兼容修改，并在交付说明列出。

### 前端测试

- 本阶段不编写组件单元测试。
- TypeScript 编译和 Vite 生产构建成功即作为前端自动验收。
- 手工检查加载、成功和失败三种状态。

### 最终验收命令

按顺序执行：

1. `bun install`。
2. `bun run build`。
3. `make web`。
4. `make wire`。
5. `gofmt` 格式化全部新增 Go 文件。
6. `go mod tidy`。
7. `go test ./...`。
8. `go build ./...`。

最终手工验收：

1. 使用预先创建的开发配置启动服务。
2. 确认监听 `:8095`。
3. 请求 `/version` 并核对五个字段。
4. 浏览器打开根页面，确认显示 FileDock、状态和版本。
5. 刷新一个未知前端路由，确认返回 SPA 首页。
6. 停止服务，确认 HTTP、SQLite 和日志资源正常关闭。

## Out of Scope

- 所有 FileDock 业务功能和业务接口。
- 数据库迁移与业务表。
- 配置热更新、环境变量覆盖、敏感配置管理和部署级严格校验。
- `status` 的数据库、磁盘或其他依赖实时探测。
- Redis、BadgerDB、对象存储和缓存接入。
- SSE 连接、状态管理器和 IndexedDB 数据结构。
- 前端业务页面、视觉设计和组件测试。
- CORS 白名单配置。
- protobuf 和 gRPC。
- Windows 与 macOS 专用发布命令。
- Docker、systemd、Nginx 和正式部署文档。
- Git 提交、推送、PR 或外部 issue 创建。

## Further Notes

- 架构规范已在讨论阶段修正为只保留 `/version`，不得再从旧参考项目复制 `/health`。
- `docs/first/todo.md` 已记录正式部署前完善配置加载与真实健康状态验证。
- 当前工作区存在用户已有或未跟踪文件；实施时必须保留，不得清理或覆盖无关内容。
- `invoice-flow` 只是参考项目。复制通用包时不得把发票领域代码、配置默认值、端口、名称、路由或 UI 文案带入 FileDock。
- 本 PRD 获得用户明确批准前，不得开始基础架构代码实现。
