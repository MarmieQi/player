# PlayerLogger 日志采集脚本

[PlayerLogger.js](PlayerLogger.js) 是仓库原先放在根目录的日志插件脚本，本次只调整存放位置，没有改动脚本内容。

服务端安装时仍使用文件名 `PlayerLogger.js`，脚本内部的配置路径仍为 `plugins/PlayerLogger/config.json`。仓库目录名不会改变服务端安装路径。

统计系统通过 CSV 接入日志，前后端运行不依赖这个脚本的源码路径；使用原生 PlayerLogger 插件时，仍可按同一 CSV 格式接入。CSV 目录和部署配置见 [项目说明](../../README.md)。
