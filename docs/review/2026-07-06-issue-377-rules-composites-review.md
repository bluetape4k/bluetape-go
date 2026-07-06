# Issue #377 Rules Composite and Inference Review

Date: 2026-07-06

Scope:

- `rules/`
- `README.md`
- `README.ko.md`

## Evidence

- Issue #377 requires activation, conditional, and unit composite groups plus
  bounded inference on top of the first-party `rules` core from #375.
- Issue #37 research requires deterministic ordering, context-preserving
  cancellation, no scripting/annotation/reflection registration, no external
  DSL dependency, and bounded rather than unbounded inference.
- The implementation keeps composites as ordinary `Rule` values and keeps
  inference sequential with explicit `InferenceConfig.MaxCycles`.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | P0=0 P1=0. P2 benchmark follow-up noted for composite/inference hot paths. |
| Stability | PASS after fix | Initial P1 found false convergence with `StopOnFirstNotTriggered`; fixed by rejecting that option for inference and adding regression coverage. |
| Security | PASS after fix | Initial P1 found composite re-evaluation drift could count as applied; fixed with `ErrCompositeNotTriggered` fail-closed execution paths and drift tests. |
| Operator/Ops | PASS | P0=0 P1=0. P2 non-convergence stop reason fixed with `StatusNonConverged`; README notes max-cycle sizing. |
| Developer/API | PASS | P0=0 P1=0. Public APIs remain Go-shaped `Rule` values with typed errors. |
| User/Caller | PASS | P0=0 P1=0. Added compile-checked examples and README caveats for re-evaluation and inference. |
| Integration | PASS | Main-session integration verified P1 fixes, docs parity, and final test gate. |

## Validation

- `git diff --check`: PASS
- `go test -count=1 ./rules`: PASS
- `go test -race -count=1 ./rules`: PASS
- `go vet ./rules`: PASS
- `make fmt-check && make tidy-check && make vet && make lint && make test && make race`: PASS

## Findings

- P0: 0
- P1: 0 after fixes

Resolved P1 details:

- Composite groups now fail closed with `ErrCompositeNotTriggered` when
  `Execute` re-evaluation no longer finds the child/guard/unit match observed by
  the outer engine's `Evaluate`.
- `InferenceEngine` now rejects `EngineConfig.StopOnFirstNotTriggered`, which
  can otherwise hide later matching rules and falsely report convergence.

Accepted non-blocking follow-ups:

- Add `-benchmem` coverage before claiming composite/inference hot-path
  performance.
- Keep inference result retention and in-place `Facts` mutation visible in
  future reader/DSL work.
