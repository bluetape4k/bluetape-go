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
