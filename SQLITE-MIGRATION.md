# MySQL → SQLite：配置迁移与重新生成统计

本版正式服务只使用 SQLite。迁移仅读取旧库的管理员和配置，不迁移统计、分析结果、任务日志、分享链接；本地 CSV/TXT 不移动、不删除。

## 数据与代码结构

```text
docker-data/
├─ database/
│  ├─ player.db          # 配置、账号、可重建统计、分享快照
│  ├─ player.db-wal      # SQLite 管理，运行时可能存在
│  ├─ player.db-shm      # SQLite 管理，运行时可能存在
│  └─ player.db.lock     # 服务/离线重建的进程互斥锁
├─ synced-logs/
│  ├─ main/             # 原主服 CSV
│  ├─ sub/              # 原 2服 CSV
│  └─ chat/             # 原聊天 TXT，不解析入库
├─ temp/                # 查询与分析临时文件
└─ mysql/               # 旧库保留，新服务不挂载
```

`internal/storage` 管理 SQLite 连接、版本迁移、事务和时间编码；模块自己的 repository 管理业务 SQL。`migrations/001_initial.sql` 包含 STRICT 表、外键、约束和显式只读查询视图，视图不重复存储数据。

`smb_connections`、`log_sources`、`ingest_policy`、`task_schedules`、`sharing_settings`、`runtime_settings` 分别负责连接、来源、日期策略、计划、分享和运行调优。任务记录的唯一约束防止同一天同一时刻重启重复触发；清空展示日志不删除去重依据。

文件统计以 `import_files` 和四类文件级贡献为事实来源；`players` 管理 Unicode 折叠身份；每日统计和首次出现资料是事务内更新的可重建汇总。矿石、木材和树苗共用 `file_player_resources`，明细随文件外键级联删除。

数据库位于 NAS 本地磁盘，不能位于 SMB/NFS。必须挂载整个 `database/`，不能只挂载 `.db`。一个写连接、最多四个只读连接，WAL、外键、FULL 同步、5 秒锁等待；所有时间点保存 UTC 微秒，业务日期为北京时间日期。

## 切换前检查

- 必须等待本次 SQLite 版本的 CI、前后端镜像和独立迁移镜像全部发布成功。下面的 SHA 是占位符，不要填写旧 MySQL 版本的 SHA。
- 确认保留 `.env` 中的旧 `MYSQL_DATABASE`、`MYSQL_USER`、`MYSQL_PASSWORD`，仅迁移工具读取，不要重新复制模板覆盖旧文件。旧调优环境变量在旧库没有运行参数表时作为迁移默认值。
- 确认本地 CSV 位于原挂载目录。若使用自定义路径，下载新版 Compose 后先保留原映射。
- 查看 `docker inspect player-stats-mysql`，确认所在 Docker 网络。默认网络为 `player_default`，不同则设置 `MIGRATION_NETWORK`。迁移默认使用网络内地址 `mysql:3306`，必要时设置 `MIGRATION_MYSQL_ADDRESS`。
- 本流程不执行数据库备份或删除；保留旧 MySQL 数据和部署文件供切回。

## 发布后执行

在 `/vol1/1000/docker/player` 执行以下命令；把 SHA 占位符替换为本次 CI 和三个镜像均成功的完整版本号。保留原 `.env`，不要用模板覆盖旧密码。脚本只更新部署描述与镜像，不备份、删除或移动旧数据库及 CSV/TXT。

```bash
cd /vol1/1000/docker/player
release='本次通过CI的40位SHA'
curl -fsSL "https://raw.githubusercontent.com/MarmieQi/player/$release/scripts/migrate-sqlite.sh" -o migrate-sqlite.sh
sh migrate-sqlite.sh "$release"
```

脚本自动检测旧 MySQL 容器的 Docker 网络。若旧库连接了多个网络，先设置 `export MIGRATION_NETWORK=实际网络名`；旧容器不叫 `player-stats-mysql` 时设置 `MIGRATION_MYSQL_CONTAINER`。它从旧 `.env` 读取 MySQL 凭据，先拉取三个版本镜像，停止旧 backend/frontend，只读迁移配置，再分文件重建统计；全部成功后才切换 Compose 并等待新服务健康。

如果配置迁移已经成功、但随后的重建被中断或失败，排查后使用相同版本续跑：

```bash
sh migrate-sqlite.sh "$release" --resume
```

`--resume` 要求迁移数据库已存在，不重新覆盖配置；已完成的 CSV 自动跳过。如果配置迁移本身失败，应修复原因后重新执行默认命令。脚本在正式切换前保留旧 Compose，切换后保存为 `docker-compose.mysql.yml`。首次 SQLite 切换完成后，日常升级不再运行迁移脚本，只更新镜像版本。

迁移工具拒绝已有 `player.db`，不会覆盖它。迁移失败时先排查错误，不要删除旧 MySQL 或 CSV。目标发布采用同目录临时文件、完整性/外键检查、检查点、关闭后非覆盖发布；失败临时文件会清理。

