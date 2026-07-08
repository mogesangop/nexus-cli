# AI 通过 nexus-cli 完成 Nexus 常用操作用户故事拆分

> 来源：[AI通过nexus-cli完成Nexus常用操作PRD.md](./AI通过nexus-cli完成Nexus常用操作PRD.md)
> 拆分方法：3C（Card、Conversation、Confirmation）+ INVEST
> 范围约束：AI 只通过 `nexus-cli` 操作 Nexus；v1 不内置自然语言解析；v1 不开放任意 Nexus REST API 透传；默认文本输出保持兼容。

## 1. 角色

| 角色 | 核心任务 |
| --- | --- |
| Nexus / DevOps 管理员 | 用自然语言委托 AI 完成常见 Nexus 运维，并验收执行计划 |
| SRE / 运维人员 | 关注 dry-run、确认门禁、错误可定位、审计可追踪 |
| AI Agent / Codex | 读取 CLI 能力和当前状态，生成计划并调用命令 |
| nexus-cli 维护者 | 实现结构化输出、安全门禁、测试和文档 |

## 2. Epic 与发布范围

| Epic | 目标 | 版本 | 优先级 |
| --- | --- | --- | --- |
| E1 AI 可发现能力 | 让 AI 知道哪些命令可调用、风险如何、是否需要确认 | v1 | P0 |
| E2 结构化输出 | 让 AI 稳定解析状态、计划、错误和审计路径 | v1 | P0 |
| E3 安全写操作 | 让写操作先 dry-run，真实执行必须显式确认 | v1 | P0 |
| E4 测试与兼容 | 保持旧用法不变，并补齐 AI 调用相关测试 | v1 | P0 |
| E5 远期增强 | 增加更多对象查询、制品操作和策略校验 | future | P2 |

## 3. v1 用户故事

### AI-001 输出 AI 可调用能力清单

**Card**：作为 AI Agent，我希望读取 `nexus-cli` 支持的常用操作清单，以便知道哪些命令可以直接调用、哪些命令需要先预览或等待人工确认。

**Conversation**：

- 能力清单可以先以文档形式交付，后续可增加 `nexus-cli ai capabilities --output json`。
- 清单应覆盖命令、风险等级、是否只读、是否支持 dry-run、真实执行是否需要确认。
- 清单不能包含真实密码、Token、URL 中的敏感参数或环境变量值。
- 设计参考：PRD §7.2 AI 可发现能力。

**Confirmation**：

1. 文档列出核心命令：health、repo、blobstore、guest、share、repo lifecycle、ha。
2. 每个命令标明风险等级：只读、低风险写、高风险写。
3. 每个写操作标明是否支持 `--dry-run`。
4. 每个真实写操作标明需要 `--yes` 或专用确认参数。
5. AI 可以根据清单判断某个任务是否允许直接执行。
6. 清单不暴露密码、Token 或 Authorization header。

**依赖**：无。

### AI-002 补充 AI 调用指南文档

**Card**：作为 Nexus / DevOps 管理员，我希望有一份 AI 调用指南，以便团队知道如何让 AI 安全使用 `nexus-cli`。

**Conversation**：

- 指南放在 `doc/` 或 README 链接中，语言保持中文。
- 指南说明推荐流程：读取状态、dry-run、人工确认、执行、检查审计。
- 指南给出常见任务示例，例如创建仓库、保护访客权限、分享 raw 目录、预览生命周期清理。
- 指南明确 AI 不应直接调用 Nexus REST API。
- 设计参考：PRD §7.1 UX / Commands。

**Confirmation**：

1. 指南包含只读命令示例和写操作示例。
2. 指南说明默认使用 `--output json` 给 AI 解析。
3. 指南说明真实写操作必须显式确认。
4. 指南说明密码只通过环境变量读取。
5. 指南说明失败后应先读错误码和审计日志，再决定是否重试。
6. README 或相关文档能链接到该指南。

**依赖**：AI-001。

### AI-101 为只读命令增加 `--output text|json`

**Card**：作为 AI Agent，我希望只读命令支持 JSON 输出，以便稳定读取 Nexus 当前状态。

**Conversation**：

- 默认输出仍为 `text`，保持人类用户和旧脚本兼容。
- JSON 输出覆盖 `health check`、`repo list/get`、`blobstore list/get`、`guest check`、`ha status/health`。
- JSON 字段应稳定，避免依赖表格列宽或自然语言文本。
- 设计参考：PRD §7.2 结构化输出。

**Confirmation**：

1. 每个目标只读命令都接受 `--output text|json`。
2. 未传 `--output` 时输出与当前文本输出保持兼容。
3. 传 `--output json` 时 stdout 输出合法 JSON。
4. JSON 中包含命令名、结果状态和核心数据。
5. 不支持的输出格式返回 `UNSUPPORTED_OUTPUT`。
6. 只读命令不会写审计日志或修改 Nexus 状态。

