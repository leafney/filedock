# FileDock 房间工作区原型

该目录用于验证新版房间工作区布局，不属于生产前端。

## 隔离边界

- 不引用 `frontend/` 内的组件、样式或依赖。
- 不调用 FileDock API、SSE、数据库或真实文件存储。
- 不修改 Vite、Go 静态资源嵌入、路由或构建流程。
- 不使用 CDN；原型资源全部保存在当前目录。
- 原型内模拟文件、权限与容量数据；刷新页面恢复固定夹具。

## 本地查看

在仓库根目录执行：

```bash
python3 -m http.server 4173 --directory examples/room-workspace-prototype
```

浏览器打开：

```text
http://127.0.0.1:4173/
```

原型使用 ES 模块，必须通过 HTTP 服务查看。

## 自动测试

在仓库根目录执行：

```bash
bun test examples/room-workspace-prototype/tests
```

## 当前目录

```text
room-workspace-prototype/
├── README.md
├── index.html
├── styles.css
├── js/
│   ├── app.js
│   ├── capacity.js
│   ├── i18n.js
│   ├── mock-data.js
│   ├── permissions.js
│   ├── render.js
│   └── state.js
└── tests/
    ├── capacity.test.js
    └── permissions.test.js
```
