# LanShare —— 局域网文件协作传输系统
## 项目开发规划文档 v1.0

> **文档状态**：初稿  
> **创建日期**：2026-07-28  
> **适用范围**：局域网内多端文件传输与协作系统，覆盖 PC（Windows / Linux / macOS）及移动端浏览器  
> **主要读者**：全栈开发工程师、AI 辅助开发工具

---

## 目录

1. [项目概述](#1-项目概述)
2. [技术选型](#2-技术选型)
3. [系统架构](#3-系统架构)
4. [数据模型设计](#4-数据模型设计)
5. [目录与文件存储结构](#5-目录与文件存储结构)
6. [API 接口规范](#6-api-接口规范)
7. [SSE 事件规范](#7-sse-事件规范)
8. [核心业务逻辑](#8-核心业务逻辑)
9. [前端设计规范](#9-前端设计规范)
10. [安全与权限模型](#10-安全与权限模型)
11. [配置与管理](#11-配置与管理)
12. [分阶段开发计划](#12-分阶段开发计划)
13. [非功能性要求](#13-非功能性要求)
14. [已知边界与约束](#14-已知边界与约束)

---

## 1. 项目概述

### 1.1 项目背景

现有局域网文件传输工具（如 LANDrop）体积较大（80+ MB），且依赖特定客户端安装。本项目目标是实现一个**轻量化、无需客户端安装、基于浏览器访问**的局域网文件传输与协作工具。

### 1.2 核心设计原则

- **零客户端安装**：接收方和发送方均只需浏览器，无需额外软件
- **服务端中转**：文件上传至服务端暂存，支持历史下载、重复获取、离线接收
- **房间隔离**：所有数据在房间维度隔离，房间自动过期并清理
- **最小权限**：接收者只能看到发送给自己的内容
- **单文件部署**：服务端为单二进制文件，内嵌 Web UI，开箱即用

### 1.3 核心功能范围

| 功能模块 | 第一阶段 MVP | 第二阶段 | 第三阶段 | 第四阶段 |
|---|---|---|---|---|
| 匿名会话与身份 | ✅ | | | |
| 4 位房间号管理 | ✅ | | | |
| 房间生命周期（12h TTL） | ✅ | | | |
| 成员管理与在线状态 | ✅ | | | |
| 文件定向发送（一对一/一对多） | ✅ | | | |
| 接收确认与拒绝 | ✅ | | | |
| 发送者撤回文件 | ✅ | | | |
| 文件重复下载历史 | ✅ | | | |
| 房间容量限制（2 GiB） | ✅ | | | |
| 二维码与 URL 加入房间 | ✅ | | | |
| 移动端响应式 UI | ✅ | | | |
| SSE 实时事件推送 | | ✅ | | |
| 聊天（文字 + 表情 + 图片） | | ✅ | | |
| 上传进度实时反馈 | | ✅ | | |
| 文件预览（图片/文本/PDF/媒体） | | | ✅ | |
| 分块上传与断点续传 | | | ✅ | |
| 文件完整性校验（BLAKE3） | | | ✅ | |
| 管理后台 | | | | ✅ |
| Docker 镜像与部署文档 | | | | ✅ |

### 1.4 目标部署环境

- 局域网内单台 Linux / Windows / macOS 机器作为服务端
- 同网络内所有设备通过浏览器访问服务端 IP 地址
- 不依赖互联网连接、云服务或第三方 CDN

---

## 2. 技术选型

### 2.1 后端

| 组件 | 选型 | 说明 |
|---|---|---|
| 编程语言 | Go 1.22+ | 单文件编译、交叉编译简单、网络库成熟 |
| Web 框架 | Fiber v2 | 高性能、中间件完善 |
| 数据库 | SQLite（WAL 模式） | 嵌入式、零依赖、支持事务 |
| ORM / 查询 | sqlc + database/sql | 类型安全、性能好 |
| 唯一 ID | ULID | 可排序、可读性好 |
| 文件校验 | BLAKE3 | 快速、现代 |
| 二维码生成 | go-qrcode | 服务端生成 SVG/PNG |
| 配置 | 环境变量 + 配置文件 | 支持 `.env` 和 YAML |
| 日志 | zerolog | 结构化 JSON 日志 |
| 任务调度 | 内置 goroutine + ticker | 清理任务 |
| Web UI 嵌入 | `go:embed` | 前端构建产物内嵌二进制 |

### 2.2 前端

| 组件 | 选型 | 说明 |
|---|---|---|
| 框架 | React 18 + TypeScript | 类型安全、生态完整 |
| 构建工具 | Vite | 快速 HMR，优化产物 |
| 路由 | React Router v6 | SPA 路由 |
| 服务端状态 | TanStack Query v5 | 数据请求与缓存 |
| 客户端状态 | Zustand | 轻量、简洁 |
| 样式 | Tailwind CSS v3 | 响应式、移动端友好 |
| 实时通信 | 原生 EventSource（SSE） | 服务端推送 |
| 上传进度 | XMLHttpRequest | fetch 上传进度兼容性不足 |
| 本地存储 | IndexedDB（idb 库） + localStorage | 客户端缓存与偏好 |
| 图标 | Lucide React | 轻量 SVG 图标 |
| 通知 | Web Notifications API | 系统桌面通知（可选） |

### 2.3 不引入的组件

| 组件 | 原因 |
|---|---|
| BadgerDB | SQLite 已足够；引入后需要维护两套存储的一致性 |
| Redis | 局域网规模无需外部缓存服务 |
| WebSocket | SSE + REST 已满足需求，减少协议复杂度 |
| WebRTC | 当前为服务端中转模式，P2P 直传留作后续可选功能 |
| Electron | 避免体积膨胀；系统浏览器即可访问 |
| Docker（第一阶段） | 第四阶段补充 |

---

## 3. 系统架构

### 3.1 整体架构图

```
┌─────────────────────────────────────────────────────┐
│                   局域网客户端                         │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │  PC 浏览器 │  │ 手机浏览器 │  │ 平板浏览器 │          │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘          │
│       │              │              │                │
│    HTTP REST      SSE 事件       HTTP 文件流          │
└───────┼──────────────┼──────────────┼────────────────┘
        │              │              │
┌───────▼──────────────▼──────────────▼────────────────┐
│                  Go + Fiber v2 服务端                   │
│                                                       │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐  │
│  │  REST API   │  │   SSE Hub    │  │  文件流处理  │  │
│  │  路由与中间件 │  │  事件分发    │  │  上传/下载  │  │
│  └──────┬──────┘  └──────┬───────┘  └──────┬──────┘  │
│         │                │                  │         │
│  ┌──────▼──────────────────▼──────────────────▼──────┐│
│  │                   业务服务层                       ││
│  │  AuthService  RoomService  TransferService        ││
│  │  ChatService  MemberService  StorageService       ││
│  │  CleanupJob   PreviewService  AdminService        ││
│  └──────┬──────────────────────────────────┬─────────┘│
│         │                                  │          │
│  ┌──────▼──────┐                  ┌────────▼────────┐  │
│  │   SQLite    │                  │  文件系统存储    │  │
│  │  (WAL 模式)  │                  │  data/rooms/    │  │
│  └─────────────┘                  └─────────────────┘  │
│                                                       │
│  ┌──────────────────────────────────────────────────┐ │
│  │              Go 内存运行时状态                    │ │
│  │  SSE 连接池  上传进度  在线成员  事件缓冲区        │ │
│  └──────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────┘
```

### 3.2 模块职责划分

| 模块 | 职责 |
|---|---|
| **REST API** | 会话创建、房间操作、文件元数据、聊天操作 |
| **SSE Hub** | 管理客户端长连接，按房间广播事件 |
| **文件流处理** | HTTP 流式上传、Range 下载、预览响应 |
| **AuthService** | 会话令牌签发与校验，成员身份绑定 |
| **RoomService** | 房间创建、加入、延期、销毁，房间号分配 |
| **TransferService** | 文件上传任务、容量预留、接收确认、撤回 |
| **ChatService** | 消息持久化、图片附件引用 |
| **MemberService** | 成员关系、在线状态计算、离开逻辑 |
| **StorageService** | 文件目录管理、临时文件清理、空间统计 |
| **CleanupJob** | 定时扫描过期房间、孤儿文件、长时间卡住的上传任务 |
| **PreviewService** | 异步生成缩略图与文本预览 |
| **AdminService** | 配置查看、房间管理、容量管理（第四阶段） |
| **Go 内存状态** | SSE 连接映射、上传进度、下载引用计数、事件环形缓冲 |
| **SQLite** | 所有业务持久化数据，唯一事实来源 |
| **文件系统** | 实际文件内容与预览内容 |

### 3.3 前端数据流

```
服务端 SQLite（唯一事实来源）
        │
        │  HTTP API（命令与快照查询）
        ▼
   React 内存状态（TanStack Query 缓存 + Zustand）
        │
        │  IndexedDB 缓存（历史快照 + 最后事件序号）
        ▼
   UI 渲染（页面展示）
        ▲
        │  SSE 事件（增量更新）
        │
   SSE EventSource 连接
```

---

## 4. 数据模型设计

> 所有时间字段均存储 UTC Unix 时间戳（秒）。  
> 所有 ID 字段均使用 ULID 字符串（26 位可排序）。

### 4.1 `users` 表（匿名用户身份）

```sql
CREATE TABLE users (
    id             TEXT PRIMARY KEY,          -- ULID
    display_name   TEXT NOT NULL,             -- 用户显示名称
    device_hint    TEXT,                      -- 浏览器/设备类型提示（可选）
    created_at     INTEGER NOT NULL,          -- UTC Unix 时间戳
    last_seen_at   INTEGER NOT NULL           -- 最后活跃时间
);
```

### 4.2 `sessions` 表（浏览器会话）

```sql
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,             -- ULID，也是 session_token
    user_id     TEXT NOT NULL REFERENCES users(id),
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
    revoked_at  INTEGER                       -- 主动吊销时间
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
```

> 会话令牌通过 `HttpOnly; SameSite=Lax` Cookie 下发，有效期 30 天，刷新页面自动续期。

### 4.3 `rooms` 表（房间）

```sql
CREATE TABLE rooms (
    id               TEXT PRIMARY KEY,        -- ULID
    code             TEXT NOT NULL,           -- 4 位字符串，如 "0001"
    owner_user_id    TEXT NOT NULL REFERENCES users(id),
    status           TEXT NOT NULL DEFAULT 'active',
                                              -- active | extending | destroying | destroyed
    capacity_bytes   INTEGER NOT NULL DEFAULT 2147483648,  -- 默认 2 GiB
    used_bytes       INTEGER NOT NULL DEFAULT 0,
    reserved_bytes   INTEGER NOT NULL DEFAULT 0,
    created_at       INTEGER NOT NULL,
    expires_at       INTEGER NOT NULL,        -- 创建时间 + 12h
    extended_at      INTEGER,                 -- 不为 NULL 表示已延期
    destroyed_at     INTEGER
);

-- 活动房间内房间号唯一
CREATE UNIQUE INDEX idx_rooms_code_active 
    ON rooms(code) WHERE status = 'active';
CREATE INDEX idx_rooms_expires_at ON rooms(expires_at);
CREATE INDEX idx_rooms_status ON rooms(status);
```

**状态机：**

```
active ──延期──▶ active（expires_at += 12h，extended_at 写入）
active ──过期──▶ destroying ──清理完成──▶ destroyed
active ──房主手动销毁──▶ destroying ──▶ destroyed
```

**容量字段说明：**

```
可用容量 = capacity_bytes - used_bytes - reserved_bytes
上传开始时：reserved_bytes += file_size（原子操作）
上传完成时：reserved_bytes -= file_size, used_bytes += file_size
上传失败时：reserved_bytes -= file_size（回滚）
文件撤回时：used_bytes -= file_size（物理删除后立即生效）
```

### 4.4 `room_members` 表（房间成员关系）

```sql
CREATE TABLE room_members (
    id             TEXT PRIMARY KEY,          -- ULID
    room_id        TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id        TEXT NOT NULL REFERENCES users(id),
    role           TEXT NOT NULL DEFAULT 'member',  -- owner | member
    display_name   TEXT NOT NULL,             -- 加入时快照的显示名
    joined_at      INTEGER NOT NULL,
    left_at        INTEGER,                   -- NULL 表示仍在房间
    last_seen_at   INTEGER NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active'   -- active | left | kicked
);

CREATE UNIQUE INDEX idx_room_members_room_user 
    ON room_members(room_id, user_id) WHERE status = 'active';
CREATE INDEX idx_room_members_room_id ON room_members(room_id);
CREATE INDEX idx_room_members_user_id ON room_members(user_id);
```

> **在线状态计算规则（运行时，不存入数据库）：**
> - SSE 连接存在 → `online`
> - SSE 断开但 `last_seen_at` 在 60 秒内 → `away`
> - 超过 60 秒 → `offline`
> - 同一 `user_id` 有多个 SSE 连接时，任一存在即为在线

### 4.5 `files` 表（文件元数据）

```sql
CREATE TABLE files (
    id               TEXT PRIMARY KEY,        -- ULID
    room_id          TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_id        TEXT NOT NULL REFERENCES users(id),
    original_name    TEXT NOT NULL,           -- 仅用于展示，不用于路径
    storage_name     TEXT NOT NULL,           -- 服务端随机文件名（ULID）
    mime_type        TEXT NOT NULL,           -- 客户端声明类型
    detected_type    TEXT,                    -- 服务端检测类型（上传完成后填写）
    size_bytes       INTEGER NOT NULL,        -- 声明大小，用于预留容量
    actual_bytes     INTEGER,                 -- 实际大小（上传完成后填写）
    hash_blake3      TEXT,                    -- 上传完成后填写
    status           TEXT NOT NULL DEFAULT 'uploading',
                                              -- uploading | available | deleted | failed
    preview_status   TEXT NOT NULL DEFAULT 'pending',
                                              -- pending | ready | unsupported | failed
    scope            TEXT NOT NULL DEFAULT 'direct',  -- direct（定向发送）
    created_at       INTEGER NOT NULL,
    upload_started_at INTEGER NOT NULL,
    upload_finished_at INTEGER,
    deleted_at       INTEGER                  -- 逻辑删除时间（发送者撤回）
);

CREATE INDEX idx_files_room_id ON files(room_id);
CREATE INDEX idx_files_sender_id ON files(sender_id);
CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_files_created_at ON files(created_at);
```

**文件状态机：**

```
uploading ──上传完成──▶ available ──发送者撤回──▶ deleted
uploading ──上传失败/超时──▶ failed
available ──房间过期──▶（随房间级联删除）
```

### 4.6 `file_recipients` 表（文件接收关系）

```sql
CREATE TABLE file_recipients (
    id                   TEXT PRIMARY KEY,    -- ULID
    file_id              TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    recipient_user_id    TEXT NOT NULL REFERENCES users(id),
    status               TEXT NOT NULL DEFAULT 'pending',
                                              -- pending | accepted | declined | revoked
    offered_at           INTEGER NOT NULL,
    accepted_at          INTEGER,
    declined_at          INTEGER,
    first_download_at    INTEGER,
    last_download_at     INTEGER,
    download_count       INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX idx_file_recipients_file_user 
    ON file_recipients(file_id, recipient_user_id);
CREATE INDEX idx_file_recipients_recipient ON file_recipients(recipient_user_id);
CREATE INDEX idx_file_recipients_file_id ON file_recipients(file_id);
```

> **关键权限规则：接收者只能查询自己在 `file_recipients` 表中存在记录的文件。**

### 4.7 `messages` 表（聊天消息）

```sql
CREATE TABLE messages (
    id           TEXT PRIMARY KEY,            -- ULID
    room_id      TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_id    TEXT NOT NULL REFERENCES users(id),
    scope        TEXT NOT NULL DEFAULT 'room',  -- room | direct | multi
    content_text TEXT,                        -- 文字内容（纯文本，含 Unicode 表情）
    deleted_at   INTEGER,
    created_at   INTEGER NOT NULL,
    sequence     INTEGER NOT NULL             -- 房间内单调递增序号
);

CREATE INDEX idx_messages_room_id ON messages(room_id);
CREATE INDEX idx_messages_created_at ON messages(room_id, created_at);
CREATE INDEX idx_messages_sequence ON messages(room_id, sequence);
```

### 4.8 `message_recipients` 表（定向消息接收关系）

```sql
CREATE TABLE message_recipients (
    message_id         TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    recipient_user_id  TEXT NOT NULL REFERENCES users(id),
    read_at            INTEGER,
    PRIMARY KEY (message_id, recipient_user_id)
);

CREATE INDEX idx_message_recipients_user ON message_recipients(recipient_user_id);
```

> `scope=room` 时不插入此表，房间所有当前成员可见。  
> `scope=direct/multi` 时必须在此表插入接收者记录。

### 4.9 `message_attachments` 表（消息图片附件）

```sql
CREATE TABLE message_attachments (
    id          TEXT PRIMARY KEY,             -- ULID
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_id     TEXT NOT NULL REFERENCES files(id),
    sort_order  INTEGER NOT NULL DEFAULT 0
);
```

### 4.10 `room_events` 表（事件序列，用于 SSE 断线补发）

```sql
CREATE TABLE room_events (
    id          TEXT PRIMARY KEY,             -- ULID
    room_id     TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sequence    INTEGER NOT NULL,             -- 房间内单调递增
    type        TEXT NOT NULL,               -- 事件类型
    actor_id    TEXT,                        -- 发起用户 ID
    payload     TEXT NOT NULL,               -- JSON 字符串
    occurred_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_room_events_seq ON room_events(room_id, sequence);
CREATE INDEX idx_room_events_occurred_at ON room_events(room_id, occurred_at);
```

> 只保留最近 **15 分钟**内的事件，超出后无法补发，要求客户端重新同步快照。  
> CleanupJob 定期清理超过 15 分钟的事件记录。

### 4.11 `upload_chunks` 表（第三阶段：分块上传状态）

```sql
CREATE TABLE upload_chunks (
    file_id       TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    chunk_index   INTEGER NOT NULL,
    chunk_size    INTEGER NOT NULL,
    received_at   INTEGER NOT NULL,
    hash_blake3   TEXT,
    PRIMARY KEY (file_id, chunk_index)
);
```

---

## 5. 目录与文件存储结构

### 5.1 服务端目录布局

```
<DATA_DIR>/                         # 默认 ./data，可通过配置修改
├── app.db                          # SQLite 数据库主文件
├── app.db-wal                      # SQLite WAL 文件
├── app.db-shm                      # SQLite 共享内存文件
├── rooms/
│   └── <room_id>/                  # 内部 room_id（ULID），不是 room_code
│       ├── files/
│       │   └── <storage_name>      # 随机命名的实际文件（ULID，无扩展名）
│       ├── previews/
│       │   └── <storage_name>.webp # 图片缩略图
│       └── thumbnails/
│           └── <storage_name>.webp # 更小尺寸缩略图（消息内联显示）
└── tmp/
    └── uploads/
        └── <upload_id>.part        # 正在上传的临时文件
```

### 5.2 文件命名规则

| 类型 | 命名方式 | 说明 |
|---|---|---|
| 房间目录 | `<room_id>`（ULID） | 与房间号（0001）无关，避免冲突 |
| 实际文件 | `<file_id>`（ULID） | 随机名称，无扩展名 |
| 临时上传 | `<file_id>.part` | 上传完成后原子重命名 |
| 预览图 | `<file_id>.webp` | WebP 格式，固定后缀 |
| 缩略图 | `<file_id>.webp` | 同格式，尺寸更小 |

### 5.3 上传文件落盘流程

```
1. 检查成员身份与容量预留（原子 SQL）
2. 创建 files 记录（status=uploading）
3. 创建 file_recipients 记录（status=pending）
4. 写入 tmp/uploads/<file_id>.part（流式写入）
5. 同时计算 BLAKE3 哈希（流式，无需全部读完再计算）
6. 同时检测真实 MIME 类型（读取文件头）
7. 上传完成后校验大小与声明大小
8. 原子重命名：tmp/uploads/<file_id>.part → rooms/<room_id>/files/<file_id>
9. 更新 files 记录（status=available, hash, detected_type, actual_bytes）
10. 更新房间容量（reserved_bytes-, used_bytes+）
11. 推送 SSE 事件：file.available
12. 异步触发预览生成任务
```

### 5.4 文件撤回（发送者删除）流程

```
1. 校验请求者为文件 sender_id
2. 将 files.status 改为 deleted，写入 deleted_at
3. 删除物理文件（rooms/<room_id>/files/<file_id>）
4. 删除预览与缩略图（rooms/<room_id>/previews/<file_id>.webp）
5. 更新房间 used_bytes（released_bytes）
6. 将所有 file_recipients.status 改为 revoked
7. 推送 SSE 事件：file.revoked（通知所有相关接收者）
8. 不删除 files 和 file_recipients 记录（保留历史，UI 显示灰色）
```

### 5.5 房间销毁流程

```
1. 原子将 rooms.status 改为 destroying
2. 拒绝所有新的加入、上传和消息请求
3. 向房间所有 SSE 连接推送 room.destroyed 事件
4. 等待 5 秒（允许进行中下载有机会完成）
5. 强制关闭房间内所有 SSE 连接
6. 删除文件目录：data/rooms/<room_id>/
7. 删除 SQLite 中所有关联记录（利用 CASCADE）
8. 删除或标记 rooms 记录（status=destroyed, destroyed_at）
```

---

## 6. API 接口规范

### 6.1 通用规范

- **Base URL**：`/api/v1`
- **认证方式**：`HttpOnly` Cookie `session_token`，所有接口（除 `POST /sessions`）均需携带
- **响应格式**：

```json
// 成功
{ "code": 200, "data": { ... } }

// 失败
{
  "code": 40001,
  "message": "房间容量不足",
  "details": { ... }
}
```

- **错误码规范**：

| 范围 | 含义 |
|---|---|
| `40001–40099` | 参数校验错误 |
| `40101–40199` | 身份验证错误 |
| `40301–40399` | 权限错误 |
| `40401–40499` | 资源不存在 |
| `40901–40999` | 业务冲突（如容量不足、房间已满） |
| `50001–50099` | 服务端内部错误 |

### 6.2 会话接口

#### `POST /api/v1/sessions` — 创建匿名会话

**请求体：**
```json
{
  "displayName": "Alice",
  "deviceHint": "Chrome/macOS"
}
```

**响应：**
```json
{
  "code": 200,
  "data": {
    "userId": "01JXYZ...",
    "displayName": "Alice",
    "sessionExpiresAt": 1788000000
  }
}
```

> 服务端通过 `Set-Cookie` 设置 `session_token`（HttpOnly, SameSite=Lax）。  
> 若请求已携带有效 Cookie，刷新现有会话有效期并返回用户信息。

#### `PUT /api/v1/sessions/me` — 更新当前用户信息

**请求体：**
```json
{ "displayName": "Bob" }
```

### 6.3 房间接口

#### `POST /api/v1/rooms` — 创建房间

**请求体：**
```json
{
  "displayName": "Alice 的房间"
}
```

**响应：**
```json
{
  "code": 200,
  "data": {
    "roomId": "01JROOM...",
    "roomCode": "1234",
    "expiresAt": 1785300000,
    "canExtend": true,
    "capacityBytes": 2147483648,
    "qrCodeUrl": "/api/v1/rooms/1234/qrcode"
  }
}
```

#### `POST /api/v1/rooms/join` — 加入房间

**请求体：**
```json
{
  "code": "1234",
  "displayName": "Bob"
}
```

**响应：**
```json
{
  "code": 200,
  "data": {
    "roomId": "01JROOM...",
    "roomCode": "1234",
    "role": "member",
    "expiresAt": 1785300000,
    "canExtend": false,
    "members": [ ... ],
    "lastEventSequence": 42
  }
}
```

#### `GET /api/v1/rooms/:code` — 获取房间快照

> 用于重新连接时同步当前状态。

**响应：**
```json
{
  "code": 200,
  "data": {
    "roomId": "01JROOM...",
    "roomCode": "1234",
    "status": "active",
    "expiresAt": 1785300000,
    "canExtend": true,
    "capacityBytes": 2147483648,
    "usedBytes": 536870912,
    "members": [
      {
        "userId": "01JUSER...",
        "displayName": "Alice",
        "role": "owner",
        "onlineStatus": "online"
      }
    ],
    "lastEventSequence": 42
  }
}
```

#### `POST /api/v1/rooms/:id/extend` — 房主延期房间

> 仅房主可操作，仅可延期一次，延期 12 小时。

**响应：**
```json
{
  "code": 200,
  "data": {
    "expiresAt": 1785343200
  }
}
```

#### `DELETE /api/v1/rooms/:id` — 房主解散房间

#### `POST /api/v1/rooms/:id/leave` — 成员离开房间

#### `GET /api/v1/rooms/:id/qrcode` — 获取房间二维码

> 返回 SVG 格式的二维码，内容为 `http://<服务器IP>:<端口>/rooms/<code>`。

### 6.4 成员接口

#### `GET /api/v1/rooms/:id/members` — 获取成员列表

**响应：**
```json
{
  "code": 200,
  "data": {
    "members": [
      {
        "userId": "01JUSER...",
        "displayName": "Alice",
        "role": "owner",
        "onlineStatus": "online",
        "joinedAt": 1785200000
      }
    ]
  }
}
```

### 6.5 文件传输接口

#### `POST /api/v1/rooms/:id/uploads` — 创建上传任务

> 上传前先校验容量并预留，然后再传输字节流。

**请求体：**
```json
{
  "fileName": "video.mp4",
  "fileSize": 1073741824,
  "mimeType": "video/mp4",
  "recipients": ["01JUSER_B...", "01JUSER_C..."]
}
```

**响应：**
```json
{
  "code": 200,
  "data": {
    "fileId": "01JFILE...",
    "uploadUrl": "/api/v1/uploads/01JFILE..."
  }
}
```

#### `PUT /api/v1/uploads/:fileId` — 上传文件字节流

**请求头：**
```
Content-Type: application/octet-stream
Content-Length: <文件大小>
```

> 流式接收，边接收边写入临时文件，完成后原子重命名。  
> 客户端使用 `XMLHttpRequest` 以获取 `upload.onprogress` 回调。

**响应：**
```json
{
  "code": 200,
  "data": {
    "fileId": "01JFILE...",
    "hashBlake3": "abc123...",
    "actualBytes": 1073741824,
    "status": "available"
  }
}
```

#### `POST /api/v1/files/:id/accept` — 接收者接受文件

**响应：**
```json
{
  "code": 200,
  "data": {
    "downloadUrl": "/api/v1/files/01JFILE.../download"
  }
}
```

#### `POST /api/v1/files/:id/decline` — 接收者拒绝文件

#### `GET /api/v1/files/:id/download` — 下载文件

**请求头（可选）：**
```
Range: bytes=0-1048575
```

> 支持 `Range` 断点续下。  
> 响应头包含：
> ```
> Content-Disposition: attachment; filename*=UTF-8''<编码后文件名>
> Content-Type: <detected_type>
> Accept-Ranges: bytes
> ```

#### `GET /api/v1/files/:id/preview` — 获取预览内容

> 仅对 `preview_status=ready` 的文件有效。  
> 图片类返回 WebP 缩略图；文本类返回前 256 KiB 内容；媒体类重定向至文件 URL。

#### `DELETE /api/v1/files/:id` — 发送者撤回文件

#### `GET /api/v1/rooms/:id/inbox` — 获取收件箱

**查询参数：**
- `cursor`：上一页最后一条记录的 `offeredAt` 时间戳，用于游标分页
- `limit`：每页条数，默认 20，最大 100
- `status`：`pending | accepted | declined | revoked | all`

**响应：**
```json
{
  "code": 200,
  "data": {
    "items": [
      {
        "fileId": "01JFILE...",
        "fileName": "photo.jpg",
        "fileSize": 2048000,
        "mimeType": "image/jpeg",
        "senderName": "Alice",
        "status": "accepted",
        "offeredAt": 1785210000,
        "downloadCount": 2,
        "previewAvailable": true
      }
    ],
    "nextCursor": 1785200000,
    "hasMore": false
  }
}
```

#### `GET /api/v1/rooms/:id/sent` — 获取已发送列表

**查询参数**：同收件箱。

**响应**：包含每个文件的接收者状态列表。

### 6.6 聊天接口

#### `POST /api/v1/rooms/:id/messages` — 发送消息

**请求体：**
```json
{
  "contentText": "你好，看这张图片 😊",
  "scope": "room",
  "recipientIds": [],
  "attachmentFileIds": ["01JFILE..."]
}
```

**响应：**
```json
{
  "code": 200,
  "data": {
    "messageId": "01JMSG...",
    "sequence": 43,
    "createdAt": 1785210050
  }
}
```

#### `GET /api/v1/rooms/:id/messages` — 获取聊天历史

**查询参数：**
- `cursor`：游标（序号），从此序号向前或向后翻页
- `direction`：`before | after`，默认 `before`
- `limit`：每页条数，默认 30，最大 100

#### `DELETE /api/v1/messages/:id` — 撤回消息（发送者）

### 6.7 SSE 事件流接口

#### `GET /api/v1/rooms/:id/events` — 建立 SSE 连接

**查询参数：**
- `lastSeq`：客户端最后处理的事件序号，服务端补发之后的事件

**响应头：**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

---

## 7. SSE 事件规范

### 7.1 事件格式

```
id: <event_id>
event: <event_type>
data: <json_payload>
```

所有事件的 `data` 字段均遵循以下基础结构：

```json
{
  "version": 1,
  "id": "01JEVT...",
  "type": "file.available",
  "roomId": "01JROOM...",
  "sequence": 43,
  "actorId": "01JUSER...",
  "timestamp": 1785210050,
  "payload": { ... }
}
```

### 7.2 事件类型清单

#### 房间类

| 事件类型 | 触发时机 | Payload 关键字段 |
|---|---|---|
| `room.member_joined` | 新成员加入 | `userId, displayName, role` |
| `room.member_left` | 成员离开或被踢 | `userId, displayName, reason` |
| `room.member_online` | 成员上线 | `userId` |
| `room.member_offline` | 成员离线 | `userId` |
| `room.expiring` | 距到期剩余 30 分钟 | `expiresAt` |
| `room.extended` | 房间被延期 | `expiresAt, extendedBy` |
| `room.destroying` | 房间即将销毁 | `reason, destroyAt` |
| `room.destroyed` | 房间已销毁 | `reason` |

#### 文件类

| 事件类型 | 触发时机 | 推送对象 | Payload 关键字段 |
|---|---|---|---|
| `file.offered` | 文件开始上传，接收者可见 | 发送者 + 接收者 | `fileId, fileName, fileSize, mimeType, senderName, recipientIds, uploadProgress` |
| `file.available` | 上传完成，可下载 | 发送者 + 接收者 | `fileId, hashBlake3, previewStatus` |
| `file.upload_failed` | 上传失败 | 发送者 + 接收者 | `fileId, reason` |
| `file.accepted` | 接收者接受 | 发送者 + 该接收者 | `fileId, recipientId, recipientName` |
| `file.declined` | 接收者拒绝 | 发送者 + 该接收者 | `fileId, recipientId` |
| `file.revoked` | 发送者撤回 | 发送者 + 所有接收者 | `fileId, revokedAt` |
| `file.preview_ready` | 预览生成完成 | 发送者 + 接收者 | `fileId, previewUrl, thumbnailUrl` |
| `file.download_started` | 接收者开始下载 | 发送者 | `fileId, recipientId` |

#### 聊天类

| 事件类型 | 触发时机 | Payload 关键字段 |
|---|---|---|
| `chat.message_created` | 新消息 | `messageId, senderId, senderName, contentText, attachments, scope, sequence, createdAt` |
| `chat.message_deleted` | 消息撤回 | `messageId, deletedAt` |

#### 系统类

| 事件类型 | 触发时机 | Payload 关键字段 |
|---|---|---|
| `system.heartbeat` | 每 20 秒 | `serverTime` |
| `system.reconnect_required` | 断线超出补发窗口 | `reason` |

### 7.3 客户端 SSE 重连逻辑

```
1. EventSource 自动重连
2. 重连成功后携带 lastSeq 参数
3. 服务端补发 15 分钟内的事件（若存在）
4. 若无法补发，推送 system.reconnect_required
5. 客户端收到后调用 GET /rooms/:code 重新获取快照
6. 快照返回后更新 IndexedDB，重新渲染界面
```

### 7.4 进度事件节流

文件上传进度更新通过 SSE 向发送者广播，限制频率：

- 每个文件每 **500 毫秒**最多推送一次进度事件
- 以下节点不受节流限制（立即推送）：上传开始、上传完成、上传失败

---

## 8. 核心业务逻辑

### 8.1 房间号分配算法

```
1. 生成范围 [1, 9999] 的随机整数
2. 格式化为 4 位零填充字符串（0001～9999）
3. 查询 rooms 表：WHERE code = ? AND status = 'active'
4. 同时查询冷却记录：rooms WHERE code = ? 
   AND status = 'destroyed' AND destroyed_at > now()-900（15 分钟冷却）
5. 若冲突，重试最多 20 次
6. 若 20 次仍冲突，从可用编号集合中扫描选择
7. 若无可用编号（极端情况），返回错误：房间号暂时耗尽
```

### 8.2 容量预留原子操作

```sql
-- 上传开始时（原子预留）
UPDATE rooms
SET reserved_bytes = reserved_bytes + :fileSize
WHERE id = :roomId
  AND status = 'active'
  AND capacity_bytes - used_bytes - reserved_bytes >= :fileSize
-- 返回受影响行数，若为 0 则拒绝上传

-- 上传成功时
BEGIN;
UPDATE files SET status='available', actual_bytes=:actual, hash_blake3=:hash 
WHERE id = :fileId;
UPDATE rooms 
SET reserved_bytes = reserved_bytes - :fileSize,
    used_bytes = used_bytes + :actual
WHERE id = :roomId;
COMMIT;

-- 上传失败/取消时
UPDATE rooms 
SET reserved_bytes = reserved_bytes - :fileSize
WHERE id = :roomId;
UPDATE files SET status='failed' WHERE id = :fileId;
```

### 8.3 文件传输状态机（接收者视角）

```
pending   ──接受──▶ accepted ──开始下载──▶ downloading ──完成──▶ downloaded
pending   ──拒绝──▶ declined
pending   ──发送者撤回──▶ revoked
accepted  ──发送者撤回──▶ revoked
downloaded ──发送者撤回──▶ revoked（物理文件删除，但记录保留）
pending/accepted ──房间过期──▶ expired（随房间级联删除）
```

### 8.4 房间延期规则

- 仅 `role=owner` 的成员可延期
- 仅当 `extended_at IS NULL` 时可操作（仅允许延期一次）
- 延期逻辑：`expires_at = expires_at + 43200`（加 12 小时）
- 延期成功后写入 `extended_at`，并推送 `room.extended` SSE 事件
- 建议 UI 在房间到期前 30 分钟显示提醒，但不强制限制延期时机

### 8.5 在线状态计算

在线状态由 **Go 内存** 维护，**不写入 SQLite**：

```go
type UserPresence struct {
    UserID      string
    Connections int        // 当前活跃 SSE 连接数
    LastSeenAt  time.Time  // 最后心跳或消息时间
}

func (p *UserPresence) Status() string {
    if p.Connections > 0 {
        return "online"
    }
    if time.Since(p.LastSeenAt) < 60*time.Second {
        return "away"
    }
    return "offline"
}
```

SSE 断开时：`Connections--`，若减到 0，启动 60 秒延迟判定，超时则广播 `room.member_offline`。

### 8.6 清理任务调度

| 任务 | 执行频率 | 说明 |
|---|---|---|
| 过期房间扫描 | 每 1 分钟 | 查询 `expires_at <= now AND status='active'` |
| 滞留 `destroying` 房间恢复 | 启动时 + 每 10 分钟 | 恢复上次进程崩溃的清理 |
| 孤儿 `.part` 文件清理 | 每 5 分钟 | 删除超过 2 小时的 `.part` 文件 |
| 僵死上传任务处理 | 每 5 分钟 | 将 `upload_started_at < now()-2h AND status=uploading` 标记为 `failed` |
| SSE 心跳 | 每 20 秒 | 向所有连接推送心跳注释 |
| SQLite WAL 检查点 | 每 10 分钟 | 防止 WAL 文件过大 |
| 事件日志清理 | 每 15 分钟 | 删除 `occurred_at < now()-900` 的 room_events |

### 8.7 文件预览生成规则

| 文件类型 | 生成内容 | 条件 |
|---|---|---|
| JPEG/PNG/GIF/WebP | 最大 1920px 的 WebP 缩略图 + 300px 小图 | 像素数 < 50M，文件 < 50 MB |
| SVG | 不生成（安全风险） | 始终 `unsupported` |
| 纯文本/JSON/Markdown | 读取前 256 KiB 的文本内容 | 编码检测为 UTF-8 或 ASCII |
| PDF | 标记 `supported`，由浏览器原生渲染 | 文件 < 100 MB |
| MP4/WebM/MP3/OGG | 标记 `supported`，由浏览器原生媒体标签播放 | 文件 < 房间容量上限 |
| ZIP | 仅列出目录结构（不解压内容） | 文件 < 10 MB |
| 其他 | `unsupported` | — |

> 预览生成均在**独立 goroutine 池**中异步执行，不阻塞上传完成响应。

---

## 9. 前端设计规范

### 9.1 页面路由

| 路由 | 页面 | 说明 |
|---|---|---|
| `/` | 首页 | 创建/加入房间入口 |
| `/rooms/:code` | 房间页 | 主协作界面 |
| `/rooms/:code/inbox` | 收件箱 | 可选独立页或侧边栏 |
| `/rooms/:code/sent` | 已发送 | 可选独立页或侧边栏 |
| `/admin` | 管理后台 | 第四阶段 |

### 9.2 首页功能

- 显示服务名称与说明
- **创建房间**：输入显示名称，点击创建，跳转至房间页
- **加入房间**：输入 4 位房间号，点击加入，跳转至房间页
- 展示本地最近访问的房间（来自 localStorage）
- 显示设备显示名称，可修改

### 9.3 房间页主布局

```
┌────────────────────────────────────────────────────┐
│ 顶栏：房间号 | 成员数 | 剩余时间 | 容量 | 二维码 | 延期 | 退出 │
├────────┬───────────────────────────────┬───────────┤
│        │                               │           │
│ 成员列表 │      聊天时间线区域            │  文件面板  │
│        │（消息 + 文件提醒 按时间顺序）      │（收件箱/  │
│ 在线成员 │                               │  已发送）  │
│ 发送目标 │                               │           │
│  选择   │                               │           │
│        ├───────────────────────────────┤           │
│        │ 底部输入区：文本 | 图片 | 文件   │           │
└────────┴───────────────────────────────┴───────────┘
```

**移动端布局：**

```
顶栏：房间号 | 成员（折叠） | 菜单
┌──────────────────────────────────────┐
│           消息时间线                  │
│                                      │
│                                      │
├──────────────────────────────────────┤
│ 底部输入区                            │
└──────────────────────────────────────┘
成员和文件面板通过底部 Tab 或抽屉切换
```

### 9.4 文件发送交互流程

```
1. 用户选择接收者（可多选）
2. 拖放或点击选择文件
3. 显示发送前预览：文件名、大小、接收者列表
4. 确认发送
5. 调用 POST /rooms/:id/uploads 创建任务
6. 使用 XHR 上传字节流，显示上传进度条（百分比 + 速度 + 剩余时间）
7. 上传完成后，SSE 推送 file.available 事件
8. 已发送列表实时更新每个接收者的接收状态
```

### 9.5 文件接收交互流程

```
1. 收到 SSE file.offered 事件
2. 消息时间线出现文件卡片（如文件仍在上传中，显示进度）
3. 文件就绪后，卡片显示文件名、大小、发送者、预览缩略图（若可用）
4. 接收者可点击"预览"（打开预览弹层）
5. 接收者选择"接受"或"拒绝"
6. 接受后自动触发浏览器下载
```

### 9.6 文件状态视觉规范

| 状态 | 视觉 |
|---|---|
| 上传中 | 进度条 + 百分比 + 取消按钮 |
| 可下载 | 蓝色"下载"按钮 + 文件图标 |
| 已接受（可重复下载） | 绿色"已接受" + 灰色"重新下载" |
| 已拒绝 | 橙色"已拒绝" |
| 发送者已撤回 | 置灰卡片 + "发送者已撤回" + 不可点击 |
| 上传失败 | 红色"上传失败" + 重试按钮（发送者视角） |
| 房间已过期 | 置灰 + "房间已过期，文件不可用" |

### 9.7 客户端状态持久化规范

| 数据 | 存储位置 | 说明 |
|---|---|---|
| 会话令牌 | HttpOnly Cookie（服务端设置） | 前端无法读取 |
| 显示名称 | localStorage | `lanshare_display_name` |
| 最近房间 | localStorage | `lanshare_recent_rooms`（数组，最多 5 条） |
| UI 偏好（主题等） | localStorage | `lanshare_ui_prefs` |
| 消息历史缓存 | IndexedDB（idb） | `messages` store，按 roomId 分区 |
| 文件收发缓存 | IndexedDB | `files` store |
| 最后事件序号 | IndexedDB | `room_meta` store |
| 聊天草稿 | IndexedDB | `drafts` store |

> 所有客户端缓存均为只读副本，页面重新打开时必须与服务端快照同步。

---

## 10. 安全与权限模型

### 10.1 身份认证

- 所有请求必须携带有效 `session_token` Cookie
- 令牌为 128 位随机字节的 Base64 URL 编码，存储 BLAKE3 哈希到 SQLite
- 令牌默认有效期 30 天，每次请求自动续期
- 服务端不允许在 URL 参数中传递令牌

### 10.2 权限矩阵

| 操作 | 房主 | 普通成员 | 非成员 |
|---|---|---|---|
| 查看房间信息 | ✅ | ✅ | ❌ |
| 延期房间 | ✅ | ❌ | ❌ |
| 解散房间 | ✅ | ❌ | ❌ |
| 踢除成员 | ✅ | ❌ | ❌ |
| 发送文件 | ✅ | ✅ | ❌ |
| 接收/查看发给自己的文件 | ✅ | ✅ | ❌ |
| 查看他人之间的文件 | ❌ | ❌ | ❌ |
| 撤回自己发送的文件 | ✅ | ✅ | ❌ |
| 撤回他人发送的文件 | ❌ | ❌ | ❌ |
| 发送聊天消息 | ✅ | ✅ | ❌ |
| 撤回自己的消息 | ✅ | ✅ | ❌ |
| 访问管理后台 | ❌（管理员独立身份） | ❌ | ❌ |

### 10.3 文件安全

- 存储路径使用 ULID 随机命名，不包含原始文件名
- 原始文件名仅存入数据库用于展示，经过字符净化
- 下载时 `Content-Disposition: attachment`，禁止内联执行
- 文件名净化规则：移除 `../`、`\`、`:`、`\0`、控制字符、Windows 保留设备名（`CON`, `PRN` 等）
- 预览内容使用独立接口，设置严格 CSP
- 禁止 SVG 内联预览
- 禁止 HTML 文件内联预览
- 服务端检测真实 MIME 类型（检查文件头，不信任客户端声明）
- 图片解码设置像素数上限，防止解压炸弹（如 PNG Bomb）

### 10.4 局域网访问控制

- 服务端默认只监听局域网接口（非 `0.0.0.0`，可配置）
- 加入接口限速：同一 IP 每分钟最多 20 次尝试
- 上传接口限速：同一用户同时最多 2 个上传任务
- 全局并发下载上限可配置
- 单文件大小不超过房间剩余容量（硬性约束）

---

## 11. 配置与管理

### 11.1 配置项清单

通过环境变量或 YAML 配置文件设置：

```yaml
# 服务器
server:
  host: "0.0.0.0"          # 监听地址
  port: 8080               # 监听端口
  readTimeout: 30s
  writeTimeout: 120s       # 大文件下载需要更长超时
  idleTimeout: 30s
  maxUploadSize: 2147483648  # 单次上传最大体积（服务端保护，2 GiB）

# 数据
data:
  dir: "./data"            # 数据根目录
  dbPath: "./data/app.db"  # SQLite 路径

# 房间默认设置
room:
  defaultCapacityBytes: 2147483648  # 默认房间容量 2 GiB
  maxCapacityBytes: 10737418240     # 管理员可设置的最大值 10 GiB
  defaultTTL: "12h"
  extensionDuration: "12h"
  codeCooldownSeconds: 900          # 房间号冷却 15 分钟

# 传输
transfer:
  maxConcurrentUploadsPerUser: 2
  maxConcurrentDownloads: 50
  uploadPartTimeout: "2h"    # 超时后标记为失败
  chunkSize: 8388608         # 第三阶段：8 MiB 分块

# 预览
preview:
  enabled: true
  maxImagePixels: 50000000   # 5000×10000 像素
  maxImageBytes: 52428800    # 50 MB
  textPreviewMaxBytes: 262144 # 256 KiB
  workerCount: 2

# 管理
admin:
  token: ""                  # 若为空，管理接口不可用
  enabled: false

# 日志
log:
  level: "info"
  format: "json"
```

### 11.2 管理接口（第四阶段）

```
GET    /admin/stats               -- 全局统计：房间数、文件数、磁盘占用
GET    /admin/rooms               -- 列出所有活跃房间
DELETE /admin/rooms/:id           -- 强制销毁房间
PUT    /admin/rooms/:id/capacity  -- 修改单个房间容量
PUT    /admin/config              -- 修改全局配置（运行时）
POST   /admin/vacuum              -- 手动触发 SQLite Vacuum
GET    /admin/health              -- 健康检查
```

管理接口通过 `Authorization: Bearer <ADMIN_TOKEN>` 鉴权，与用户会话体系完全分离。

---

## 12. 分阶段开发计划

### 第一阶段 — MVP：房间与文件传输基础

**目标**：实现最小可用版本，单文件部署，浏览器访问，基本文件发送与接收。

**功能列表：**

- [ ] **后端初始化**
  - [ ] 项目结构搭建（按领域划分包）
  - [ ] Fiber v2 应用初始化、中间件配置（Logger、CORS、Recover）
  - [ ] SQLite 初始化与 Schema 迁移（使用 golang-migrate）
  - [ ] 配置加载（环境变量 + YAML）
  - [ ] `go:embed` 嵌入前端构建产物
  - [ ] 结构化日志（zerolog）

- [ ] **会话与身份**
  - [ ] `POST /api/v1/sessions`：创建匿名会话，签发 HttpOnly Cookie
  - [ ] `PUT /api/v1/sessions/me`：更新显示名称
  - [ ] 会话中间件：所有 API 请求校验 session_token

- [ ] **房间管理**
  - [ ] 房间号随机分配算法（含冷却期检查）
  - [ ] `POST /api/v1/rooms`：创建房间
  - [ ] `POST /api/v1/rooms/join`：加入房间（含路由 `/rooms/:code` 页面自动触发）
  - [ ] `GET /api/v1/rooms/:code`：获取房间快照
  - [ ] `POST /api/v1/rooms/:id/extend`：房主延期（+12h，仅一次）
  - [ ] `DELETE /api/v1/rooms/:id`：房主解散
  - [ ] `POST /api/v1/rooms/:id/leave`：成员退出
  - [ ] `GET /api/v1/rooms/:id/members`：成员列表
  - [ ] `GET /api/v1/rooms/:id/qrcode`：生成二维码 SVG

- [ ] **文件传输（轮询版，无 SSE）**
  - [ ] `POST /api/v1/rooms/:id/uploads`：创建任务 + 容量预留
  - [ ] `PUT /api/v1/uploads/:fileId`：流式上传（XHR）
  - [ ] `POST /api/v1/files/:id/accept`：接受文件
  - [ ] `POST /api/v1/files/:id/decline`：拒绝文件
  - [ ] `GET /api/v1/files/:id/download`：下载（含 Range 支持）
  - [ ] `DELETE /api/v1/files/:id`：撤回
  - [ ] `GET /api/v1/rooms/:id/inbox`：收件箱（游标分页）
  - [ ] `GET /api/v1/rooms/:id/sent`：已发送列表

- [ ] **生命周期管理**
  - [ ] CleanupJob：每分钟扫描并销毁过期房间
  - [ ] CleanupJob：清理孤儿 `.part` 文件
  - [ ] CleanupJob：清理僵死上传任务
  - [ ] 服务启动时恢复 `destroying` 状态的房间

- [ ] **前端（React + TypeScript + Vite）**
  - [ ] 项目初始化（Vite + React + TypeScript + Tailwind）
  - [ ] 首页：创建/加入房间表单，最近房间记录
  - [ ] 房间页：成员列表、文件发送（拖放 + 选择）、收件箱、已发送列表
  - [ ] 文件卡片：进度条、状态标识、撤回/接受/拒绝按钮
  - [ ] 房间顶栏：房间号、剩余时间倒计时、容量进度条、二维码弹层
  - [ ] 移动端响应式布局（成员和文件通过底部 Tab 切换）
  - [ ] 轮询方案：每 5 秒拉取收件箱和成员状态（SSE 第二阶段替代）
  - [ ] localStorage：显示名称和最近房间
  - [ ] 路由 `/rooms/:code` 自动弹出加入确认或进入已加入的房间

**交付物：**
- Go 单二进制，嵌入前端，约 15–25 MB
- Windows x64 / Linux x64 / macOS Universal 三平台构建

---

### 第二阶段 — 实时协作：SSE + 聊天

**目标**：替换轮询为 SSE 实时推送，增加聊天功能，提升协作体验。

**功能列表：**

- [ ] **SSE Hub 实现**
  - [ ] 房间订阅者连接池（内存）
  - [ ] 按 userId / roomId 推送事件的接口
  - [ ] `GET /api/v1/rooms/:id/events`：SSE 长连接接口
  - [ ] 心跳注释（每 20 秒）
  - [ ] 断线补发：携带 `lastSeq` 参数，补发 15 分钟内事件
  - [ ] `system.reconnect_required` 事件（超出补发窗口）

- [ ] **在线状态**
  - [ ] 内存 UserPresence 状态维护
  - [ ] SSE 连接建立/断开时更新状态
  - [ ] 60 秒离线判定与 `room.member_offline` 推送
  - [ ] `room.member_online` 事件推送

- [ ] **文件传输 SSE 集成**
  - [ ] 上传完成时推送 `file.available`
  - [ ] 发送者撤回时推送 `file.revoked`
  - [ ] 接受/拒绝时推送对应事件给发送者
  - [ ] 上传进度节流推送（500ms）

- [ ] **聊天功能**
  - [ ] `POST /api/v1/rooms/:id/messages`：发送文字消息（含表情）
  - [ ] `GET /api/v1/rooms/:id/messages`：历史消息（游标分页）
  - [ ] `DELETE /api/v1/messages/:id`：撤回消息
  - [ ] 图片消息：先上传图片文件，再引用 fileId 发送消息
  - [ ] SSE 推送 `chat.message_created` 和 `chat.message_deleted`

- [ ] **前端 SSE 集成**
  - [ ] 替换轮询为 SSE EventSource
  - [ ] SSE 断线重连逻辑（含快照同步）
  - [ ] 在线状态显示（绿/黄/灰点）
  - [ ] 聊天时间线（消息 + 文件卡片混合展示）
  - [ ] 聊天输入框：文字输入、表情选择器、图片附件
  - [ ] IndexedDB 缓存消息历史与最后事件序号

---

### 第三阶段 — 可靠性：预览、校验与续传

**目标**：提升大文件可靠性，增加文件预览，支持断点续传。

**功能列表：**

- [ ] **文件预览**
  - [ ] 图片：服务端生成 WebP 缩略图（1920px + 300px）
  - [ ] 文本：读取前 256 KiB 内容
  - [ ] PDF/媒体：标记为浏览器原生支持
  - [ ] `GET /api/v1/files/:id/preview`：预览接口
  - [ ] SSE 推送 `file.preview_ready`
  - [ ] 前端预览弹层（图片/文本/PDF iframe/媒体播放器）

- [ ] **BLAKE3 完整性校验**
  - [ ] 上传时流式计算 BLAKE3
  - [ ] 下载时响应头携带 `X-Content-Hash`
  - [ ] 前端可选校验

- [ ] **分块上传（第三阶段）**
  - [ ] `upload_chunks` 表实现
  - [ ] 分块上传协议（8 MiB 块）
  - [ ] 恢复上传：查询已完成块位图
  - [ ] 服务端通过 `WriteAt` 将块写入预分配文件

- [ ] **服务端崩溃恢复**
  - [ ] 启动时扫描 `uploading` 状态超时文件
  - [ ] 恢复分块上传任务
  - [ ] 重建 used_bytes（与文件系统对齐校验）

- [ ] **磁盘管理**
  - [ ] SQLite WAL 检查点周期任务
  - [ ] 增量 Vacuum 触发条件
  - [ ] 磁盘剩余空间检查（低于阈值时拒绝上传）
  - [ ] 管理接口统计空闲页比例

- [ ] **并发控制**
  - [ ] 每用户上传并发限制
  - [ ] 全局下载并发限制
  - [ ] 预览生成 goroutine 池大小限制

---

### 第四阶段 — 管理与部署

**目标**：补全管理后台，完成容器化，满足生产部署需求。

**功能列表：**

- [ ] **管理后台 API**
  - [ ] `GET /admin/stats`
  - [ ] `GET /admin/rooms`
  - [ ] `DELETE /admin/rooms/:id`（强制销毁）
  - [ ] `PUT /admin/rooms/:id/capacity`
  - [ ] `PUT /admin/config`（运行时修改配置）
  - [ ] `POST /admin/vacuum`
  - [ ] `GET /admin/health`

- [ ] **管理后台前端页面**
  - [ ] 全局统计看板（房间数、用户数、文件数、磁盘占用）
  - [ ] 房间列表（支持强制销毁、修改容量）
  - [ ] 配置面板（默认容量、TTL、并发限制）
  - [ ] SQLite 维护操作（手动 Vacuum、WAL 检查点）

- [ ] **Docker 支持**
  - [ ] 多阶段构建 Dockerfile（Go 构建 + 前端构建 + 最小运行时）
  - [ ] docker-compose.yml（含数据卷挂载）
  - [ ] 健康检查配置
  - [ ] 环境变量文档

- [ ] **部署文档**
  - [ ] Linux 直接运行（systemd service 配置）
  - [ ] Docker 部署
  - [ ] Nginx 反向代理配置（含大文件上传超时设置）
  - [ ] 防火墙/Windows Defender 配置指引
  - [ ] 备份策略（SQLite 文件 + data/rooms 目录）

- [ ] **结构化日志与监控**
  - [ ] 请求日志（方法、路径、状态码、耗时、用户 ID、房间 ID）
  - [ ] 业务日志（房间创建/销毁、文件上传/撤回、错误）
  - [ ] 可选 Prometheus 指标暴露端点（/metrics）

---

## 13. 非功能性要求

### 13.1 性能目标

| 指标 | 目标值 |
|---|---|
| 首页加载时间 | < 1 秒（局域网） |
| API 响应时间（非文件操作） | < 100ms（P99） |
| 文件上传吞吐量 | ≥ 网络瓶颈的 90%（不引入额外瓶颈） |
| 文件下载吞吐量 | ≥ 网络瓶颈的 90% |
| 并发用户数 | ≥ 50 个同时在线（局域网） |
| SSE 连接数 | ≥ 200 个并发连接 |

### 13.2 体积目标

| 产物 | 目标大小 |
|---|---|
| Go 单二进制（含嵌入 UI） | < 25 MB |
| 前端初始加载资源 | < 500 KB（gzip 后） |

### 13.3 兼容性要求

| 客户端 | 最低版本 |
|---|---|
| Chrome | 90+ |
| Firefox | 90+ |
| Safari | 14+ |
| Edge | 90+ |
| 移动端 Chrome（Android） | 90+ |
| 移动端 Safari（iOS） | 14+ |

### 13.4 可靠性要求

- 服务进程重启后，活跃房间状态完全恢复
- 上传中断后可以重新提交（第三阶段支持断点续传）
- 文件完整性通过 BLAKE3 哈希验证（第三阶段）
- CleanupJob 执行失败不影响主服务可用性（独立 goroutine，panic 隔离）

---

## 14. 已知边界与约束

### 14.1 设计边界

| 边界 | 说明 |
|---|---|
| 仅局域网使用 | 不针对公网规模优化，不考虑 CDN、负载均衡 |
| 服务端中转 | 不使用 WebRTC P2P，文件必须经过服务端，占用双倍网络流量 |
| 单机部署 | 不支持集群或分布式部署 |
| 无账号体系 | 所有身份均为匿名临时会话 |
| 无移动端原生应用 | 移动端通过浏览器访问 |
| 房间号范围 | 0001–9999，最多 9999 个同时活跃房间 |
| 房间容量 | 默认 2 GiB，管理员最大可调至 10 GiB |
| 数据保留 | 房间销毁后数据立即清除，无备份恢复 |

### 14.2 浏览器限制说明

| 限制 | 影响 | 处理方式 |
|---|---|---|
| `fetch` 上传无进度 API | 无法实时显示上传百分比 | 使用 `XMLHttpRequest` |
| 后台标签页 SSE 可能被冻结 | 用户切换标签后实时通知中断 | 重连后快照同步 |
| iOS Safari 下载行为 | 下载文件触发方式与其他浏览器不同 | 使用 `Content-Disposition: attachment` + 新窗口打开 |
| 内存限制影响大文件预览 | 前端不可将完整大文件读入 Blob | 预览走服务端接口 |
| IndexedDB 存储回收 | 浏览器可能清理本地缓存 | 客户端数据仅作缓存，不作权威来源 |

### 14.3 已知不支持的功能（明确排除）

- WebRTC 直传（第一阶段不做，留作未来可选模式）
- 文件加密传输（明确不在设计范围，依赖局域网可信环境）
- DOCX / XLSX / PPTX 文件内容预览
- 文件内容搜索
- 断点续上传（第一阶段，第三阶段支持）
- 原生 Android / iOS 客户端
- 多服务端实例集群

---

*文档结束*

---
> **版本历史**  
> v1.0 — 2026-07-28 — 初稿，基于多轮需求讨论整理
