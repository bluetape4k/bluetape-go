# Cumulative Hardening Cleanup Lessons

## L1: README examples must not reintroduce weak cleanup contracts

Issue #429 re-applied lessons through 0.12.0 after the retrospective P1 fixes.
The Toxiproxy README pair still showed an upstream Redis container terminated
with `testcontainers.TerminateContainer` and a Docker network removed with the
test start context. That example drifted from the #201 Testcontainers rule:
cleanup must use a bounded context derived with `context.WithoutCancel`.

Prevention:

- Keep Docker cleanup examples aligned with the helper implementation.
- Use a fresh bounded cleanup context for each external resource cleanup.
- Avoid README snippets that are weaker than the package tests they describe.

## L2: Docs-only examples still need errcheck-shaped cleanup

The Redis near-cache, Redis coordinator, and JWT README examples had bare
`defer Close()` calls. They were not compile-checked examples, but they taught a
pattern that had already failed the repository errcheck gate in earlier work.

Prevention:

- Prefer `defer func() { _ = value.Close() }()` in README snippets when cleanup
  failure cannot change the example result.
- Keep README and README.ko pairs synchronized when tightening example
  contracts.
