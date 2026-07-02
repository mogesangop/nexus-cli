# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

nexus-cli is a Go CLI that governs Nexus Repository 3.76 guest/anonymous
access. The core problem it solves: Nexus has no "grant browse to all repos
*except* X" permission, so this CLI reads the repo list, builds per-repo
`repository-view` privileges, and binds them to a guest role — `browse+read`
for public repos, `read`-only (UI-hidden but URL-downloadable) for protected
repos. Full spec: `doc/nexus-cli第一版本PRD.md`.

## Build, test, run

```sh
make build        # -> ./nexus-cli  (CGO disabled, pure Go)
make test         # go test ./...   (no network; pure-logic unit tests)
make vet          # go vet ./...
make run-help     # build + ./nexus-cli --help
```

All `make` targets inject `GOPROXY=https://goproxy.cn,direct` and
`CGO_ENABLED=0`. CGO is off because this machine has no Xcode command-line
tools; do not re-enable it. If `goproxy.cn` is unreachable, override with
`make build GOPROXY=https://proxy.golang.org,direct`.

Run one test: `CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct go test ./internal/guest/ -run TestPolicyFor -v`

## Architecture

Entry point `cmd/nexus-cli/main.go` is a thin shim that calls
`cli.NewRoot()`. All logic lives under `internal/`:

- `cli/` — cobra commands. Each command reads `--config`, builds a Nexus
  client, delegates to a guest/nexus package. `guest.go` owns audit logging.
- `config/` — YAML model, `Load`, `Validate`, `Default` (template for
  `config init`), `Marshal`, and `Password()` (resolves the admin password
  from the env var named by `nexus.passwordEnv`).
- `nexus/` — REST client (`client.go`) + `repositories.go`, `privileges.go`,
  `roles.go`. One `Client` type, Basic Auth, typed `APIError` with
  `IsNotFound` helper.
- `guest/` — the engine. `planner.go` (pure: policy → target permissions →
  `SyncPlan`), `syncer.go` (applies plan idempotently, dry-run aware),
  `checker.go` (read-only drift check).
- `naming/` — privilege name generation + `IsManaged` prefix check.
- `audit/` — JSONL audit logger. Records never carry the password.
- `report/` — console + file rendering for sync/check reports.

### Three invariants that shape the design

1. **Policy precedence: `deny > readOnly > browseRead > defaultPolicy`.**
   Implemented in `guest/planner.go` `PolicyFor`. Changing this order changes
   security behavior — never reorder casually.

2. **Managed-privilege boundary.** `nexus-cli` only owns privileges whose name
   starts with `priv_guest_` (see `naming.Generator.IsManaged`). During sync,
   non-managed privileges on the guest role are preserved **except** those in
   `forbiddenPrivileges`, which are removed regardless of management status.
   This is an intentional, user-approved deviation from a strict "managed-only"
   rule — do not "fix" it by skipping non-managed forbidden privileges.

3. **Password never in config, never in logs.** `config.Password()` reads
   `os.Getenv(passwordEnv)`. The audit `Record` type has no password field by
   construction. When adding fields to `audit.Record` or any new log path,
   keep this property.

### Idempotency

`guest sync` must be idempotent (PRD §14): an unchanged second run creates
nothing and removes nothing. The flow in `syncer.apply` is read-state → diff
against plan → create missing, skip existing, remove stale-managed +
forbidden. Privilege name stability (actions deduped + sorted in
`naming.PrivilegeName`) is what makes "same logical permission" map to the
same name across runs — preserve that normalization.

### Nexus API caveat

REST field names in `nexus/privileges.go` and `nexus/roles.go` follow the
standard Nexus 3.76 `/service/rest/v1` endpoints but are marked with `NOTE`
comments because minor versions differ. Verify against the target instance's
Swagger (UI → Settings → System → API) before trusting create/update calls.
GET/list calls are stable across versions.

## Working style (existing project conventions)

- State assumptions before implementing; ask when uncertain rather than
  guessing silently.
- Minimum code that solves the problem. No speculative abstractions, no
  configurability that wasn't requested, no error handling for impossible
  cases.
- Surgical changes: touch only what the task requires. Don't refactor
  adjacent code or "improve" formatting. Match existing style.
- Prefer editing existing files over creating new ones.
- Default to no comments; add one only when the *why* is non-obvious
  (e.g. the managed-privilege deviation above).
