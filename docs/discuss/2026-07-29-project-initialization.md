# FileDock 项目基础架构初始化讨论记录

日期：2026-07-29

## 原始需求

> 根据 `docs/first/architecture-spec-single.md` 文档和当前提供的参考项目 `invoice-flow`，先将当前项目架构进行初始化配置。

本次讨论使用 `grill-me` 逐项确认架构范围。项目架构实现以 `docs/first/architecture-spec-single.md` 为准，功能逻辑以 `docs/first/LanShare_PRD.md` 为准，`invoice-flow` 作为具体实现参考。

## 问答记录

### Q1：初始化范围做到多深？

**推荐答案：** 只实现可运行架构骨架，包括 Go、Fiber、Wire、SQLite、TOML 配置、日志、服务级接口、前端骨架、静态资源嵌入和构建命令，不实现会话、房间、文件传输等业务。

**用户答案：** 架构实现遵循架构规范，功能逻辑遵循产品文档；初始化阶段只实现基础架构，不实现具体功能接口。

### Q2：初始化是否实际接通 SQLite？

**推荐答案：** 实际接通，启用 WAL，验证连接并支持优雅关闭；不创建业务表。

**用户答案：** 确认。

### Q3：项目正式名称使用 FileDock 还是 LanShare？

**推荐答案：** 统一使用 FileDock，模块、二进制、配置和日志均使用 `filedock`；LanShare 文档仅作为需求来源。

**用户答案：** 确认使用 FileDock，配置保持默认 `config.toml`。

### Q4：SQLite 使用什么驱动与数据访问方式？

**推荐答案：** 最初建议 `database/sql` 与纯 Go SQLite 驱动，不引入 ORM。

**用户答案：** 改为 `github.com/libtnb/sqlite` 与 GORM；同时把参考项目 `invoice-flow` 的全部 `pkg` 目录复制到当前项目，并修改 import。

### Q5：`pkg` 全量迁移边界是什么？

**推荐答案：** 复制全部源码和测试；改为 FileDock 模块路径；保留 Badger、Redis、S3、JWT 等暂未使用包，但不接入 Wire；全部原有测试必须通过；接受新增依赖。

**用户答案：** 确认。

### Q6：默认配置文件不存在时如何处理？

**推荐答案：** 默认读取 `data/config.toml`；文件不存在时用代码默认值启动，不自动生成；提交配置示例；配置错误或目录创建失败时启动失败。

**用户答案：** 确认。开发阶段会预先创建配置文件，正式部署时再完善配置加载逻辑；要求在 `docs/first/todo.md` 记录待办。

### Q7：核心日志使用哪个实现？

**推荐答案：** 使用迁入的 `pkg/zlogx`，通过 Wire 显式注入，不再引入 zerolog。

**用户答案：** 使用 `zlogx`。

### Q8：初始化阶段如何处理数据库迁移？

**推荐答案：** 因当前没有业务模型，不创建空迁移服务；首批业务模型加入时再建立迁移能力。

**用户答案：** 先不考虑迁移操作。

### Q9：首版配置包含哪些内容？

**推荐答案：** 只包含应用、HTTP、日志和 SQLite 基础配置，不加入房间、传输、预览、管理等业务配置。

**用户答案：** 确认；数据库只考虑 SQLite，使用一级配置 `[sqlite]`，其中 `path = ""`。

### Q10：HTTP 默认监听端口是什么？

**推荐答案：** 最初建议 `:8080`，使局域网设备可直接访问。

**用户答案：** 改用 `:8195`。

### Q11：服务级接口保留哪些内容？

**推荐答案：** 最初建议分别提供 `/health` 与 `/version`。

**用户答案：** 删除 `/health`，只保留 `/version`；把 `status` 合入版本响应，删除 `service`；同时修正架构规范，只保留 `/version`。

### Q12：前端初始化依赖范围是什么？

**推荐答案：** 最初建议当前引入 React、TypeScript、Vite、Router、Tailwind、axios、TanStack Query，后续按需引入 idb、Lucide React、Zustand 和 `fetch-event-source`。

**用户追问：** idb、Lucide React、TanStack Query 分别用于什么？

**说明：** idb 用于 IndexedDB 缓存；Lucide React 用于界面图标；TanStack Query 用于服务端数据请求、缓存、重试和刷新。

**用户答案：** 所有依赖现在全部引入，后续直接进入功能开发。

### Q13：开发与生产环境如何处理跨域和静态资源？

**推荐答案：** 不开启全局 CORS；开发环境由 Vite 代理后端路径；生产环境通过 `go:embed` 内嵌 `static/dist`，由 Fiber 提供静态资源和 SPA 回退。

**用户答案：** 按参考项目实现，通过 Go embed 内嵌前端页面；完整沿用推荐构建链。

### Q14：最小前端首页是否调用 `/version`？

**推荐答案：** 调用现有 `/version` 并展示 FileDock 名称、服务状态、版本和失败提示，用于验证前后端、代理与嵌入链路。

**用户追问：** 前端调用与上面提供的版本接口有什么区别？

**说明：** `/version` 是后端 JSON 接口；前端调用是使用 axios 消费同一接口并展示结果，不新增接口。

**用户答案：** 确认调用。

### Q15：初始化阶段提供哪些构建命令？

**推荐答案：** 提供 `deps`、`dev`、`web`、`run`、`build`、`build-lin`、`build-lin-arm`、`air`、`wire`、`tidy`、`test`；当前无协议文件，不提供 `proto`；Windows 和 macOS 发布命令后续补充。

**用户答案：** 确认。

### Q16：初始化测试范围是什么？

