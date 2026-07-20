# AI 调用指南

> 来源：[AI通过nexus-cli完成Nexus常用操作PRD.md](./AI通过nexus-cli完成Nexus常用操作PRD.md) 与 [AI可调用能力清单.md](./AI可调用能力清单.md)
> 对应任务：AI-002 补充 AI 调用指南文档
> 目标：让 AI Agent 通过 `nexus-cli` 安全完成 Nexus 常用操作。

## 1. 总原则

- AI 只能通过 `nexus-cli` 操作 Nexus，不直接调用 Nexus REST API。
- AI 先读取状态，再生成 dry-run 或预览计划，最后等待人工确认后执行真实写操作。
- 管理员密码只通过配置里的 `nexus.passwordEnv` 指定环境变量读取，不写入配置、日志、JSON 响应或对话总结。
- 只读命令可以直接执行；写操作必须先 dry-run 或 preview；高风险操作必须等待人工确认。
- AI 调用只读命令和 dry-run 计划时默认追加 `--output json`，只在人类排查需要时使用默认文本输出。
- 真实写操作必须显式追加 `--yes`；当前真实执行结果保持文本输出，执行后再用只读 JSON 命令复查状态。

## 2. 推荐调用流程

### 2.1 读取状态

AI 在执行任何变更前，应先读取 Nexus 和目标资源状态。

```sh
nexus-cli health check --config config.yaml --output json
nexus-cli repo list --config config.yaml --output json
nexus-cli blobstore list --config config.yaml --output json
nexus-cli guest check --config config.yaml --output json
```

HA 场景先读取双节点状态：

```sh
nexus-cli ha status --config config.yaml --output json
nexus-cli ha health --config config.yaml --output json
```

### 2.2 生成计划

写操作必须先 dry-run 或 preview。AI 应把计划摘要给用户确认，而不是直接执行真实变更。

```sh
nexus-cli repo apply --config config.yaml --dry-run --output json
nexus-cli blobstore apply --config config.yaml --dry-run --output json
nexus-cli guest protect --config config.yaml --dry-run --output json
nexus-cli user create-readonly --config config.yaml \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --dry-run \
  --output json
nexus-cli repo lifecycle preview --config config.yaml --repo devops-prod-generic
```

### 2.3 等待确认

AI 应向用户说明：

- 将修改哪些资源。
- 哪些资源会创建、更新、复用、删除或跳过。
- 是否会创建用户、修改权限、删除制品或执行外部同步命令。
- 是否会产生审计记录。

未获得明确确认时，AI 不执行真实写操作。

### 2.4 执行变更

用户确认后，AI 才执行真实写操作。当前已存在确认门禁的命令必须带对应参数。

```sh
nexus-cli repo apply --config config.yaml --yes
nexus-cli blobstore apply --config config.yaml --yes
nexus-cli guest protect --config config.yaml --yes
nexus-cli user create-readonly --config config.yaml \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --yes
nexus-cli repo lifecycle run --config config.yaml --repo devops-prod-generic --yes
```

HA 故障切换必须由人确认 fencing 已完成：

```sh
nexus-cli ha failover --config config.yaml \
  --from primary \
  --to standby \
  --fencing-confirmed
```

### 2.5 检查结果和审计

真实写操作后，AI 应再次读取状态，并提示用户查看审计日志。

```sh
nexus-cli guest check --config config.yaml --output json
nexus-cli repo list --config config.yaml --output json
nexus-cli blobstore list --config config.yaml --output json
```

审计日志路径由配置中的 `audit.logPath` 决定，例如：

```yaml
audit:
  enabled: true
  logPath: "./logs/nexus-cli-audit.log"
```

## 3. 常见任务示例

### 3.1 创建或更新通用仓库

适用场景：用户希望用配置文件管理 npm、maven2、docker 等仓库。

```sh
nexus-cli repo list --config config.yaml --output json
nexus-cli repo apply --config config.yaml --dry-run --output json
```

AI 向用户确认 `repo apply --dry-run` 输出的 create / update / unchanged 结果。用户确认后执行：

```sh
nexus-cli repo apply --config config.yaml --yes
```

### 3.2 创建或更新 Blob Store

适用场景：用户希望用配置文件管理 file Blob Store。

