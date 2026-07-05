# 构建与发布

nexus-cli 是纯 Go(CGO 关闭),交叉编译只需设置 `GOOS`/`GOARCH`,无需 C 交叉工具链。本文档说明本地构建和 GitHub Actions 自动发布流程。

## 本地构建

```sh
bash scripts/build.sh [version]
```

- 不传 `version` 时,从 `git describe --tags` 推导;无 tag 则为 `dev`。
- 构建前自动跑 `go vet ./...` 和 `go test ./...`,失败则中止。
- 产物输出到 `dist/`:

| 文件 | 说明 |
|---|---|
| `nexus-cli-linux-amd64` | Linux x86-64 二进制 |
| `nexus-cli-linux-arm64` | Linux aarch64 二进制 |
| `nexus-cli-linux-amd64.sha256` | 对应 sha256 校验文件 |
| `nexus-cli-linux-arm64.sha256` | 对应 sha256 校验文件 |

也可用 `make dist` 触发同一脚本。

校验产物:

```sh
cd dist && sha256sum -c nexus-cli-linux-amd64.sha256
file nexus-cli-linux-amd64   # ELF 64-bit LSB executable, x86-64
file nexus-cli-linux-arm64   # ELF 64-bit LSB executable, aarch64
```

## CI 自动发布(GitHub Actions)

流水线定义在 `.github/workflows/release.yml`。**打 `v` 开头的 tag** 即触发:

```sh
git tag v1.0.0
git push origin v1.0.0
```

流程:checkout → setup-go(按 go.mod 选版本)→ 跑 build.sh(含 vet+test)→ 把两个二进制及 sha256 上传到该 tag 的 GitHub Release，并自动生成 release notes。

arm64 在 x64 runner 上通过纯 Go 交叉编译,无需 QEMU。日常 push 和 pull request 由 `.github/workflows/ci.yml` 执行 race test、覆盖率、vet 和 build，覆盖率文件作为 Actions artifact 保存。

发布后到仓库的 **Releases** 页面下载,附件共 4 个(2 二进制 + 2 sha256)。

## GOPROXY 说明

构建默认用 `GOPROXY=https://goproxy.cn,direct`(与本机/CLAUDE.md 约定一致)。脚本和 workflow 都允许环境变量 override:

- 本地:`GOPROXY=https://proxy.golang.org,direct bash scripts/build.sh`
- CI:在 workflow 的 `env` 块改 `GOPROXY`(GitHub runner 在境外,若 goproxy.cn 不稳,换成 `https://proxy.golang.org,direct`)。

## 新增目标架构/OS

当前只出 Linux amd64/arm64。如需扩展:

1. `scripts/build.sh` 的 `for ARCH in amd64 arm64` 循环里加架构;若要换 OS,把 `GOOS=linux` 参数化。
2. `.github/workflows/release.yml` 的 `matrix.goarch` 加架构;若加 macOS/Windows,文件名里的 `linux` 也要相应调整(脚本和 workflow 都要改)。

## 版本号注入(可选,当前未启用)

`scripts/build.sh` 已用 `-ldflags "-X main.version=<ver>"` 注入版本号,但 `cmd/nexus-cli/main.go` 当前**没有** `version` 变量,链接器会安全忽略未定义的 `-X`,不影响构建。

如需 `nexus-cli --version` 显示版本,后续在 `cmd/nexus-cli/main.go` 加 `var version = "dev"`,并在 `internal/cli/root.go` 的根命令加 `--version` flag 打印它。届时 build.sh 无需改动即自动生效。
