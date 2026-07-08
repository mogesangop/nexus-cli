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

A fourth use case supports **warm-standby HA operations** for Nexus OSS: two
independent Nexus nodes, one active F5 upstream, periodic blob / metadata
replication, manual fencing, and guided failover. The CLI does not automate F5
or claim zero RPO; it provides dual-node health/status, one-shot sync command
execution, fencing gates, and audit records. See
`doc/nexus主从HA模式PRD.md`.

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

The yum repo is a static tree served from this project's GitHub Pages and
is rebuilt on every release tag. It provides x86_64 and aarch64 packages;
all RPMs are GPG-signed.

#### dnf (Fedora, RHEL 8+, Rocky, Alma)

```sh
# Add the repository configuration and import its signing key.
sudo curl -o /etc/yum.repos.d/nexus-cli.repo \
  https://mogesangop.github.io/nexus-cli/nexus-cli.repo
sudo rpm --import https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli

# Install and verify.
sudo dnf install nexus-cli
nexus-cli --help
rpm -q nexus-cli
```

#### yum (RHEL 7, CentOS 7)

```sh
# Add the repository configuration and import its signing key.
sudo curl -o /etc/yum.repos.d/nexus-cli.repo \
  https://mogesangop.github.io/nexus-cli/nexus-cli.repo
sudo rpm --import https://mogesangop.github.io/nexus-cli/RPM-GPG-KEY-nexus-cli

# Install and verify.
sudo yum install nexus-cli
nexus-cli --help
rpm -q nexus-cli
```

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
| `repo list [--format F] [--type T]` | List repositories, optionally filtered by format/type. |
| `repo get --name R --format F --type T` | Show one repository's full API payload. |
| `repo apply [--dry-run]` | Apply generic repositories declared in `repositories.managed`. |
| `repo ensure --name R --format F --type T --settings FILE [--dry-run]` | Create or update one generic repository from YAML/JSON settings. |
| `repo raw apply [--dry-run]` | Apply declared raw hosted repositories. |
| `repo raw ensure --name R --blob-store B [...]` | Create or safely update one raw hosted repository. |
| `repo lifecycle preview --repo R [...]` | Read-only preview of expired raw components. |
| `repo lifecycle run --repo R --yes [...]` | Delete expired raw components. |
| `blobstore list` | List blob stores. |
| `blobstore get --name B --type file` | Show one file blob store. |
| `blobstore apply [--dry-run]` | Apply file blob stores declared in `blobStores.file`. |
| `blobstore ensure --name B --path P [...]` | Create or update one file blob store. |
| `guest sync [--dry-run] [--report FILE]` | Synchronize guest role privileges from config. |
| `guest check` | Read-only check that the guest role matches config. |
| `share grant --repo R --path /p/ --user U --email E` | Create a path-scoped browse+read grant for a named user. |
| `health check` | Connectivity / API / auth health check. |
| `ha status` | Show both HA node health plus last blob / metadata sync time and lag. |
| `ha health` | Run API health checks against both HA nodes. |
| `ha sync --once [--timeout 30m]` | Execute configured blob and metadata sync commands once and update HA state. |
| `ha failover --from primary --to standby --fencing-confirmed` | Guide a safe manual failover, optionally run catch-up sync, print F5 steps, and write audit. |

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
- `repositories.managed` — generic repository desired state for any
  format/type. `settings` is passed through to the Nexus repository API body.
- `blobStores.file` — desired file blob stores.
- `ha` — optional warm-standby settings: node pair, replication commands,
  state file, and manual failover safety gates.
- `guestAccess` — target role, repository policies, forbidden/warn privileges.
- `privilegeNaming` — prefix (`priv_guest`), separator, dash replacement.
- `audit` — JSONL audit log path and masking.
- `report` — report directory and format (`text` | `json`).

## Warm-standby HA usage

The HA mode follows the product constraint in
`doc/nexus主从HA模式PRD.md`: Nexus Repository OSS has no native active-active or
primary/standby replication. This CLI therefore implements an operator-guided
warm standby workflow, not synchronous replication.

### 1. Configure two nodes and sync commands

Add an enabled `ha` section to `config.yaml`. Passwords are still read only from
environment variables.

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

