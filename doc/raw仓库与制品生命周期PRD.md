# Raw 仓库与制品生命周期 PRD

## 1. 目标

为 Nexus Repository Community/OSS 3.76 提供 `raw/hosted` 仓库的声明式及单次命令管理，并为 raw 制品提供可预览、可审计的生命周期清理。

原生 Cleanup Policies REST API 在目标版本中属于 Pro 能力，本功能不调用非官方内部 API。生命周期由 `nexus-cli` 通过官方 Components API 自行执行。

## 2. 功能

- `repo raw apply` 幂等应用 `repositories.raw` 配置；`--dry-run` 只展示动作。
- `repo raw ensure` 通过命令行参数创建或更新单仓库。
- 已有同名仓库必须是 `raw/hosted`。blob store 不一致、format/type 不一致时失败，不迁移、不删除重建。
- 可安全更新 online、严格内容类型校验、write policy 和 content disposition，并保留服务端已有的其他配置。
- `repo lifecycle preview` 分页扫描 component，按保留天数、包含路径和排除路径生成候选清单。
- `repo lifecycle run --yes` 删除候选 component。未传 `--yes` 时拒绝执行。

## 3. 生命周期语义

- raw 中每个文件视为一个 component。
- component 含多个 asset 时，使用最新的 `lastModified`，避免误删仍被更新的 component。
- component 年龄达到或超过 `retentionDays` 才进入候选。
- `includePaths` 使用 Go RE2 正则；为空表示匹配全部。
- `excludePaths` 使用 Go RE2 正则并始终优先。
- 缺失或无法解析时间的 component 跳过并告警。
- 删除时遇到 404 视为并发删除，记录后继续；其他错误立即停止。

## 4. 运维与安全

推荐先运行 preview，再由 cron 调用 run：

```cron
30 2 * * * /opt/nexus-cli repo lifecycle run --config /etc/nexus-cli/config.yaml --repo protected-repo-example --yes
```

管理员密码只从配置指定的环境变量读取。仓库变更与生命周期执行写入 JSONL 审计日志，包含扫描数、候选数和删除数，不包含密码或 Authorization header。

删除 component 是逻辑内容删除，不等于立即释放 blob store 的磁盘空间。磁盘回收仍需在 Nexus 中配置并运行相应的 compact blob store 任务。

## 5. 验收标准

- 重复应用相同仓库配置不产生写请求。
- 类型或 blob store 冲突时不修改仓库。
- preview 不产生删除请求，run 未确认时不连接 Nexus。
- 过期、包含且未被排除的文件才会删除。
- 分页扫描、无效时间、并发 404 和中途 API 失败均有确定输出及审计记录。
