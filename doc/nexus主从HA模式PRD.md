# Nexus 主从高可用(HA)模式 PRD

## 1. Summary(摘要)

本文档为 Nexus Repository 3.76 设计主从高可用模式:两台独立部署的 Nexus
节点分主、从角色,主节点故障时手动切换流量到从节点。由于 Nexus 3 无原生
主从复制,本设计采用**温备(warm standby)+ 定时复制**模式,并约定
`nexus-cli` 在 v2 增加 `ha` 命令组支撑检测、同步、引导切换。

## 2. Contacts(干系人)

| 角色 | 职责 |
|---|---|
| DevOps 平台管理员 | 主从部署、F5 配置、触发切换 |
| 制品库管理员 | 复制策略调优、RPO 验证 |
| 运维/SRE | 日常健康巡检、故障应急、fencing 执行 |
| nexus-cli 维护者 | `ha` 命令组与配置扩展实现(v2) |

## 3. Background(背景)

### 3.1 为什么现在做

当前 `nexus-cli` 的配置模型(`config.NexusConfig`)与 REST 客户端
(`nexus.Client`)只面向**单个** Nexus 实例:`nexus.baseUrl` 解析为一个
`Client`。生产 Nexus 是制品分发的关键依赖,单节点即单点故障。需要主从结
构降低风险。

### 3.2 必须先讲清的硬约束(Nexus 3 无原生 HA)

**Nexus Repository 3(含 3.76)没有原生主从复制,也没有 active-active
集群。** OrientDB 元数据库是嵌入式、非外部可复制的;Sonatype 官方对 OSS
版的 HA 指引本质是"单节点 + 备份/恢复做 DR"。

因此本设计**不是同步复制,RPO ≠ 0**,只能实现为温备(warm standby)模
式。请勿误以为存在"提升从库"的原生命令——本设计的"切换"本质是"让已就绪
的从节点接管流量",Nexus 进程无需特殊 promote。

### 3.3 已确认的关键决策

| 维度 | 决策 |
|---|---|
| Blob 存储 | 本地磁盘各自独立(两节点各自本地 blob store) |
| 切换方式 | 手动切换(运维触发,避免脑裂) |
| 部署形态 | 2 个独立容器,分别部署在 2 台独立 VM;前置 F5,手动切换 upstream |
| 元数据 RPO | 可容忍 ≤15 分钟 |

## 4. Objective(目标与关键结果)

### 4.1 目标

让 Nexus 在单节点硬件/进程故障下,可在人工介入的短时间内恢复对外服务,
制品分发与访客访问不中断超过运维切换时长。

### 4.2 关键结果(SMART)

- KR1:主节点宕机后,从 RTO ≤15 分钟(F5 切换 + fencing 确认)恢复匿名
  制品下载。
- KR2:元数据 RPO ≤15 分钟(Export/Import 周期);blob RPO ≤5 分钟
  (rsync 周期)。
- KR3:`nexus-cli ha status` 可同时报告两节点健康、上次同步时间、复制
  延迟,作为日常巡检单一入口。
- KR4:切换前 fencing 步骤可拦截"未停旧主就切流量"的误操作,杜绝脑裂
  双写。

## 5. Market Segment(目标场景与约束)

本设计面向**自建 Nexus OSS、对制品分发连续性有要求、但不采购 Nexus Pro
/不使用对象存储共享后端**的中小规模 DevOps 团队。

约束:

- 两节点跨独立 VM,不存在共享磁盘;blob 各自本地。
- 出口统一经 F5,不依赖 DNS 轮询或 keepalived VIP。
- 团队接受 ≤15 分钟元数据丢失窗口,接受人工切换 RTO。
- 不引入外部分布式数据库或第三方 HA 中间件。

## 6. Value Proposition(价值主张)

- **降低单点故障**:单 VM/容器故障不再等于制品库全站不可用。
- **可预测的恢复**:手动切换 + 明确 runbook,RTO/RPO 可量化、可演练。
- **无需 Pro 授权**:仅用 OSS 版能力 + 原生 Export/Import,无额外许可成
  本。
