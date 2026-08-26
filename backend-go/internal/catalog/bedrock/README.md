# 基岩版中文名称目录

这个目录为日志查询提供 Minecraft 基岩版方块和物品的中文显示名。CSV、查询条件和审计数据始终保留原始标识符；API 仅额外返回中文显示字段。

## 文件职责

- `catalog.go`：运行时查询入口，嵌入生成后的目录。
- `data/names_zh_cn.json`：生成文件，不手工修改。
- `data/overrides_zh_cn.json`：无法从官方语言键确定的少量名称，以及需要修正的特殊名称。
- `data/source.json`：固定的 Minecraft 版本、官方 Git 标签、提交和覆盖数量。
- `generator/`：确定性生成逻辑。任何官方方块或物品缺少中文名时都会失败。
- `cmd/bedrock-catalog-gen/`：本地生成命令入口。

## 数据来源

数据源固定为 [Mojang/bedrock-samples](https://github.com/Mojang/bedrock-samples) 的稳定版本，使用：

- `metadata/vanilladata_modules/mojang-blocks.json`
- `metadata/vanilladata_modules/mojang-items.json`
- `resource_pack/texts/en_US.lang`
- `resource_pack/texts/zh_CN.lang`

官方源文件受仓库内 `LICENSE.md` 和 Minecraft EULA 约束。完整的官方仓库只应放在项目外的临时目录，不要复制或提交到本项目。应用运行时不读取该临时目录，只使用 `data/names_zh_cn.json` 中已生成的名称。

## 更新流程

### 1. 选择固定版本

不要直接基于会变化的分支生成。先在 Mojang 官方仓库确认要支持的稳定标签，再修改下面的 `$catalogTag`。当前版本和提交记录在 `data/source.json`。

### 2. 稀疏检出官方数据

下面的命令只检出生成器需要的目录，且把源码放进系统临时目录：

```powershell
$catalogTag = "v1.26.40.05"
$catalogSource = Join-Path ([System.IO.Path]::GetTempPath()) "player-bedrock-samples-$catalogTag"

git clone --filter=blob:none --no-checkout https://github.com/Mojang/bedrock-samples.git $catalogSource
git -C $catalogSource sparse-checkout init --cone
git -C $catalogSource sparse-checkout set metadata/vanilladata_modules resource_pack/texts
git -C $catalogSource checkout --detach $catalogTag
```

如果目标目录已经存在，不要直接覆盖或递归清理不确定的路径；换一个明确的新目录，或者先人工确认旧目录的绝对路径和内容。

### 3. 生成目录

在 `backend-go` 目录运行：

```powershell
go run ./cmd/bedrock-catalog-gen -source $catalogSource
```

命令会拒绝以下输入：非 Mojang 官方远端、有本地修改、HEAD 没有精确标签、空或重复标识符、过期人工覆盖，以及任何没有中文名称的官方标识符。生成器采用以下顺序：官方序列化语言键、唯一英文名称关联、明确的基岩版家族规则、人工覆盖；禁止使用模糊相似度猜测。

### 4. 处理新增名称

如果生成器报告缺失标识符：

1. 先检查官方 `zh_CN.lang` 是否已有对应语言键。
2. 能确定为同一家族规则时，在 `generator/` 中增加明确规则和测试。
3. 只有无法可靠推导的个别名称才加入 `data/overrides_zh_cn.json`。
4. 不要为了通过生成而填写未经核实的猜测名称。

覆盖表中的标识符如果已经不在官方方块或物品数据中，生成器会失败，避免长期积累失效配置。

### 5. 验证和提交

```powershell
go test -count=1 ./...
go vet ./...
```

还需要人工确认：

- `data/source.json` 的标签和提交与本次选择一致。
- `uniqueIdentifiers` 与 `mappedIdentifiers` 相等。
- `data/names_zh_cn.json` 只包含预期的数据升级变化。
- 提交生成文件、覆盖表、来源元数据、生成器代码和测试；不要提交临时官方仓库。

### 6. 清理临时目录

生成、测试和差异检查完成后可以删除临时官方仓库；删除它不会影响构建或线上运行。清理不是必须立即执行，系统临时目录也可能由操作系统回收。

手工清理时只删除本次明确创建的 `$catalogSource` 目录。删除前再次输出并确认它位于系统临时目录，且目录名以 `player-bedrock-samples-` 开头；不要对项目目录、用户目录或来源不明的变量执行递归删除。
