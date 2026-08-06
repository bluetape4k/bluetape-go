# Issue #225 Corrective Series Closure 교훈

Issue: #225
날짜: 2026-06-24

- closure audit는 corrective blocker와 future roadmap priority label을 분리해야 한다.
  open P1/P0 future issue가 현재 milestone의 P1/P0 blocker로 자동 승격되지는 않는다.
- source-parity decision은 issue number에 묶어 둔다. 그렇지 않으면 implementation
  slice가 merge된 뒤 deferred work가 보이지 않게 된다.
- JVM, framework, DSL-shaped source behavior에 대한 명시적 non-goal을 기록해 나중의
  agent가 Kotlin-to-Go API cloning을 반복 논쟁하지 않게 한다.
- documentation-only closure라도 report가 release readiness를 주장한다면 fresh
  repository gate가 필요하다.
