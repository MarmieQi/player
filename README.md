# 玩家日志统计

基岩版 Minecraft 玩家日志统计工具。项目读取 PlayerLogger 生成的 CSV，支持本地目录或 SMB 远程复制，导入本地 SQLite 后提供玩家统计、矿物统计、日志查询、矿透分析、分享页面和 AstrBot 插件接口。

当前后端是 Go：`backend-go`。前端目录是 Vue 3：`frontend-vue`。

> 本项目只支持 SQLite，存储与配置职责见 [SQLite 存储说明](SQLITE-STORAGE.md)。

## Docker 部署

GitHub Actions 在 `main` 分支检查通过后发布前后端镜像。服务器只需 Docker / Docker Compose，不需要安装 Go、Node 或单独的 MySQL。

服务器部署文件分工：

- `docker-compose.yml`：仓库维护的服务结构，不再填写服务器密码。
- `.env`：服务器自己的端口、密码、JWT 密钥、数据目录和镜像标签，不提交 Git。
- [.env.example](.env.example)：每项均有中文说明的模板，只在首次安装时复制。
- `docker-data/`：数据库、归档 CSV 和临时任务文件。

首次部署在空目录中执行：

```bash
mkdir -p /vol1/1000/docker/player
cd /vol1/1000/docker/player
curl -fsSL https://raw.githubusercontent.com/MarmieQi/player/main/docker-compose.yml -o docker-compose.yml
curl -fsSL https://raw.githubusercontent.com/MarmieQi/player/main/.env.example -o .env.example
cp .env.example .env
chmod 600 .env
```

编辑 `.env`，填写 `APP_ADMIN_PASSWORD`、`APP_ADMIN_JWT_SECRET` 两个必填项，再执行：

```bash
docker compose config --quiet &&
docker compose pull &&
docker compose up -d

docker compose ps
docker compose logs --tail=100 backend frontend
```

`config --quiet` 只校验，不打印展开后的密码。Compose 会自动读取同目录 `.env`；同名 shell 环境变量优先于文件，排查时注意已有的导出变量。密码含 `$`、`#`、空格时在 `.env` 使用英文单引号，例如 `APP_ADMIN_PASSWORD='你自己的密码'`。

默认公开端口为 `9493`，后台端口为 `8843`，初始管理员名为 `admin`。密码由你填写，没有可直接用于生产的内置部署密码。已有管理员不会因修改初始密码而被覆盖，后续在后台“个人中心”修改密码。

也可以 clone 源码后部署：在仓库根目录首次复制 `.env.example` 为 `.env`、填写配置，然后执行同样的 Compose 命令。

如果拉取镜像提示 `unauthorized`，先确认对应 GitHub Actions 构建成功、镜像标签存在，以及 GHCR 包可公开拉取；私有包需使用具有 `read:packages` 权限的账号登录 `ghcr.io`。

## 更新部署

先确认本次 GitHub Actions 的前后端镜像均已发布成功。日常更新保留现有 `.env` 和 `docker-compose.yml`，只拉取应用镜像：

```bash
cd /vol1/1000/docker/player
docker compose config --quiet &&
docker compose pull backend frontend &&
docker compose up -d --no-deps backend frontend

docker compose ps
docker compose logs --tail=100 backend frontend
```

`.env` 的 `IMAGE_TAG` 同时控制前后端版本，默认 `latest`。需要配套部署或回滚时，填写同一次成功构建的完整提交 SHA；回滚应用前另行确认数据库迁移兼容性，不能仅靠切换镜像恢复数据库。

只有服务结构发生变化时才更新 Compose：先下载为 `docker-compose.yml.new`，对照并保留本地定制，备份原文件后再替换；不要重复复制模板覆盖 `.env`。clone 部署先确认本地配置已迁出 Compose，再拉取代码。更新会直接复用现有 SQLite 配置和统计数据，无需重新导入。

## 配置维护

服务器启动参数只在 `.env` 中修改，SMB、日志源、跳过当天及自动任务、分享有效期在后台“系统设置”中修改。管理员密码在“个人中心”修改。

配置只有两个入口：部署参数位于 `.env`；业务设置位于 SQLite，通过 `/api/admin/settings/` 分组保存。SMB、任务、分享及运行参数不从环境变量导入，也没有旧混合配置接口。

保留当前 `.env` 的密码、JWT 密钥、端口、镜像标签与数据路径，升级时不要重新复制模板。启动参数修改后通过 `docker compose up -d` 重建对应容器生效；直接运行 Go 后端仍读取进程环境变量。

`docker-data/` 或 `DATA_DIRECTORY` 指向的目录是持久化数据，不能删除。备份需包含服务器 `.env` 和数据库、CSV；SQLite 使用 WAL，不能在运行时只复制主 `.db` 文件而遗漏尚未检查点的数据。

## CSV 目录

Docker 会把宿主机目录挂载到容器：

```text
./docker-data/synced-logs/main  -> 主服 CSV
./docker-data/synced-logs/sub   -> 2服 CSV
./docker-data/database          -> SQLite 数据库与 WAL/SHM（整个目录挂载）
./docker-data/synced-logs/chat   -> 群服聊天 TXT
./docker-data/temp              -> 矿透分析及当天日志查询临时文件（自动清理）
```

CSV 文件名保持插件格式，例如：

