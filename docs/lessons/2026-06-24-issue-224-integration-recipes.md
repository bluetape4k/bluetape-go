# Lessons: Issue 224 Integration Recipes

Issue: #224
Date: 2026-06-24

- Cross-package examples belong under `examples/` unless a caller needs a new
  reusable API. This keeps recipe code copyable without freezing helper
  contracts.
- Docker-backed recipes should be env-gated so ordinary `go test ./...` remains
  local and deterministic.
- For integration docs, prove the failure path in code. A retry/skip recipe is
  more useful than a happy-path-only snippet.
- Keep English and Korean root README links in sync when adding public example
  packages.
