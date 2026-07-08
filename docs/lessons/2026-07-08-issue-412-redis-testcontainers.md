# Issue 412 Redis Testcontainers Coverage Lesson

## Context

Issue #411 added Redis HyperLogLog behavior and Testcontainers-backed tests, but
issue #412 still needed explicit coverage evidence for bounded contexts, serial
Testcontainers execution, and the local command surface.

## Lesson

When a follow-up issue is mostly satisfied by a previous feature PR, close the
remaining gap with evidence rather than duplicate structures. For
Testcontainers-backed Go packages, make startup, readiness, live operations, and
cleanup timeouts visible in code, then document the package-local test and race
commands in both README locale files.

## Applied

- Added reusable Redis test timeout constants and `redisTestContext(t)`.
- Replaced unbounded Redis integration operation contexts in Bloom and
  HyperLogLog tests.
- Documented Redis image, bounded context policy, coverage matrix, stress
  helpers, and serial execution guidance.
