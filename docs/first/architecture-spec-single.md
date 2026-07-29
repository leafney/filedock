# 单服务项目架构设计规范

日期：2026-07-02

## 1. 目标

本文档用于约束单服务项目的基础架构设计，适用于：

- 单个 Go 后端服务
- 可选 React 前端
- 单仓开发模式
- 使用 Wire、Fiber、Bun、`go:embed` 的项目

本文档重点规范：

- 单服务目录结构
- 分层职责边界
- 路由组织方式
- 显式依赖注入方式
- 文件命名规范
- 接口响应规范
- 构建与运行约定

---

## 2. 总体原则

### 2.1 保留显式依赖注入

项目统一采用显式依赖注入。

推荐方式：

- Wire 负责依赖装配
- `server`、`router`、`api`、`biz`、`dal`、`service` 显式声明依赖

禁止方式：

- 引入 `AppCtx`、`ServiceLocator`、全局依赖容器
- 用全局变量管理业务依赖

原因：

- 依赖关系真实
- 模块边界清楚
- 测试和重构成本更低

### 2.2 样板最少化

禁止为了“以后可能扩展”提前增加无价值抽象，例如：

- 单实现接口
- 空工厂
- 自动注册中心
- 无真实读写逻辑的复杂包装层

只有在真实业务已经需要时，才允许补抽象。

### 2.3 构造即完整

核心运行对象必须在构造阶段完成装配，禁止在运行阶段对核心依赖做延迟初始化。

推荐方式：

- `NewServer(...)` 返回时对象已经可运行
- Fiber App、路由表、静态资源注册、中间件装配在构造阶段完成
- 缺失依赖应在 Wire 或构造阶段直接失败
- 静态资源、模板、嵌入文件异常应在构造阶段直接返回错误

禁止方式：

- 在 `Run()` 中通过 `if s.app == nil { ... }` 再补初始化
- 对核心依赖做 `nil` 容错后继续运行
- 将依赖装配问题推迟到运行阶段暴露

原因：

- Wire 已经保证核心依赖应在注入阶段完成
- 运行期延迟初始化会掩盖装配错误
- 构造即完整可以让对象状态更可预测

---

## 3. 标准目录结构

单服务项目标准结构如下：

```text
.
├── config/
├── core/
├── data/
├── frontend/
├── internal/
│   ├── api/
│   ├── biz/
│   ├── dal/
│   ├── dto/
│   ├── model/
│   └── service/
├── static/
│   └── dist/
├── wire/
├── docs/
├── pkg/
├── proto/
├── main.go
├── Makefile
└── go.mod
```

如果项目没有前端，可以不建立：

- `frontend/`
- `static/dist/`

如果项目没有共享协议，可以暂时不建立：

- `proto/`

---

## 4. 各目录职责

### 4.1 `main.go`

职责：

- 服务入口
- 读取启动参数
- 注入构建版本信息
- 调用 Wire 初始化服务
- 调用 `app.Run()`

禁止：

- 在 `main.go` 中直接写业务逻辑
- 在 `main.go` 中注册路由
- 在 `main.go` 中手工拼装复杂依赖

### 4.2 `config/`

职责：

- 定义配置结构
- 定义默认配置
- 加载配置文件
- 处理启动前必要的配置准备动作

约束：

- 必须提供 `config.example.toml`
- 默认配置文件路径应落在 `data/config.toml`

### 4.3 `core/`

`core` 只负责服务级装配，不负责业务实现。

#### `core/server.go`

职责：

- 创建 Fiber App
- 注册全局中间件
- 注册错误处理
- 注册静态资源服务
- 启动和关闭 HTTP 服务

约束：

- `NewServer(...)` 必须完成核心依赖装配
- `Run()` 只负责启动，不负责补注册、补初始化、补判空
- 至少提供全局错误处理和 panic recover

#### `core/router.go`

职责：

- 统一维护路由表
- 定义路由分组
- 将路由绑定到 API handler

约束：

- `router.go` 只定义路由
- `server.go` 负责装配，不直接展开大量业务路由

### 4.4 `data/`

职责：

- 当前服务运行数据根目录

通常包含：

- 配置文件
- 日志文件
- SQLite 文件
- BadgerDB 目录
- 本地缓存文件

### 4.5 `frontend/`

职责：

- 当前服务前端项目

约束：

- 统一使用 Bun
- 必须支持 `bun install`
- 必须支持 `bun run dev`
- 必须支持 `bun run build`
- HTTP API 调用统一使用 `axios`
- 禁止在业务组件中继续直接散落原生 `fetch`
- SSE 统一使用 `@microsoft/fetch-event-source`
- SSE 鉴权统一通过 `Authorization: Bearer <access_token>` 请求头传递
- 禁止新增基于 URL query 传递长期 `access_token` 的 SSE 方案
- 每个服务前端只维持一条统一 SSE 数据流
- 多种 SSE 消息统一通过 `event_type` 区分，不按业务拆多条流
- `event_type` 命名风格统一使用小写下划线，例如 `file_changed`
- 前端收到 SSE 事件后，当前阶段统一采用短防抖合并刷新相关列表或目录
- 默认防抖窗口为 `500ms`