```sh
nexus-cli blobstore list --config config.yaml --output json
nexus-cli blobstore apply --config config.yaml --dry-run --output json
```

用户确认后执行：

```sh
nexus-cli blobstore apply --config config.yaml --yes
```

### 3.3 收敛访客权限

适用场景：用户希望 anonymous / guest 只能看到允许公开的仓库，不能访问受保护仓库。

```sh
nexus-cli guest check --config config.yaml --output json
nexus-cli guest protect --config config.yaml --dry-run --output json
```

AI 应重点说明：

- 哪些仓库会授予 browse + read。
- 哪些仓库会完全拒绝 guest 访问。
- 哪些高危权限会被移除。

用户确认后执行：

```sh
nexus-cli guest protect --config config.yaml --yes
nexus-cli guest check --config config.yaml --output json
```

### 3.4 授予 raw 仓库目录访问权限

适用场景：用户希望某个用户只能访问 raw 仓库下的一个目录。

```sh
nexus-cli user create-readonly --config config.yaml \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice \
  --last-name Team \
  --dry-run \
  --output json
```

AI 应说明将创建或复用的 selector、privilege、role 和 user。用户确认后执行：

```sh
nexus-cli user create-readonly --config config.yaml \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice \
  --last-name Team \
  --yes
```

真实执行后生成的密码只会打印一次。AI 不得把该密码写入审计日志或长期记录。

### 3.5 预览并清理过期 Raw 制品

适用场景：用户希望删除超过保留天数且符合路径规则的 raw component。

```sh
nexus-cli repo lifecycle preview --config config.yaml --repo devops-prod-generic
```

AI 应列出候选数量、路径规则和风险。用户确认后执行：

```sh
nexus-cli repo lifecycle run --config config.yaml --repo devops-prod-generic --yes
```

`repo lifecycle run` 会删除 component，必须先 preview，且必须带 `--yes`。

### 3.6 查看 HA 状态和执行故障切换

适用场景：用户需要检查温备状态或执行人工故障切换。

```sh
nexus-cli ha status --config config.yaml --output json
nexus-cli ha health --config config.yaml --output json
```

故障切换前，AI 必须提醒用户人工确认旧主节点已停止或隔离。确认后才可执行：

```sh
nexus-cli ha failover --config config.yaml \
  --from primary \
  --to standby \
  --fencing-confirmed
```

AI 不自动调用 F5，也不承诺零 RPO。

## 4. 失败处理规则

- 配置读取失败：提示用户检查 `--config` 路径和 YAML 字段。
- 密码环境变量缺失：提示用户导出配置中 `passwordEnv` 对应的环境变量，不要求用户把密码发给 AI。
- Nexus 认证失败：提示用户检查账号、密码环境变量和权限。
- Nexus API 错误：保留状态码和简短错误，不复述敏感请求头。
- 统一错误码上线后：AI 应先读取 JSON 响应中的 `errors[].code` 和 `errors[].message`，再结合审计日志决定是否重试、请求用户补充信息或停止。
- dry-run 失败：停止执行，不进入真实写操作。
- 真实写操作部分失败：提示用户查看审计日志和命令输出；优先使用同一命令幂等重试，避免手工反向修复。

## 5. AI 不应执行的行为

- 不直接调用 Nexus REST API 绕过 `nexus-cli`。
- 不在配置文件中写入管理员密码。
- 不把密码、Token、Authorization header 写入日志、文档或对话总结。
- 不在未 dry-run 或未 preview 的情况下执行写操作。
- 不在未确认 fencing 的情况下执行 `ha failover`。
- 不删除仓库、Blob Store 或用户；这些能力不在当前 v1 范围内。

## 6. AI-002 验收对照

| 验收项 | 覆盖位置 |
| --- | --- |
| 指南包含只读命令示例和写操作示例。 | 第 2 节和第 3 节。 |
| 指南说明默认使用 `--output json` 给 AI 解析。 | 第 1 节、第 2 节和第 3 节示例。 |
| 指南说明真实写操作必须显式确认。 | 第 2.3、2.4 节。 |
| 指南说明密码只通过环境变量读取。 | 第 1 节和第 4 节。 |
| 指南说明失败后应先读错误码和审计日志，再决定是否重试。 | 第 2.5 节和第 4 节。 |
| README 或相关文档能链接到该指南。 | README / README.zh 的文档入口。 |
