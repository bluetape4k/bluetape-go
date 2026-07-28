# Issue 26 State Package Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #26
Milestone: 0.4.0
Research: `docs/superpowers/research/2026-06-05-issue-26-state-inventory.md`

## 맥락

#26 implements the first 0.4.0 package. It must be independent from
`workreport` and `workflow`, because later issues consume the state package but
the FSM itself does not need workflow reports.

## 목표s

- Add a public `state` package.
- Define state/event transition contracts.
- Support guard functions that can veto transitions.
- Return explicit transition results.
- Expose deterministic transition errors.
- Keep methods safe for concurrent callers.
- Add package docs, README, and compile-checked examples.
- Add stress and cancellation tests using repository test helpers.

## Non-Goals

- Do not add Kotlin-style builders or reflection-heavy DSLs.
- Do not implement coroutine, reactive, observer, or event/effect layers.
- Do not implement nested state-family transitions.
- Do not add dependencies.
- Do not modify root README tables in #26; #132 owns milestone README linking.

## 설계 Options

| Option | Decision | Rationale |
|---|---|---|
| Mutable `Machine` with explicit transition definitions | Adopt | Matches #135, keeps the package small, supports `State`, `Transition`, `CanTransition`, and stress/race validation without adding dependencies. |
| Stateless transition validator returning only next-state values | Reject | Too small for #26 because it cannot own current-state concurrency, final-state behavior, or caller inspection APIs required by the issue. |
| Kotlin-style DSL, callbacks, observer, or reactive runtime | Reject | Explicitly out of scope in #26 and would blur later workflow/event-effect issues. |
| External FSM dependency | Reject | Prior states comparison kept external libraries as reference only; the 0.4.0 plan requires first-party, framework-free Go packages. |

## Public API

```go
type Guard[S comparable, E comparable] func(context.Context, S, E) error

type Transition[S comparable, E comparable] struct {
    From  S
    Event E
    To    S
    Guard Guard[S, E]
}

type Result[S comparable, E comparable] struct {
    Previous S
    Event    E
    Current  S
}

type Machine[S comparable, E comparable] struct { ... }

func NewMachine[S comparable, E comparable](
    initial S,
    transitions []Transition[S, E],
    options ...Option[S, E],
) (*Machine[S, E], error)

func WithFinalStates[S comparable, E comparable](states ...S) Option[S, E]
```

Methods:

- `State() S`
- `Transition(ctx context.Context, event E) (Result[S, E], error)`
- `CanTransition(ctx context.Context, event E) (bool, error)`
- `AllowedEvents() []E`

## Error Contract

Sentinel errors:

- `ErrInvalidTransition`
- `ErrGuardRejected`
- `ErrFinalState`
- `ErrConcurrentTransition`
- `ErrDuplicateTransition`
- `ErrUnknownInitialState`

`Transition` and `CanTransition` should preserve context errors and guard
errors through `errors.Is`. Transition-specific errors should use
`TransitionError[S,E]` where details help callers debug.

`TransitionError[S,E]` should carry a sentinel `Kind`, `From`, `Event`, optional
`To`, and optional `Cause`. Its `Is` method should match `Kind`, and `Unwrap`
should expose `Cause` so callers can check both package sentinel errors and
guard/context causes with `errors.Is`.

## Behavior Contract

- `NewMachine` rejects duplicate `(from,event)` transitions.
- `NewMachine` rejects an initial state that is not part of any transition or
  final-state set.
- `State` returns the current state.
- `Transition` checks context cancellation before reading state.
- `Transition` rejects events from final states.
- `Transition` rejects missing `(state,event)` transitions.
- Guards run outside the internal lock.
- Guard errors veto transition and do not mutate state.
- After a guard passes, `Transition` re-checks that the current state is still
  the original state before committing.
- `Transition` checks context cancellation before lookup and again before
  commit. During guard execution, the guard is responsible for observing the
  supplied context.
- If the state changed while a guard ran, `Transition` returns
  `ErrConcurrentTransition` and does not mutate state again.
- `CanTransition` evaluates guard logic without mutating state. Because this
  can execute caller-supplied guard code, guards used with `CanTransition` must
  be safe for inquiry calls and should avoid irreversible side effects.
- `AllowedEvents` returns events registered for the current state in
  registration order.
- `AllowedEvents` is a structural registry query. It does not evaluate guards
  and is not a guarantee that `Transition` will succeed.
- A nil context is normalized to `context.Background()` to match existing
  package entry-point tolerance.

## 위험 And Failure Modes

- Guard execution under lock can deadlock if a guard calls back into the
  machine. Guards must run outside the internal lock and state must be rechecked
  before commit.
- Concurrent callers can evaluate the same guarded transition. Exactly one
  caller may commit; callers that lose the state recheck return
  `ErrConcurrentTransition`.
- Guard side effects can surprise users when `CanTransition` is called. README
  and Go doc must state that guards should be safe for inquiry calls.
- `AllowedEvents` can be mistaken for guard-approved events. Public docs and
  tests must show that it returns registered events only.
- Generic comparable event values cannot be sorted reliably. Preserve
  transition registration order in tests and docs.

## Tests

- valid transition returns previous/event/current.
- invalid transition returns `ErrInvalidTransition` and leaves state unchanged.
- guard success allows transition.
- guard rejection returns `ErrGuardRejected`, preserves the guard error, and
  leaves state unchanged.
- final state returns `ErrFinalState`.
- duplicate transitions are rejected by `NewMachine`.
- unknown initial state is rejected by `NewMachine`.
- `CanTransition` does not mutate state and reports guard outcome.
- `CanTransition` docs/tests demonstrate that guard execution must be safe for
  inquiry calls.
- `AllowedEvents` returns current-state registered events in registration order
  without evaluating guards.
- `TransitionError` supports `errors.Is` for both package sentinel errors and
  wrapped guard/context causes.
- `GoroutineStressTester` proves concurrent guarded transitions commit exactly
  once and return deterministic errors for the rest.
- `AsyncJobTester` proves cancellation is propagated through guard execution.
- `go test -race -count=1 ./state` passes.

## Documentation

- Add `state/doc.go`.
- Add `state/README.md`.
- Add `state/state_example_test.go`.
- Update `CHANGELOG.md` Unreleased section.
- Update `WIP.md` to mark #26 as active/in progress.

## Definition Of Done

- `go test -count=1 ./state` passes.
- `go test -race -count=1 ./state` passes.
- `go test -count=1 ./...` passes.
- `git diff --check` passes.
- Local 7-tier review artifact records `P0=0 P1=0`.
