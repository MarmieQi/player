# SQLite 存储与配置

正式程序只支持 SQLite。当前版本没有 MySQL 驱动、一次性配置导入工具、迁移镜像或混合配置接口。已经运行的 SQLite 实例可以直接更新镜像，已有配置、账号和统计不需要重新迁移或重建。

## 文件结构

```text
docker-data/
├─ database/
│  ├─ player.db
│  ├─ player.db-wal
│  ├─ player.db-shm
│  └─ player.db.lock
├─ synced-logs/
│  ├─ main/             # 主服 CSV
│  ├─ sub/              # 2服 CSV
│  └─ chat/             # 群服聊天 TXT
└─ temp/                # 查询与分析的临时文件
```

数据库存放在 NAS 本地磁盘，整个 `database/` 挂载到容器 `/data/database`。SQLite 自动管理 WAL/SHM；服务与离线重建通过进程锁互斥。运行中不能只复制主数据库文件作为一致备份。

## 配置职责

| 入口 | 职责 | 存储 |
|---|---|---|
| `.env` | 镜像、端口、挂载、时区、管理员初始化和 JWT | 部署目录 |
| `/api/admin/settings/smb` | SMB 连接和 CSV/TXT 目录 | `smb_connections`、`log_sources` |
| `/api/admin/settings/schedules` | CSV/TXT 自动计划及共用日期策略 | `task_schedules`、`ingest_policy` |
| `/api/admin/settings/sharing` | 分享有效期 | `sharing_settings` |
| `/api/admin/settings/runtime` | 三个运行参数的保存值与运行值 | `runtime_settings` |
| `/api/admin/settings/plugin-key` | 读取插件密钥；POST `/reset` 重置 | `sharing_settings` |

业务设置只通过对应分组保存，跨表保存使用同一事务。CSV/TXT 执行模块只读取所需配置，不持有另一套配置保存入口。自动任务读取任务计划，不拼装 SMB、分享和任务的混合对象。

辅助接口为 POST `/api/admin/settings/smb/test`、GET `/api/admin/settings/smb/files?sourceId=main`、GET `/api/admin/settings/smb/chat-files`。文件列表预览只读取已保存的远端目录元数据，不执行同步或解析。

`liveCacheSeconds`、`maxFilesPerRun`、`xrayRetainedJobs` 在页面保存后重启后端生效；环境变量不能覆盖已保存的运行参数。端口、挂载等部署参数修改后需要通过 Compose 重建容器。

## 数据生命周期

- `schema_migrations` 保留 SQLite 表版本和校验值，用于后续升级；不得修改已经发布的迁移文件。
- `database_metadata` 保留实例标识及离线重建保护状态，切换数据库实例会使旧登录失效。
- `import_files` 和文件级贡献保存 CSV 的来源、相对路径、内容指纹及解析版本。重复文件跳过，变化文件在一个事务内替换贡献和汇总。
- `players` 管理 Unicode 大小写折叠身份；每日统计和首次出现资料可从文件级事实重建。
- TXT 原文不入统计数据库。聊天查询读取文件，玩家名补全与日志查询、矿透分析共用全服/按服玩家索引接口。
- 分享、分析任务、通知队列各自持久化；任务计划的执行去重依据不因清空页面日志而删除。

数据库连接使用 WAL、外键、FULL 同步和 5 秒锁等待。写连接串行执行 `BEGIN IMMEDIATE`，独立只读池最多 4 个连接；文件下载、CSV 解析和快照压缩在事务外完成。

## 维护

日常更新复用现有 `.env`、`docker-compose.yml` 和数据目录，按 README 更新配对的镜像版本。SQLite 已完成初始化的实例不要重新运行任何旧 MySQL 迁移脚本。

需要主动重建统计时先停止应用，再运行 `docker compose run --rm --no-deps backend rebuild`。重建按文件提交，默认跳过当天和未来文件；失败保留进度，完成完整性检查后才允许服务恢复。正常升级无需运行此命令。

部署目录中的旧 `compose.migrate.yml`、`migrate-sqlite*.sh`、`docker-compose.mysql.yml` 已不参与当前服务。旧数据库文件不会被程序自动删除。

旧 `.env` 中的 `MYSQL_*`、`MIGRATION_*`、`PLAYER_LOGS_SMB_*`、`PLAYER_LOGS_IMPORTER_SKIP_TODAY`、`PLAYER_LOGS_LIVE_CACHE_SECONDS`、`PLAYER_LOGS_IMPORTER_MAX_FILES_PER_RUN`、`PLAYER_LOGS_XRAY_ANALYSIS_RETAINED_JOBS` 已不被使用，可以清理；保留 `.env.example` 中的部署参数及真实密码/JWT，不要用模板覆盖现有文件。
