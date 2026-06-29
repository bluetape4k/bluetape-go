# Issue #359 Zero Default Helper Review

Issue: #359
Branch: `feat/issue-359-zero-default`
Date: 2026-06-29

## Scope

Small public API addition in `core`: `IfZeroOrDefault[T comparable]`.

## 7-Tier Local Review

| Lane | P0 | P1 | Verdict | Evidence |
|---|---:|---:|---|---|
| Performance | 0 | 0 | PASS | Helper delegates to `IsZero` and adds no allocation or shared state. |
| Stability | 0 | 0 | PASS | Comparable-only generic contract matches existing zero helpers. |
| Security | 0 | 0 | PASS | No input parsing, IO, crypto, auth, or secret handling. |
| Operator/Ops | 0 | 0 | PASS | No runtime configuration, logging, or deployment surface. |
| Developer/API | 0 | 0 | PASS | Exported helper has a Go doc comment and README locale set was updated. |
| User/Caller | 0 | 0 | PASS | Tests cover zero and non-zero fallback behavior. |
| Integration | 0 | 0 | PASS | Targeted `core` tests and race test pass. |

## Validation

- `git diff --check`: PASS
- `go test -count=1 ./core`: PASS
- `go test -race -count=1 ./core`: PASS
- `go test -count=1 ./core -run TestZeroHelpers -v`: PASS

P0=0 P1=0
