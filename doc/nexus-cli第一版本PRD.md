角色声明

我将以 Nexus Repository 3 权限治理与 DevOps CLI 产品设计顾问 的身份回答，下面输出一版可直接交给 CodeX 开发的 nexus-cli 第一版本 PRD。

⸻

nexus-cli 第一版本 PRD

> 后续需求变更（当前实现以此为准）：
> - “受保护仓库”现在表示 guest / anonymous 完全不可见、不可通过精确 URL 下载，应配置到 `guestAccess.deny.repositories`。
> - `guestAccess.readOnly.repositories` 仅保留为高级场景：UI 不可见但精确 URL 可下载，不再代表“受保护仓库”。
> - 推荐命令为 `nexus-cli guest protect`；`nexus-cli guest sync` 仅作为兼容别名保留。
> - `user create-readonly` 仅支持 raw 仓库，并在创建用户前检查其他非 admin 用户是否已有同仓库或重叠目录访问。

1. 文档信息

项目	内容
产品名称	nexus-cli
版本	V1.0
目标 Nexus 版本	Nexus Repository 3.76
主要管理对象	Anonymous / Guest 访客权限
主要仓库	devops-prod-generic
第一版本定位	解决访客通过 Nexus UI 看到过多仓库和制品的问题
推荐开发语言	Go
推荐 CLI 框架	Cobra + Viper
主要使用人	DevOps 平台管理员、制品库管理员、运维人员

⸻

2. 一句话结论

nexus-cli 第一版本不做复杂的系统账号管理，优先解决访客权限治理问题：

让访客在 Nexus UI 中看不到 devops-prod-generic，
但仍然可以通过精确 URL 下载 devops-prod-generic 中的制品；
同时其他允许公开浏览的仓库仍然可以在 Nexus UI 中正常看到。

第一版本核心命令建议只做：

nexus-cli guest sync
nexus-cli guest check
nexus-cli repo list
nexus-cli config init

⸻

3. 背景说明

3.1 当前现状

当前 Nexus 中存在多个仓库，至少包括：

devops-prod-generic

其中 devops-prod-generic 是生产制品 Raw 仓库，用于存放各系统投产制品，例如：

/repository/devops-prod-generic/ades_xxxxxxx_xxxx/20260702/app.tar
/repository/devops-prod-generic/devops_xxxxx_xxxxx/20260702/app.tar

当前问题是：

访客或匿名用户可以通过 Nexus UI 页面看到所有仓库、所有目录、所有制品内容。

这会导致生产制品暴露范围过大。

⸻

3.2 目标状态

期望达到：

仓库	Nexus UI 是否可见	curl 精确 URL 是否可下载
devops-prod-generic	不可见	可以
其他允许展示仓库	可见	可以
明确禁止仓库	不可见	不可下载

⸻

3.3 关键技术约束

Nexus 权限模型是 累加授权模型，不是拒绝模型。

也就是说，Nexus 不支持：

nx-repository-view-*-*-browse
但排除 devops-prod-generic

因此不能通过一个通配权限完成：

所有仓库可浏览，只有 devops-prod-generic 不可浏览

正确做法是：

移除全局 browse 权限；
读取 Nexus 全部仓库；
对允许展示的仓库逐个创建 browse + read 权限；
对 devops-prod-generic 只创建 read 权限；
把这些权限统一绑定到访客角色。

⸻

4. 产品目标

4.1 第一版本目标

第一版本聚焦 访客权限同步，目标如下：

编号	目标
G01	自动读取 Nexus 全部仓库列表
G02	自动为访客角色生成仓库级权限
G03	对 devops-prod-generic 只授予 read，不授予 browse
G04	对其他允许仓库授予 browse + read
G05	自动移除访客角色中的高危全局权限
G06	支持 dry-run，先预览变更计划
G07	支持权限检查，判断当前访客权限是否符合配置
G08	支持审计日志
G09	支持配置文件驱动
G10	支持幂等执行，重复执行不会重复创建权限

⸻

4.2 第一版本非目标

第一版本暂不做以下能力：

