# 仓库与 Blob Store 基础增改查 PRD

## 1. Summary

本功能为 `nexus-cli` 增加基础 Nexus 资源管理能力：所有格式仓库的新增、修改、查询，以及 file 类型 Blob Store 的新增、修改、查询。它让平台管理员可以用配置和命令行管理仓库基础设施，减少手工 UI 操作。

## 2. Contacts

| Name | Role | Comment |
| --- | --- | --- |
| 平台管理员 | 用户 / 验收方 | 使用 CLI 管理 Nexus 仓库和 Blob Store。 |
| SRE / DevOps | 用户 / 运维方 | 需要 dry-run、审计和可重复执行。 |
| nexus-cli 维护者 | 开发方 | 负责实现 CLI、配置、测试和文档。 |

## 3. Background

当前 `nexus-cli` 已支持 raw hosted 仓库和 raw 生命周期清理，但 Nexus 实际使用中常见仓库格式还包括 npm、maven2、docker 等。用户希望用同一个 CLI 管理基础仓库资源。

同时，仓库通常依赖 Blob Store。当前 Blob Store 只能由用户提前在 Nexus UI 中创建，导致仓库配置无法完全自动化。

## 4. Objective

目标是提供一个最基础、可审计、可重复执行的资源管理能力。

Key Results：

- KR1：用户可以列出所有仓库，并按 format/type 筛选。
- KR2：用户可以查询任意 format/type 仓库详情。
- KR3：用户可以通过配置或命令创建、更新任意 format/type 仓库。
- KR4：用户可以查询、创建、更新 file Blob Store。
- KR5：重复执行同一配置时，状态一致的资源输出 `unchanged`，不产生写请求。
- KR6：写操作支持 dry-run 和审计日志。

## 5. Market Segments

本功能服务于 Nexus 平台管理员、SRE、DevOps 工程师。

他们的共同任务是让 Nexus 资源可以被脚本化、审计化、重复执行。约束是 Nexus 仓库字段随 format/type 变化较大，v1 不适合为每种格式提供强类型配置。

## 6. Value Propositions

- 用户可以少进 Nexus UI，减少手工配置错误。
- 用户可以把仓库和 Blob Store 目标状态放进配置文件。
- 用户可以用 dry-run 先预览变更。
- 用户可以通过审计日志追踪谁变更了什么。
- CLI 保留现有 raw 专用命令，同时新增更通用的仓库管理入口。

## 7. Solution

### 7.1 UX / Commands

仓库命令：

```sh
nexus-cli repo list [--format npm] [--type hosted]
nexus-cli repo get --name npm-hosted --format npm --type hosted
nexus-cli repo apply [--dry-run]
nexus-cli repo ensure --name npm-hosted --format npm --type hosted --settings npm-hosted.yaml [--dry-run]
```

Blob Store 命令：

```sh
nexus-cli blobstore list
nexus-cli blobstore get --name default --type file
nexus-cli blobstore apply [--dry-run]
nexus-cli blobstore ensure --name default --path /nexus-data/blobs/default [--dry-run]
```

### 7.2 Key Features

- `repositories.managed[]` 支持通用仓库声明。
- `repositories.managed[].settings` 原样映射 Nexus API 请求体，CLI 自动补充 `name`。
- `blobStores.file[]` 支持 file Blob Store 声明。
- manager 层负责 create/update/unchanged/conflict 判断。
- 同名仓库 format/type 冲突时失败。
- 同名 Blob Store 类型不是 file 时失败。
- 写操作记录审计日志。

### 7.3 Technology

新增 Nexus REST client：

- `GET /repositories`
- `GET /repositories/{format}/{type}/{name}`
- `POST /repositories/{format}/{type}`
- `PUT /repositories/{format}/{type}/{name}`
- `GET /blobstores`
- `GET /blobstores/file/{name}`
- `POST /blobstores/file`
- `PUT /blobstores/file/{name}`

配置新增：

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

blobStores:
  file:
    - name: "default"
      path: "/nexus-data/blobs/default"
```

### 7.4 Assumptions

- “blob” 指 Nexus Blob Store，不是制品文件。
- v1 不做删除。
- v1 不做制品文件上传、下载、删除。
- v1 Blob Store 只支持 file 类型。
- 通用仓库字段由用户参考 Nexus Swagger 填写。

## 8. Release

### v1

- 通用仓库 list/get/apply/ensure。
- file Blob Store list/get/apply/ensure。
- 配置、审计、README、示例配置。
- 单元测试和 HTTP client 测试。

### Future

- 删除能力，但需要单独设计确认机制。
- S3/Azure/Google/group Blob Store。
- 常用仓库格式的强类型快捷命令。
- 更友好的配置 diff 输出。
