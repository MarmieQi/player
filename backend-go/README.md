# Go 后端

这是玩家日志统计项目的 Go 后端，已替代原 Java 后端并接入 `docker-compose.yml`。

## 功能

- 管理员登录和 JWT 鉴权
- 玩家总体统计、每日统计、玩家详情、排行榜
- 本地 CSV 导入
- SMB 远程 CSV 列表、复制和导入
- 当天 SMB 日志只读临时快照查询
- 基岩版方块与物品中文名称目录（API 同时保留原始标识符）
- 自动复制/自动解析任务
- 日志查询
- 矿透分析和矿透分享
- AstrBot 插件密钥接口

## 本地运行

```powershell
go test ./...
go vet ./...
go run ./cmd/player-stats
```

默认监听：

```text
http://127.0.0.1:8080
```

健康检查：

```text
GET /api/health
GET /api/stats/server-time
```

## API 文档

- [玩家 ID 存在性查询 API](../docs/api/player-presence.md)：包含插件密钥鉴权、查询参数、当天日志兜底、共享缓存及错误状态语义。

## Docker

项目根目录执行：

```powershell
docker compose up -d
```

正式镜像使用纯 Go SQLite 驱动，不需要 CGO 或 MySQL 服务。数据库位于 `/data/database/player.db`，挂载整个目录。连接、表版本及配置职责见根目录 SQLITE-STORAGE.md。
