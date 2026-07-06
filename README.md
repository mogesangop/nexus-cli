# nexus-cli

**English** | [中文](README.zh.md)

[![CI](https://github.com/231397220/nexus-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/231397220/nexus-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A CLI for governing **Nexus Repository 3.76** guest / anonymous access.

The first version solves one problem: a guest (anonymous user) can see too
many repositories and artifacts in the Nexus UI. Nexus does not support
"grant browse to all repositories *except* one", so `nexus-cli` reads the
repository list, builds per-repository `repository-view` privileges, and binds
them to a guest role — granting `browse+read` to public repos and only `read`
(no `browse`) to repos that must stay hidden from the UI while remaining
downloadable via exact URL.

See `doc/nexus-cli第一版本PRD.md` for the full product spec.

A second use case is **per-user path-scoped sharing**: `share grant` creates a
content selector, a path-scoped `browse+read` privilege, a role, and a user so
a named person can browse/download artifacts under one directory of one repo —
without exposing anything else. Share resources use a separate `priv_share_`
prefix and their own `role_share_*` roles, so they are invisible to the guest
subsystem and vice versa.

A third use case manages `raw/hosted` repositories and artifact retention on
Nexus Community/OSS. The CLI safely reconciles repository settings and can
preview or delete old files using last-modified age and path rules. See
`doc/raw仓库与制品生命周期PRD.md`.

## Install

Prebuilt binaries are published with each release. Pick whichever channel
fits your environment.

### npm (cross-platform)

```sh
# Install globally.
npm i -g @mogesang/nexus-cli
nexus-cli --help

# Or run it once without a global install.
npx @mogesang/nexus-cli --help
```

Supported: linux / macOS / Windows on x64 / arm64. The package is a thin
wrapper whose `postinstall` downloads the matching binary from GitHub
Releases and verifies its sha256.

### RPM via yum / dnf (RHEL, CentOS, Rocky, Alma, Fedora)

```sh
# Add the repository configuration and import its signing key.
sudo curl -o /etc/yum.repos.d/nexus-cli.repo \
  https://mogesangop.github.io/nexus-cli/nexus-cli.repo
sudo rpm --import https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli

# Install and verify.
sudo dnf install nexus-cli   # use `yum install nexus-cli` on yum-based systems
nexus-cli --help
rpm -q nexus-cli
```

The yum repo is a static tree served from this project's GitHub Pages and
is rebuilt on every release tag. It provides x86_64 and aarch64 packages;
all RPMs are GPG-signed.

### Direct download

Grab the archive for your platform from the
[latest release](https://github.com/mogesangop/nexus-cli/releases/latest),
extract it, and put `nexus-cli` on your `PATH`.

## Build

```sh
make build          # produces ./nexus-cli
# or directly:
CGO_ENABLED=0 go build -o nexus-cli ./cmd/nexus-cli
```

> The default `GOPROXY` in the Makefile is `https://goproxy.cn,direct`. Override
> with `make build GOPROXY=https://proxy.golang.org,direct` if needed.

## Quick start

```sh
# 1. Generate a config template. Without --output it lands at
#    ~/.nexus-cli/config.yaml (dir created with 0700 if missing).
./nexus-cli config init

# 2. Edit the config: set baseUrl, roleName, and the readOnly / browseRead
#    repository lists. Then export the admin password:
export NEXUS_ADMIN_PASSWORD='your_password'

# 3. Verify connectivity. --config is optional; if unset the CLI searches
#    ./config.yaml, ~/.nexus-cli/config.yaml, /etc/nexus-cli/config.yaml
#    (first match wins).
./nexus-cli health check

# 4. Preview the plan (no changes applied).
./nexus-cli guest sync --dry-run

# 5. Apply.
./nexus-cli guest sync

# 6. Verify drift.
./nexus-cli guest check
```

### Grant a user path-scoped access to a repo

```sh
# Dry-run first: prints the selector/privilege/role/user that would be created.
./nexus-cli share grant \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice --last-name Team \
  --dry-run

# Apply. The generated password is printed ONCE to stdout — save it now.
./nexus-cli share grant \
  --repo devops-prod-generic \
  --path /team-a/ \
  --user alice.team-a \
  --email alice@example.com \
  --first-name Alice --last-name Team
```

The grant is idempotent: re-running with the same args reuses the existing
selector, privilege, and role. An existing user is an **error** — the password
is never reset. Partial progress is not rolled back, so re-running is safe.

## Commands

All commands accept an optional `--config <path>`. When omitted (or `--config ""`),
the CLI searches `./config.yaml`, `~/.nexus-cli/config.yaml`, then
`/etc/nexus-cli/config.yaml` — first existing file wins. An explicit `--config`
is used verbatim (no search; a typo surfaces as a read error).

| Command | Description |
| --- | --- |
| `config init [--output config.yaml]` | Generate a config template (default: `~/.nexus-cli/config.yaml`). |
| `repo list` | List all repositories (name, format, type). |
| `repo raw apply [--dry-run]` | Apply declared raw hosted repositories. |
| `repo raw ensure --name R --blob-store B [...]` | Create or safely update one raw hosted repository. |
| `repo lifecycle preview --repo R [...]` | Read-only preview of expired raw components. |
| `repo lifecycle run --repo R --yes [...]` | Delete expired raw components. |
| `guest sync [--dry-run] [--report FILE]` | Synchronize guest role privileges from config. |
| `guest check` | Read-only check that the guest role matches config. |
| `share grant --repo R --path /p/ --user U --email E` | Create a path-scoped browse+read grant for a named user. |
| `health check` | Connectivity / API / auth health check. |

### `share grant` flags

| Flag | Required | Description |
| --- | --- | --- |
| `--repo` | yes | Repository name. |
| `--path` | yes | Directory path, must start with `/`, e.g. `/team-a/`. |
| `--user` | yes | User id to create. Must not already exist. |
| `--email` | yes | User email address. |
| `--first-name` / `--last-name` | no | User display name parts. |
| `--format` | no | Repository format; auto-detected from `repo list` if omitted. |
| `--password-length` | no | Generated password length (default 24). |
| `--dry-run` | no | Print the plan without creating anything or generating a password. |

## Configuration

See `examples/config.example.yaml`. Key sections:

- `nexus` — connection + credentials. `passwordEnv` names the env var holding
  the admin password (the password is never written to the file).
- `repositories.raw` — desired raw hosted repositories and CLI retention rules.
- `guestAccess` — target role, repository policies, forbidden/warn privileges.
- `privilegeNaming` — prefix (`priv_guest`), separator, dash replacement.
- `audit` — JSONL audit log path and masking.
- `report` — report directory and format (`text` | `json`).

### Policy precedence (per repository)

```
deny > readOnly > browseRead > defaultPolicy
```

A repository in `deny.repositories` gets no privilege. In `readOnly` it gets
`read` only (hidden from UI, still downloadable). Matching `browseRead` (and
not excluded) gets `browse+read`. Otherwise `defaultPolicy` decides.

### Privilege naming

`priv_guest_{format}_{sanitizedRepo}_{sortedActions}` — e.g.
`priv_guest_raw_devops_prod_generic_read`. Dashes, dots and slashes in the
repo name are replaced with `_`.

### Managed privileges

`nexus-cli` only manages privileges whose name starts with `priv_guest_`.
Privileges on the role that are **not** managed are preserved — **except**
those listed in `forbiddenPrivileges` (e.g. `nx-all`, `nx-admin`,
`nx-repository-view-*-*-browse`), which are always removed from the guest
role during `sync`. `warnPrivileges` (e.g. `nx-search-read`) are flagged in
`guest check` but not removed by default.

## Idempotency

`guest sync` is idempotent: a second run with unchanged state creates nothing
and removes nothing. Existing managed privileges that match the config are
skipped; stale managed privileges are removed.

`repo raw apply` is also idempotent and never migrates a blob store or
delete/recreates a conflicting repository. Preview changes and retention first:

```sh
./nexus-cli repo raw apply --dry-run
./nexus-cli repo lifecycle preview --repo devops-prod-generic
./nexus-cli repo lifecycle run --repo devops-prod-generic --yes
```

The lifecycle run can be scheduled with cron. Deleting a Nexus component does
not immediately reclaim disk space; Nexus blob-store compaction is still
required.

## Security

- The admin password is read from the environment, never from the config file.
- Audit logs never contain the password or `Authorization` header.
- `--dry-run` computes and prints the plan without modifying Nexus.
- Lifecycle deletion requires explicit `--yes`; excluded paths always win.

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| 401 | Wrong admin password | Check `NEXUS_ADMIN_PASSWORD`. |
| 403 | Account lacks security-management privileges | Use an admin-level account. |
| 404 on a privilege/role endpoint | API path differs in this Nexus minor version | Verify against Nexus UI → Settings → System → API (Swagger). |
| TLS error | Self-signed cert | Set `insecureSkipTLSVerify: true` or add your CA. |
| Timeout | Slow network / large repo list | Increase `nexus.timeoutSeconds`. |

> **API field accuracy:** The REST request/response field names used by this
> CLI follow the standard Nexus 3.76 `/service/rest/v1` endpoints. Different
> minor versions may emit different fields; verify against your target
> instance's Swagger before production use.

## Tests

```sh
make test    # unit tests (naming, planner, config) — no network needed
make vet     # go vet
```

> Maintainer release & distribution setup (npm token, GPG key, GitHub Pages)
> is documented in [`doc/publishing.md`](doc/publishing.md).