非目标	原因
不创建每个系统的 Nexus 用户	这是第二阶段目录级隔离能力
不做每个系统目录级 Content Selector 授权	第一版本只解决访客 UI 暴露问题
不做用户密码修改	第一版本不管理系统账号
不做批量创建用户	放到 V2
不做制品上传下载封装	Nexus 本身已有下载能力
不做 Nexus 仓库创建	当前问题不是仓库创建
不做 LDAP / AD 集成	不是当前优先级

⸻

5. 用户角色

用户角色	诉求
DevOps 平台管理员	快速收敛访客权限，避免生产制品在 UI 中暴露
Nexus 管理员	不想手工逐仓库创建 Privilege
运维人员	希望执行前能 dry-run，确认无误后再修改
审计人员	希望知道访客角色具备哪些权限，是否有高危权限

⸻

6. 核心痛点

6.1 手工配置成本高

如果 Nexus 有几十个仓库，要实现：

除了 devops-prod-generic 外，其他仓库都 browse + read

就必须逐仓库创建权限。

手工方式问题：

问题	影响
配置量大	效率低
容易漏配	某些仓库突然不可见
容易误配	生产仓库可能被授予 browse
不可重复	换环境需要重新手工配置
不易审计	不知道当前配置是否符合预期

⸻

6.2 Nexus 不支持排除式通配权限

不能配置：

nx-repository-view-*-*-browse except devops-prod-generic

只能用白名单方式逐仓库授权。

⸻

6.3 访客权限风险不可控

当前访客角色可能存在：

nx-repository-view-*-*-browse
nx-repository-view-*-*-*
nx-all
nx-admin
nx-search-read

这些权限会导致 UI 暴露、搜索暴露或权限过大。

⸻

7. 第一版本功能清单

7.1 功能总览

编号	功能	优先级	命令
F01	初始化配置文件	P0	nexus-cli config init
F02	查看仓库列表	P0	nexus-cli repo list
F03	访客权限 dry-run	P0	nexus-cli guest sync --dry-run
F04	访客权限同步	P0	nexus-cli guest sync
F05	访客权限检查	P0	nexus-cli guest check
F06	高危权限扫描	P0	集成在 guest check
F07	审计日志	P0	所有写操作自动记录
F08	执行报告输出	P0	--report
F09	幂等执行	P0	guest sync 内置
F10	连接健康检查	P1	nexus-cli health check

第一版本建议 P0 必须完成，P1 可根据开发时间选择实现。

⸻

8. 命令设计

8.1 命令结构

nexus-cli
├── config
│   └── init
├── repo
│   └── list
├── guest
│   ├── sync
│   └── check
└── health
    └── check

⸻

8.2 初始化配置文件

命令

nexus-cli config init --output config.yaml

功能说明

生成默认配置文件模板。

验收标准

执行后生成：

config.yaml

文件中包含：

Nexus 地址
管理员账号
仓库访问策略
访客角色
read-only 仓库
browse-read 仓库规则
高危权限规则
审计日志配置

⸻

8.3 查看仓库列表

命令

nexus-cli repo list --config config.yaml

输出示例

Repository List
Name                  Format     Type
devops-prod-generic   raw        hosted
maven-public          maven2     group
npm-public            npm        group
raw-public            raw        hosted
Total: 4

功能说明

调用 Nexus API 获取当前所有仓库。

用途

用于确认：

nexus-cli 是否能连接 Nexus
仓库名是否正确
仓库 format 是否能正确识别
配置规则是否可执行

⸻

8.4 访客权限 dry-run

命令

nexus-cli guest sync --config config.yaml --dry-run

功能说明

只生成执行计划，不实际修改 Nexus。

输出示例

Guest Access Sync Plan
Target Role:
  role_guest_repository_access
Read Only Repositories:
  - devops-prod-generic
Browse + Read Repositories:
  - maven-public
  - npm-public
  - raw-public
Deny Repositories:
  - security-prod-generic
Risky Privileges To Remove:
  - nx-repository-view-*-*-browse
  - nx-repository-view-*-*-*
  - nx-all
  - nx-admin
