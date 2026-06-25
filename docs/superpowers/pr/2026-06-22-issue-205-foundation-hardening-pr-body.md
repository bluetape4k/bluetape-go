## Summary

Closes #205

This PR hardens the existing `core`, `collections`, `codec`, and
`serialization` foundation contracts before milestone `0.6.3` adds new parity
APIs.

## Background

Issue #205 is the P0 audit task under the `0.6.3` foundation milestone. The
current packages already provide core helpers, collections transforms, codec
helpers, and raw serializers, but several text/binary, nil/empty, malformed
input, and documentation contracts were implicit.

## What This Solves

- Makes invalid UTF-8 text failures caller-visible through
  `core.ErrInvalidUTF8`.
- Keeps byte codec helpers and `BytesSerializer` binary-safe for arbitrary
  payloads.
- Separates malformed codec input failures from decoded invalid-text failures.
- Documents no-error string encoder helpers as compatibility string-to-byte
  conversions that cannot report invalid UTF-8.
- Adds regression coverage for collection nil/empty and nil callback
  precedence behavior.

## Work Done

- `core`: added `ErrInvalidUTF8` and made `TruncateUTF8Bytes` reject invalid
  UTF-8 while preserving rune-boundary truncation.
- `codec`: routed string decoders through UTF-8 validation and kept byte
  decoders binary-capable.
- `serialization`: made `StringSerializer` validate UTF-8 on marshal/unmarshal
  and kept `BytesSerializer` binary-capable.
- `collections`: added nil/empty/callback regression coverage without changing
  existing helper behavior.
- Docs/examples: updated English/Korean README files and examples for
  `errors.Is(err, core.ErrInvalidUTF8)` plus byte fallback paths.
- Workflow evidence: added spec, plan, Step 2-R, Step 3-R, Step 5, Step 6-R,
  and lessons artifacts.

## Validation

- `go test -count=1 ./core ./collections ./codec ./serialization`: PASS
- `go test -run Example -count=1 ./codec ./serialization`: PASS
- `go list -deps ./codec ./serialization | rg '^github.com/bluetape4k/bluetape-go/core$'`: PASS
- `go test -race -count=1 ./codec`: PASS
- `go test -race -count=1 ./serialization`: PASS
- `make ci`: PASS
- `git diff --check`: PASS

## Review Notes

- P0/P1: 0
- P2/P3: none recorded for follow-up
- Review evidence:
  - `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-2r-spec-review.md`
  - `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-3r-plan-review.md`
  - `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-5-verifier.md`
  - `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-6r-code-review.md`
  - `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-7r-pr-review.md`

## Metadata

- Issue: #205, milestone `0.6.3`, assignee `debop`
- PR: #252, milestone `0.6.3`, assignee `debop`
- CI: pending GitHub checks

## DoD Status

| Step | Status | Evidence |
|------|--------|----------|
| Step 0 - Worktree | PASS | `.worktrees/issue-205-foundation-hardening`, branch ahead `origin/develop` by two commits before PR. |
| Step 1/1-R - Requirements and research | PASS | Issue #205 metadata inspected; current package sources, tests, README files, and prior docs inspected. |
| Step 2 - Spec | PASS | `docs/superpowers/specs/2026-06-21-issue-205-foundation-hardening-design.md` |
| Step 2-R - Spec review | PASS | `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-2r-spec-review.md`, P0=0 P1=0 |
| Step 3 - Plan | PASS | `docs/superpowers/plans/2026-06-21-issue-205-foundation-hardening-plan.md` |
| Step 3-R - Plan review | PASS | `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-3r-plan-review.md`, P0=0 P1=0 |
| Step 4 - TDD implementation | PASS | RED failures observed for missing `core.ErrInvalidUTF8`; implementation commit `7f80b73`. |
| Step 4-T - Tests | PASS | Targeted tests, examples, race checks, dependency check, `make ci`, and `git diff --check` passed. |
| Step 5 - Verifier | PASS | `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-5-verifier.md` |
| Step 6-R - Code review | PASS | `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-6r-code-review.md`, P0=0 P1=0 |
| Step 7 - Lessons | PASS | `docs/lessons/2026-06-22-issue-205-foundation-hardening.md`, committed before PR creation. |
| Step 7-P - PR | PASS | PR #252 created; assignee `debop`; milestone `0.6.3`; labels match issue #205. |
| Step 7-R - PR review | PASS | `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-7r-pr-review.md`, P0=0 P1=0. |
| Step 8 - CI | PENDING | To check after PR creation. |

Final status: PR #252 pending review and CI.