- **与 nexus-cli 现有治理衔接**:访客权限(`priv_guest_*` 托管权限、
  guest 角色)随元数据导出/导入,从节点接管后权限状态与主节点一致,无需
  在切换后重跑 `guest sync` 才恢复访问治理(仍建议接管后跑一次
  `guest check` 确认)。
- **规避脑裂**:单写原则 + 强制 fencing,从机制上避免双写导致的数据分叉。

## 7. Solution(方案)

### 7.1 架构总览

```
                 ┌──────────────────────────┐
                 │      F5 Load Balancer     │
                 │  active pool = 主节点 only │
                 │  (手动切换 upstream → 从)  │
                 └─────────────┬─────────────┘
              路由流量 │                       │ (无流量 / 仅被同步)
            ┌──────────▼──────────┐  ┌────────▼──────────┐
            │  Nexus-A (PRIMARY)  │  │  Nexus-B (STANDBY) │
            │  VM-1 / container   │  │  VM-2 / container  │
            │  本地 blob store    │  │  本地 blob store   │
            │  OrientDB 元数据    │  │  OrientDB 元数据   │
            └──────────┬──────────┘  └────────▲──────────┘
                       │     定时复制(主 → 从)    │
                       └─────────────────────────┘
           1. blob:    rsync/rclone 主→从(immutable,append-only)
           2. metadata: Nexus Export/Import database 定时任务
```

- **主节点**:F5 active pool 唯一成员,承担读 + 写(发布、匿名浏览)。
- **从节点**:Nexus 进程常驻运行,不接 F5 流量;定期接收主节点的 blob 与
  元数据,保持"可接管的温备"状态。
- **单写原则**:任一时刻只有一个节点可写。这是防脑裂的核心约束,由"F5
  active pool 只含一个成员 + 切换前先 fence 旧主"共同保证。

### 7.2 复制策略

#### 7.2.1 Blob(制品二进制)— rsync/rclone 定时同步

- 主→从 rsync(或 rclone,若 blob 在对象存储)。
- **关键 enabling factor**:`config.example.yaml` 中 raw 仓库
  `writePolicy: allow_once` —— 制品不可变、只追加,rsync 不会遇到撕裂写,
  幂等可重跑。删除(由 `nexus-cli repo lifecycle run` 执行)在 rsync 侧表
  现为从节点删除对应文件,同样幂等。
- 频率:每 5 分钟一次 cron(远小于元数据 RPO,因为二进制通常更大,需更勤
  跑以缩小切换时缺口)。
- 注意:blob store 目录结构需两节点一致(同名 blob store、同路径),否则
  rsync 目标路径与 Nexus 元数据引用不匹配。两节点的 blob store 名必须
  与 `repositories.raw[].storage.blobStoreName` 配置一致。

#### 7.2.2 元数据(OrientDB:仓库/权限/组件索引)— Nexus Export/Import

- 主节点配置 Nexus 内置定时任务 **"Export database"**(每 15 分钟),产物
  为 JSON 导出包;rsync 到从节点;从节点跑 **"Import database"** 任务载入。
- 这是 Sonatype 官方支持的 OSS DR 路径,无需冷停 Nexus。
- RPO ≤15 分钟,满足用户要求。
- `nexus-cli` 当前治理的访客权限(`priv_guest_*` 托管权限、guest 角色、
  `forbiddenPrivileges` 移除结果)随元数据一起导出/导入,从节点接管后权
  限状态与主节点一致 —— 这是与现有 CLI 的关键衔接点。

#### 7.2.3 复制时序与一致性窗口

| 数据 | 方法 | 周期 | RPO | 一致性保证 |
|---|---|---|---|---|
| Blob 二进制 | rsync 主→从 | 5 min | ≤5 min | allow_once 不可变,幂等 |
| 元数据 | Export/Import | 15 min | ≤15 min | 整库快照,原子导入 |
| 组件索引 | 随元数据 | 15 min | ≤15 min | 与仓库定义同包 |