Privileges To Create:
  - priv_guest_raw_devops_prod_generic_read
  - priv_guest_maven2_maven_public_browse_read
  - priv_guest_npm_npm_public_browse_read
  - priv_guest_raw_raw_public_browse_read
No changes applied because dry-run is enabled.

⸻

8.5 访客权限同步

命令

nexus-cli guest sync --config config.yaml

功能说明

按照配置文件同步访客角色权限。

核心动作：

1. 获取 Nexus 仓库列表。
2. 根据规则计算每个仓库的目标权限。
3. 创建缺失 Privilege。
4. 创建或更新访客 Role。
5. 从访客 Role 中移除高危权限。
6. 将目标 Privileges 绑定到访客 Role。
7. 输出同步结果。
8. 写入审计日志。

执行结果示例

Guest Access Sync Completed
Target Role:
  role_guest_repository_access
Created Privileges:
  - priv_guest_raw_devops_prod_generic_read
  - priv_guest_maven2_maven_public_browse_read
  - priv_guest_npm_npm_public_browse_read
Updated Role:
  role_guest_repository_access
Removed Risky Privileges:
  - nx-repository-view-*-*-browse
Summary:
  repositories total: 4
  browse+read: 2
  read-only: 1
  denied: 1
  created privileges: 3
  skipped privileges: 1
  removed risky privileges: 1

⸻

8.6 访客权限检查

命令

nexus-cli guest check --config config.yaml

功能说明

只检查当前 Nexus 访客权限是否符合配置，不做修改。

检查项

检查项	结果
访客角色是否存在	PASS / FAIL
devops-prod-generic 是否只有 read	PASS / FAIL
devops-prod-generic 是否存在 browse	PASS / FAIL
其他允许仓库是否存在 browse + read	PASS / WARN / FAIL
是否存在 nx-repository-view-*-*-browse	PASS / FAIL
是否存在 nx-repository-view-*-*-*	PASS / FAIL
是否存在 nx-all	PASS / FAIL
是否存在 nx-admin	PASS / FAIL
是否存在 nx-search-read	PASS / WARN

输出示例

Guest Access Check Result
Role:
  role_guest_repository_access
PASS:
  devops-prod-generic has read permission
  devops-prod-generic has no browse permission
  no nx-admin
  no nx-all
WARN:
  nx-search-read exists, UI search may expose artifacts
FAIL:
  nx-repository-view-*-*-browse exists
Suggestion:
  Run:
  nexus-cli guest sync --config config.yaml --dry-run

⸻

8.7 健康检查

命令

nexus-cli health check --config config.yaml

功能说明

检查 Nexus 连接和 API 可用性。

检查项

检查项	说明
Nexus 地址可访问	HTTP 连通性
管理员账号可用	Basic Auth 验证
API 可访问	/service/rest/v1
仓库列表可读取	repositories API
角色可读取	security roles API
权限可读取	security privileges API

⸻

9. 配置文件设计

9.1 config.yaml 示例

nexus:
  baseUrl: "http://nexus.example.com"
  username: "admin"
  passwordEnv: "NEXUS_ADMIN_PASSWORD"
  timeoutSeconds: 30
  insecureSkipTLSVerify: false
guestAccess:
  enabled: true
  roleName: "role_guest_repository_access"
  anonymousUserId: "anonymous"
  defaultPolicy: "browseRead"
  # 可选值：
  # browseRead：默认其他仓库 browse + read
  # none：默认不给权限，只按 includeRepositories 授权
  browseRead:
    includeRepositories:
      - "*"
    excludeRepositories:
      - "devops-prod-generic"
  readOnly:
    repositories:
      - "devops-prod-generic"
  deny:
    repositories: []
  actions:
    browseRead:
      - browse
      - read
    readOnly:
      - read
  forbiddenPrivileges:
    - "nx-repository-view-*-*-browse"
    - "nx-repository-view-*-*-*"
    - "nx-all"
    - "nx-admin"
  warnPrivileges:
    - "nx-search-read"
privilegeNaming:
  prefix: "priv_guest"
  separator: "_"
  replaceDashWithUnderscore: true
