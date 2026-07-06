# 发布与分发运维手册

nexus-cli 是纯 Go 静态二进制,本身没有运行时依赖,但为了让用户能用
`npm` / `yum` / `dnf` 这些熟悉的包管理器安装它,本项目在 GitHub
Release 之外搭了两条分发渠道:

- **npm**:`@mogesang/nexus-cli`,一个 thin wrapper 包,`postinstall`
  时按平台从 GitHub Releases 下载预编译二进制并校验 sha256。
- **yum / dnf**:GPG 签名的 RPM 包,托管在本仓库的 GitHub Pages 上的
  静态 yum 仓库,用户放一个 `.repo` 文件即可 `yum install`。

两条渠道都由 `.github/workflows/release.yml` 在打 `v*` tag 时自动构建
发布。本文档详细说明**一次性配置**(secrets、GPG 密钥、GitHub Pages)
和**日常发布**操作。

> 本地构建与交叉编译脚本说明见 [`build.md`](build.md)。本文档聚焦于
> 发布到 npm/yum 的运维操作。

---

## 目录

- [1. 分发渠道总览](#1-分发渠道总览)
- [2. 所需 GitHub Secrets 一览](#2-所需-github-secrets-一览)
- [3. 一次性配置](#3-一次性配置)
  - [3.1 npm 渠道配置](#31-npm-渠道配置)
  - [3.2 yum/RPM 渠道配置](#32-yumrpm-渠道配置)
  - [3.3 GitHub Pages 配置](#33-github-pages-配置)
- [4. 日常发布流程](#4-日常发布流程)
- [5. 发布后验证清单](#5-发布后验证清单)
- [6. 故障排查](#6-故障排查)
- [7. 密钥与 Token 安全维护](#7-密钥与-token-安全维护)
- [8. 进阶操作](#8-进阶操作)
- [9. 常见问题](#9-常见问题)

---

## 1. 分发渠道总览

| 渠道 | 包名 / 地址 | 适用场景 | 产物 |
|---|---|---|---|
| GitHub Release | `https://github.com/mogesangop/nexus-cli/releases` | 直接下载二进制;npm postinstall 的下载源 | 6 个平台 archive(`.tar.gz`/`.zip`)+ sha256 |
| npm | `@mogesang/nexus-cli` | 跨平台 npm 生态用户(CI 镜像、devcontainer) | 一个 scoped 包,`postinstall` 拉二进制 |
| yum / dnf | `https://mogesangop.github.io/nexus-cli/rpm/` | RHEL/CentOS/Rocky/Alma/Fedora 服务器 | GPG 签名的 x86_64 + aarch64 RPM |

### 发布流水线

打 `v*` tag 后,`.github/workflows/release.yml` 跑三个 job:

```
git push origin v1.1.0
        │
        ▼
┌───────────────────┐
│  build job        │  交叉编译 6 平台 → 上传 GitHub Release 资产
│  (ubuntu-latest)  │  + 上传 dist/ 作为 artifact 给下游 job
└─────────┬─────────┘
          │ needs: build
          ├──────────────────────┐
          ▼                      ▼
┌───────────────────┐   ┌───────────────────┐
│  npm job          │   │  yum job          │
│  设置版本号        │   │  下载 dist artifact │
│  npm publish       │   │  fpm 打 rpm        │
│  (--ignore-scripts)│   │  GPG 签名          │
└───────────────────┘   │  createrepo        │
                        │  推 gh-pages 分支   │
                        └───────────────────┘
```

三个 job **互相独立**:npm 失败不影响 yum,反之亦然。可在 Actions
UI 单独重跑失败的 job,无需重新打 tag。

### 产物命名约定

npm 用 `process.platform` / `process.arch`,所以 asset 名遵循 npm 约定:

| npm platform | npm arch | asset 文件名 |
|---|---|---|
| `linux` | `x64` | `nexus-cli-linux-x64.tar.gz` |
| `linux` | `arm64` | `nexus-cli-linux-arm64.tar.gz` |
| `darwin` | `x64` | `nexus-cli-darwin-x64.tar.gz` |
| `darwin` | `arm64` | `nexus-cli-darwin-arm64.tar.gz` |
| `win32` | `x64` | `nexus-cli-win32-x64.zip` |
| `win32` | `arm64` | `nexus-cli-win32-arm64.zip` |

> **注意**:v1.0.0 的 Release 资产用的是旧命名
> (`nexus-cli-linux-amd64` 裸二进制),与 npm postinstall 和 yum job
> **不兼容**。从 v1.1.0 起统一用新命名。首次正确发布需打 `v1.1.0+`。

---

## 2. 所需 GitHub Secrets 一览

在 **GitHub 仓库 → Settings → Secrets and variables → Actions →
Repository secrets** 中配置以下 4 个 secret。全部是**一次性**配置,
之后每次发版自动复用。

| Secret 名 | 用途 | 必填 | 配置位置 |
|---|---|---|---|
| `NPM_TOKEN` | npm 发布鉴权 | 是(npm job) | [3.1](#31-npm-渠道配置) |
| `GPG_PRIVATE_KEY` | 签名 RPM 的 GPG 私钥(ASCII-armored) | 是(yum job) | [3.2](#32-yumrpm-渠道配置) |
| `GPG_PASSPHRASE` | GPG 私钥的口令(无口令则填空字符串) | 是(yum job) | [3.2](#32-yumrpm-渠道配置) |
| `GPG_KEY_ID` | GPG 签名 key id(用于指定用哪把密钥签名) | 是(yum job) | [3.2](#32-yumrpm-渠道配置) |

`GITHUB_TOKEN` 不需要手动配置,GitHub Actions 自动注入,`yum` job 用
它推 `gh-pages` 分支。

> **没配好 secrets 就打 tag 会怎样?**
> - `build` job 正常,GitHub Release 照常发布。
> - `npm` job 在 `npm publish` 步骤失败(401 Unauthorized)。
> - `yum` job 在 `build-rpm.sh` 步骤失败(脚本检测到
>   `GPG_PRIVATE_KEY` 为空会主动 `exit 1`)。
> 补配 secret 后到 Actions UI 点 "Re-run failed jobs" 即可,不用重新打 tag。

---

## 3. 一次性配置

### 3.1 npm 渠道配置

#### 3.1.1 包结构说明

npm 包代码在 `packaging/npm/`,发布时整个目录作为一个 npm 包:

```
packaging/npm/
├── package.json          # 包元数据,bin 字段指向 shim
├── bin/
│   └── nexus-cli.js      # shim:spawn vendor/nexus-cli,透传参数和退出码
├── scripts/
│   └── postinstall.js    # 安装时下载二进制 + 校验 sha256 + 解压
└── README.md             # npm 包页面展示的说明
```

**工作原理**:

1. 用户 `npm i -g @mogesang/nexus-cli`,npm 把包解压到全局
   `node_modules`,然后执行 `scripts/postinstall.js`。
2. `postinstall.js` 读取 `process.platform` / `process.arch`(例如
   `darwin` / `arm64`),拼出 asset 名
   `nexus-cli-darwin-arm64.tar.gz`。
3. 从
   `https://github.com/mogesangop/nexus-cli/releases/download/v<version>/`
   下载该 asset 和对应的 `.sha256` 文件。
4. 用 `crypto.createHash('sha256')` 计算下载内容的哈希,与 `.sha256`
   文件里的期望值比对,不一致则报错退出(防止下载损坏或被篡改)。
5. 解压到包内的 `vendor/` 目录(`.tar.gz` 用 `tar -xzf`,`.zip` 用
   `tar -xf`——GitHub runner 上的 `tar` 能解 zip;macOS 的 bsdtar
   也能)。
6. 非 Windows 上 `chmod 755 vendor/nexus-cli`。
7. 用户执行 `nexus-cli ...` 时,npm 的 `bin` 链接指向
   `bin/nexus-cli.js`,该 shim 用 `child_process.spawn` 调起
   `vendor/nexus-cli`,透传 `process.argv` 和 stdio,并转发退出码。

**版本号来源**:`package.json` 里写的是占位版本(当前 `1.0.0`),CI
在发布前用 `npm --no-git-tag-version version "${GITHUB_REF_NAME#v}"`
覆盖成 tag 去掉 `v` 前缀的值。所以打 `v1.1.0` → npm 包版本 `1.1.0`。

#### 3.1.2 注册 npm 账号并获取 scope

1. 打开 https://www.npmjs.com/signup 注册账号。
   - 用户名用 `mogesang`(你的 npm 账号;注意与 GitHub 用户名
     `mogesangop` 不同,npm scope 与 GitHub 账号相互独立)。
   - 注册后该账号自动拥有 `@mogesang` scope,可以发布
     `@mogesang/<任意包名>`。
2. 登录后在 **Account → Account Settings**:
   - 确认邮箱已验证(未验证邮箱无法发布)。
   - 建议开启 **Two-Factor Authentication**(2FA)。开启后,**必须**用
     Automation token 发布(见下一步),否则 CI 发布会被要求输入 OTP。

> **为什么用 scoped 包名 `@mogesang/nexus-cli`?**
> 因为 unscoped 的 `nexus-cli` 已被占用
> (https://www.npmjs.com/package/nexus-cli,0.0.3,2017 年的 Nexus 2.x
> 凭证工具,与本项目无关)。npm 不允许重名,只能用 scope 隔离。

#### 3.1.3 创建 Automation Access Token

1. 登录 npmjs.com → 右上角头像 → **Access Tokens**。
2. 点 **Generate New Token** → 选 **Classic Token**。
3. **Type 选 `Automation`**(关键):
   - `Automation` token 专为 CI 设计,**发布时不触发 2FA 验证**。
   - 如果选 `Read-only` 无法发布;选 `Publish` 在账号开了 2FA 时会
     卡在 OTP 输入。
4. 复制生成的 token(形如 `npm_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
   **只显示一次**)。

> 也可用 **Granular Access Token**(细粒度),限定权限为
> "Read and write" + 包名 `@mogesang/nexus-cli` + 过期时间。比
> Classic Automation token 更安全,推荐生产环境使用。但对新手来说
> Classic Automation token 更简单。

#### 3.1.4 配置 NPM_TOKEN secret

1. GitHub 仓库 → **Settings → Secrets and variables → Actions**。
2. 点 **New repository secret**。
3. **Name**: `NPM_TOKEN`
4. **Secret**: 粘贴上一步复制的 token。
5. 点 **Add secret**。

#### 3.1.5 本地验证 token 是否可用(可选但推荐)

```sh
# 用 token 临时登录(不写入 ~/.npmrc 的话用环境变量)
npm config set //registry.npmjs.org/:_authToken "$NPM_TOKEN" --location=user

# 查看当前登录身份
npm whoami
# 期望输出: mogesang

# 验证完成后清除 token(避免本地残留)
npm config delete //registry.npmjs.org/:_authToken --location=user
```

#### 3.1.6 本地手动发布(应急用,正常走 CI)

> 正常发布由 CI 自动完成。这里仅当 CI 故障需要应急手动发布时使用。

```sh
cd packaging/npm

# 1. 设置与 tag 一致的版本号(假设 tag 是 v1.1.0)
npm --no-git-tag-version version 1.1.0

# 2. 确认 GitHub Releases 上 v1.1.0 的 6 个 archive 资产已存在
#    (postinstall 依赖它们;手动发布前务必先跑 build job 或本地
#     bash scripts/build.sh v1.1.0 后手动上传 Release)

# 3. 发布。--ignore-scripts 跳过 postinstall(发布机不需要下载二进制)
#    --access public 让 scoped 包公开可见(默认是私有的,需要付费账号)
NPM_TOKEN=你的token npm publish --access public --ignore-scripts

# 4. 验证
npm view @mogesang/nexus-cli version
# 期望输出: 1.1.0
```

---

### 3.2 yum/RPM 渠道配置

#### 3.2.1 架构说明

yum 仓库是一个普通的 HTTP 目录,只要包含:
- 若干 `.rpm` 包
- `repodata/` 目录(`createrepo` 生成的索引)
- 公钥文件 `RPM-GPG-KEY-nexus-cli`

用户在 `/etc/yum.repos.d/` 放一个 `.repo` 文件指向该 URL 即可。

本项目用 **GitHub Pages** 托管这个目录:把 `gh-pages` 分支当作静态
站点,`yum` job 每次发版把新 RPM 和重新生成的 repodata 推上去。
GitHub Pages 免费、零运维、与发版同源。

RPM 用 **GPG 签名**,用户 `rpm --import` 公钥后即可验证包的完整性
和来源。

#### 3.2.2 生成 GPG 签名密钥

在**本地机器**(不是 CI)上生成,私钥永远不进 CI runner 的磁盘
(只在发布时通过 secret 注入,用完即弃)。

**方式 A:交互式(推荐新手)**

```sh
gpg --full-generate-key
```

会依次提问,按以下选择:

| 提问 | 选择 | 说明 |
|---|---|---|
| Please select what kind of key you want | `4` (RSA (sign only)) | 只签名,不需要加密能力 |
| What keysize do you want | `3072` 或 `4096` | 3072 够用;4096 更稳但签名稍慢 |
| Key is valid for | `0` (key does not expire) | 不过期;或填 `2y` 两年后轮换 |
| Real name | `nexus-cli release` | 用途明确的名字 |
| Email address | `release@mogesang.local` | 任意,不用于通信 |
| Comment | `RPM signing` | 可留空 |
| 确认 | 输入 `y` | — |

然后设置一个**强口令**(passphrase),这会成为 `GPG_PASSPHRASE` secret。

**方式 B:非交互式(可复制粘贴)**

```sh
gpg --batch --full-generate-key <<EOF
%no-protection
Key-Type: RSA
Key-Length: 3072
Key-Usage: sign
Name-Real: nexus-cli release
Name-Email: release@mogesang.local
Expire-Date: 0
%commit
EOF
```

> 上面的 `%no-protection` 生成**无口令**密钥。简单,但安全性稍低
> (私钥泄露即可直接签名)。如果要口令,把 `%no-protection` 换成
> `%passphrase 你的口令`。本文档后续按"有口令"说明。

#### 3.2.3 查看 key id

```sh
gpg --list-secret-keys --keyid-format=long
```

输出形如:

```
sec   rsa3072/ABCD1234EFGH5678 2026-07-06 [SC]
      K1K2K3K4K5K6K7K8K9K0L1L2L3L4L5L6L7L8L9L0M1M2
uid         [ultimate] nexus-cli release <release@mogesang.local>
```

其中 `sec` 行 `/` 后面的 `ABCD1234EFGH5678` 就是 **key id**
(指纹的后 16 位十六进制),这会成为 `GPG_KEY_ID` secret。

#### 3.2.4 导出私钥和公钥

```sh
# 1. 导出 ASCII-armored 私钥(整段输出就是 GPG_PRIVATE_KEY secret 的值)
gpg --export-secret-keys --armor ABCD1234EFGH5678
# 输出形如:
# -----BEGIN PGP PRIVATE KEY BLOCK-----
# ...(很多行)...
# -----END PGP PRIVATE KEY BLOCK-----

# 2. 导出公钥(保存一份本地备份;CI 也会从私钥重新导出)
gpg --export --armor ABCD1234EFGH5678 > RPM-GPG-KEY-nexus-cli

# 3. 导出完整指纹(可选,用于核对)
gpg --fingerprint --with-colons ABCD1234EFGH5678 | grep '^fpr:' | head -1
```

把私钥输出**完整复制**(从 `-----BEGIN` 到 `-----END` 含换行)。

#### 3.2.5 备份与离线保管(重要!)

```sh
# 1. 把私钥导出到加密文件(用你的口令加密)
gpg --export-secret-keys ABCD1234EFGH5678 > nexus-cli-signing-private.key
# 文件是二进制,大小约几 KB

# 2. 备份到至少两个独立位置:
#    - 加密 U 盘(放保险柜)
#    - 密码管理器(1Password / Bitwarden / KeePass 的附件)
#    - 公司内部密钥管理服务(如有)

# 3. 验证备份能恢复:
gpg --delete-secret-keys ABCD1234EFGH5678   # 先删除本地
gpg --import nexus-cli-signing-private.key  # 再从备份恢复
gpg --list-secret-keys --keyid-format=long  # 确认回来了

# 4. 清理临时文件
shred -u nexus-cli-signing-private.key  # 或 srm / rm -P(macOS)
```

> **为什么要备份?** 如果私钥丢失,无法再用同一把密钥签名,用户
> 已 import 的旧公钥会拒绝新 RPM,只能换新密钥并要求所有用户重新
> import。这是运维事故,务必避免。

#### 3.2.6 配置三个 GPG secret

回到 GitHub 仓库 → **Settings → Secrets and variables → Actions**
→ **New repository secret**,依次添加:

| Secret 名 | 值 | 注意事项 |
|---|---|---|
| `GPG_PRIVATE_KEY` | 上一步 `gpg --export-secret-keys --armor` 的**完整输出** | 必须包含 `-----BEGIN PGP PRIVATE KEY BLOCK-----` 和 `-----END PGP PRIVATE KEY BLOCK-----` 两行,中间所有换行保留 |
| `GPG_PASSPHRASE` | 生成密钥时设的口令 | 如果用 `%no-protection` 无口令,这里填空字符串(直接保存空值) |
| `GPG_KEY_ID` | `ABCD1234EFGH5678` 这样的 key id | 从 `--list-secret-keys --keyid-format=long` 的 `sec` 行取 |

**验证 secret 是否正确配置**(本地模拟 CI 的签名流程):

```sh
# 准备一个测试 rpm(或随便下个现成的)
echo "$GPG_PRIVATE_KEY" | gpg --batch --import
gpg --list-secret-keys --keyid-format=long  # 确认 key 已导入

# 写 ~/.rpmmacros(与 build-rpm.sh 里的一致)
cat > ~/.rpmmacros <<EOF
%_signature gpg
%_gpg_name $GPG_KEY_ID
EOF

# 给测试 rpm 签名
gpg --batch --yes --pinentry-mode loopback \
    --passphrase "$GPG_PASSPHRASE" \
    --detach-sign --armor test.rpm

# 验证签名
rpm --checksig --verbose test.rpm
# 期望输出包含: test.rpm: signatures OK
```

如果上面报 `no default secret key`,说明 `GPG_KEY_ID` 与导入的私钥
不匹配;报 `Bad passphrase`,说明 `GPG_PASSPHRASE` 错误。

#### 3.2.7 RPM 构建脚本说明

`scripts/build-rpm.sh` 在 `yum` job 里执行,流程:

1. 接收版本号和两个 linux 二进制路径(x64 + arm64)。
2. 用 `fpm -s dir -t rpm` 把二进制打包成标准 RPM:
   - 安装路径 `/usr/bin/nexus-cli`
   - 包名 `nexus-cli`,版本取 tag 去掉 `v`
   - `--iteration 1 --rpm-dist el9`(生成 `1.1.0-1.el9` 这样的
     完整版本串)
3. 导入 `GPG_PRIVATE_KEY`,导出公钥到 `RPM-GPG-KEY-nexus-cli`。
4. 写 `~/.rpmmacros` 配置签名身份,用 `rpm --addsign` 给两个 RPM
   签名。
5. `createrepo` 在 `dist/yum-repo/` 生成 repodata 索引。

产物:

```
dist/
├── nexus-cli-1.1.0-1.el9.x86_64.rpm
├── nexus-cli-1.1.0-1.el9.aarch64.rpm
├── RPM-GPG-KEY-nexus-cli
└── yum-repo/
    ├── nexus-cli-1.1.0-1.el9.x86_64.rpm
    ├── nexus-cli-1.1.0-1.el9.aarch64.rpm
    ├── repodata/
    │   ├── repomd.xml
    │   ├── primary.xml.gz
    │   └── ...
    └── (历史版本 RPM 也会累积在这里)
```

#### 3.2.8 .repo 文件与公钥分发

`packaging/rpm/nexus-cli.repo` 是给用户的源配置模板:

```ini
[nexus-cli]
name=nexus-cli
baseurl=https://mogesangop.github.io/nexus-cli/rpm/
enabled=1
gpgcheck=1
gpgkey=https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli
```

- `baseurl` 指向 GitHub Pages 上的 `rpm/` 目录(累积所有版本 RPM)。
- `gpgkey` 指向公钥文件,yum/dnf 会在首次安装时自动 import,
  无需用户手动 `rpm --import`。

这个文件和公钥每次发版都会被 `yum` job 拷贝到 `gh-pages` 根目录。

---

### 3.3 GitHub Pages 配置

yum 仓库和公钥都托管在 GitHub Pages 上。需要一次性开启:

1. **首次打 tag 触发 release workflow**:
   `yum` job 里的 `peaceiris/actions-gh-pages@v4` 会自动创建
   `gh-pages` 分支并推送内容。

   ```sh
   # 配好 secrets 后,打第一个 tag
   git tag v1.1.0
   git push origin v1.1.0
   ```

2. 等 Actions 跑完(约 5-10 分钟),到仓库 **Settings → Pages**:
   - **Build and deployment → Source** = `Deploy from a branch`
   - **Branch** = `gh-pages` / `(root)`
   - 点 **Save**

3. 等 1-2 分钟,访问以下 URL 确认可达:
   - `https://mogesangop.github.io/nexus-cli/`(应返回 404 但不是
     404——GitHub Pages 根没有 index.html 时会显示仓库文件列表或
     空白,这是正常的)
   - `https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli`
     (应显示 `-----BEGIN PGP PUBLIC KEY BLOCK-----`)
   - `https://mogesangop.github.io/nexus-cli/nexus-cli.repo`
     (应显示 .repo 文件内容)
   - `https://mogesangop.github.io/nexus-cli/rpm/repodata/repomd.xml`
     (应显示 XML)

> **Settings → Pages 在哪?** 仓库主页 → 右侧栏 Settings → 左侧菜单
> Code and automation 下 Pages。如果看不到,确认仓库是 public
> (private 仓库的 GitHub Pages 需要付费 plan)。

---

## 4. 日常发布流程

完成 [第 3 节](#3-一次性配置) 的一次性配置后,日常发版非常简单:

### 4.1 标准发布

```sh
# 1.(可选)更新 CHANGELOG / 文档,提交
git add -A && git commit -m "docs: prep v1.1.0"
git push origin master

# 2. 打 tag(tag 名必须 v 开头,触发 release.yml 的 on.push.tags)
git tag v1.1.0
git push origin v1.1.0

# 3. 到 Actions 页面观察:
#    https://github.com/mogesangop/nexus-cli/actions
#    会看到 "Release" workflow 跑三个 job
```

### 4.2 三 Job 独立性与重跑

| Job | 失败的影响 | 重跑方式 |
|---|---|---|
| `build` | GitHub Release 没有资产;下游 npm/yum job 不会启动(needs: build) | Actions UI → 点失败的 run → 右上 "Re-run all jobs" |
| `npm` | npm 包没更新;GitHub Release 和 yum 不受影响 | Actions UI → 点 npm job → "Re-run failed jobs" |
| `yum` | yum 仓库没更新;GitHub Release 和 npm 不受影响 | 同上 |

**不需要重新打 tag**。补配 secret 或修 bug 后直接在 UI 重跑即可。

### 4.3 预发布 / RC 版本

如果要发预发布版本(如 `v1.2.0-rc.1`):

```sh
git tag v1.2.0-rc.1
git push origin v1.2.0-rc.1
```

- `release.yml` 的 `on.push.tags: v*` 会匹配,正常触发三 job。
- GitHub Release 会自动标记为 prerelease(因为 tag 名含 `-rc.`)。
- npm 包版本会是 `1.2.0-rc.1`(npm 支持 semver 预发布标签)。
- 用户 `npm i @mogesang/nexus-cli@latest` 不会装到 RC
  (npm 的 `latest` dist-tag 不指向预发布版本),需显式
  `npm i @mogesang/nexus-cli@1.2.0-rc.1`。

---

## 5. 发布后验证清单

每次发版后,逐项确认:

### GitHub Release

- [ ] https://github.com/mogesangop/nexus-cli/releases/tag/v1.1.0 页面
      有 12 个资产(6 archive + 6 sha256)。
- [ ] 资产名符合 `nexus-cli-<plat>-<arch>.tar.gz|.zip` 约定。
- [ ] 点开 `nexus-cli-linux-x64.tar.gz.sha256`,内容是 64 位十六进制
      + 文件名。

### npm

```sh
# 1. 版本号正确
npm view @mogesang/nexus-cli version
# 期望: 1.1.0

# 2. 在干净环境装一下
npx --package=@mogesang/nexus-cli@1.1.0 -- nexus-cli --help
# 或
npm i -g @mogesang/nexus-cli@1.1.0
nexus-cli --help

# 3. 确认二进制能跑(在对应平台)
nexus-cli --version 2>/dev/null || nexus-cli --help | head -1
```

- [ ] `npm view` 显示正确版本。
- [ ] 全局安装后 `nexus-cli --help` 正常输出。
- [ ] 在另一个平台(如 macOS)上也验证一次(可选)。

### yum

```sh
# 在一台 RHEL/Rocky/Alma/Fedora 机器上:

# 1. 放 .repo 文件
sudo curl -o /etc/yum.repos.d/nexus-cli.repo \
  https://mogesangop.github.io/nexus-cli/nexus-cli.repo

# 2. import 公钥(也可让 yum 首次安装时自动 import)
sudo rpm --import \
  https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli

# 3. 查看可用包
yum list available nexus-cli
# 期望显示: nexus-cli.x86_64  1.1.0-1.el9  @nexus-cli

# 4. 安装
sudo yum install nexus-cli

# 5. 验证
nexus-cli --help
rpm -qi nexus-cli          # 看 Version / Release / Signature
rpm --checksig $(rpm -ql nexus-cli | head -1 | xargs dirname)/nexus-cli
# 期望: sha256 md5 OK

# 6. 验证签名
rpm -K $(rpm -ql nexus-cli | head -1 | xargs dirname)/nexus-cli 2>&1 || \
rpm --checksig --verbose $(rpm -ql nexus-cli | head -1)
# 期望包含: signatures OK
```

- [ ] `.repo` 文件下载成功。
- [ ] `yum list available` 能看到包。
- [ ] `yum install` 成功,`gpgcheck` 通过。
- [ ] `nexus-cli --help` 正常输出。
- [ ] `rpm --checksig` 显示签名 OK。

- [ ] (aarch64 机器上重复上述验证)。

---

## 6. 故障排查

| 症状 | 可能原因 | 解决 |
|---|---|---|
| `npm` job: `npm ERR! This command requires you to be logged in` | `NPM_TOKEN` 未配置或已过期 | 到 npmjs.com 重新生成 token,更新 GitHub secret |
| `npm` job: `npm ERR! 403 Forbidden - You cannot publish over the previously published versions` | 该版本号已发布过(npm 不允许覆盖) | 打新 tag(如 `v1.1.1`),或先 `npm deprecate` 旧版本 |
| `npm` job: `npm ERR! You must be logged in to publish packages` | scope 归属错误(账号不属于 `@mogesang` scope) | 确认 npm 账号是 `mogesang`,且包名用 `@mogesang/nexus-cli` |
| `npm install` 后 `nexus-cli: binary not installed` | postinstall 被跳过(如 `--ignore-scripts` 安装)或下载失败 | 重新 `npm rebuild @mogesang/nexus-cli`;检查网络能否访问 github.com |
| `npm install` 后 `sha256 mismatch` | Release 资产与 sha256 文件不一致(极少见,通常是手动改过资产) | 重新打 tag 触发 build job 重新上传 |
| `yum` job: `error: GPG_PRIVATE_KEY and GPG_KEY_ID must be set` | 三个 GPG secret 之一未配置 | 到 Settings → Secrets 补配,见 [3.2.6](#326-配置三个-gpg-secret) |
| `yum` job: `gpg: no default secret key` | `GPG_KEY_ID` 与导入的私钥不匹配 | 重新 `gpg --list-secret-keys --keyid-format=long` 核对 key id |
| `yum` job: `gpg: Bad passphrase` | `GPG_PASSPHRASE` 错误 | 重新核对口令;若密钥无口令,secret 填空字符串 |
| `yum` job: `createrepo: command not found` | CI 环境问题(应已被 apt 安装步骤覆盖) | 检查 `Install packaging tools` step 是否成功 |
| 用户 `yum install`: `GPG check FAILED` | 公钥未 import,或 `gpgkey` URL 不可达 | 确认 `https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli` 可访问;手动 `rpm --import` |
| 用户 `yum install`: `Cannot download repomd.xml` | GitHub Pages 未开启,或 `gh-pages` 分支不存在 | 确认 Settings → Pages 配置为 `gh-pages` / root;确认首个 release tag 已成功跑完 yum job |
| `gh-pages` 推送失败: `fatal: couldn't find remote ref gh-pages` | 首次发布,`gh-pages` 分支还不存在 | `peaceiris/actions-gh-pages` 会自动创建;若失败,手动 `git checkout --orphan gh-pages && git push origin gh-pages` 后再重跑 |
| GitHub Release 资产名是旧的 `nexus-cli-linux-amd64` | 用了 v1.0.0 的旧 tag 或旧代码 | 确认 `scripts/build.sh` 是新版(产出 6 平台 archive);打 `v1.1.0+` |

---

## 7. 密钥与 Token 安全维护

### 7.1 轮换 npm token

建议每 6-12 个月轮换,或怀疑泄露时立即轮换:

1. npmjs.com → Access Tokens → 删除旧 token。
2. 生成新 Automation token。
3. GitHub Settings → Secrets → 更新 `NPM_TOKEN`。
4. (可选)打一个 patch tag 验证 npm job 能正常发布。

### 7.2 轮换 GPG 密钥

GPG 密钥轮换较复杂,因为已安装旧版本的用户公钥不会自动更新。流程:

1. 按 [3.2.2](#322-生成-gpg-签名密钥) 生成新密钥。
2. 更新三个 GPG secret(`GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` /
   `GPG_KEY_ID`)。
3. 打新 tag,CI 用新密钥签名新 RPM。
4. 更新 `packaging/rpm/nexus-cli.repo` 的 `gpgkey` URL 指向新公钥
   (或保持同 URL——CI 会用新私钥导出新公钥覆盖 `gh-pages` 上的
   `RPM-GPG-KEY-nexus-cli`)。
5. **通知用户**:用户需要重新 `rpm --import` 新公钥:
   ```sh
   sudo rpm --import \
     https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli
   ```
   或者删除旧公钥再 import:
   ```sh
   sudo rpm -e $(rpm -q gpg-pubkey --qf '%{name}-%{version}-%{release}\n' | grep -i nexus)
   sudo rpm --import https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli
   ```
6. 旧 RPM 仍可用旧公钥验证(签名是历史记录,不随轮换失效);新 RPM
   必须用新公钥验证。

> **建议**:除非密钥泄露或到期,否则不轮换。GPG 密钥越稳定,用户
> 运维成本越低。

### 7.3 密钥泄露应急响应

**npm token 泄露**:

1. 立即到 npmjs.com 删除泄露的 token(撤销即失效)。
2. 生成新 token,更新 GitHub secret。
3. 检查 npm 包历史是否有异常发布(`npm view @mogesang/nexus-cli
   time`)。
4. 如有恶意版本,`npm deprecate @mogesang/nexus-cli@<ver>
   "compromised, do not use"`,并在 72h 内 `npm unpublish`
   (超过 72h 只能 deprecate)。

**GPG 私钥泄露**:

1. 立即按 [7.2](#72-轮换-gpg-密钥) 轮换新密钥并更新 secret。
2. 在 GitHub Release / README 发布安全公告,通知用户 import 新公钥。
3. 用新密钥重签所有仍需维护的历史版本 RPM(可选,工作量大;通常
   只保证新版本用新密钥即可)。

---

## 8. 进阶操作

### 8.1 本地完整复现 CI 的 yum 产物

```sh
# 前置:macOS 需要 brew install fpm createrepo rpm gpg
#      Linux: apt install ruby-dev rpm createrepo-c && gem install fpm

# 1. 构建 linux 二进制
bash scripts/build.sh v1.1.0

# 2. 提取二进制
mkdir -p /tmp/stage && cd /tmp/stage
tar -xzf /path/to/nexus-ctl/dist/nexus-cli-linux-x64.tar.gz
mv nexus-cli nexus-cli-linux-x64
tar -xzf /path/to/nexus-ctl/dist/nexus-cli-linux-arm64.tar.gz
mv nexus-cli nexus-cli-linux-arm64

# 3. 设置 GPG 环境变量(本地)
export GPG_PRIVATE_KEY="$(gpg --export-secret-keys --armor ABCD1234EFGH5678)"
export GPG_PASSPHRASE="你的口令"
export GPG_KEY_ID="ABCD1234EFGH5678"

# 4. 跑构建脚本
cd /path/to/nexus-ctl
bash scripts/build-rpm.sh v1.1.0 \
  /tmp/stage/nexus-cli-linux-x64 \
  /tmp/stage/nexus-cli-linux-arm64

# 5. 检查产物
ls -lh dist/nexus-cli-*.rpm
rpm --checksig dist/nexus-cli-1.1.0-1.el9.x86_64.rpm
```

### 8.2 给历史 RPM 重新签名(换密钥后)

```sh
# 假设旧 rpm 在 dist/yum-repo/
rpm --delsign dist/yum-repo/nexus-cli-1.0.0-1.el9.x86_64.rpm

# 用新密钥重签(需先按 3.2.6 配好 ~/.rpmmacros)
rpm --addsign dist/yum-repo/nexus-cli-1.0.0-1.el9.x86_64.rpm

# 重新生成索引
createrepo dist/yum-repo/
```

### 8.3 查看已发布的所有版本

```sh
# npm
npm view @mogesang/nexus-cli versions --json

# yum(看 gh-pages 上的 rpm 目录)
curl -s https://mogesangop.github.io/nexus-cli/rpm/ | grep -o 'nexus-cli-[^"]*\.rpm'

# GitHub Releases
gh release list -R mogesangop/nexus-cli
```

### 8.4 取消发布 / 撤回版本

**npm**:

```sh
# 72 小时内可彻底取消发布某版本
npm unpublish @mogesang/nexus-cli@1.1.0

# 超过 72 小时只能标记弃用(用户安装时会看到警告)
npm deprecate @mogesang/nexus-cli@1.1.0 "security issue, upgrade to 1.1.1"

# 如果整个包都要弃用(慎用)
npm deprecate @mogesang/nexus-cli "package deprecated, see README"
```

**yum**:yum 仓库是累积的,要撤回某版本只需从 `gh-pages` 的 `rpm/`
目录删除对应 `.rpm` 并重新 `createrepo`:

```sh
git clone --branch gh-pages https://github.com/mogesangop/nexus-cli.git site
cd site/rpm
rm nexus-cli-1.1.0-1.el9.x86_64.rpm
createrepo .
git add -A
git commit -m "remove compromised 1.1.0 rpm"
git push origin gh-pages
```

已通过 yum 安装该版本的用户不受影响(本地缓存仍在),但新安装会拿
不到该版本。

**GitHub Release**:到 Releases 页面编辑对应 release,删除资产或
将 release 标记为 "draft"(隐藏)。

---

## 9. 常见问题

**Q: 为什么 npm 包不直接包含二进制,而要用 postinstall 下载?**

A: npm 包有大小限制(单包 100MB,解压后 500MB),且每个平台一个
二进制会让包体积爆炸。postinstall 模式只下载当前平台的一个二进制
(约 6MB),包本体只有几 KB 的 JS。这是 esbuild / biome / turbo /
prisma 等工具的标准做法。

**Q: 用户在内网/离线环境,npm install 时下载不了 GitHub 二进制怎么办?**

A: 方案 A:把 GitHub Releases 镜像到内网 Nexus,改
`packaging/npm/scripts/postinstall.js` 里的 `base` URL。方案 B:
预下载二进制放到 `vendor/` 目录,`npm install --ignore-scripts`
跳过 postinstall。方案 C:改用方案 B(平台子包 +
optionalDependencies),把二进制直接打进子包发布到内网 npm
registry,完全离线。当前实现是方案 A(postinstall),如需方案 C
需重构 packaging 结构。

**Q: yum 仓库能加 Fedora 吗?**

A: 可以。当前 RPM 用 `--rpm-dist el9` 标记为 EL9,但二进制是纯
静态 Go,在任何 RHEL 系发行版上都能跑。Fedora / Rocky / Alma /
CentOS 7+ 都能直接 `yum install`。如果要在 `.repo` 文件里限定
`$releasever`,可以把 baseurl 改成
`https://mogesangop.github.io/nexus-cli/rpm/$releasever/`,但需要
CI 按发行版分别构建并放到子目录——当前为简化,所有发行版共用一
个 `rpm/` 目录。

**Q: 为什么不用 Fedora Copr 或 OBS?**

A: Copr 自动构建多发行版 + 自动签名,但它托管在
`copr.fedorainfracloud.org`,URL 不够品牌化,且主要面向 Fedora/
EPEL 生态。GitHub Pages 自托管完全自控、与发版同源、免费,适合
起步。两者可并行(Copr 做额外分发,gh-pages 做主源)。

**Q: npm 包的 `postinstall` 下载失败会不会让 `npm install` 整体失败?**

A: 会。`postinstall` 退出码非 0 时 npm 会标记该包安装失败。这是
有意为之——如果二进制装不上,`nexus-cli` 命令也跑不了,不如让
安装失败让用户立刻发现。用户可 `npm install --ignore-scripts`
跳过,但之后需手动下载二进制放到 `vendor/`。

**Q: 多个版本能在 yum 仓库里共存吗?**

A: 能。`yum-repo/rpm/` 累积所有发版的 RPM,`createrepo` 每次重
建索引时会包含全部。用户 `yum install nexus-cli` 默认装最新,
`yum install nexus-cli-1.0.0-1.el9` 可指定旧版本。

**Q: 如何让用户不验证 GPG 签名(简化内网部署)?**

A: 改 `packaging/rpm/nexus-cli.repo`,把 `gpgcheck=1` 改成
`gpgcheck=0` 并删除 `gpgkey` 行。**不推荐生产环境使用**,失去了
包完整性验证。当前默认开启签名验证。
