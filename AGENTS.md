# AGENTS.md - bluetape-go

This repository inherits the workspace guidance from `../AGENTS.md`.
Read and follow the workspace root guide first. This file only adds
Go-specific layout, commands, domain rules, and local exceptions.

Go backend utilities and distributed infrastructure packages for the bluetape
ecosystem. This is an idiomatic Go library line, not a mechanical Kotlin port.

## Skills

- Use `bluetape4k-workflow` for task classification and issue/PR discipline.
- Use `bluetape-go-patterns` for Go implementation, tests, concurrency,
  package design, and review.

## Commands

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
go test ./...
```

## Rules

- Prefer small first-party Go packages over wrappers around Kotlin/JVM shapes.
- Use `context.Context` for cancellation, deadlines, and request-scoped work.
- Keep exported APIs documented with Go doc comments.
- Do not add dependencies when a small idiomatic standard-library
  implementation is clearer.
- Run Testcontainers-backed packages sequentially when Docker resources or
  ports are shared.
- Keep package README files and `README.ko.md` in sync when public package
  behavior changes.

## 7-Tier Review Gate

- Step 2-R, Step 3-R, Step 6-R, and Step 7-R use the same 7-Tier shape:
  six independent lanes plus one main-session integration review.
- The six lanes are performance, stability, security, operator/Ops,
  developer/API, and user/caller. Do not spawn a seventh integration subagent;
  the main Codex session owns integration and the final P0/P1 verdict.
- Subagents are helpers, not the critical path. After spawning lanes, continue
  local verification, docs, PR cleanup, and integration work.
- Cap each `wait_agent` call at 10 minutes, report long waits every 2-3 minutes,
  close completed agents immediately, and clear stale slots before spawning.
- If a lane times out or shuts down, close it and record `lane timed out; main
  integration fallback performed`; the main session completes that perspective
  read-only. Long blocking waits are forbidden.
