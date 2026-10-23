# 部署辅助脚本

## 配置回归检查

```bash
python3 scripts/check-compose.py
python3 scripts/test_migrate_config.py
```

需要 Python 3.10+ 和支持 `docker compose config --format json --no-env-resolution` 的 Docker Compose CLI；不需要启动 Docker 引擎。测试只使用临时目录和假凭据，不读取真实 `.env`、连接数据库或启动容器。

## 一次性旧配置迁移

`migrate_legacy_config.py` 只在旧 Compose 中写有服务器参数、需要迁入 `.env` 时执行一次。正常部署和更新不依赖这个脚本。

输入是**服务器原有的 Compose 文件**，不是仓库里刚下载的新模板。输出是带中文注释的私有配置，程序不修改输入、数据库、容器或后台设置，不生成新密码，不覆盖已有输出文件。Linux 下新文件权限为 `600`；Windows 使用目录的现有访问权限。

准备好当前版本的源码（包含新 `docker-compose.yml`、`.env.example` 和脚本）后，在原部署目录执行：

```bash
# /path/to/new-player 是新版本源码目录，替换成实际路径。
# ./docker-compose.yml 是原部署文件，仍保留原密码、端口和挂载。
python3 /path/to/new-player/scripts/migrate_legacy_config.py \
  --from ./docker-compose.yml \
  --output ./.env.migrated
```

如果备份文件放在别的目录，用 `--project-directory /原部署目录` 明确旧相对路径的解析位置。原部署若使用过 `docker compose -p 名称`，同时传 `--project-name 原名称`；没有使用过则不传。

生成后先在本机核对 `.env.migrated`。工具会保存原 Compose 项目名、将数据根目录转成绝对路径，以避免读取另一套数据库。特殊字符经过必要转义，Compose 解析后仍为原值，不要手工删除转义符。

首次迁移、且目标 `.env` 不存在时，将核对后的文件命名为 `.env`，备份旧 Compose 后换成新模板，然后执行：

```bash
docker compose config --quiet
# 上一步成功后再更新应用容器；首次启用的数据库应按项目 README 的步骤部署。
docker compose pull backend frontend &&
docker compose up -d --no-deps backend frontend
```

如果已有 `.env`，不要直接覆盖，先人工合并并确认。**不要把原 Compose、迁移后的 .env 或展开后的 Compose 输出发到聊天或上传仓库。** 正式升级前先做好数据库备份；镜像回滚不能代替数据库恢复。

支持自动迁移的范围：

- 项目原来的 MySQL、Go 后端、Vue 前端三个服务及内置数据库连接。
- 相同的前后端镜像标签、原端口、数据库凭据、JWT 和管理员初始化参数。
- 同一数据根目录下的 `mysql/`、`synced-logs/`、`temp/` 三个绑定挂载。
- 新模板实际支持的启动及首次初始化参数。

旧版未被 Go 读取的变量不迁入；SMB、任务时间、分享设置及现有账号继续保留在数据库中。

工具会对迁移前后的 Compose 解析模型作比较。遇到额外服务、外部数据库、不同镜像标签、额外挂载、端口绑定 IP、无法表达的环境参数或外部变量引用时停止，只报告配置字段名，不打印密码。此时必须人工保留相应定制，不能强行套用默认模板。