### 4.6 `static/dist/`

职责：

- 存放前端构建产物
- 作为 `go:embed` 输入目录

禁止：

- 运行时直接依赖 `frontend/dist`

### 4.7 `wire/`

职责：

- 当前服务的 Wire 依赖注入定义与生成文件

约束：

- `wire.go` 只定义注入关系
- `wire_gen.go` 只由 Wire 生成，不手写业务逻辑

### 4.8 `internal/api/`

职责：

- 定义 HTTP handler
- 解析请求参数
- 调用 `biz`
- 输出响应

约束：

- `api` 不负责注册路由
- `api` 不直接访问数据库
- `api` 不写复杂业务编排

### 4.9 `internal/biz/`

职责：

- 承载业务规则和业务编排

约束：

- `biz` 是业务核心层
- `biz` 可依赖 `dal`、`model`、`service`
- `biz` 不依赖 `dto` 作为内部核心对象

### 4.10 `internal/dal/`

职责：

- 数据访问层
- 组织数据库、缓存和持久化读写

约束：

- 统一命名为 `dal`
- 即使当前逻辑很薄，也允许保留该层以维持调用链稳定

### 4.11 `internal/model/`

职责：

- 领域模型
- 数据库存储模型
- 业务内部基础结构

约束：

- `model` 不依赖 `dto`
- `model` 不感知 HTTP 层

### 4.12 `internal/service/`

职责：

- 外部系统适配
- 基础设施能力封装
- 可复用底层服务

典型内容：

- HTTP 客户端
- Redis / SQLite / Badger 包装
- gRPC client/server 适配
- 第三方 SDK 包装

禁止：

- 将纯业务编排放进 `service`

### 4.13 `internal/dto/`

职责：

- 接口输入结构
- 接口输出结构
- 前端 JSON 结构

约束：

- `dto` 只服务 `api`
- `biz` / `dal` / `model` 不应直接用 `dto` 作为内部领域对象

---

## 5. 标准调用链

统一调用链：

```text
api -> biz -> dal -> model
```

涉及外部能力时：

```text
api -> biz -> service
api -> biz -> dal
```

允许：

- `biz` 调用 `service`
- `biz` 调用 `dal`
- `dal` 使用 `model`

禁止：

- `api` 直接调数据库
- `api` 直接调 `dal`
- `dal` 依赖 `dto`
- `model` 反向依赖 `biz`

---

## 6. 路由设计规范

### 6.1 路由注册位置

所有路由必须在 `core/router.go` 中集中注册。

禁止：

- 在 `internal/api/*.api.go` 中写 `Register()`
- 在 `main.go` 中注册路由
- 在多个文件里分散维护根路由

### 6.2 路由分层

服务级接口走根路径：

- `/version`

业务接口统一走：

- `/api/...`

未来开放接口可预留：

- `/open/...`
- `/public/...`

### 6.3 推荐写法

```go
func registerRoutes(app *fiber.App, versionAPI *api.VersionAPI, userAPI *api.UserAPI) {
	app.Get("/version", versionAPI.HandleVersion)

	apiGroup := app.Group("/api")
	apiGroup.Get("/users", userAPI.ListUsers)
	apiGroup.Post("/users", userAPI.CreateUser)
}
```

---

## 7. 响应规范

### 7.1 服务级接口

以下接口允许直接返回简单 JSON：

- `/version`

统一返回字段：

```json
{
  "status": "ok",
  "version": "dev",
  "git_branch": "unknown",
  "git_commit": "unknown",
  "build_time": "unknown"
}
```

其中 `status` 用于判断服务健康状态，其余字段用于查看构建版本。

原因：

- 便于探针检查
- 便于排障

### 7.2 业务接口

所有 `/api/...` 接口统一使用响应包装。

### 7.3 列表接口分页规范

所有列表类业务接口必须默认支持分页，禁止直接返回完整数组。

统一分页查询参数：

- `page`
- `size`

统一默认值：

- `page=1`
- `size=10`

兜底规则：

- `page < 1` 时按 `1` 处理
- `size < 1` 时按 `10` 处理

统一分页响应结构：

```json
{
  "list": [],
  "page": 1,
  "size": 10,
  "total": 0
}
```

当前阶段约束：

- 暂不强制增加搜索参数
- 暂不强制增加排序参数
- 列表内部统一按新增时间倒序返回

新增规则：

- 后续新增的所有列表类接口，必须沿用这套分页参数与返回结构
- 不允许某些列表返回裸数组、某些列表返回分页对象的混用情况

统一格式：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

建议复用：

- `pkg/response`

禁止：

- 同一服务内部出现多套业务响应格式

---

## 8. 文件命名规范

统一按职责命名：

- `xx.api.go`
- `xx.biz.go`
- `xx.dal.go`
- `xx.m.go`
- `xx.svc.go`
- `xx.dto.go`

