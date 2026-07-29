# FileDock · 码头

文件靠岸，随取随传。浏览器即开即用的局域网文件传输工具。

当前提交完成单服务基础架构初始化，暂未实现房间、文件传输、会话、聊天或 SSE 业务。

## 技术栈

- 后端：Go 1.25.12、Fiber v2、Wire、GORM
- 数据库：`github.com/libtnb/sqlite`，通过迁入的 `pkg/gormx` 装配
- 日志：迁入的 `pkg/zlogx`
- 前端：React 18、TypeScript、Vite、Tailwind CSS v3、Bun
- 前端依赖：React Router、TanStack Query、Zustand、axios、idb、Lucide React、`@microsoft/fetch-event-source`
- 发布方式：Vite 构建后由 Go `embed` 内嵌

## 环境要求

- Go `1.25.12`
- Bun
- 可选：Air
- 可选：Wire 命令。当前仓库可使用 `make wire` 生成依赖注入代码。

## 配置

默认配置路径为 `data/config.toml`。开发阶段可以先复制示例：

```bash
mkdir -p data
cp config/config.example.toml data/config.toml
```

配置文件不存在时，服务使用代码默认值；不会自动生成配置文件。默认监听地址为 `:8195`，SQLite 默认路径为 `data/filedock.db`。

配置只包含基础架构所需的四组：`app`、`http`、`log` 和一级 `sqlite`。SQLite 的 WAL、外键、事务锁、忙等待和单连接限制由代码固定，不通过配置覆盖。

## 快速开始

安装前端依赖：

```bash
make deps
```

启动后端：

```bash
make run
```

启动前端开发服务器：

```bash
make dev
```

开发页面地址为 `http://127.0.0.1:5173`，后端地址为 `http://127.0.0.1:8195`。Vite 会把 `/version` 和 `/api` 代理到后端。

## 版本接口

当前只提供服务级接口：

```text
GET /version
```

返回 `status`、`version`、`git_branch`、`git_commit` 和 `build_time`。开发阶段 `status` 固定为 `ok`；SQLite 连接失败会阻止服务启动。

## 构建

构建前端并同步到 Go embed 目录：

```bash
make web
```

构建当前平台二进制：

```bash
make build
```

构建 Linux 版本：

```bash
make build-lin
make build-lin-arm
```

## 常用命令

```text
make deps          安装前端依赖
make dev           启动 Vite 开发服务器
make web           构建前端并同步静态资源
make run           启动 Go 服务
make build         构建当前平台二进制
make air           使用 Air 热更新
make wire          生成 Wire 文件
make tidy          整理 Go 依赖
make test          运行 Go 测试
make clean         清理构建产物
```

## 目录结构

```text
config/             配置结构和示例
core/               Fiber 服务、路由和生命周期
data/               运行数据，不提交 Git
frontend/           React 前端
internal/api/       HTTP handler
internal/biz/       业务编排
internal/dal/       数据访问层，当前留空
internal/dto/       HTTP DTO
internal/model/     领域模型，当前留空
internal/service/   基础服务和版本信息
pkg/                迁入的通用基础包
static/             Go embed 静态资源
wire/               Wire provider 和生成文件
```

## 当前边界

本阶段不包含数据库迁移、业务表、匿名会话、房间、成员、文件上传下载、聊天、SSE、预览、缓存、Redis、BadgerDB、对象存储和管理后台。后续业务实现必须继续遵守 `docs/first/architecture-spec-single.md` 与 `docs/first/LanShare_PRD.md`。
