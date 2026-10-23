# 项目文档

## 开发与部署

- [项目总览、Docker 部署与更新](../README.md)
- [Go 后端开发](../backend-go/README.md)
- [Vue 前端开发](../frontend-vue/README.md)
- [管理后台组件职责](../frontend-vue/src/admin/README.md)
- [AstrBot 插件](../astrbot_plugin_player_stats/README.md)
- [日志采集脚本](../integrations/playerlogger/README.md)
- [基岩版中文名称目录维护](../backend-go/internal/catalog/bedrock/README.md)

## 接口约定

- [玩家 ID 存在性查询](api/player-presence.md)

## 配置

- [部署配置模板（逐项中文注释）](../.env.example)
- 服务器私有参数写入 `.env`；SMB、任务等业务设置由后台保存到数据库。
- [一次性旧配置迁移工具](../scripts/README.md#一次性旧配置迁移)；生成中文注释的私有配置，不覆盖文件，不启动容器。
- 旧服务器迁移和日常更新步骤见项目根目录 README。
- `python scripts/check-compose.py` 校验配置模板、必填值、参数传递和特殊字符；只调用 Compose 配置解析，不启动容器，也不读取真实 `.env`。

## 维护约定

跨模块接口文档放在 `docs/api/`，模块专属说明保留在模块目录。源码入口、Docker 构建上下文和 CI 路径保持稳定，移动文件时同步修改文档链接。

旧版迁移审计和已过时的部署资料已清理。现行部署以根目录 README、Compose 和 Dockerfile 为准。
