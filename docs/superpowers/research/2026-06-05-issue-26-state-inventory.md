# Issue 26 State Package Inventory

Issue: #26
Milestone: 0.4.0
Parent planning: #135

## Current Repository Evidence

`state` package는 아직 없다. 기존 public package는 다음 local pattern을 만든다.

- package docs를 위한 `doc.go`
- 작은 API file과 focused implementation file
- package-local README
- compile-checked examples
- concurrency/cancellation contract를 위한 `GoroutineStressTester`와 `AsyncJobTester`

## 0.4.0 Planning Evidence

#135는 `state`를 FSM state, event, transition, guard, transition result, transition error를
위한 독립 package로 정의한다. Kotlin DSL, coroutine, reactive, `StateFlow`, event/effect layer는
#26에서 명시적으로 제외한다.

## Kotlin Reference Evidence

#135와 #26에서 읽은 source path:

- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/src/main/kotlin/io/bluetape4k/states/api/StateMachine.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/src/main/kotlin/io/bluetape4k/states/api/TransitionResult.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/src/main/kotlin/io/bluetape4k/states/core/DefaultStateMachine.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/states/README.md`

유용한 concept:

- typed states and events
- current state와 event에 따른 transition lookup
- final state는 추가 transition을 멈춘다
- guard condition은 transition을 veto할 수 있다
- transition result는 previous state, event, current state를 기록한다
- `CanTransition`과 allowed event query는 caller가 state를 inspect하게 돕는다
- concurrency safety는 contract 일부다

제외할 concept:

- Kotlin DSL builders
- suspend state machine과 `StateFlow`
- reactive event/effect runtime
- nested state-family transitions

## Go Package Decision

`github.com/bluetape4k/bluetape-go/state`를 구현한다.

채택:

- `S comparable`과 `E comparable` generics.
- explicit `Transition` slices.
- `context.Context`를 받는 `Guard` functions.
- `errors.Is`로 사용할 수 있는 sentinel errors.
- from/event/to와 wrapped cause를 담는 `TransitionError`.
- state와 transition registry 보호를 위한 `sync.RWMutex`.

거절:

- reflection-driven event types.
- #26 안의 callbacks.
- dependency additions.
- mutable global state.

## Risk Notes

- machine lock을 잡은 채 guard를 실행하면, guard가 machine을 다시 호출할 때 deadlock이 날 수 있다.
  guard는 lock 밖에서 실행하고 commit 전에 state가 여전히 같은지 re-check한다.
- guard가 느린 동안 concurrent caller가 같은 transition을 평가하면 정확히 하나만 commit하고,
  나머지는 deterministic concurrent transition error를 받아야 한다.
- `AllowedEvents`는 arbitrary comparable event value를 정렬할 수 없다. transition registration
  order를 보존한다.
