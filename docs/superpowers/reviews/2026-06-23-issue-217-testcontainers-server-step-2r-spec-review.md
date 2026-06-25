# Issue #217 Step 2-R Spec Review

Issue: [#217](https://github.com/bluetape4k/bluetape-go/issues/217)  
Spec: `docs/superpowers/specs/2026-06-23-issue-217-testcontainers-server-design.md`  
Date: 2026-06-23

## Scope

Reviewed the proposed `testcontainers/server` abstraction, existing wrapper
migration path, environment export behavior, cleanup contract, and documentation
requirements.

Subagent lanes were treated as local independent perspectives for this review
iteration to avoid blocking the implementation lane on slow worker startup. The
main session performed the integration verdict.

## Six-Lane Findings

| Tier | Perspective | Verdict | Findings |
|---|---|---:|---|
| 1 | Performance | Pass | The abstraction delegates to Testcontainers and adds only small map clones and string conversions. No hot-path benchmark is required for test fixtures. |
| 2 | Stability | Pass after edit | `ExportEnv` now validates before mutation and documents `testing.TB.Setenv` parallel-test limits. Cleanup remains bounded through `internal/testcleanup`. |
| 3 | Security | Pass | No hidden global export or secret persistence is introduced. Env export is explicit and test-scoped. |
| 4 | Operator/Ops | Pass | Dynamic mapped ports remain the default. Fixed host ports stay out of scope and must be documented as collision-prone follow-up work. |
| 5 | Developer/API | Pass after edit | `New` now returns `(*Started, error)` and accepts a narrow `Container` interface, avoiding panic-only validation and making fake-container contract tests realistic. |
| 6 | User/Caller | Pass | Existing `Start(ctx, testing.TB)` APIs remain source-compatible; `StartServer` is opt-in for callers that need generic details. |

## Resolved P1 Items

| Severity | Item | Resolution |
|---|---|---|
| P1 | Constructor validation had no error path while claiming to validate public inputs. | Spec changed `New` to return `(*Started, error)` and wrapper helpers fail through `testing.TB` when configuration is invalid. |
| P1 | Public adapter depended on the full `testcontainers.Container` interface, making non-Docker unit tests unrealistic. | Spec added a narrow `server.Container` interface with only `Host`, `MappedPort`, `PortEndpoint`, and `Terminate`. |
| P1 | `ExportEnv` failed via `tb.Fatalf`, making missing-key behavior hard to test and risking partial mutation. | Spec changed `ExportEnv` to validate first and return errors. |
| P1 | `tb.Setenv` parallel-test restrictions were not explicit. | Spec now documents that `ExportEnv` must not be used in tests with `t.Parallel` or parallel ancestors. |

## Remaining P2/P3 Watch Items

| Severity | Item | Required Follow-Up |
|---|---|---|
| P2 | Kafka `[]string` ergonomics can be weakened if generic details become the only internal source. | Keep existing `Start` returning `[]string`; generic map uses comma-separated `kafka.brokers` for export/reporting only. |
| P2 | Mutable `ConnectionDetails` maps can leak if returned directly. | Implementation must clone on return and tests must prove caller mutation does not affect stored details. |
| P3 | Docs can drift across English/Korean READMEs. | Update both locale files for each changed wrapper package and include fixed-port collision language. |

## Integrated Verdict

P0: 0  
P1: 0  
P2: 2  
P3: 1

The spec is implementation-ready after the recorded edits. Step 3 planning must
turn the P2/P3 watch items into concrete implementation and validation tasks.