**依赖**：无。

### AI-102 定义统一 JSON 响应结构

**Card**：作为 nexus-cli 维护者，我希望定义统一 JSON 响应结构，以便所有命令用同一种方式表达成功、失败、变更和告警。

**Conversation**：

- 统一 envelope 包含 `command`、`dryRun`、`result`、`changes`、`warnings`、`errors`、`auditLogPath`。
- 只读命令可以在同一 envelope 中增加 `data`。
- 写操作使用 `changes` 表达 planned、created、updated、unchanged、skipped、failed 等动作。
- 设计参考：PRD §7.3 Technology。

**Confirmation**：

1. 成功响应包含 `result: "success"`。
2. 失败响应包含 `result: "failed"` 和至少一个 error。
3. dry-run 响应明确包含 `dryRun: true`。
4. 写操作响应包含 `changes` 数组，即使没有变化也返回空数组或 unchanged 项。
5. 启用审计时返回 `auditLogPath`。
6. JSON 响应不包含密码、环境变量值或 Authorization header。

**依赖**：AI-101。

### AI-103 为错误输出增加机器可读 code

**Card**：作为 AI Agent，我希望错误中包含机器可读 code，以便判断下一步是重试、请求用户补充信息，还是停止执行。

**Conversation**：

- 错误码先覆盖高频场景：配置错误、密码环境变量缺失、认证失败、Nexus API 错误、确认参数缺失、输出格式不支持、资源冲突。
- 文本输出仍可保持人类可读错误。
- JSON 输出中每个错误包含 `code` 和 `message`。
- 设计参考：PRD §7.3 建议错误码。

**Confirmation**：

1. 配置校验失败返回 `CONFIG_INVALID`。
2. 密码环境变量缺失返回 `PASSWORD_ENV_MISSING`。
3. Nexus 认证失败返回 `NEXUS_AUTH_FAILED`。
4. Nexus API 非预期错误返回 `NEXUS_API_ERROR`。
5. 缺少确认参数返回 `CONFIRMATION_REQUIRED`。
6. 同名资源类型冲突返回 `OPERATION_CONFLICT`。

**依赖**：AI-102。

### AI-201 统一写操作 dry-run 计划输出

**Card**：作为 SRE / 运维人员，我希望 AI 在执行写操作前先输出 dry-run 计划，以便我能看到将要修改哪些 Nexus 资源。

**Conversation**：

- 覆盖 `repo apply/ensure`、`blobstore apply/ensure`、`guest protect`、`share grant`、`repo lifecycle run`。
- `repo lifecycle preview` 仍作为只读预览命令存在。
- dry-run 不应创建用户、不应生成真实密码、不应写入 Nexus。
- 设计参考：PRD §7.2 安全写操作。

**Confirmation**：

1. 目标写操作均支持 `--dry-run --output json`。
2. dry-run 输出列出资源类型、资源名和计划动作。
3. dry-run 不向 Nexus 发送创建、更新、删除请求。
4. `share grant --dry-run` 不生成真实密码。
5. 生命周期 dry-run 或 preview 能列出候选删除制品，不执行删除。
6. dry-run 失败时返回标准 JSON 错误。

**依赖**：AI-102。

### AI-202 为真实写操作统一确认门禁

**Card**：作为 Nexus / DevOps 管理员，我希望真实写操作必须显式确认，以便 AI 不会在我未确认时修改 Nexus。

**Conversation**：

- 普通真实写操作要求 `--yes`。
- `ha failover` 继续要求 `--fencing-confirmed`，不只依赖 `--yes`。
- `repo lifecycle run` 继续要求 `--yes`，因为它会删除制品。
- `--dry-run` 不需要 `--yes`。
- 设计参考：PRD §7.2 安全写操作。

**Confirmation**：

1. 未传 `--dry-run` 且未传确认参数时，写操作拒绝执行。
2. 拒绝执行时不修改 Nexus。
3. 拒绝执行时 JSON 错误码为 `CONFIRMATION_REQUIRED`。
4. `--dry-run` 可以不传 `--yes`。
5. `ha failover` 在需要 fencing 时仍必须传 `--fencing-confirmed`。
6. 文本文案清楚提示用户需要哪个确认参数。

**依赖**：AI-201。

### AI-203 确保写操作审计字段完整

**Card**：作为 SRE / 运维人员，我希望 AI 触发的写操作也能完整写入审计日志，以便事后追踪变更来源和结果。

**Conversation**：

- 审计记录包含命令、目标资源、dry-run、结果、错误信息和执行人。
- 审计记录不能包含密码、环境变量值或 Authorization header。
- dry-run 是否写审计可保持现有策略，但真实写操作必须写审计。
- 设计参考：PRD §7.2 安全写操作。

**Confirmation**：

