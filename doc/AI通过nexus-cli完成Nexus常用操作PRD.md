# AI 通过 nexus-cli 完成 Nexus 常用操作 PRD

## 1. Summary

本需求让 AI 可以通过 `nexus-cli` 完成 Nexus Repository 的常用查询、配置、授权、清理和运维操作。核心不是让 AI 直接操作 Nexus UI 或任意 REST API，而是把 `nexus-cli` 作为安全边界，提供稳定、可审计、可预览的命令接口。

第一版本重点补齐 AI 调用所需的结构化输出、dry-run 计划、确认门禁、错误分类和调用指南，让 AI 能先读状态、再生成计划、最后在明确确认后执行变更。

## 2. Contacts

| Name | Role | Comment |
| --- | --- | --- |
| Nexus / DevOps 管理员 | 用户 / 验收方 | 希望用自然语言让 AI 辅助完成 Nexus 运维。 |
| SRE / 运维人员 | 使用方 | 关注安全、dry-run、审计、失败可定位和误操作防护。 |
| nexus-cli 维护者 | 开发方 | 负责 CLI 能力、结构化输出、测试和文档。 |
| AI Agent / Codex | 调用方 | 通过命令行读取状态、生成计划、执行经确认的操作。 |

## 3. Background

当前 `nexus-cli` 已支持多类 Nexus 常见运维能力：

- 仓库基础管理：`repo list/get/apply/ensure`。
- Blob Store 管理：`blobstore list/get/apply/ensure`。
- 访客权限治理：`guest protect/check`。
- 用户目录级只读用户创建：`user create-readonly`。
- Raw 制品生命周期：`repo lifecycle preview/run`。
- 健康检查和温备 HA 运维：`health check`、`ha status/health/sync/failover`。

这些能力已经适合人类在终端中使用，但 AI 调用时还缺少统一契约：

- 部分输出更适合人读，不适合机器稳定解析。
- 写操作虽然已有 dry-run，但缺少统一的计划输出格式。
- AI 需要知道哪些命令可直接运行，哪些命令必须先 dry-run，哪些必须人工确认。
- 错误信息需要能被 AI 分类，例如配置错误、认证失败、Nexus API 错误、确认参数缺失。
- 审计日志需要覆盖 AI 触发的变更，但仍不能记录密码或 Authorization header。

因此，本需求把现有 CLI 能力整理为 AI 可调用的安全操作层。

## 4. Objective

目标是让 AI 能通过 `nexus-cli` 安全完成 Nexus 日常运维，同时保留人类管理员对高风险变更的控制权。

Key Results：

- KR1：AI 可以用结构化输出读取 Nexus 当前状态，覆盖核心只读命令。
- KR2：AI 可以对写操作生成 dry-run 计划，并稳定解析计划结果。
- KR3：真实写操作必须有显式确认参数，AI 不能静默执行高风险变更。
- KR4：所有写操作都有审计日志，日志不包含密码、环境变量值或 Authorization header。
- KR5：AI 可以覆盖 80% 以上日常 Nexus 运维任务，包括查询、仓库配置、Blob Store 配置、访客权限、目录分享、生命周期预览和 HA 状态检查。
- KR6：同一目标状态重复执行时保持幂等，不重复创建资源。
- KR7：失败时返回清晰错误，AI 能判断是否可重试、是否需要用户补充信息、是否应停止。

## 5. Market Segment(s)

本功能服务于需要自动化 Nexus 运维的 DevOps、SRE、平台管理员。

他们的核心任务是快速完成以下工作：

- 检查 Nexus 是否健康。
- 查看仓库、Blob Store、权限、用户授权和 HA 状态。
- 创建或更新仓库。
- 创建或更新 file Blob Store。
- 控制 anonymous / guest 权限。
- 给用户授予某个 raw 仓库目录访问权限。
- 预览或执行过期 Raw 制品清理。
- 查看主从温备状态，执行受控同步或故障切换指引。

主要约束：

- Nexus 权限模型复杂，错误授权风险高。
- AI 不能保存或打印管理员密码。
- 删除、授权、清理、故障切换属于高风险操作，必须有防护门禁。
- Nexus 小版本 API 字段可能不同，CLI 要给出清晰错误和文档说明。
- 现有人类用户和脚本不能因为新增 AI 能力而被破坏。

## 6. Value Proposition(s)

- 管理员可以用自然语言描述目标，由 AI 调用 `nexus-cli` 完成操作。
- AI 可以先查看现状，再生成执行计划，减少盲目变更。
- 写操作统一支持 dry-run，降低误操作风险。
- 高风险操作要求显式确认，避免 AI 自动越过安全边界。
- 审计日志让团队知道谁在什么时候改了什么。
- CLI 成为 Nexus 运维的稳定安全边界，而不是让 AI 直接调用任意 REST API。
- 常用运维任务可以自动化，复杂或危险动作仍保留人工控制。

## 7. Solution

### 7.1 UX / Commands

AI 的标准流程是：先读状态，再生成计划，最后在确认后执行。

只读状态命令示例：

```sh
nexus-cli health check --output json
nexus-cli repo list --output json
nexus-cli repo get --name npm-hosted --format npm --type hosted --output json
nexus-cli blobstore list --output json
nexus-cli blobstore get --name default --type file --output json
nexus-cli guest check --output json
nexus-cli ha status --output json
nexus-cli ha health --output json
```