**窗口错配风险**:blob 5 分钟、元数据 15 分钟,可能出现"元数据已含某制
品记录,但 blob 尚未同步到从节点"的窗口(最长 5 分钟)。切换时若命中此
窗口,从节点对个别制品的下载会 404。缓解:切换前先跑
`nexus-cli ha sync --once` 强制对齐一次;无法对齐则接受个别制品缺口
(在 RPO 内,可由客户端重试或等下次 rsync 后恢复)。

### 7.3 故障切换流程(手动,运维执行)

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│ 1. 检测故障   │ →  │ 2. 确认+Fence │ →  │ 3. 追赶同步    │
│ (F5 健康/CLI) │     │ (停旧主,杜双写)│     │ (ha sync --once)│
└─────────────┘     └──────────────┘     └──────────────┘
                                                │
┌─────────────┐     ┌──────────────┐     ┌──────▼───────┐
│ 6. 审计记录   │ ←  │ 5. 从节点接管 │ ←  │ 4. 切 F5      │
│ (audit.Record)│     │ (直接对外服务) │     │ (active→从)   │
└─────────────┘     └──────────────┘     └──────────────┘
```

1. **检测**:运维发现主节点不响应(F5 健康检查失败,或
   `nexus-cli ha status` 显示主节点 DOWN)。
2. **确认 + Fence**:确认主节点真宕机(非网络分区)。若主节点仍可达,
   **先停掉主节点 Nexus**(fencing),杜绝双写。这是单写原则的最后保障。
3. **追赶(可选)**:若主节点还残留最后一次同步后的增量,执行
   `nexus-cli ha sync --once` 推完;若主节点硬宕则跳过,接受 RPO 内缺口。
4. **切 F5**:运维在 F5 把 active pool 从主节点切到从节点。
5. **提升**:从节点 Nexus 已在运行并持有最新导入的元数据 + 同步的 blob,
   直接对外服务。Nexus 3 无需特殊 promote 命令。
6. **审计**:`nexus-cli` 记一条 failover 审计(复用 `audit.Record`,不含
   密码 —— 遵守安全不变式 #3)。
7. **恢复(事后)**:修复旧主,从新主反向同步元数据 + blob,降级回从节
   点。注意:反向同步前需确保旧主已停且不再可写,流程同切换但方向相反。

### 7.4 与 nexus-cli 的关系(v2 实现,本 PRD 仅设计)

当前 `config.NexusConfig`(`internal/config/config.go:57`)只有单个
`baseUrl`。HA 需扩展配置模型,新增 `ha` 段与 `ha` 命令组:

```yaml
ha:
  enabled: true
  role: "primary"            # 本节点角色:primary | standby
  nodes:
    - name: "primary"
      baseUrl: "http://nexus-a.example.com"
      passwordEnv: "NEXUS_PRIMARY_PASSWORD"
    - name: "standby"
      baseUrl: "http://nexus-b.example.com"
      passwordEnv: "NEXUS_STANDBY_PASSWORD"
  replication:
    blobSync:    { method: "rsync", schedule: "*/5 * * * *" }
    metadataSync:{ method: "export-import", schedule: "*/15 * * * *" }
  failover:
    mode: "manual"           # v1 仅支持 manual
    requireFencing: true     # 切换前必须确认旧主已停