1. `repo apply/ensure` 的审计记录包含仓库名和动作。
2. `blobstore apply/ensure` 的审计记录包含 Blob Store 名和动作。
3. `guest protect` 的审计记录包含目标角色和权限变更。
4. `share grant` 的审计记录包含目标仓库、路径、用户和角色。
5. `repo lifecycle run` 的审计记录包含目标仓库和删除结果摘要。
6. 审计日志不包含密码、Authorization header 或环境变量值。

**依赖**：AI-202。

### AI-301 补齐 JSON 输出单元测试

**Card**：作为 nexus-cli 维护者，我希望为 JSON 输出增加测试，以便后续修改不会破坏 AI 调用契约。

**Conversation**：

- 测试覆盖成功、失败、dry-run、空结果、多变更场景。
- 优先使用现有 CLI 层测试风格。
- 对 JSON 字段做结构校验，不依赖字段顺序。
- 设计参考：PRD §8 Release。

**Confirmation**：

1. 至少一个只读命令测试成功 JSON 输出。
2. 至少一个写操作测试 dry-run JSON 输出。
3. 至少一个失败场景测试 `errors[].code`。
4. 空结果时 JSON 仍可解析。
5. 多个 changes 时 JSON 数组内容正确。
6. 测试不需要真实 Nexus 网络连接。

**依赖**：AI-101、AI-102、AI-103、AI-201。

### AI-302 补齐确认门禁测试

**Card**：作为 nexus-cli 维护者，我希望确认门禁有自动化测试，以便 AI 不能绕过真实写操作确认。

**Conversation**：

- 测试缺少 `--yes` 的普通写操作。
- 测试 `--dry-run` 不需要 `--yes`。
- 测试 `ha failover` 的 fencing 门禁仍生效。
- 设计参考：PRD §7.2 安全写操作。

**Confirmation**：

1. 普通写操作缺少 `--yes` 时返回错误。
2. 缺少确认参数时不调用 Nexus 写 API。
3. `--dry-run` 路径不要求 `--yes`。
4. `repo lifecycle run` 缺少 `--yes` 时拒绝删除。
5. `ha failover` 缺少 `--fencing-confirmed` 时拒绝进入切换步骤。
6. JSON 输出下错误码为 `CONFIRMATION_REQUIRED`。

**依赖**：AI-202。

### AI-303 保持旧文本输出兼容

**Card**：作为现有 nexus-cli 用户，我希望新增 AI 能力后旧命令默认行为不变，以便已有脚本和人工操作不会被破坏。

**Conversation**：

- `--output` 默认值为 `text`。
- 原有命令名、必要参数和主要文本输出保持不变。
- 新增 `--yes` 可能影响真实写操作，需要在发布说明中明确迁移方式。
- 设计参考：PRD §7.4 Assumptions。

**Confirmation**：

1. 不传 `--output` 时，现有只读命令输出仍为文本表格或文本报告。
2. 现有配置文件无需修改即可加载。
3. `guest sync` 兼容别名继续可用。
4. README 说明真实写操作新增确认参数。
5. 旧文本输出相关测试仍通过。
6. `make test` 通过。

**依赖**：AI-101、AI-202。

## 4. Future 用户故事

### AI-401 增加更多 Nexus 对象查询

**Card**：作为 AI Agent，我希望查询用户、角色、权限和 Content Selector，以便在授权前判断现有访问范围。

**Conversation**：

- 该能力有助于解释 `share grant` 冲突和访客权限漂移。
- 第一阶段只读，不做删除或批量修改。

**Confirmation**：

1. 可以列出用户、角色、权限和 Content Selector。
2. 可以查询单个对象详情。
3. JSON 输出不包含密码。
4. 查询失败返回标准错误结构。

**依赖**：AI-102。

### AI-402 支持制品查询和下载链接生成

**Card**：作为 Nexus / DevOps 管理员，我希望 AI 能查询制品并生成下载链接，以便快速定位和交付构建产物。

**Conversation**：

- v1 不封装上传下载，本故事作为远期增强。
- 下载链接生成应尊重仓库权限和路径规则。

**Confirmation**：

1. 可以按仓库、路径、时间范围查询制品。
2. 可以生成可供用户使用的 Nexus 下载 URL。
3. 查询命令为只读。
4. JSON 输出包含仓库、路径、大小和最后修改时间。

**依赖**：AI-101。

### AI-403 支持策略文件校验

**Card**：作为 SRE / 运维人员，我希望 AI 在执行前校验策略文件，以便提前发现危险配置。

**Conversation**：

- 校验对象包括 guestAccess、repositories、blobStores、ha。
- 校验只读，不连接 Nexus 时也能发现基本错误。

**Confirmation**：

1. 可以校验配置文件语法和字段合法性。
2. 可以提示高风险 guest 权限配置。
3. 可以提示 HA fencing 被关闭等风险。
4. JSON 输出包含 warnings 和 errors。

**依赖**：AI-102。
