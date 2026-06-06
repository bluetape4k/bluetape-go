# Issue 26 State Package Inventory

Issue: #26
Milestone: 0.4.0
Parent planning: #135

## Current Repository Evidence

`state` does not exist yet. Existing public packages establish the local
pattern:

- `doc.go` for package docs.
- small API files and focused implementation files.
- package-local README.
- compile-checked examples.
- `GoroutineStressTester` and `AsyncJobTester` for concurrency/cancellation
  contracts.

## 0.4.0 Planning Evidence

#135 defines `state` as an independent package for FSM states, events,
transitions, guards, transition results, and transition errors. It explicitly
excludes Kotlin DSL, coroutine, reactive, `StateFlow`, and event/effect layers
from #26.

## Kotlin Reference Evidence

Source paths read for #135 and #26:

- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/src/main/kotlin/io/bluetape4k/states/api/StateMachine.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/src/main/kotlin/io/bluetape4k/states/api/TransitionResult.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/src/main/kotlin/io/bluetape4k/states/core/DefaultStateMachine.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/README.md`

Useful concepts:

- typed states and events.
- transition lookup by current state and event.
- final states stop further transition.
- guard conditions can veto transition.
- transition result records previous state, event, and current state.
- `CanTransition` and allowed event queries help callers inspect state.
- concurrency safety is part of the contract.

Excluded concepts:

- Kotlin DSL builders.
- suspend state machine and `StateFlow`.
- reactive event/effect runtime.
- nested state-family transitions.

## Go Package Decision

Implement `github.com/bluetape4k/bluetape-go/state`.

Adopt:

- `S comparable` and `E comparable` generics.
- explicit `Transition` slices.
- `Guard` functions that receive `context.Context`.
- sentinel errors usable with `errors.Is`.
- a `TransitionError` carrying from/event/to and the wrapped cause.
- `sync.RWMutex` for state and transition registry protection.

Reject:

- reflection-driven event types.
- callbacks in #26.
- dependency additions.
- mutable global state.

## Risk Notes

- If guard execution happens while holding the machine lock, a guard that calls
  back into the machine can deadlock. Run guards outside the lock and re-check
  that the state is still unchanged before committing.
- If concurrent callers evaluate the same transition while a guard is slow,
  exactly one should commit and the rest should receive a deterministic
  concurrent transition error.
- `AllowedEvents` cannot sort arbitrary comparable event values. Preserve
  transition registration order instead.