audit:
  enabled: true
  logPath: "./logs/nexus-cli-audit.log"
  maskSensitive: true
report:
  enabled: true
  outputDir: "./reports"
  format: "text"

⸻

9.2 配置字段说明

Nexus 配置

字段	必填	默认值	说明
nexus.baseUrl	是	无	Nexus 地址
nexus.username	是	无	管理员用户名
nexus.passwordEnv	是	NEXUS_ADMIN_PASSWORD	管理员密码环境变量
nexus.timeoutSeconds	否	30	HTTP 超时时间
nexus.insecureSkipTLSVerify	否	false	是否跳过 TLS 校验

⸻

访客权限配置

字段	必填	默认值	说明
guestAccess.enabled	是	true	是否启用访客权限同步
guestAccess.roleName	是	无	访客角色名
guestAccess.anonymousUserId	否	anonymous	匿名用户 ID
guestAccess.defaultPolicy	否	browseRead	默认仓库策略
guestAccess.browseRead.includeRepositories	是	["*"]	允许 UI 浏览的仓库
guestAccess.browseRead.excludeRepositories	否	[]	从 UI 浏览中排除的仓库
guestAccess.readOnly.repositories	否	[]	只允许下载、不允许 UI 浏览的仓库
guestAccess.deny.repositories	否	[]	完全禁止访客访问的仓库
guestAccess.forbiddenPrivileges	是	内置默认值	高危权限
guestAccess.warnPrivileges	否	["nx-search-read"]	警告权限

⸻

10. 权限计算规则

10.1 仓库权限分类

CLI 读取所有仓库后，对每个仓库计算目标权限。

优先级从高到低：

deny > readOnly > browseRead > defaultPolicy

⸻

10.2 权限分类表

仓库命中规则	权限结果
命中 deny.repositories	不授予任何权限
命中 readOnly.repositories	read
命中 browseRead.includeRepositories 且未命中 exclude	browse + read
defaultPolicy = browseRead	browse + read
defaultPolicy = none	不授予权限

⸻

10.3 示例

配置：

browseRead:
  includeRepositories:
    - "*"
  excludeRepositories:
    - "devops-prod-generic"
readOnly:
  repositories:
    - "devops-prod-generic"
deny:
  repositories:
    - "security-prod-generic"

仓库列表：

devops-prod-generic
maven-public
npm-public
security-prod-generic

计算结果：

仓库	命中规则	目标权限
devops-prod-generic	readOnly	read
maven-public	browseRead	browse + read
npm-public	browseRead	browse + read
security-prod-generic	deny	无权限

⸻

11. Privilege 生成规则

11.1 Repository View Privilege

第一版本使用：

Repository View

不使用：

Repository Content Selector

原因：

第一版本治理目标是 仓库级 UI 可见性，不是系统目录级隔离。

⸻

11.2 普通仓库 browse + read 权限

仓库：

maven-public

Format：

maven2

Privilege 名称：

priv_guest_maven2_maven_public_browse_read

Actions：

browse
read

⸻

11.3 生产仓库 read-only 权限

仓库：

devops-prod-generic

Format：

raw

Privilege 名称：

priv_guest_raw_devops_prod_generic_read

Actions：

read

禁止包含：

browse

⸻

11.4 名称转换规则

由于 Nexus 权限名中建议避免特殊字符，CLI 生成 Privilege 名称时应做转换：

原字符	转换后
-	_
.	_
/	_
空格	不允许

示例：

devops-prod-generic

转换为：

devops_prod_generic

最终权限名：

priv_guest_raw_devops_prod_generic_read

⸻

12. Role 处理规则

12.1 目标 Role

配置字段：

guestAccess:
  roleName: "role_guest_repository_access"

如果 Role 不存在：

自动创建

如果 Role 已存在：

自动更新

⸻

12.2 Role 权限同步模式

第一版本采用 托管权限同步模式。

也就是说，CLI 只管理自己创建的权限，避免误删人工配置的其他权限。

托管权限识别规则

默认通过名称前缀识别：

priv_guest_

例如：

priv_guest_raw_devops_prod_generic_read
priv_guest_maven2_maven_public_browse_read

