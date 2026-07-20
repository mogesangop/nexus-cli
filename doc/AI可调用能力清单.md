# AI 可调用能力清单

> 来源：[AI通过nexus-cli完成Nexus常用操作PRD.md](./AI通过nexus-cli完成Nexus常用操作PRD.md) 与 [AI通过nexus-cli完成Nexus常用操作用户故事拆分.md](./AI通过nexus-cli完成Nexus常用操作用户故事拆分.md)
> 对应任务：AI-001 输出 AI 可调用能力清单
> 目标读者：AI Agent、DevOps 平台管理员、SRE / 运维人员

## 1. 使用边界

- AI 只能通过 `nexus-cli` 调用 Nexus 常用操作，不直接调用 Nexus REST API。
- AI 不读取、保存、打印 Nexus 管理员密码；密码仍只从配置中的 `passwordEnv` 指定环境变量读取。
- 能力清单只描述命令能力、风险等级和安全门禁，不包含真实 URL、Token、密码或 Authorization header。
- 只读命令和写操作 dry-run 支持 `--output json`；真实写操作必须带 `--yes`，当前保持文本执行结果输出。

## 2. 风险等级定义

| 风险等级 | 定义 | AI 默认动作 |
| --- | --- | --- |
| 只读 | 只查询 Nexus 或本地状态，不修改 Nexus 或本地持久状态。 | 可直接执行。 |
| 低风险写 | 创建或更新目标状态资源，通常幂等，不删除数据。 | 必须先 dry-run；真实执行需人工确认。 |
| 高风险写 | 修改权限、创建用户、删除制品、执行外部同步命令或故障切换。 | 必须先 dry-run 或预览；真实执行需人工确认或专用确认参数。 |

## 3. 命令能力清单

| 命令 | 主要用途 | 风险等级 | 是否只读 | dry-run / 预览 | 真实执行确认 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| `config init` | 生成本地配置模板。 | 低风险写 | 否 | 不支持 | 当前不需要 `--yes` | 写本地配置文件，不访问 Nexus。 |
| `health check` | 检查 Nexus 连接、认证和关键 API。 | 只读 | 是 | 不适用 | 不需要 | 支持 `--output json`。 |
| `repo list` | 列出 Nexus 仓库，可按 format/type 筛选。 | 只读 | 是 | 不适用 | 不需要 | 支持 `--output json`。 |
| `repo get` | 查询单个仓库完整 API 配置。 | 只读 | 是 | 不适用 | 不需要 | 支持统一 JSON envelope。 |
| `repo apply` | 应用配置中的通用仓库目标状态。 | 低风险写 | 否 | `--dry-run --output json` | `--yes` | 幂等创建、更新或 unchanged。 |
| `repo ensure` | 从 settings 文件创建或更新一个通用仓库。 | 低风险写 | 否 | `--dry-run --output json` | `--yes` | settings 字段透传 Nexus 仓库 API。 |
| `repo raw apply` | 应用配置中的 raw hosted 仓库目标状态。 | 低风险写 | 否 | `--dry-run` | `--yes` | 安全更新 raw hosted 仓库可变字段。 |
| `repo raw ensure` | 创建或更新单个 raw hosted 仓库。 | 低风险写 | 否 | `--dry-run` | `--yes` | 拒绝 blob store 迁移。 |
| `repo lifecycle preview` | 预览 raw 制品生命周期候选删除项。 | 只读 | 是 | 命令本身即预览 | 不需要 | 不删除 component。 |
| `repo lifecycle run` | 删除过期 raw component。 | 高风险写 | 否 | `--dry-run --output json` 或先执行 `preview` | `--yes` | 删除 component 前必须确认。 |
| `blobstore list` | 列出 Blob Store。 | 只读 | 是 | 不适用 | 不需要 | 支持 `--output json`。 |
| `blobstore get` | 查询单个 file Blob Store。 | 只读 | 是 | 不适用 | 不需要 | 支持统一 JSON envelope。 |
| `blobstore apply` | 应用配置中的 file Blob Store 目标状态。 | 低风险写 | 否 | `--dry-run --output json` | `--yes` | 幂等创建、更新或 unchanged。 |
| `blobstore ensure` | 创建或更新单个 file Blob Store。 | 低风险写 | 否 | `--dry-run --output json` | `--yes` | 仅支持 file 类型。 |
| `guest check` | 校验 guest / anonymous 权限是否符合配置。 | 只读 | 是 | 不适用 | 不需要 | 支持 `--output json`。 |
| `guest protect` | 按配置收敛 guest / anonymous 权限。 | 高风险写 | 否 | `--dry-run --output json` | `--yes` | 修改 Nexus 权限和角色绑定。 |
| `guest sync` | `guest protect` 的兼容别名。 | 高风险写 | 否 | `--dry-run` | `--yes` | 已废弃但保留兼容。 |
| `user create-readonly` | 创建拥有 raw 仓库目录级只读访问权限的用户。 | 高风险写 | 否 | `--dry-run --output json` | `--yes` | 创建 user、selector、privilege、role；真实执行会生成一次性密码。 |
| `ha status` | 查看 HA 节点健康和复制延迟。 | 只读 | 是 | 不适用 | 不需要 | 读取 HA 状态文件并检查节点。 |
| `ha health` | 对 HA 两节点执行 API 健康检查。 | 只读 | 是 | 不适用 | 不需要 | 支持 `--output json`。 |
| `ha sync --once` | 执行配置中的 blob 和 metadata 同步命令一次。 | 高风险写 | 否 | 不支持 | `--once` | 会执行外部命令并更新 HA 状态文件；由 `--once` 明确触发。 |
| `ha failover` | 引导人工故障切换并记录审计。 | 高风险写 | 否 | 不支持 | `--fencing-confirmed` | 可能执行末次同步；不自动调用 F5。 |