`blobSync.command` and `metadataSync.command` are local operator-owned commands
or scripts. They should be idempotent and return non-zero on failure. A typical
metadata script wraps the Nexus Export database task, transfers the completed
export package, then triggers Import database on the standby node.

Export the node passwords before running HA commands:

```sh
export NEXUS_PRIMARY_PASSWORD='primary_admin_password'
export NEXUS_STANDBY_PASSWORD='standby_admin_password'
```

### 2. Check both nodes

```sh
nexus-cli ha health --config config.yaml
nexus-cli ha status --config config.yaml
```

`ha health` checks repository, privilege, and guest-role API access on both
nodes. `ha status` also reads `ha.replication.stateFile` to show the last
successful blob and metadata sync timestamps, lag, and last error.

### 3. Run one catch-up sync

```sh
nexus-cli ha sync --once --config config.yaml --timeout 45m
```

The command runs `blobSync.command` first, then `metadataSync.command`. It stops
after the first failure and writes the result to the HA state file. Empty sync
commands fail fast with a message telling you which config field to fill.

For scheduled replication, put the actual sync script in cron/systemd timer, or
call `nexus-cli ha sync --once` from the scheduler after the commands are
configured.

### 4. Manual failover

When the primary fails:

```sh
# First stop or isolate the old primary so there is no split-brain write path.
# Then run:
nexus-cli ha failover \
  --config config.yaml \
  --from primary \
  --to standby \
  --fencing-confirmed
```

By default `ha failover` runs a final catch-up sync before printing the F5
switch checklist. If the old primary is hard down and sync cannot run, use
`--skip-sync` only after accepting the RPO gap:

```sh
nexus-cli ha failover \
  --config config.yaml \
  --from primary \
  --to standby \
  --fencing-confirmed \
  --skip-sync
```

After switching F5 so the standby is the only active pool member, verify:

```sh
nexus-cli ha status --config config.yaml
nexus-cli guest check --config config.yaml
```

Every `ha sync --once` and `ha failover` attempt writes a JSONL audit record
through the existing audit logger. The record never includes passwords or
authorization headers.

### Policy precedence (per repository)

```
deny > readOnly > browseRead > defaultPolicy
```

A repository in `deny.repositories` gets no privilege. In `readOnly` it gets
`read` only (hidden from UI, still downloadable). Matching `browseRead` (and
not excluded) gets `browse+read`. Otherwise `defaultPolicy` decides.

### Protected repository: hidden in UI, downloadable by exact URL

A protected repository keeps anonymous `read` access but does not grant
`browse`. Nexus UI repository listing and tree browsing depend on `browse`;
exact URL downloads depend on `read`. Configure it in two places:

1. Add the repository to `browseRead.excludeRepositories` so it does not get
   `browse+read`.
2. Add the same repository to `readOnly.repositories` so it still gets `read`.

Example for `devops-prod-generic`:

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
```

Before running, make sure `role_guest_repository_access` exists in Nexus and
the anonymous user (`anonymous`) has that role. Then apply and check:

```sh
export NEXUS_ADMIN_PASSWORD='your_password'
./nexus-cli guest sync --config config.yaml --dry-run
./nexus-cli guest sync --config config.yaml
./nexus-cli guest check --config config.yaml
```

Verify the behavior:

```sh
# 1. Open the Nexus UI as anonymous / logged out. devops-prod-generic should
#    not appear in the repository list.

# 2. Exact artifact URLs should still download. Use a real artifact path.
curl -fL \
  'http://nexus.example.com/repository/devops-prod-generic/path/to/artifact.tar' \
  -o /tmp/artifact.tar
```

If direct download fails, check that the repository is not listed in
`deny.repositories`, and that the anonymous user has the configured guest role.
If the repository is still visible in the UI, the anonymous user probably has
another role or privilege that grants `browse`. `guest sync` removes broad
entries listed in `forbiddenPrivileges`, but it does not delete every
non-managed role.

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

`repo apply` and `blobstore apply` are also idempotent. For generic repositories,
the CLI compares the declared `settings` fields against the live API payload and
allows extra read-only fields returned by Nexus. A repository with the same name
but a different format/type fails instead of being migrated.

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
