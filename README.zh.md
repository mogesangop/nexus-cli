# nexus-cli

[English](README.md) | **中文**

[![CI](https://github.com/231397220/nexus-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/231397220/nexus-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

一个用于治理 **Nexus Repository 3.76** 访客 / 匿名访问的命令行工具。

第一版本解决一个问题：访客（匿名用户）在 Nexus UI 中能看到过多仓库与制品。Nexus 不支持「给所有仓库授予 browse，但排除某一个」的权限模型，因此 `nexus-cli` 会读取仓库列表，为每个仓库构建 `repository-view` 权限并绑定到访客角色 —— 对公开仓库授予 `browse+read`，对受保护仓库不授予任何访客权限，使其不可见且不可通过直链下载。旧的 `readOnly` 策略仍保留，用于“UI 不可见但精确 URL 可下载”的高级场景。

完整产品规格见 `doc/nexus-cli第一版本PRD.md`。

第二个用例是**按用户路径范围分享**：`share grant` 会创建内容选择器、路径范围的 `browse+read` 权限、角色和用户，让指定用户只能浏览/下载某一个 raw 仓库某一个目录下的制品。创建前会检查现有非 admin 用户，若其他用户已经拥有该仓库级或重叠目录访问权限，则拒绝继续，避免产生“看似独占但实际不隔离”的授权。分享类资源使用独立的 `priv_share_` 前缀和 `role_share_*` 角色，与访客子系统互不可见、互不影响。

第三个用例是管理 `raw/hosted` 仓库及 Community/OSS 环境下的制品生命周期。CLI 可以幂等创建或安全更新仓库，并按文件最后修改时间与路径规则预览、删除过期制品。完整规格见 `doc/raw仓库与制品生命周期PRD.md`。

第四个用例是支撑 **Nexus OSS 主从温备 HA 运维**：两台独立 Nexus 节点、F5 单活入口、定时复制 blob / 元数据、人工 fencing 与引导式故障切换。CLI 不自动调用 F5，也不承诺零 RPO；它提供双节点健康/状态、一次性同步命令执行、fencing 门禁和审计记录。完整规格见 `doc/nexus主从HA模式PRD.md`。

AI Agent 调用 `nexus-cli` 时，请先阅读 `doc/AI可调用能力清单.md` 和 `doc/AI调用指南.md`，按“读取状态 → dry-run / preview → 人工确认 → 执行 → 审计检查”的顺序操作。

## 安装

每次发布都会提供预编译二进制，可根据运行环境选择 npm 或 RPM 安装。

### npm（跨平台）

```sh
# 全局安装。
npm i -g @mogesang/nexus-cli
nexus-cli --help

# 或者不全局安装，仅运行一次。
npx @mogesang/nexus-cli --help
```

支持 Linux、macOS 和 Windows 的 x64 / arm64 平台。npm 包是一个轻量
包装器：安装时会从 GitHub Releases 下载与当前平台匹配的二进制，并校验
SHA-256。

### 通过 yum / dnf 安装 RPM（RHEL、CentOS、Rocky、Alma、Fedora）

YUM 仓库由本项目的 GitHub Pages 托管，每次发布标签触发后自动重建。
仓库提供 x86_64 和 aarch64 软件包，所有 RPM 均带有 GPG 签名。

#### dnf（Fedora、RHEL 8+、Rocky、Alma）

```sh
# 添加软件源配置并导入 RPM 签名公钥。
sudo curl -o /etc/yum.repos.d/nexus-cli.repo \
  https://mogesangop.github.io/nexus-cli/nexus-cli.repo
sudo rpm --import https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli

# 安装并验证。
sudo dnf install nexus-cli
nexus-cli --help
rpm -q nexus-cli
```

#### yum（RHEL 7、CentOS 7）

```sh
# 添加软件源配置并导入 RPM 签名公钥。
sudo curl -o /etc/yum.repos.d/nexus-cli.repo \
  https://mogesangop.github.io/nexus-cli/nexus-cli.repo
sudo rpm --import https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli

# 安装并验证。
sudo yum install nexus-cli
nexus-cli --help
rpm -q nexus-cli
```

### 直接下载

从 [最新版本](https://github.com/mogesangop/nexus-cli/releases/latest)
下载对应平台的压缩包，解压后将 `nexus-cli` 放入 `PATH`。

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
# 1. 生成配置模板。不传 --output 时默认写到 ~/.nexus-cli/config.yaml
#    （目录不存在则按 0700 权限创建）。
./nexus-cli config init

# 2. 编辑配置：设置 baseUrl、roleName，以及 deny / browseRead
#    仓库列表。受保护仓库放入 deny.repositories。然后导出管理员密码：
export NEXUS_ADMIN_PASSWORD='your_password'

# 3. 验证连通性。--config 可省略；未指定时按以下顺序搜索首个存在的文件：
#    ./config.yaml → ~/.nexus-cli/config.yaml → /etc/nexus-cli/config.yaml
./nexus-cli health check

# 4. 预览执行计划（不修改 Nexus）。
./nexus-cli guest protect --dry-run

# 5. 执行保护。
./nexus-cli guest protect

# 6. 校验漂移。
./nexus-cli guest check
```

### 为用户授予某个仓库目录的访问权限

```sh
# 先 dry-run：打印将会创建的 selector/privilege/role/user。
./nexus-cli share grant \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice --last-name Team \
  --dry-run

# 正式执行。生成的密码只会打印一次到 stdout，请立即保存。
./nexus-cli share grant \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice --last-name Team
```

该授权是幂等的：使用相同参数重复执行会复用已存在的 selector、privilege 和 role。若用户已存在则**报错**——绝不重置已有用户密码。`share grant` 仅支持 raw 仓库，并会在创建任何资源前检查是否存在其他非 admin 用户的冲突访问。失败时不回滚已完成的步骤，因此可安全重试。

## 命令

所有命令都接受可选的 `--config <path>`。未指定（或传 `--config ""`）时，CLI 按以下顺序搜索首个存在的文件：
`./config.yaml` → `~/.nexus-cli/config.yaml` → `/etc/nexus-cli/config.yaml`。
显式传入 `--config` 时直接使用该路径（不搜索；路径写错会报读取错误）。

| 命令 | 说明 |
| --- | --- |
| `config init [--output config.yaml]` | 生成配置模板（默认：`~/.nexus-cli/config.yaml`）。 |
| `repo list [--format F] [--type T]` | 列出仓库，可按 format/type 筛选。 |
| `repo get --name R --format F --type T` | 查看单个仓库的完整 API 配置。 |
| `repo apply [--dry-run] [--yes]` | 应用 `repositories.managed` 中声明的通用仓库；真实变更必须带 `--yes`。 |
| `repo ensure --name R --format F --type T --settings FILE [--dry-run] [--yes]` | 从 YAML/JSON settings 创建或更新单个通用仓库；真实变更必须带 `--yes`。 |
| `repo raw apply [--dry-run] [--yes]` | 应用配置中声明的 raw hosted 仓库；真实变更必须带 `--yes`。 |
| `repo raw ensure --name R --blob-store B [...] [--dry-run] [--yes]` | 创建或安全更新单个 raw hosted 仓库；真实变更必须带 `--yes`。 |
| `repo lifecycle preview --repo R [...]` | 只读预览过期 raw 制品。 |
| `repo lifecycle run --repo R --yes [...]` | 删除过期 raw 制品。 |
| `blobstore list` | 列出 Blob Store。 |
| `blobstore get --name B --type file` | 查看单个 file Blob Store。 |
| `blobstore apply [--dry-run] [--yes]` | 应用 `blobStores.file` 中声明的 file Blob Store；真实变更必须带 `--yes`。 |
| `blobstore ensure --name B --path P [...] [--dry-run] [--yes]` | 创建或更新单个 file Blob Store；真实变更必须带 `--yes`。 |
| `guest protect [--dry-run] [--yes] [--report FILE]` | 按配置保护访客访问权限；真实变更必须带 `--yes`。 |
| `guest sync [--dry-run] [--yes] [--report FILE]` | `guest protect` 的兼容别名，已不推荐；真实变更必须带 `--yes`。 |
| `guest check` | 只读校验访客角色是否符合配置。 |
| `share grant --repo R --path /p/ --user U --email E [--dry-run] [--yes]` | 为指定用户创建路径范围的 browse+read 授权；真实变更必须带 `--yes`。 |
| `health check` | 连接 / API / 认证健康检查。 |
| `ha status` | 查看双节点健康、blob / 元数据最后同步时间与延迟。 |
| `ha health` | 对两个 HA 节点分别执行 API 健康检查。 |
| `ha sync --once [--timeout 30m]` | 执行配置中的 blob 与元数据同步命令一次，并更新 HA 状态文件。 |
| `ha failover --from primary --to standby --fencing-confirmed` | 引导安全人工切换：fencing 门禁、可选追赶同步、打印 F5 步骤并写审计。 |

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
- `repositories.raw` —— raw hosted 仓库目标状态及 CLI 生命周期规则。
- `repositories.managed` —— 任意 format/type 的通用仓库目标状态。`settings` 会透传到 Nexus 仓库 API 请求体。
- `blobStores.file` —— file 类型 Blob Store 目标状态。
- `ha` —— 可选主从温备配置：双节点、复制命令、状态文件与人工切换安全门禁。
- `guestAccess` —— 目标角色、仓库策略、禁止/警告权限。
- `privilegeNaming` —— 前缀（`priv_guest`）、分隔符、短横线替换。
- `audit` —— JSONL 审计日志路径与脱敏开关。
- `report` —— 报告目录与格式（`text` | `json`）。

## 主从温备 HA 用法

HA 模式遵循 `doc/nexus主从HA模式PRD.md` 的硬约束：Nexus Repository OSS 没有原生 active-active 或主从同步。本 CLI 提供的是运维引导式温备流程，不是同步复制。

### 1. 配置双节点与同步命令

在 `config.yaml` 增加启用的 `ha` 段。密码仍只从环境变量读取。

```yaml
ha:
  enabled: true
  role: "primary"
  nodes:
    - name: "primary"
      role: "primary"
      baseUrl: "http://nexus-a.example.com"
      username: "admin"
      passwordEnv: "NEXUS_PRIMARY_PASSWORD"
    - name: "standby"
      role: "standby"
      baseUrl: "http://nexus-b.example.com"
      username: "admin"
      passwordEnv: "NEXUS_STANDBY_PASSWORD"
  replication:
    stateFile: "./logs/nexus-cli-ha-state.json"
    blobSync:
      method: "rsync"
      schedule: "*/5 * * * *"
      command: "rsync -a --delete nexus-a:/nexus-data/blobs/default/ nexus-b:/nexus-data/blobs/default/"
    metadataSync:
      method: "export-import"
      schedule: "*/15 * * * *"
      command: "/opt/nexus-ha/sync-metadata.sh"
  failover:
    mode: "manual"
    requireFencing: true
```

`blobSync.command` 和 `metadataSync.command` 是运维维护的本地命令或脚本，需可幂等重跑，失败时返回非零状态。元数据脚本通常封装 Nexus Export database → 传输完成包 → 在从节点触发 Import database。

运行前导出两个节点密码：

```sh
export NEXUS_PRIMARY_PASSWORD='primary_admin_password'
export NEXUS_STANDBY_PASSWORD='standby_admin_password'
```

### 2. 巡检双节点

```sh
nexus-cli ha health --config config.yaml
nexus-cli ha status --config config.yaml
```

`ha health` 对两节点分别检查仓库、权限和 guest role API；`ha status` 还会读取 `ha.replication.stateFile`，展示 blob / 元数据最后成功同步时间、延迟和最近错误。

### 3. 执行一次追赶同步

```sh
nexus-cli ha sync --once --config config.yaml --timeout 45m
```

命令先执行 `blobSync.command`，再执行 `metadataSync.command`。任一阶段失败即停止，并把结果写入 HA 状态文件。同步命令为空会快速失败，并提示需要补哪个配置字段。

定时复制可由 cron/systemd timer 调用实际同步脚本，也可在配置好命令后定时调用 `nexus-cli ha sync --once`。

### 4. 人工故障切换

主节点故障时，先停止或隔离旧主，确认不存在双写路径，然后执行：

```sh
nexus-cli ha failover \
  --config config.yaml \
  --from primary \
  --to standby \
  --fencing-confirmed
```

默认会先执行一次最终追赶同步，再打印 F5 切换检查项。如果旧主硬故障导致无法同步，只能在接受 RPO 缺口后使用 `--skip-sync`：

```sh
nexus-cli ha failover \
  --config config.yaml \
  --from primary \
  --to standby \
  --fencing-confirmed \
  --skip-sync
```

在 F5 将 standby 设置为唯一 active pool member 后，建议验证：

```sh
nexus-cli ha status --config config.yaml
nexus-cli guest check --config config.yaml
```

每次 `ha sync --once` 和 `ha failover` 都会通过现有审计日志写入 JSONL 记录；记录中不会包含密码或 Authorization header。

### 仓库策略优先级（每个仓库）

```
deny > readOnly > browseRead > defaultPolicy
```

命中 `deny.repositories` 的仓库不授予任何权限；命中 `readOnly` 的只授予 `read`（UI 不可见，仍可下载）；匹配 `browseRead` 且未被排除的授予 `browse+read`；其余由 `defaultPolicy` 决定。

### 设置受保护仓库：UI 不可见，且不可直链下载

所谓“受保护仓库”就是不向匿名访客授予任何 `browse` 或 `read`。Nexus UI 的仓库列表和目录浏览依赖 `browse`；精确 URL 下载依赖 `read`。因此配置时要做两件事：

1. 从 `browseRead.excludeRepositories` 排除该仓库，避免授予 `browse+read`。
2. 加入 `deny.repositories`，不授予任何访客权限。

例如要保护 `devops-prod-generic`：

```yaml
guestAccess:
  enabled: true
  roleName: "role_guest_repository_access"
  anonymousUserId: "anonymous"
  defaultPolicy: "browseRead"
  browseRead:
    includeRepositories:
      - "*"
    excludeRepositories:
      - "devops-prod-generic"
  readOnly:
    repositories: []
  deny:
    repositories:
      - "devops-prod-generic"
  actions:
    browseRead:
      - browse
      - read
    readOnly:
      - read
```

运行前确认 Nexus 中已存在 `role_guest_repository_access`，并且匿名用户 `anonymous` 已绑定这个角色。然后执行：

```sh
export NEXUS_ADMIN_PASSWORD='your_password'
./nexus-cli guest protect --config config.yaml --dry-run
./nexus-cli guest protect --config config.yaml --yes
./nexus-cli guest check --config config.yaml
```

验证结果：

```sh
# 1. 用匿名 / 未登录浏览器打开 Nexus UI，仓库列表中不应看到 devops-prod-generic。

# 2. 匿名 / 未登录用户访问精确制品 URL 也应失败。
curl -fL \
  'http://nexus.example.com/repository/devops-prod-generic/path/to/artifact.tar'
```

如果直链仍可下载或 UI 仍能看到该仓库，通常是匿名用户还有其它角色或权限包含了该仓库访问能力；`guest protect` 会移除 `forbiddenPrivileges` 中列出的宽泛 browse/admin 权限，但不会删除所有非托管角色。

### 权限命名

`priv_guest_{format}_{sanitize后的仓库名}_{排序后的actions}` —— 例如
`priv_guest_raw_devops_prod_generic_read`。仓库名中的 `-`、`.`、`/` 会被替换为 `_`。

### 托管权限

`nexus-cli` 只管理名称以 `priv_guest_` 开头的权限。角色上**非托管**的权限会被保留 —— **例外**是 `forbiddenPrivileges` 中列出的（如 `nx-all`、`nx-admin`、`nx-repository-view-*-*-browse`），它们在 `protect` 时无论是否托管都会从访客角色移除。`warnPrivileges`（如 `nx-search-read`）在 `guest check` 中告警，但默认不移除。

## 幂等性

`guest protect` 是幂等的（PRD §14）：状态未变时第二次执行不会创建也不会移除任何内容。已存在且符合配置的托管权限会被跳过；陈旧的托管权限会被移除。`guest sync` 保留为兼容别名，但已不推荐。

`repo raw apply` 同样幂等。它不会迁移 blob store，也不会删除重建同名仓库。建议先执行：

```sh
./nexus-cli repo raw apply --dry-run
./nexus-cli repo lifecycle preview --repo devops-prod-generic
./nexus-cli repo lifecycle run --repo devops-prod-generic --yes
```

生命周期可由 cron 定时调用。`run` 会删除 Nexus component，但磁盘空间仍需 Nexus 的 blob store compact 任务回收。

`repo apply` 和 `blobstore apply` 也具备幂等性。通用仓库会将配置中声明的 `settings` 字段与线上 API 返回值比较，允许 Nexus 返回额外只读字段。同名仓库如果 format/type 不一致会失败，不迁移、不删除重建。

## 安全

- 管理员密码从环境变量读取，永不写入配置文件。
- 审计日志不含密码，也不含 `Authorization` 头。
- `--dry-run` 只计算并打印计划，不修改 Nexus。
- 生命周期实际删除必须显式传入 `--yes`；`excludePaths` 始终优先于 `includePaths`。

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