标准基础文件：

- `config.go`
- `config_test.go`
- `router.go`
- `server.go`
- `app.go`
- `wire.go`
- `wire_gen.go`
- `embed.go`

禁止：

- `util.go`
- `common.go`
- `helper.go`

除非文件极小且职责绝对单一。

---

## 9. 依赖注入规范

统一使用 Wire。

约束：

- Provider 只做依赖构造
- Provider 不夹带业务流程
- 不通过一个大容器对象透传所有依赖

推荐方式：

```text
wire -> config/logger/service/biz/api/server
```

---

### 9.3 必需依赖失败快速原则

通过 Wire 注入的核心依赖默认为必需依赖，包括数据库、日志、SSE、DAL、业务服务和下游业务组件。

约束：

- 构造函数必须接收明确的强类型依赖，不使用 `...interface{}` 或通过类型断言选择性装配核心依赖。
- 核心依赖缺失必须在构造阶段返回错误，或通过项目统一的启动失败机制终止初始化。
- 业务方法中禁止使用 `if dependency == nil { return }`、`if dependency == nil { return nil }` 等方式静默跳过核心逻辑。
- Wire 生成完成后，应用启动必须验证所有核心组件已经装配；装配失败不得继续提供 HTTP、gRPC 或 SSE 服务。
- 测试不得通过传入 `nil` 来模拟核心依赖；应提供真实的最小实现或明确的测试替身。

以下 `nil` 判断可以保留：

- HTTP、gRPC 请求中的可选对象和请求参数；
- 数据库查询可能不存在的记录；
- protobuf oneof 或可选消息；
- `*time.Time` 等可空数据字段；
- 明确设计为可选的配置能力，但必须在配置结构中显式表达，不能把核心服务缺失当作可选能力。

## 10. 前后端组织规范

### 10.1 前端管理

有前端时，必须位于：

- `frontend/`

统一使用 Bun：

- `bun install`
- `bun run dev`
- `bun run build`

前端请求规范：

- HTTP API 调用统一使用 `axios`
- 公共请求逻辑应收敛到最小 `api client` 封装
- SSE 统一使用 `@microsoft/fetch-event-source`
- SSE 鉴权统一通过 `Authorization: Bearer <access_token>` 请求头传递
- 不再新增原生 `EventSource + URL token` 方案
- 每个服务前端只维持一条统一 SSE 数据流
- 多种 SSE 消息统一通过 `event_type` 区分，不按业务拆多条流
- `event_type` 命名风格统一使用小写下划线，例如 `file_changed`
- 前端收到 SSE 事件后，当前阶段统一采用短防抖合并刷新相关列表或目录
- 默认防抖窗口为 `500ms`

### 10.2 静态资源发布

前端构建产物必须复制到：

- `static/dist`

后端通过 `go:embed` 内嵌静态资源。

---

## 11. 构建与运行规范

单服务项目建议具备以下命令：

- `make deps`
- `make dev`
- `make web`
- `make run`
- `make build`
- `make build-lin`
- `make build-lin-arm`
- `make air`
- `make wire`
- `make proto`
- `make tidy`
- `make test`

如果项目无前端，可去掉：

- `deps`
- `dev`
- `web`

如果项目无协议文件，可保留 `proto` 命令但允许空实现或显式提示无 `.proto` 文件。

---

## 12. 协议与共享层规范

### 12.1 `proto/`

共享协议统一放：

- `proto/*.proto`
- `proto/*.pb.go`
- `proto/*_grpc.pb.go`

### 12.2 `pkg/`

`pkg` 只用于通用基础能力。

允许：

- 配置
- 日志
- 缓存
- JWT
- 数据库工具
- 校验器
- 响应包装
- 对象存储封装

禁止：

- 当前项目的具体业务逻辑
- 当前项目的接口 handler
- 当前项目的业务规则

---

## 13. 适用建议

单服务项目建议按以下顺序初始化：

1. 建立标准目录结构
2. 建立 `config/core/wire/internal/*`
3. 建立 `/version`
4. 建立最小前端骨架
5. 建立 `static/dist` 与 `embed.go`
6. 建立 Makefile 基础命令
7. 建立最小测试
8. 再开始具体业务开发

---

## 14. 禁止事项

以下做法默认禁止：

- 在 `api` 中注册路由
- 在 `api` 中写复杂业务逻辑
- 在 `biz` 中直接写 HTTP 响应
- 在 `dto` 中承载内部领域模型
- 在 `service` 中写纯业务编排
- 使用 `dao` 命名替代 `dal`
- 使用 `AppCtx` 替代显式注入
- 将探针接口混入 `/api`

---

## 15. 结论

单服务项目不等于可以随意简化层次。

本规范要求的是：

- 服务只有一个，但分层仍要清楚
- 样板可以少，但职责不能混
- 依赖必须真实
- 路由必须集中
- 业务接口必须统一

如果未来项目演化为多服务，可在保留本文核心原则的前提下，再升级为多服务架构规范。
