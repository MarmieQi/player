# 部署配置检查

```bash
python3 scripts/check-compose.py
```

需要 Python 3.10+ 和支持 `docker compose config --format json` 的 Docker Compose CLI；不需要启动 Docker 引擎。测试只使用临时目录和假凭据，不读取真实 `.env`、连接数据库或启动容器。

检查内容包括模板注释与 Compose 参数对应关系、必填参数、前后端镜像配对、数据库凭据复用、端口、数据目录及特殊字符密码。

一次性旧配置迁移工具已完成迁移并从当前版本移除。后续部署直接使用 `.env` 和 `docker-compose.yml`；SMB、任务等业务设置在后台保存到数据库。
