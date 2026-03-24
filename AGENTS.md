# AGENTS.md — Agent coordination for factory-pilot

## Overview

factory-pilot is itself an AI agent. It uses Claude Opus as its reasoning engine and Claude Code as its code-editing tool. This document describes how agents (including factory-pilot itself) should interact with this codebase and its runtime behavior.

## Agent roles

### factory-pilot (this agent)

- **Purpose:** Autonomous operations manager for the Claude Code Factory pipeline.
- **Reasoning model:** Claude Opus via CLI (`claude -p ... --model opus --max-turns 3`).
- **Actions:** kubectl commands, Claude Code sessions for code/prompt fixes, SQLite build retries.
- **Cycle:** Every 6 hours by default. Diagnose, think, act, report.
- **Safety:** Dry-run by default. Verb-whitelisted kubectl. Namespace-locked. Max 10 fixes per cycle.

### Claude Code (tool agent)

- **Invoked by:** factory-pilot's `act/claude.go` module.
- **Purpose:** Apply code fixes and prompt template rewrites to factory repos.
- **Working directory:** Set to the target repo under `$FACTORY_GIT_DIR`.
- **Max turns:** 5 per fix session.
- **Expected behavior:** Make minimal changes, run tests, commit.
- **Code fixes:** Format is `repo:owner/name instruction text`. Without prefix, defaults to `claude-code-factory`.
- **Prompt fixes:** Always target the `claude-code-factory` repo.

### Coding agents working on this repo

When making changes to factory-pilot itself:

1. **Read CLAUDE.md first.** It contains the design constraints.
2. **Do not weaken safety mechanisms.** The kubectl whitelist, blocked patterns, dry-run default, and namespace lock exist for production safety.
3. **Preserve the diagnose-think-act-report loop structure.** New data sources go in `internal/diagnose/`. New fix types go in `internal/act/`. New analysis logic goes in `internal/think/`.
4. **Test with `make test`.** All changes must pass.
5. **Keep it backend-only.** No UI, no frontend, no HTML templates.

## Data flow between agents

```
┌──────────────┐     status report (text)     ┌─────────────┐
│   diagnose   │ ──────────────────────────▶  │  Claude Opus │
│  collectors  │                               │  (thinker)   │
└──────────────┘                               └──────┬──────┘
                                                      │
                                               JSON issues[]
                                                      │
                                               ┌──────▼──────┐
                                               │   executor   │
                                               │              │
                                     ┌─────────┼──────────────┼─────────┐
                                     │         │              │         │
                                  kubectl   claude code    retry     config
                                  runner      runner       runner    (kubectl)
                                     │         │              │         │
                                     ▼         ▼              ▼         ▼
                                   K8s      Git repos     SQLite     K8s
                                   pods     (code fix)    (builds)   manifests
```

## Issue format (think -> act contract)

The thinker returns a JSON array that the executor consumes. Each issue must conform to:

```json
{
  "issue": "brief title",
  "severity": "critical|high|medium|low",
  "root_cause": "why this is happening",
  "fix_type": "kubectl|code|prompt|retry|config",
  "fix_commands": ["exact command 1", "exact command 2"],
  "expected_outcome": "what should change after fix"
}
```

- `fix_type` determines which runner handles the commands.
- `kubectl` commands must start with `kubectl` and use only whitelisted verbs.
- `code` commands use format `repo:owner/name instruction text`.
- `retry` commands use format `retry build <id>`, `retry all-failed`, or `retry recent <n>`.
- `prompt` commands are free-text instructions sent to Claude Code targeting `claude-code-factory`.
- `config` commands are routed through the kubectl runner.

## Integration points

| System | Protocol | Purpose |
|---|---|---|
| Postgres | `pgx` TCP | Read papers/candidates, write pilot_reports |
| SQLite | `database/sql` file | Read/write build registry (`factory.db`) |
| Kubernetes | `kubectl` CLI | Pod health diagnosis, fix execution |
| GitHub API | `go-github` HTTP | Repo stats (stars, forks, count) |
| llm-router | HTTP `/stats` | Request volume, error rate, latency |
| Claude CLI | subprocess | Opus analysis, Code fix sessions |

## Adding new collectors

1. Create `internal/diagnose/newcollector.go`.
2. Define a struct with a `Collect*(ctx context.Context)` method returning `(YourStats, error)`.
3. Add a stats type to `collector.go` and a field to `SystemStatus`.
4. Add the collector field to the `Collector` struct, initialize it in `NewCollector`.
5. Add a goroutine in `Collect()` following the existing pattern (mutex-protected, error-captured via `addError`).
6. Add the new stats to `FormatReport()` so the thinker sees them.

## Adding new fix types

1. Create a runner in `internal/act/newrunner.go` with a `Run(command string) (string, error)` method.
2. Add the runner field to the `Executor` struct in `executor.go`, initialize it in `NewExecutor`.
3. Add a `case` in `executeOne()` for the new fix type.
4. Update the `diagnosisPrompt` in `internal/think/thinker.go` to include the new fix type category so Opus knows it can use it.