```text
player_actions_2026-05-02.csv
```

默认会跳过当天或未来日期的 CSV，避免读取仍在写入的日志。

## SMB 同步与业务设置

SMB 地址、账号、共享名、各日志源目录、自动复制/解析时间和分享有效期，在后台“系统设置”中修改，保存到 SQLite。已有部署以后台数据库记录为准，重建容器不会用 `.env` 覆盖这些值。

SMB 连接和日志源只由后台管理；是否自动复制、解析由每个服务器的任务开关控制。新数据库的 SMB 连接初始为空，需要在后台填写；已有数据库记录保持不变。管理员密码在“个人中心”修改，AstrBot 密钥在后台生成或重置。

SMB 复制先把远程 CSV 拉到本地归档目录，再从本地解析入库。正式同步默认跳过当天文件；当天日志查询使用短期只读快照。缓存秒数、单轮处理上限和分析保留数量在“运行参数”中设置，保存后重启后端生效。

## 本地开发

Go 后端（直接运行时读取进程环境变量，不会自动加载 Compose 的 `.env`）：

```powershell
cd backend-go
go test ./...
go vet ./...
go run ./cmd/player-stats
```

Vue 前端：

```powershell
cd frontend-vue
npm install
npm run dev
```

生产构建验证：

```powershell
cd backend-go
go build -trimpath ./cmd/player-stats

cd ../frontend-vue
npm run build
```

## 矿透分析

矿透分析采用保守策略：

- 拐弯追矿：直线通道至少 8 格，转向角度大于 85 度，拐弯后 2-5 格内命中钻石或远古残骸，路径贴合度至少 95%。
- 矿脉直达：两个矿脉间距至少 8 格，时间 10-120 秒，中间普通方块不超过 6 个，并去重已使用矿脉。
- 绿宝石仍计入数量统计，但不作为追矿证据。
- 只有短直线弱证据时严格限分，避免高效正常玩家误判。
- 下界远古残骸包含床炸场景的单独保守证据。

分析结果默认只保留最近 1 次。

## AstrBot 插件

插件开发与维护参考 [AstrBot 官方插件开发指南](https://docs.astrbot.app/dev/star/plugin-new.html)。

后端玩家 ID 查询的完整接口约定见 [玩家 ID 存在性查询 API](docs/api/player-presence.md)。

插件目录：

```text
astrbot_plugin_player_stats
```

把该目录复制到 AstrBot 插件目录后，在插件配置里填写：

```text
api_base_url: http://服务器IP:9493
share_base_url: http://服务器IP:9493
api_key: 管理后台“系统设置”里的 AstrBot 插件密钥
```

命令示例：

```text
/绑定游戏id Steve
/我的游戏信息
/查日志 主服 -191 -34 750 5
```

`/查日志` 匹配成功后直接返回临时详情链接，公开页面使用压缩快照和后端分页展示全部匹配日志。QQ群主和管理员不限查询次数；普通成员需绑定游戏 ID，每小时次数在 AstrBot 插件中调整，日志链接有效期在管理后台调整。过期快照由后端定时清理。

日志 API 会保留 CSV 中的原始基岩版标识符，并额外返回中文显示名。管理员日志页和公开日志页优先显示中文名称，详情中仍可查看和复制 `minecraft:...` 原始值。中文目录由固定版本的 Mojang 官方基岩数据生成，更新方法见 `backend-go/internal/catalog/bedrock/README.md`。

公开坐标查日志支持原生 PlayerLogger 的“使用桶放置”和“使用桶收取”，并兼容旧“使用桶交互”。桶日志第二组坐标缺省时沿用第一组坐标；页面保留放置/收取名称，放置摘要优先显示详细信息 2 的桶类型，收取摘要优先显示详细信息 1 的液体类型，不改写原始详情。

“放入容器”和“从容器取出”按第二组容器坐标查询，不回退到玩家坐标。页面摘要显示物品中文名称与数量，例如“熔岩桶 ×1”；原始物品 ID 和 `x1` 数量保留。放入使用绿色，取出使用橙色。

## 项目结构

```text
backend-go/                  Go 后端源码、测试和 Dockerfile
frontend-vue/                Vue 3 前端源码、测试和 Dockerfile
astrbot_plugin_player_stats/ AstrBot 插件，保留独立插件目录
integrations/playerlogger/   日志采集脚本与接入说明
docs/                       项目文档索引与公共 API 约定
.github/workflows/           GitHub Actions 构建和镜像发布
scripts/                    部署配置校验脚本
docker-compose.yml          部署结构模板
.env.example                带中文注释的私有配置模板（实际 .env 不提交）
.local/                      本地临时文件与运行日志（不提交）
```

- [文档索引](docs/README.md)
- [后端开发说明](backend-go/README.md)
- [前端开发说明](frontend-vue/README.md)
- [日志采集脚本说明](integrations/playerlogger/README.md)

前后端源码路径、插件目录名和根目录 `docker-compose.yml` 保持稳定，供 CI、Docker 构建和服务器更新使用。模块内部文档放在对应模块旁，跨模块接口约定放在 `docs/api/`。

已停用的 Java 后端、过时迁移资料和旧调试日志已清理。新的本地调试日志统一写入 `.local/logs/`，不参与构建或 Git 提交。运行数据继续使用 `docker-data/`。