重建被取消或失败时，新服务会拒绝启动，避免恢复定时任务后生成不完整统计。重跑 `backend rebuild` 会跳过已完成文件；重建完成才解除保护。切换后旧登录失效，旧分享链接不迁移。登录后台确认账号、SMB、目录、时间计划、统计与聊天查询，再停止旧容器：

```bash
docker stop player-stats-mysql
```

不要使用 `--remove-orphans` 删除旧库容器。需要切回时先停止 SQLite 应用，再使用保留的旧 Compose 和旧 MySQL 版本；切换后新增的 SQLite 配置不会自动反向同步至旧库。

## 验证与限制

本地集成测试使用真实临时 SQLite 文件，包含配置回滚、账号、Unicode 查询、读写并行、取消、幂等导入、替换/删除汇总、重建、持久任务去重、消息队列和分享。MySQL 读取迁移集成测试仅在显式测试 DSN 下执行，并拒绝非 `_test` 数据库。

业务接口保持兼容；内部旧混合配置、分服务器调度列、重复资源表和绝对路径哈希已移除。运行参数在后台保存后执行 `docker compose restart backend` 生效；部署端口、挂载和 JWT 等环境变量仍需通过 Compose 重建容器更新。

### 本地真实 CSV 验证（2026-10-24）

使用用户提供的 2026-10-20 至 2026-10-22 三份 CSV，只复制到独立的 `.local/sqlite-preview` 测试目录，未改写原文件。输入合计 23,982,619 字节、209,441 行；其中 31,644 行为不参与方块统计的其他行为。

| 日期 | 破坏 | 放置 | 合计 |
|---|---:|---:|---:|
| 2026-10-20 | 36,380 | 18,114 | 54,494 |
| 2026-10-21 | 42,238 | 33,264 | 75,502 |
| 2026-10-22 | 33,743 | 14,058 | 47,801 |
| 总计 | 112,361 | 65,436 | 177,797 |

- 86 位玩家参与方块操作，109 位玩家有首次出现记录；两种人数的统计口径不同。
- 用独立的 Python CSV 读取器逐项核对了 264 组“日期＋玩家＋行为”、109 个首次出现时间、242 组资源统计及文件 SHA-256，全部一致。矿石 13,895、木材 7,263、树苗 156。
- 重复解析及离线重建没有重复累加。玩家详情、排行榜分享、日志关键词查询和矿透结果持久化已运行；关键词查询的 7 条命中另用原始 CSV 独立核对。
- 本机三文件首次解析约 0.53 秒，仅作这批文件的观测，不代表服务器性能保证。测试页面使用真实 SQLite 后端及上述数据。
- 自动化集成测试另外覆盖 1,001 文件跨轮推进、停用来源的离线重建、文件替换/删除、保存点回滚、读写并行、取消等待、进程锁和运行参数重启。

本地尚未运行真实 MySQL 迁移读取及 Linux race 检测；CI 已配置相应数据库与检查。正式切换仍需这两项通过，且不能把本地模拟配置视为生产配置迁移验收。

### 公开模块补充验证（2026-10-24）

公开接口不是脱离数据库的静态页：玩家档案、排行榜读取统计及来源配置，日志分享读取 share_links/share_pages，矿透分享读取结果快照；插件鉴权和通知读取 sharing_settings/notification_outbox。

新增 `backend-go/internal/httpapi/public_sqlite_integration_test.go`，通过真实临时 SQLite、完整 HTTP 路由和鉴权中间件验证：

- 玩家分享：Unicode 大小写身份、主/2服切换、数量、首次记录、矿物/木材/树苗及里程碑。
- 排行榜：总榜/活跃榜、按来源统计、保存的日期范围。
- 日志分享：262 行跨三个页面及数据库内部 200 行分片的拼接，无重复/遗漏，不泄露本机文件路径；统计明细删除后快照仍可读。
- 矿透分享：匿名读取结果快照、插件待发消息和投递确认；注入队列写入失败时，分享和队列一起回滚，且不广播失败消息。
- 安全与过期：缺失/错误插件密钥、插件不能读取管理员配置、错误类型 token、未知 token、过期状态和分页级联清理。
- WebSocket：匿名握手拒绝、已排队消息重放、Ping/Pong 控制帧、实时通知和正常断线。

真实 CSV 预览另外生成了四种公开分享并逐页检查。玩家报告和排行榜显示与后台一致；日志分享的 85 行由原 CSV 独立核对并验证第二页；矿透页面验证证据展开及 299 个矿物位置的翻页。没有向真实群发送消息。测试页面中的矿透风险分数仅是算法输出，不作人工定性。

本轮修复了矿透分享与通知入队之间缺少原子事务、WebSocket 缺少控制帧读取、零操作时放置占比错误显示 100% 三处问题。CI race 检测增加 HTTP 模块。尚不能声称所有正式环境功能已验收：真实 SMB、插件外部群投递、真实 MySQL 迁移及 Linux race 均仍需对应环境验证。
