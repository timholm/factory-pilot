# factory-pilot

Autonomous improvement agent for the Claude Code Factory pipeline. Runs a continuous diagnose-think-act loop: collects health data from every factory subsystem, sends it to Claude Opus for root-cause analysis, then executes the fixes — kubectl rollouts, code patches via Claude Code, prompt template rewrites, and failed-build retries — all without human intervention.

## Architecture

```
                  ┌─────────────────────────────────────┐
                  │          factory-pilot (Go)          │
                  │                                      │
                  │   ┌──────────┐   ┌──────────┐       │
                  │   │ diagnose │──▶│  think   │       │
                  │   │          │   │ (Claude  │       │
                  │   │ postgres │   │  Opus)   │       │
                  │   │ sqlite   │   └────┬─────┘       │
                  │   │ k8s      │        │             │
                  │   │ github   │   ┌────▼─────┐       │
                  │   │ router   │   │   act    │       │
                  │   └──────────┘   │          │       │
                  │                  │ kubectl  │       │
                  │   ┌──────────┐   │ claude   │       │
                  │   │  report  │◀──│ retry    │       │
                  │   │ (PG)     │   └──────────┘       │
                  │   └──────────┘                      │
                  │                                      │
                  │   ┌──────────┐                      │
                  │   │  api     │ :8090                │
                  │   │ /health  │                      │
                  │   │ /status  │                      │
                  │   │ /reports │                      │
                  │   │ /issues  │                      │
                  │   └──────────┘                      │
                  └─────────────────────────────────────┘
```

Each cycle (default every 6 hours) has four phases:

1. **Diagnose** — Parallel collectors query Postgres (papers, candidates), SQLite (build registry), Kubernetes (pod health), GitHub API (repo stats), and the llm-router service (request metrics).
2. **Think** — The full system status report is sent to Claude Opus via the Claude CLI. Opus returns a severity-ranked JSON array of issues with exact fix commands.
3. **Act** — The executor dispatches each fix by type: `kubectl` commands (verb-whitelisted, blocked-pattern filtered), `code` fixes (Claude Code sessions targeting a repo), `prompt` template rewrites, `retry` operations against the SQLite build registry, or `config` changes via kubectl.
4. **Report** — A structured daily report is generated, persisted to `pilot_reports` in Postgres, and printed to stdout.

## Commands

```bash
factory-pilot run                 # start the continuous improvement loop (dry-run by default)
factory-pilot run --execute       # start the loop and actually apply fixes
factory-pilot diagnose            # run one diagnosis cycle, print results, exit
factory-pilot report              # print the most recent daily report from Postgres
factory-pilot serve               # start the HTTP monitoring API on :8090
factory-pilot version             # print version
```

## Configuration

All configuration is via environment variables. Defaults are production-ready for the `factory` K8s namespace.

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_URL` | `postgres://localhost:5432/factory?sslmode=disable` | Postgres connection string (papers, candidates, reports) |
| `FACTORY_DATA_DIR` | `/data/factory` | Path to the SQLite build registry (`factory.db`) |
| `FACTORY_GIT_DIR` | `/data/repos` | Local clone root for code/prompt fixes |
| `GITHUB_TOKEN` | (none) | GitHub PAT for repo stats collection |
| `GITHUB_USER` | `timholm` | GitHub username to scan |
| `CLAUDE_BINARY` | `claude` | Path to the Claude CLI binary |
| `K8S_NAMESPACE` | `factory` | Kubernetes namespace to monitor and act on |
| `CYCLE_INTERVAL` | `6h` | Duration between improvement cycles |
| `MAX_FIXES_PER_CYCLE` | `10` | Max issues to fix per cycle |
| `API_PORT` | `8090` | HTTP API listen port |

## Safety

- **Dry-run by default.** Fixes are only logged, never applied, unless `--execute` is passed.
- **kubectl verb whitelist.** Only `get`, `describe`, `logs`, `rollout`, `scale`, `patch`, `apply`, `set`, `annotate`, `label` are allowed. No `delete`, `exec`, `--force`, or `--grace-period=0`.
- **Namespace-locked.** All kubectl commands are pinned to the configured namespace. Cross-namespace operations are blocked.
- **Max fixes cap.** Each cycle stops after `MAX_FIXES_PER_CYCLE` fixes.
- **Full audit trail.** Every cycle's issues, actions, and outcomes are persisted to Postgres.

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness probe — returns `{"status":"ok"}` |
| `GET` | `/status` | Live system status snapshot (runs all collectors) |
| `GET` | `/reports` | Last 30 daily reports |
| `GET` | `/reports/{date}` | Report for a specific date (YYYY-MM-DD) |
| `GET` | `/issues` | Issues from the most recent report |

## Build

```bash
make build        # compile binary
make test         # run all tests
make lint         # golangci-lint
make docker       # build Docker image
make deploy       # kubectl apply -f deploy/deployment.yaml
```

## Deploy

The K8s manifests in `deploy/deployment.yaml` include the Deployment, Service, ServiceAccount, Role, and RoleBinding. The pod mounts a PVC at `/data` for the SQLite build registry and repo clones. Secrets (`factory-secrets`) must provide `postgres-url` and `github-token`.

```bash
kubectl create secret generic factory-secrets -n factory \
  --from-literal=postgres-url='postgres://user:pass@pg:5432/factory?sslmode=disable' \
  --from-literal=github-token='ghp_xxx'

kubectl apply -f deploy/deployment.yaml
```

## Dependencies

- Go 1.23+
- `kubectl` in PATH (for K8s diagnosis and fixes)
- Claude CLI (`@anthropic-ai/claude-code`) in PATH (for think and code/prompt fixes)
- Postgres (papers, candidates, reports)
- SQLite (build registry at `$FACTORY_DATA_DIR/factory.db`)
- llm-router service in-cluster (`http://llm-router.factory.svc.cluster.local:8080`)

## License

Proprietary. Internal use only.
