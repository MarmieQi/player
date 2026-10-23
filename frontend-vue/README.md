# Vue 前端

## 本地开发

```powershell
npm ci
npm run dev
```

Vite 默认端口为 `5173`，`/api` 请求转发到 `http://localhost:8080`。端口及代理配置见 `vite.config.js`。

## 检查与构建

```powershell
npm test
npm run build
```

构建结果位于 `dist/`，不提交到 Git。Dockerfile 使用 Node 构建静态资源，运行阶段由 NGINX 提供服务；生产环境不启动 Vite。NGINX 的公开页面与管理入口配置见 `nginx.conf`，对外端口由根目录 Compose 配置。

## 源码职责

- `src/App.vue`：页面状态、接口调用和业务入口。
- `src/admin/`：管理后台布局、账号菜单、选框、导航及样式；[组件说明](src/admin/README.md)。
- `src/components/`：公开报告布局、统计卡片等展示组件。
- `src/log-presentation.js`：日志展示规则。
- `src/public-report-format.js`：公开报告的时间格式化。
- `src/*-preview.js`：已有公开页的预览数据。
- `src/styles.css`、`src/player-share-v2.css`、`src/xray-share.css`：公共及分享页面样式。
- `public/`：随前端发布的静态资源。
- `tests/`：Node 内置测试运行器执行的回归测试。

本机开发日志统一放在项目根目录的 `.local/logs/frontend/`；依赖缓存 `node_modules/` 与构建产物 `dist/` 留在本模块，不参与版本控制。
