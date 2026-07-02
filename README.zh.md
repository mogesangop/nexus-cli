# nexus-cli

一个用于治理 **Nexus Repository 3.76** 访客 / 匿名访问的命令行工具。

第一版本解决一个问题：访客（匿名用户）在 Nexus UI 中能看到过多仓库与制品。Nexus 不支持「给所有仓库授予 browse，但排除某一个」的权限模型，因此 `nexus-cli` 会读取仓库列表，为每个仓库构建 `repository-view` 权限并绑定到访客角色 —— 对公开仓库授予 `browse+read`，对需要隐藏的仓库只授予 `read`（UI 不可见，但仍可通过精确 URL 下载）。

完整产品规格见 `doc/nexus-cli第一版本PRD.md`。

第二个用例是**按用户路径范围分享**：`share grant` 会创建内容选择器、路径范围的 `browse+read` 权限、角色和用户，让指定用户只能浏览/下载某一个仓库某一个目录下的制品，其它内容对其完全不可见。分享类资源使用独立的 `priv_share_` 前缀和 `role_share_*` 角色，与访客子系统互不可见、互不影响。

## 构建

```sh
make build          # 产物 ./nexus-cli
# 或直接调用：
CGO_ENABLED=0 go build -o nexus-cli ./cmd/nexus-cli
```

> Makefile 中默认 `GOPROXY=https://goproxy.cn,direct`。如需切换，可用
> `make build GOPROXY=https://proxy.golang.org,direct`。

## 快速开始

```sh
# 1. 生成配置模板（通用占位符）。
./nexus-cli config init --output config.yaml

# 2. 编辑 config.yaml：设置 baseUrl、roleName，以及 readOnly / browseRead
#    仓库列表。然后导出管理员密码：
export NEXUS_ADMIN_PASSWORD='your_password'

# 3. 验证连通性。
./nexus-cli health check --config config.yaml

# 4. 预览执行计划（不修改 Nexus）。
./nexus-cli guest sync --config config.yaml --dry-run

# 5. 执行同步。
./nexus-cli guest sync --config config.yaml

# 6. 校验漂移。
./nexus-cli guest check --config config.yaml
```

### 为用户授予某个仓库目录的访问权限

```sh
# 先 dry-run：打印将会创建的 selector/privilege/role/user。
./nexus-cli share grant \
  --config config.yaml \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice --last-name Team \
  --dry-run

# 正式执行。生成的密码只会打印一次到 stdout，请立即保存。
./nexus-cli share grant \
  --config config.yaml \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice --last-name Team
```

该授权是幂等的：使用相同参数重复执行会复用已存在的 selector、privilege 和 role。若用户已存在则**报错**——绝不重置已有用户密码。失败时不回滚已完成的步骤，因此可安全重试。

## 命令

| 命令 | 说明 |
| --- | --- |
| `config init --output config.yaml` | 生成配置模板。 |
| `repo list --config config.yaml` | 列出所有仓库（name、format、type）。 |
| `guest sync --config config.yaml [--dry-run] [--report FILE]` | 按配置同步访客角色权限。 |
| `guest check --config config.yaml` | 只读校验访客角色是否符合配置。 |
| `share grant --config ... --repo R --path /p/ --user U --email E` | 为指定用户创建路径范围的 browse+read 授权。 |
| `health check --config config.yaml` | 连接 / API / 认证健康检查。 |

### `share grant` 参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--repo` | 是 | 仓库名。 |
| `--path` | 是 | 目录路径，必须以 `/` 开头，如 `/team-a/`。 |
| `--user` | 是 | 要创建的用户 id，不能已存在。 |
| `--email` | 是 | 用户邮箱。 |
| `--first-name` / `--last-name` | 否 | 用户姓名。 |
| `--format` | 否 | 仓库 format，省略时通过 `repo list` 自动探测。 |
| `--password-length` | 否 | 生成密码长度（默认 24）。 |
| `--dry-run` | 否 | 只打印计划，不创建任何资源，也不生成密码。 |

## 配置

参见 `examples/config.example.yaml`。主要配置段：

- `nexus` —— 连接与凭证。`passwordEnv` 指定存放管理员密码的环境变量名（密码永不写入配置文件）。
- `guestAccess` —— 目标角色、仓库策略、禁止/警告权限。
- `privilegeNaming` —— 前缀（`priv_guest`）、分隔符、短横线替换。
- `audit` —— JSONL 审计日志路径与脱敏开关。
- `report` —— 报告目录与格式（`text` | `json`）。

### 仓库策略优先级（每个仓库）

```
deny > readOnly > browseRead > defaultPolicy
```

命中 `deny.repositories` 的仓库不授予任何权限；命中 `readOnly` 的只授予 `read`（UI 不可见，仍可下载）；匹配 `browseRead` 且未被排除的授予 `browse+read`；其余由 `defaultPolicy` 决定。

### 权限命名

`priv_guest_{format}_{sanitize后的仓库名}_{排序后的actions}` —— 例如
`priv_guest_raw_devops_prod_generic_read`。仓库名中的 `-`、`.`、`/` 会被替换为 `_`。

### 托管权限

`nexus-cli` 只管理名称以 `priv_guest_` 开头的权限。角色上**非托管**的权限会被保留 —— **例外**是 `forbiddenPrivileges` 中列出的（如 `nx-all`、`nx-admin`、`nx-repository-view-*-*-browse`），它们在 `sync` 时无论是否托管都会从访客角色移除。`warnPrivileges`（如 `nx-search-read`）在 `guest check` 中告警，但默认不移除。

## 幂等性

`guest sync` 是幂等的（PRD §14）：状态未变时第二次执行不会创建也不会移除任何内容。已存在且符合配置的托管权限会被跳过；陈旧的托管权限会被移除。

## 安全

- 管理员密码从环境变量读取，永不写入配置文件。
- 审计日志不含密码，也不含 `Authorization` 头。
- `--dry-run` 只计算并打印计划，不修改 Nexus。

## 故障排查

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 401 | 管理员密码错误 | 检查 `NEXUS_ADMIN_PASSWORD`。 |
| 403 | 账号缺少安全管理权限 | 使用管理员级别账号。 |
| privilege/role 端点 404 | 该 Nexus 小版本 API 路径不同 | 对照 Nexus UI → Settings → System → API（Swagger）核对。 |
| TLS 报错 | 自签证书 | 设置 `insecureSkipTLSVerify: true` 或导入 CA。 |
| 超时 | 网络慢 / 仓库列表大 | 调大 `nexus.timeoutSeconds`。 |

> **API 字段准确性：** 本 CLI 使用的 REST 请求/响应字段名遵循 Nexus 3.76 标准 `/service/rest/v1` 端点，但不同小版本可能存在差异；接入生产环境前请对照目标实例的 Swagger 核对。

## 测试

```sh
make test    # 单元测试（naming、planner、config）—— 无需网络
make vet     # go vet
```
