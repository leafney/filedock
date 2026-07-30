# FileDock 房间工作区原型

该目录用于验证新版房间工作区布局，不属于生产前端。

## 隔离边界

- 不引用 `frontend/` 内的组件、样式或依赖。
- 不调用 FileDock API、SSE、数据库或真实文件存储。
- 不修改 Vite、Go 静态资源嵌入、路由或构建流程。
- 不使用 CDN；原型资源全部保存在当前目录。
- 当前阶段只包含静态桌面三栏骨架，交互与响应式将在用户确认后分阶段补充。

## 本地查看

在仓库根目录执行：

```bash
python3 -m http.server 4173 --directory examples/room-workspace-prototype
```

浏览器打开：

```text
http://127.0.0.1:4173/
```

直接双击 `index.html` 也能查看当前静态骨架；后续 ES 模块阶段应使用 HTTP 服务。

## 当前目录

```text
room-workspace-prototype/
├── README.md
├── index.html
└── styles.css
```

后续阶段将按已批准 PRD增加 JavaScript 模块和纯逻辑测试。
