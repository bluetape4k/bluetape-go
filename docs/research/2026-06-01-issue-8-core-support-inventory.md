# Issue 8 Core Support Inventory

## 맥락

Issue #8은 범용으로 재사용할 수 있는 `bluetape4k/core` support 개념을 idiomatic Go로 옮긴다. 원본
module은 크고 Kotlin/JVM 지향이므로, 이 inventory는 직접 구현할 항목, Go standard library를 채택할 항목,
그리고 미룰 domain을 분리한다.

## Source Inventory

관찰한 `bluetape4k-projects/bluetape4k/core` 영역:

- `support/*`: require/assert helper, string helper, value conversion, number parsing, UUID helper,
  result/lazy/timeout helper, Java type helper.
- `collections/*`: list/map/sequence helper, bounded stack, ring buffer, pagination, permutation.
- `codec/*`: Base58, Base62, Base64, Hex, URL-safe UUID encoder.
- `concurrent/*`: Future/CompletableFuture helper, lock, reducer, executor 및 thread helper.
- `ranges/*`: closed/open range model과 validation.
- `javatimes/*`: JVM time/date convenience helper.
- `apache/*`, Java/Kotlin reflection helper, JVM-specific adapter.

## 지금 구현

- `error`를 반환하는 validation helper: blank/empty text, ordered range, positive 및 non-negative numeric check.
- pointer helper: `Ptr`, `ValueOr`, `ValueOrZero`.
- zero/default helper: `Zero`, `IsZero`, `DefaultIfZero`, `FirstNonZero`.
- service code를 더 명확하게 만드는 string helper: `HasText`, defaulting helper, UTF-8 byte-safe truncation.
- 작은 numeric helper: `Clamp`, hex digit 및 prefixed hex format check.

## Standard Library 채택

- UTF-8 byte conversion은 `[]byte(s)`와 `string(b)`를 쓴다.
- comparable value의 generic equality는 `==`를 쓴다.
- numeric parsing은 future API가 bluetape-specific parsing semantics를 요구하기 전까지 `strconv`를 직접 쓴다.
- time/date helper는 반복 workflow가 생기기 전까지 `time`을 직접 쓴다.
- hashing은 generic object-hash helper 대신 `hash`, `hash/fnv`, `crypto/*`, 또는 package-specific hasher를 쓴다.

## 보류

- collection helper, bounded stack, ring buffer, pagination, permutation은 #9에서 추적한다.
- goroutine/context helper는 #10에서 추적한다.
- string codec 및 URL-safe ID는 #11에서 추적한다.
- binary serializer와 compressor abstraction은 #12 및 #13에서 추적한다.
- JVM/Kotlin reflection, Apache Commons adapter, Java Optional helper, thread/future helper는 Go로 portable하지 않으므로
  직접 port하지 않는다.

## 결정

`core`는 작게 유지한다. 반복 service code를 줄이면서 Go reader에게 명확하게 읽히는 helper만 추가한다.
Kotlin extension 모양의 utility bag은 만들지 않는다.