⸻

12.3 Role 更新逻辑

执行 guest sync 时：

1. 读取 role_guest_repository_access 当前 privileges。
2. 识别由 nexus-cli 托管的 privileges。
3. 移除不符合当前配置的托管 privileges。
4. 保留非托管 privileges，除非它命中 forbiddenPrivileges。
5. 添加当前目标 privileges。
6. 移除 forbiddenPrivileges。

⸻

13. 高危权限处理

13.1 默认高危权限

第一版本必须识别并移除：

nx-repository-view-*-*-browse
nx-repository-view-*-*-*
nx-all
nx-admin

⸻

13.2 警告权限

第一版本默认警告但不强制移除：

nx-search-read

原因：

nx-search-read 可能导致 Search 页面可用，存在制品搜索暴露风险。但有些环境可能依赖 Search 能力，因此第一版本先做 WARN。

可通过配置调整为 forbidden：

guestAccess:
  forbiddenPrivileges:
    - "nx-search-read"

⸻

13.3 高危权限处理规则

权限	sync 行为	check 行为
nx-repository-view-*-*-browse	移除	FAIL
nx-repository-view-*-*-*	移除	FAIL
nx-all	移除	FAIL
nx-admin	移除	FAIL
nx-search-read	默认不移除	WARN

⸻

14. 幂等性要求

guest sync 必须幂等。

14.1 重复执行行为

对象	已存在且一致	已存在但不一致
Privilege	跳过	更新或重建
Role	跳过	更新 privileges
高危权限	已不存在则跳过	存在则移除
仓库列表	每次实时读取	按最新仓库重算

⸻

14.2 示例

第一次执行：

Created:
  priv_guest_raw_devops_prod_generic_read
  priv_guest_maven2_maven_public_browse_read
Updated:
  role_guest_repository_access

第二次执行：

Skipped:
  priv_guest_raw_devops_prod_generic_read already exists
  priv_guest_maven2_maven_public_browse_read already exists
No changes required.

⸻

15. dry-run 要求

所有写操作必须支持：

--dry-run

dry-run 模式下：

行为	要求
读取 Nexus	执行
计算计划	执行
创建 Privilege	不执行
更新 Role	不执行
删除 / 移除权限	不执行
输出报告	执行
写审计日志	可写，但标记 dryRun=true

⸻

16. 审计日志设计

16.1 审计日志格式

建议使用 JSON Lines。

文件：

./logs/nexus-cli-audit.log

每行一条记录。

⸻

16.2 字段

字段	说明
timestamp	执行时间
operator	当前操作系统用户
command	执行命令
nexusBaseUrl	Nexus 地址
targetRole	目标角色
dryRun	是否 dry-run
action	sync / check / repo-list
result	success / failed
createdPrivileges	创建的权限
updatedRoles	更新的角色
removedPrivileges	移除的权限
errorMessage	错误信息

⸻

16.3 示例

{"timestamp":"2026-07-02T10:30:00+08:00","operator":"moge","command":"guest sync","nexusBaseUrl":"http://nexus.example.com","targetRole":"role_guest_repository_access","dryRun":false,"action":"sync","result":"success","createdPrivileges":["priv_guest_raw_devops_prod_generic_read"],"removedPrivileges":["nx-repository-view-*-*-browse"]}

⸻

17. 执行报告设计

17.1 默认控制台输出

控制台输出必须包含：

目标角色
仓库总数
browse + read 仓库数
read-only 仓库数
deny 仓库数
创建的权限
跳过的权限
移除的高危权限
警告项
失败原因

⸻

17.2 文件报告

支持参数：

--report ./reports/guest-sync-report.txt

第一版本支持：

text
json

CSV 可以放到后续版本。

⸻

18. 错误处理

18.1 常见错误

错误	原因	处理
401	Nexus 管理员账号密码错误	提示检查 NEXUS_ADMIN_PASSWORD
403	管理员权限不足	提示需要安全管理权限
404	API 路径错误	提示检查 Nexus 版本
409	Privilege 已存在	进入幂等处理
400	参数错误	输出 Nexus 返回内容
TLS error	自签证书问题	提示配置 CA 或 insecureSkipTLSVerify
timeout	网络或服务慢	提示调整 timeout
role update failed	角色更新失败	输出失败对象和原因