写操作先 dry-run：

```sh
nexus-cli repo apply --dry-run --output json
nexus-cli blobstore apply --dry-run --output json
nexus-cli guest protect --dry-run --output json
nexus-cli user create-readonly --repo devops-prod-generic --path /team-a/ --user alice --email alice@example.com --dry-run --output json
nexus-cli repo lifecycle preview --repo devops-prod-generic --output json
```

人工确认后执行：

```sh
nexus-cli repo apply --yes --output json
nexus-cli blobstore apply --yes --output json
nexus-cli guest protect --yes --output json
nexus-cli user create-readonly --repo devops-prod-generic --path /team-a/ --user alice --email alice@example.com --yes --output json
nexus-cli repo lifecycle run --repo devops-prod-generic --yes --output json
```

HA 高风险操作继续使用专用门禁：

```sh
nexus-cli ha sync --once --yes --output json
nexus-cli ha failover --from primary --to standby --fencing-confirmed --output json
```

### 7.2 Key Features

AI 可发现能力：

- 新增 AI 调用指南，列出常用命令、风险等级、是否只读、是否支持 dry-run、是否需要确认。
- 可选新增 `nexus-cli ai capabilities --output json`，让 AI 读取命令能力清单。
- 能力清单不包含密码、Token 或真实环境敏感信息。

结构化输出：

- 为核心命令增加 `--output text|json`。
- 默认保持 `text`，兼容现有人类使用方式和脚本。
- JSON 输出使用统一 envelope，方便 AI 稳定解析。
- 错误输出包含机器可读 `code` 和人类可读 `message`。

安全写操作：

- 写操作统一支持 `--dry-run`，输出 planned / would_create / would_update / would_delete / unchanged / skipped / failed。
- 真实写操作必须传 `--yes`，否则拒绝执行并返回确认缺失错误。
- 特别危险的操作继续保留专用确认参数，例如 `--fencing-confirmed`。
- 所有写操作写入审计日志。

常用操作范围：

- 只读：`health check`、`repo list/get`、`blobstore list/get`、`guest check`、`ha status/health`。
- 低风险写：`repo ensure/apply`、`blobstore ensure/apply`。
- 高风险写：`guest protect`、`user create-readonly`、`repo lifecycle run`、`ha sync`、`ha failover`。

### 7.3 Technology

新增或标准化 CLI 参数：

```sh
--output text|json
--dry-run
--yes
--config <path>
```

标准 JSON 成功响应：

```json
{
  "command": "repo apply",
  "dryRun": true,
  "result": "success",
  "changes": [
    {
      "resourceType": "repository",
      "name": "npm-hosted",
      "action": "would_update"
    }
  ],
  "warnings": [],
  "errors": [],
  "auditLogPath": "./logs/nexus-cli-audit.log"
}
```

标准 JSON 失败响应：

```json
{
  "command": "guest protect",
  "dryRun": false,
  "result": "failed",
  "changes": [],
  "warnings": [],
  "errors": [
    {
      "code": "NEXUS_AUTH_FAILED",
      "message": "authentication failed"
    }
  ],
  "auditLogPath": "./logs/nexus-cli-audit.log"
}
```

建议错误码：

| Code | Meaning |
| --- | --- |
| `CONFIG_INVALID` | 配置文件缺失、字段错误或校验失败。 |
| `PASSWORD_ENV_MISSING` | 配置指定的密码环境变量未设置。 |
| `NEXUS_AUTH_FAILED` | Nexus 认证失败。 |
| `NEXUS_API_ERROR` | Nexus API 返回非预期错误。 |
| `CONFIRMATION_REQUIRED` | 真实写操作缺少确认参数。 |
| `UNSUPPORTED_OUTPUT` | `--output` 参数值不支持。 |
| `OPERATION_CONFLICT` | 目标资源状态冲突，例如同名资源类型不匹配。 |

### 7.4 Assumptions

- AI 的入口是命令行，不新增 Web UI。
- v1 不把自然语言解析内置到 `nexus-cli`，自然语言理解由外部 AI 完成。
- v1 不开放任意 Nexus REST API 透传能力。
- v1 不做删除仓库、删除 Blob Store、删除用户。
- 密码仍只通过环境变量读取，不进入配置、日志、JSON 输出。
- 当前已有命令继续兼容，新增结构化输出不破坏旧用法。

## 8. Release

### v1

- 为核心只读命令补齐 `--output json`。
- 为核心写操作补齐统一 JSON 计划输出。
- 为写操作统一确认语义：dry-run 可直接执行，真实变更需显式确认。
- 更新 README / PRD，新增 AI 调用指南。
- 增加单元测试，覆盖 JSON 输出、错误结构、dry-run 和确认门禁。

### Future

- 增加 `nexus-cli ai capabilities`，输出 AI 可调用命令清单。
- 增加 `nexus-cli ai explain <command>`，输出命令风险等级和示例。
- 支持更多 Nexus 对象查询：用户、角色、权限、Content Selector。
- 支持制品查询、下载链接生成、上传封装。
- 支持策略文件校验，帮助 AI 在执行前发现危险配置。
