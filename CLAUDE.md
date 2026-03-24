# CLAUDE.md — Instructions for AI agents working on factory-pilot

## What this project is

factory-pilot is an autonomous improvement agent for the Claude Code Factory pipeline. It runs a continuous loop: diagnose system health, send the status to Claude Opus for analysis, execute the recommended fixes, and persist a daily report. Written in Go, deployed as a single pod in the `factory` K8s namespace.

## Repository structure

```
main.go                         Entry point, CLI commands, improvement loop
internal/
  config/config.go              Environment-based configuration
  diagnose/
    collector.go                Parallel system status collection (orchestrator)
    postgres.go                 Paper and candidate stats from Postgres
    sqlite.go                   Build pipeline stats from SQLite
    k8s.go                      Pod health via kubectl
    github.go                   Repo metrics via GitHub API
    router.go                   LLM router stats via HTTP
  think/
    types.go                    Issue struct, severity ordering
    thinker.go                  Claude Opus analysis (sends status, parses JSON issues)
  act/
    executor.go                 Fix dispatcher (routes by fix_type)
    kubectl.go                  Safe kubectl runner (verb whitelist, blocked patterns)
    claude.go                   Claude Code CLI runner (code fixes, prompt fixes)
    retry.go                    SQLite build retry operations
  report/
    report.go                   Report generation, printing, persistence orchestration
    db.go                       Postgres CRUD for pilot_reports table, DailyReport type
  api/
    server.go                   HTTP monitoring API (/health, /status, /reports, /issues)
deploy/
  deployment.yaml               K8s Deployment, Service, ServiceAccount, Role, RoleBinding
Dockerfile                      Multi-stage build (golang:1.23-alpine -> alpine:3.20)
Makefile                        build, test, lint, docker, deploy targets
```

## Build and test

```bash
make build       # produces ./factory-pilot binary
make test        # go test ./... -v -count=1
make lint        # golangci-lint run ./...
```

## Key design decisions

- **Dry-run by default.** The `DryRun` field in Config is always true unless `--execute` is explicitly passed. Never change this default.
- **kubectl safety.** The verb whitelist in `act/kubectl.go` is intentionally restrictive. Do not add `delete`, `exec`, or `create` to `allowedKubectlVerbs`. Do not remove any entry from `blockedPatterns`.
- **Parallel collection.** All diagnose collectors run concurrently via goroutines with `sync.WaitGroup`. Individual collector failures are captured in `status.Errors` and do not abort the cycle.
- **Claude CLI integration.** The thinker calls the Claude CLI binary (`claude -p ... --model opus --max-turns 3`). The code/prompt fix runners also shell out to the Claude CLI with `--max-turns 5`. Do not replace this with a direct API call — the CLI handles auth, context, and tool use.
- **Single-namespace.** The K8s RBAC Role is scoped to `factory` namespace only. Do not request cluster-wide permissions.
- **Report persistence.** Daily reports are upserted to `pilot_reports` table in Postgres (keyed by date). The API reads from this table.

## Coding conventions

- Go 1.23, standard library where possible.
- No frameworks. HTTP routing uses Go 1.22+ `ServeMux` pattern matching (`"GET /path"`).
- All exported types and functions have doc comments.
- Errors are wrapped with `fmt.Errorf("context: %w", err)`.
- Logging uses `log.Printf` with bracketed component tags: `[pilot]`, `[cycle]`, `[executor]`, `[api]`, `[serve]`.
- JSON field tags on all struct fields that get serialized.
- No global state beyond `var version`.
- Direct database access via `pgx` (Postgres) and `database/sql` (SQLite). No ORM.

## What NOT to do

- Do not add a web UI or dashboard. This is a backend-only agent.
- Do not introduce external config files (YAML, TOML). All config is env vars.
- Do not add ORM layers. Direct `pgx` and `database/sql` queries only.
- Do not remove the dry-run default. Production safety depends on it.
- Do not add `delete`, `exec`, or `--force` to the kubectl whitelist.
- Do not replace Claude CLI calls with raw HTTP API calls.
- Do not add cluster-wide RBAC. Stay namespace-scoped.
