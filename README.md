# nexus-cli

A CLI for governing **Nexus Repository 3.76** guest / anonymous access.

The first version solves one problem: a guest (anonymous user) can see too
many repositories and artifacts in the Nexus UI. Nexus does not support
"grant browse to all repositories *except* one", so `nexus-cli` reads the
repository list, builds per-repository `repository-view` privileges, and binds
them to a guest role — granting `browse+read` to public repos and only `read`
(no `browse`) to repos that must stay hidden from the UI while remaining
downloadable via exact URL.

See `doc/nexus-cli第一版本PRD.md` for the full product spec.

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
# 1. Generate a config template (generic placeholders).
./nexus-cli config init --output config.yaml

# 2. Edit config.yaml: set baseUrl, roleName, and the readOnly / browseRead
#    repository lists. Then export the admin password:
export NEXUS_ADMIN_PASSWORD='your_password'

# 3. Verify connectivity.
./nexus-cli health check --config config.yaml

# 4. Preview the plan (no changes applied).
./nexus-cli guest sync --config config.yaml --dry-run

# 5. Apply.
./nexus-cli guest sync --config config.yaml

# 6. Verify drift.
./nexus-cli guest check --config config.yaml
```

## Commands

| Command | Description |
| --- | --- |
| `config init --output config.yaml` | Generate a config template. |
| `repo list --config config.yaml` | List all repositories (name, format, type). |
| `guest sync --config config.yaml [--dry-run] [--report FILE]` | Synchronize guest role privileges from config. |
| `guest check --config config.yaml` | Read-only check that the guest role matches config. |
| `health check --config config.yaml` | Connectivity / API / auth health check. |

## Configuration

See `examples/config.example.yaml`. Key sections:

- `nexus` — connection + credentials. `passwordEnv` names the env var holding
  the admin password (the password is never written to the file).
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

## Security

- The admin password is read from the environment, never from the config file.
- Audit logs never contain the password or `Authorization` header.
- `--dry-run` computes and prints the plan without modifying Nexus.

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