⸻

18.2 失败策略

第一版本默认：

遇到单个 Privilege 创建失败，终止执行。

原因是权限治理属于高风险操作，第一版不建议“失败继续”。

后续可增加：

--continue-on-error

⸻

19. 安全要求

19.1 密码处理

要求	说明
管理员密码从环境变量读取	不建议写入配置文件
日志中不能打印密码	包括 Basic Auth
错误输出不能包含 Authorization Header	必须脱敏
配置文件模板中不放真实密码	只放 passwordEnv

示例：

export NEXUS_ADMIN_PASSWORD='your_password'

⸻

19.2 权限安全

第一版本必须防止以下误操作：

风险	防护
给 devops-prod-generic 加 browse	配置中 readOnly 优先级高于 browseRead
保留全局 browse	默认 forbidden 并在 sync 中移除
给访客加 admin 权限	默认 forbidden 并在 sync 中移除
误删非托管权限	默认只移除 forbidden 和托管权限
误操作生产	支持 dry-run

⸻

20. API 交互设计

20.1 API 基础地址

{baseUrl}/service/rest/v1

⸻

20.2 第一版本需要的 API 能力

能力	说明
获取仓库列表	获取 repository name、format、type
查询 privileges	判断权限是否存在
创建 privilege	创建 Repository View Privilege
更新 privilege	修正 actions
查询 role	获取当前 role
创建 role	创建访客角色
更新 role	同步 privileges
查询 anonymous 用户或角色	可选，用于检查 anonymous 绑定情况

⸻

20.3 注意事项

开发时必须以目标 Nexus 3.76 环境中的 Swagger 为准：

Nexus UI → Settings → System → API

因为不同小版本 API 字段可能有差异。

⸻

21. 数据模型设计

21.1 Repository

type Repository struct {
    Name   string
    Format string
    Type   string
}

⸻

21.2 TargetPermission

type TargetPermission struct {
    Repository string
    Format     string
    Actions    []string
    Policy     string // browseRead/readOnly/deny
}

⸻

21.3 SyncPlan

type SyncPlan struct {
    TargetRole              string
    RepositoriesTotal       int
    BrowseReadRepositories  []string
    ReadOnlyRepositories    []string
    DenyRepositories        []string
    PrivilegesToCreate      []string
    PrivilegesToUpdate      []string
    PrivilegesToSkip        []string
    PrivilegesToRemove      []string
    Warnings                []string
}

⸻

22. 推荐项目结构

nexus-cli/
├── cmd/
│   └── nexus-cli/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── config.go
│   │   ├── repo.go
│   │   ├── guest.go
│   │   └── health.go
│   ├── config/
│   │   └── config.go
│   ├── nexus/
│   │   ├── client.go
│   │   ├── repositories.go
│   │   ├── privileges.go
│   │   └── roles.go
│   ├── guest/
│   │   ├── planner.go
│   │   ├── syncer.go
│   │   └── checker.go
│   ├── naming/
│   │   └── naming.go
│   ├── audit/
│   │   └── audit.go
│   ├── report/
│   │   └── report.go
│   └── log/
│       └── logger.go
├── examples/
│   └── config.example.yaml
├── docs/
│   ├── PRD.md
│   ├── USER_GUIDE.md
│   └── TROUBLESHOOTING.md
├── tests/
├── Makefile
├── go.mod
└── README.md

⸻

23. 第一版本验收标准

23.1 功能验收

编号	验收项	预期
A01	能生成配置文件	config.yaml 创建成功
A02	能连接 Nexus	health check 成功
A03	能列出仓库	显示仓库名、format、type
A04	dry-run 不修改 Nexus	只输出计划
A05	guest sync 能创建目标权限	Privilege 创建成功
A06	guest sync 能创建或更新访客 Role	Role 更新成功
A07	devops-prod-generic 没有 browse	UI 不可见
A08	devops-prod-generic 有 read	curl 精确 URL 可下载
A09	其他仓库有 browse + read	UI 可见，可下载
A10	高危权限被移除	全局 browse 不存在
A11	重复执行幂等	第二次执行无重复创建
A12	审计日志生成	日志内容完整且脱敏