## 4. AI 调用判定规则

| 场景 | AI 是否可直接执行 | 原因 |
| --- | --- | --- |
| 只读命令 | 可以 | 不修改 Nexus 或本地持久状态。 |
| 带 `--dry-run` 的低风险写操作 | 可以 | 只计算计划，不写 Nexus。 |
| 带 `--dry-run` 的高风险写操作 | 可以 | 只预览权限、授权或清理计划。 |
| 无 dry-run 的低风险写操作 | 不可以 | 必须等待用户确认目标状态和执行窗口。 |
| 无 dry-run 的高风险写操作 | 不可以 | 必须等待用户确认，且使用对应确认参数。 |
| `ha failover` | 不可以自动执行 | 必须由人确认 fencing 已完成。 |

## 5. 敏感信息约束

- 能力清单、JSON 响应和审计日志不得包含 Nexus 管理员密码。
- 能力清单、JSON 响应和审计日志不得包含 Authorization header。
- `user create-readonly` 真实执行生成的用户密码只能作为一次性结果返回给调用方，不能写入审计日志。
- AI 在报错、总结或日志中不得复述环境变量值。

## 6. AI-001 验收对照

| 验收项 | 覆盖位置 |
| --- | --- |
| 文档列出核心命令：health、repo、blobstore、guest、user、repo lifecycle、ha。 | 第 3 节命令能力清单。 |
| 每个命令标明风险等级：只读、低风险写、高风险写。 | 第 3 节“风险等级”列。 |
| 每个写操作标明是否支持 `--dry-run`。 | 第 3 节“dry-run / 预览”列。 |
| 每个真实写操作标明需要 `--yes` 或专用确认参数。 | 第 3 节“真实执行确认”列。 |
| AI 可以根据清单判断某个任务是否允许直接执行。 | 第 4 节 AI 调用判定规则。 |
| 清单不暴露密码、Token 或 Authorization header。 | 第 1 节和第 5 节敏感信息约束。 |
