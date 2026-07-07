# 仓库与 Blob Store 基础增改查需求分析

## 1. 背景

`nexus-cli` 已支持访客权限治理、raw hosted 仓库声明式管理、raw 制品生命周期清理。但当前仓库管理能力集中在 `raw/hosted`，无法覆盖 npm、maven2、docker 等其他仓库格式；同时 raw 仓库配置依赖的 Blob Store 也只能事先在 Nexus UI 中创建。

这会让平台管理员在初始化或维护 Nexus 时需要在 CLI 和 UI 之间切换，难以形成可重复、可审计的基础资源管理流程。

## 2. 用户与目标

主要用户是 Nexus 平台管理员、SRE、DevOps 工程师。

目标：

- 可以查询所有仓库，并按 format/type 查看具体仓库配置。
- 可以用配置或单次命令创建、更新所有格式仓库。
- 可以查询、创建、更新本地 file 类型 Blob Store。
- 写操作支持 dry-run 和审计日志。
- 重复执行相同目标状态不会产生多余写请求。

## 3. 当前能力缺口

- `repo list` 只能列出仓库摘要，不能筛选，也不能查看仓库详情。
- `repo raw apply/ensure` 只支持 `raw/hosted`。
- 配置中没有通用仓库声明模型。
- 没有 Blob Store 查询和管理命令。
- raw 仓库依赖 Blob Store，但 CLI 无法保证 Blob Store 已存在。

## 4. 范围判断

### 本期范围

- 所有 format/type 仓库的基础增改查。
- file 类型 Blob Store 的基础增改查。
- 配置声明式 apply 和单次 ensure。
- dry-run、审计、README、示例配置、单元测试。

### 非本期范围

- 删除仓库或 Blob Store。
- 制品文件上传、下载、删除。
- S3、Azure、Google、group Blob Store 的强类型管理。
- 为每种仓库格式设计专用字段。
- 自动迁移仓库引用的 Blob Store。

## 5. 关键产品取舍

### 仓库采用 settings 透传

Nexus 的仓库创建和更新 API 按 format/type 拆分，不同格式字段差异很大。为了 v1 覆盖所有格式，仓库配置采用：

```yaml
repositories:
  managed:
    - name: "npm-hosted"
      format: "npm"
      type: "hosted"
      settings:
        online: true
        storage:
          blobStoreName: "default"
          strictContentTypeValidation: true
          writePolicy: "ALLOW"
```

CLI 会将 `settings` 作为 Nexus 请求体，并自动补充 `name`。用户需要参考目标 Nexus 实例 Swagger 填写具体字段。

### Blob Store v1 先支持 file

当前部署文档和 raw 仓库场景主要依赖本地 file Blob Store。S3 等类型字段更多，留给后续版本扩展。

## 6. 验收标准

- `repo list --format F --type T` 可筛选仓库。
- `repo get --name NAME --format F --type T` 输出仓库详情 JSON。
- `repo apply --dry-run` 不产生写请求。
- `repo ensure --settings FILE` 可以创建或更新任意 format/type 仓库。
- 同名仓库 format/type 不一致时失败，不迁移、不删除重建。
- `blobstore list/get/apply/ensure` 支持 file 类型 Blob Store。
- Blob Store 缺少 name/path 时配置校验失败。
- 写操作记录审计日志，且不泄露密码和 Authorization。
- `CGO_ENABLED=0 go test ./...` 通过。