⸻

23.2 UI 验收

访客访问 Nexus UI：

仓库	预期
devops-prod-generic	看不到或无法浏览
maven-public	可以看到
npm-public	可以看到
其他允许仓库	可以看到
禁止仓库	看不到

⸻

23.3 下载验收

对 devops-prod-generic 执行：

curl -I \
'http://nexus.example.com/repository/devops-prod-generic/ades_xxxxxxx_xxxx/20260702/app.tar'

预期：

HTTP/1.1 200 OK

对不存在或 deny 仓库执行：

curl -I \
'http://nexus.example.com/repository/security-prod-generic/test/app.tar'

预期：

403 Forbidden

或：

404 Not Found

核心判断：

不能成功下载

⸻

24. 第一版本开发边界

24.1 必须完成

config init
repo list
guest sync
guest sync --dry-run
guest check
审计日志
幂等处理
高危权限移除

⸻

24.2 可以延期

health check
json 报告
csv 报告
anonymous 用户自动绑定 role
多环境 profile

不过我建议 health check 尽量也做，排障价值很高。

⸻

25. 后续版本规划

V1.0

访客权限同步
隐藏 devops-prod-generic UI
保留 devops-prod-generic read 下载
其他仓库按规则 browse + read

V1.1

支持多环境配置
支持 JSON / CSV 报告
支持自动绑定 anonymous 用户角色
支持权限回滚计划

V2.0

系统账号创建
系统目录级 Content Selector
批量创建用户
修改密码
禁用 / 启用用户
删除用户
系统目录级权限验证

V3.0

LDAP / AD 组授权
权限导出审计
孤儿权限清理
DevOps 平台集成 API

⸻

26. 开发提示词

下面这段可以直接发给 CodeX。

请基于以下 PRD 开发一个 Go 语言 CLI 工具，名称为 nexus-cli。
一、工具定位
nexus-cli 用于治理 Nexus Repository 3.76 的访客 / anonymous 权限。
第一版本只做 Guest Access Sync，不做用户创建、不做密码管理、不做系统目录级权限。
当前核心痛点：
Nexus 不支持 nx-repository-view-*-*-browse 但排除某个仓库的配置。现在需要自动读取所有仓库，为除 devops-prod-generic 外的仓库创建 browse + read 权限；对 devops-prod-generic 只创建 read 权限，不创建 browse 权限。
目标效果：
1. 访客在 Nexus UI 中看不到 devops-prod-generic。
2. 访客可以通过精确 URL curl 下载 devops-prod-generic 中的制品。
3. 访客可以在 Nexus UI 中看到其他允许仓库。
4. 访客角色中不能存在 nx-repository-view-*-*-browse、nx-repository-view-*-*-*、nx-all、nx-admin 等高危权限。
5. 支持 dry-run，执行前先展示计划。
6. 支持幂等执行，重复执行不会重复创建权限。
7. 支持 guest check 检查当前权限是否符合配置。
8. 支持审计日志，日志中不得输出管理员密码或 Authorization Header。
二、技术要求
1. 使用 Go 语言开发。
2. 使用 Cobra 实现 CLI 命令。
3. 使用 Viper 解析 YAML 配置文件。
4. 使用 net/http 或 resty 调用 Nexus REST API。
5. 支持 Basic Auth。
6. Nexus 管理员密码从环境变量读取，例如 NEXUS_ADMIN_PASSWORD。
7. HTTP Client 支持 timeout、TLS 配置、错误解析。
8. API endpoint 以目标 Nexus 3.76 的 Swagger 为准，Swagger 在 Nexus UI 的 Settings → System → API 中查看。
9. 代码结构要清晰，至少拆分 cli、config、nexus、guest、naming、audit、report 模块。
10. 提供 README.md、config.example.yaml、使用说明和测试说明。
三、第一版本命令
需要实现以下命令：
1. 初始化配置：
nexus-cli config init --output config.yaml
2. 查看仓库：
nexus-cli repo list --config config.yaml
3. 访客权限同步 dry-run：
nexus-cli guest sync --config config.yaml --dry-run
4. 访客权限同步：
nexus-cli guest sync --config config.yaml
5. 访客权限检查：
nexus-cli guest check --config config.yaml
可选：
nexus-cli health check --config config.yaml
四、配置文件示例
nexus:
  baseUrl: "http://nexus.example.com"
  username: "admin"
  passwordEnv: "NEXUS_ADMIN_PASSWORD"
  timeoutSeconds: 30
  insecureSkipTLSVerify: false
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
    repositories:
      - "devops-prod-generic"
  deny:
    repositories: []
  actions:
    browseRead:
      - browse
      - read
    readOnly:
      - read
  forbiddenPrivileges:
    - "nx-repository-view-*-*-browse"
    - "nx-repository-view-*-*-*"
    - "nx-all"
    - "nx-admin"
  warnPrivileges:
    - "nx-search-read"