**推荐答案：** 测试配置默认值、解析和非法配置；SQLite 连接与 PRAGMA、关闭；版本响应；静态资源嵌入和 SPA 回退；运行迁入 `pkg` 的全部原有测试。不写前端组件测试，不追求覆盖率。

**用户答案：** 确认。

### Q17：最终模块范围是否匹配？

**推荐答案：** 建立配置、通用包、SQLite 与版本服务、版本调用链、核心服务装配、Wire、前端、静态嵌入、构建工具和 README；DAL 与模型层保留目录但不写业务占位代码。

**用户答案：** 确认；补充要求 `internal` 各层文件命名与方法逻辑必须遵循参考项目和架构规范。

### Q18：`/version` 的 `status` 是否实时检查 SQLite？

**推荐答案：** 开发阶段不在每次请求中探测数据库；SQLite 是必需依赖，启动连接失败时整个服务启动失败；接口可响应时固定返回 `ok`。

**用户答案：** 确认。正式部署阶段再完善 `status` 的真实验证逻辑。

## 讨论期间完成的文档调整

- 已修正架构规范，删除所有 `/health` 初始化要求，只保留 `/version`。
- 已明确 `/version` 返回 `status`、`version`、`git_branch`、`git_commit`、`build_time`。
- 已创建开发待办，记录正式部署前完善配置加载和真实健康状态验证。

## 最终共同理解

### 范围与优先级

- 当前只初始化可运行基础架构，不实现会话、房间、成员、上传、下载、聊天、SSE 等业务。
- 架构冲突时，以架构规范为准；业务逻辑以产品文档为准；参考项目提供实现模式。
- 项目统一命名为 FileDock，Go module 为 `github.com/leafney/filedock`，默认 HTTP 地址为 `:8195`。

### 后端架构

- 使用 Go 1.25.12、Fiber v2、Wire、GORM、`github.com/libtnb/sqlite`、`zlogx` 和 `go:embed`。
- 使用显式依赖注入，核心依赖构造失败即启动失败；不使用 AppCtx、全局容器或运行期补初始化。
- SQLite 启用 WAL、外键、事务即时锁和忙等待，限制单连接，启动时 Ping 并验证 PRAGMA，应用退出时关闭。
- 本阶段不实现迁移服务，不创建业务表。
- 只提供 `/version` 服务级接口；开发阶段 `status` 固定为 `ok`，不逐请求探测 SQLite。

### 配置与日志

- 默认配置路径为 `data/config.toml`，示例配置位于 `config` 目录。
- 配置文件不存在时使用代码默认值，不自动写出配置文件；当前开发过程会预先创建配置。
- 配置只包含应用、HTTP、日志和一级 SQLite 配置；SQLite 路径为空时使用 `data/filedock.db`。
- 核心日志使用 `pkg/zlogx` 并由 Wire 注入。

### 通用包

- `invoice-flow/pkg` 的源码与测试全量复制，内部模块 import 改为 FileDock。
- 暂未使用的缓存、Redis、S3、JWT 等包保留，但不接入 FileDock 运行链。
- 所有迁入包的原有测试必须通过。

### 分层与命名

- 遵循 `api -> biz -> service` 的版本接口调用链；不为了空目录编写虚假业务实现。
- `internal` 文件按职责使用 `.api.go`、`.biz.go`、`.dto.go`、`.svc.go` 等后缀。
- DAL 和模型层暂时只保留结构位置，不实现业务模型或数据访问代码。
- 所有路由集中在核心路由模块注册；服务器构造阶段完成中间件、路由和静态资源装配。

### 前端与嵌入

- 使用 React 18、TypeScript、Vite、React Router、TanStack Query、Zustand、Tailwind CSS、axios、`@microsoft/fetch-event-source`、idb 和 Lucide React。
- 最小首页使用 axios 调用 `/version`，显示 FileDock、服务状态、版本和失败提示。

- 开发环境使用 Vite 代理，不开启全局 CORS。
- 前端构建输出复制到 `static/dist`，再通过 Go embed 内嵌；Fiber 提供静态资源和 SPA 回退。

### 构建、测试与交付

- Makefile 提供已确认的前端、运行、构建、Wire、整理和测试命令；本阶段不提供 proto、Windows 和 macOS 发布命令。
- 测试仅覆盖关键基础行为和迁入包原有测试，不采用全面 TDD，不设置覆盖率目标。
- 讨论记录完成后生成本地 PRD；用户明确批准 PRD 前不得开始架构代码实现。
- 实施阶段严格按 PRD 任务顺序推进，每次重大文件变更后等待用户确认。

## PRD 审阅补充

### 2026-07-29：修正 SQLite 装配方式

- 用户明确要求直接使用迁入的 `pkg/gormx` 构造并管理 GORM 数据库。
- 不创建 `internal/service/sqlite.svc.go`，不增加 `SQLiteDBSvc` 包装层。
- SQLite DSN、连接参数和 PRAGMA 验证由 Wire provider 配置，应用直接持有并关闭 `*gormx.GormDBSvc`。

### 2026-07-29：修正默认 HTTP 端口

- 默认监听端口最终确定为 `8195`。
- 配置默认值、示例配置、Vite 代理、测试、README 和 PRD 已统一同步。

### 2026-07-29：手工验证后的接口与参数优化

- `main.go` 改用 `github.com/spf13/pflag` 解析参数。
- 版本查看同时支持 `-v` 和 `--version`。
- 配置路径同时支持 `-c` 和 `--config`。
- `internal/api` 中的 JSON 接口统一使用 `pkg/response` 返回，不直接调用 `fiber.Ctx.JSON`；`/version` 的版本字段放在统一响应的 `data` 中。
- 统一接口响应的 `code` 为 `200` 时表示成功，`pkg/errc.Success` 固定为 `200`。