```

新增命令(v2):

- `nexus-cli ha status` — 双节点健康 + 复制延迟 + 上次同步时间
- `nexus-cli ha sync --once` — 立即触发 blob + 元数据同步(对齐窗口)
- `nexus-cli ha failover` — 引导式手动切换:fence 检查 → 末次同步 → 打印
  F5 切换指引 → 写审计。**F5 upstream 本身由运维在 F5 侧操作**,v1 不自动
  调 F5 iControl API(避免引入 F5 凭证与网络依赖)。
- `nexus-cli ha health` — 复用 `internal/cli/health.go` 的检查逻辑,对双
  节点各跑一遍。

向后兼容:`ha.enabled=false` 或缺省时,`nexus-cli` 行为与现状完全一致
(单节点),不影响现有 `guest sync` / `repo` / `health` 命令。

### 7.5 假设(Assumptions)

- 两节点 Nexus 版本一致(均 3.76),blob store 命名与路径一致。
- 主→从 rsync 与 Export 产物传输有专用通道(SSH key 或内网),不在本文
  档展开。
- F5 健康检查配置正确(active pool 仅主节点,从节点不在 pool 内)。
- `nexus-cli` 运行环境可同时访问两节点 REST API(用于 `ha status`)。
- 运维有权限在主节点 VM 上执行停 Nexus 操作(fencing)。
- 制品发布仅经 F5 → 主节点;无客户端绕过 F5 直连从节点发布(否则破坏单
  写原则)。

## 8. Release(发布计划)

| 版本 | 内容 | 相对工期 |
|---|---|---|
| v1 | 本设计文档 + 运维 runbook(F5 配置模板、Export/Import 任务配置、rsync cron、故障切换 checklist) | 文档,1 周内 |
| v2 | `nexus-cli` `ha` 命令组与 `ha` 配置段实现(另开任务,含单元测试) | 2–3 周 |
| v3(远期) | 探索 blob 共享存储(S3/MinIO)消除二进制 RPO;探索 F5 iControl API 自动切流量(仍保持 fencing 人工确认) | 视需求 |

## 9. 验证(如何确认设计成立)

PRD 是设计文档,验证以"可演练、可落地"为准:

1. **桌面推演(tabletop)**:按 7.3 故障切换流程走一遍主节点宕机场景,确
   认每一步有明确执行人与产物,fencing 与单写原则无漏洞。
2. **dev 环境实测**:起两台容器化 Nexus,配置 Export/Import 定时任务 +
   rsync blob 同步;手动 kill 主节点 → 切 F5 → 验证从节点能匿名下载
   `devops-prod-generic` 制品(对应 `nexus-cli guest` 治理的仓库)。
3. **RPO 验证**:主节点发布一个制品后 10 分钟 kill,确认从节点在最近一次
   导入窗口内可见该制品(或明确缺口 ≤15 分钟)。
4. **权限一致性**:从节点接管后跑 `nexus-cli guest check`,确认
   `priv_guest_*` 托管权限与主节点一致(证明元数据导出/导入覆盖了 CLI 治
   理的权限对象)。
5. **脑裂负向验证**:故意不停主节点就切 F5,确认 PRD 的 fencing 步骤会拦
   截并告警(此为设计约束的负向验证,确保单写原则被流程强制)。

## 10. 风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| 元数据导入失败(从节点 Import 任务报错) | 从节点元数据滞后超过 RPO | `nexus-cli ha status` 监控导入时间,超阈值告警 |
| blob 与元数据窗口错配 | 切换后个别制品 404 | 切换前 `ha sync --once` 对齐;接受 RPO 内缺口 |
| 运维忘记 fencing 直接切 F5 | 脑裂双写,数据分叉 | `ha failover` 强制 fence 检查;runbook 红线标注 |
| 从节点长期不接流量,接管时发现自身已坏 | 切换失败,RTO 超标 | `ha status` 日常巡检;从节点 Nexus 常驻,定期 `ha health` |
| Export/Import 对大库耗时超 15 分钟 | 周期堆积,延迟放大 | 监控单次导出耗时,必要时调大周期或拆库 |

---

> 安全不变式延续:本设计中所有审计记录复用 `audit.Record`,绝不包含
> Nexus 密码或 Authorization 头(遵循 CLAUDE.md 安全不变式 #3)。`ha`
> 配置段中 `passwordEnv` 同样只存环境变量名,不存密码值。