privilegeNaming:
  prefix: "priv_guest"
  separator: "_"
  replaceDashWithUnderscore: true
audit:
  enabled: true
  logPath: "./logs/nexus-cli-audit.log"
  maskSensitive: true
report:
  enabled: true
  outputDir: "./reports"
  format: "text"
五、权限计算规则
对每个仓库按优先级计算：
deny > readOnly > browseRead > defaultPolicy
如果仓库在 deny.repositories 中：
不授予任何权限。
如果仓库在 readOnly.repositories 中：
只授予 read。
如果仓库匹配 browseRead.includeRepositories，并且不在 browseRead.excludeRepositories 中：
授予 browse + read。
如果 defaultPolicy = browseRead：
默认授予 browse + read。
如果 defaultPolicy = none：
默认不授予权限。
六、权限生成规则
第一版本使用 Repository View Privilege，不使用 Content Selector。
普通仓库：
Privilege 名称：priv_guest_{format}_{repository}_browse_read
Actions：browse, read
devops-prod-generic：
Privilege 名称：priv_guest_raw_devops_prod_generic_read
Actions：read
名称中的 -、.、/ 需要转换为 _。
七、Role 同步规则
目标角色由 guestAccess.roleName 指定。
如果角色不存在，自动创建。
如果角色存在，自动更新。
CLI 只管理自己创建的权限，默认根据 priv_guest_ 前缀识别托管权限。
执行 guest sync 时：
1. 读取 role 当前 privileges。
2. 移除 forbiddenPrivileges。
3. 移除不符合当前配置的托管 privileges。
4. 创建缺失 privileges。
5. 把目标 privileges 添加到 role。
6. 保留非托管、非 forbidden 的权限。
7. 输出同步报告。
八、guest check 检查规则
需要检查：
1. 目标 role 是否存在。
2. devops-prod-generic 是否只有 read，没有 browse。
3. 其他允许仓库是否存在 browse + read。
4. 是否存在 nx-repository-view-*-*-browse。
5. 是否存在 nx-repository-view-*-*-*。
6. 是否存在 nx-all。
7. 是否存在 nx-admin。
8. 是否存在 nx-search-read，如果存在默认 WARN。
九、验收标准
1. 执行 config init 可以生成配置文件。
2. 执行 repo list 可以列出 Nexus 仓库。
3. 执行 guest sync --dry-run 不修改 Nexus，只输出计划。
4. 执行 guest sync 可以创建需要的 privileges。
5. 执行 guest sync 可以创建或更新 role_guest_repository_access。
6. 执行后访客 UI 看不到 devops-prod-generic。
7. 执行后访客可以通过 curl 精确 URL 下载 devops-prod-generic 制品。
8. 执行后访客可以在 UI 中看到其他允许仓库。
9. 访客角色中不存在 nx-repository-view-*-*-browse、nx-repository-view-*-*-*、nx-all、nx-admin。
10. 重复执行 guest sync 不会重复创建权限。
11. 审计日志生成成功，并且不包含密码和 Authorization Header。

⸻
