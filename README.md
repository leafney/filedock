# FileDock · 码头

文件靠岸，随取随传。

浏览器即开即用的局域网文件传输工具。

当前提交完成单服务基础架构初始化，暂未实现房间、文件传输、会话、聊天或 SSE 业务。

## 技术栈

- 后端：Go 1.25.12、Fiber v2、Wire、GORM
- 数据库：`github.com/libtnb/sqlite`，通过迁入的 `pkg/gormx` 装配
- 日志：迁入的 `pkg/zlogx`
- 前端：React 18、TypeScript、Vite、Tailwind CSS v3、Bun
- 前端依赖：React Router、TanStack Query、Zustand、axios、i18next、react-i18next、idb、Lucide React、`@microsoft/fetch-event-source`
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

命令行参数同时支持短格式和长格式：

```bash
./bin/filedock -v
./bin/filedock --version
./bin/filedock -c data/config.toml
./bin/filedock --config data/config.toml
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

接口通过 `pkg/response` 返回统一结构，版本信息位于 `data`：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "status": "ok",
    "version": "dev",
    "git_branch": "unknown",
    "git_commit": "unknown",
    "build_time": "unknown"
  }
}
```

开发阶段 `status` 固定为 `ok`；SQLite 连接失败会阻止服务启动。

## 多语言

页面支持“简体中文”和“English”即时切换。语言选择保存在浏览器 localStorage；没有保存值时优先匹配浏览器语言，不支持时使用简体中文。

前端请求统一发送标准语言头：

```http
Accept-Language: zh-CN
```

后端支持 `zh`、`zh-CN`、`en`、`en-US` 等常见变体及标准权重列表，并返回实际采用的语言：

```http
Content-Language: zh-CN
Vary: Accept-Language
```

请求 English 版本接口：

```bash
curl -H 'Accept-Language: en' http://127.0.0.1:8195/version
```

成功响应中的 `message` 为 `Success`。业务错误、参数校验和全局 HTTP 错误同样根据请求语言返回；内部技术错误不会暴露给客户端。

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

## 自动发布

推送格式为 `v数字.数字.数字` 的稳定版本 Tag 后，GitHub Actions 会自动执行 Go 测试、前端测试、前端生产构建和多平台二进制编译，并创建正式 GitHub Release。Tag 可以指向任意分支提交，推送前请确认目标提交正确。

创建并推送 Tag：

```bash
git tag v0.2.0
git push origin v0.2.0
```

发布流程构建以下资产：

| 目标平台 | Release 资产 |
| --- | --- |
| macOS ARM64 | `filedock_v0.2.0_darwin_arm64.tar.gz` |
| Windows AMD64 | `filedock_v0.2.0_windows_amd64.zip` |
| Linux AMD64 | `filedock_v0.2.0_linux_amd64.tar.gz` |
| Linux ARM64 | `filedock_v0.2.0_linux_arm64.tar.gz` |

每个压缩包内只有一个 `filedock` 或 `filedock.exe` 二进制文件，不包含二级目录、README 或示例配置。Release 同时提供 `checksums.txt`，用于校验四个压缩包的 SHA-256；Release notes 由 GitHub 自动生成。

构建信息中的 `BuildTime` 固定使用 `Asia/Shanghai` 时区和带偏移的 RFC 3339 格式，例如 `2026-08-25T16:16:45+08:00`。同次发布的四个平台共用同一个构建时间。

Bark 通知需要在 GitHub 仓库 Secrets 中配置 `BARK_KEY`。通知会显示发布版本和成功或失败结果；Bark 配置缺失或发送失败不会改变 Release 结果。

发布产物未进行 macOS 或 Windows 代码签名。首次运行时，操作系统可能显示无法验证开发者或未知发布者的安全提示。

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
