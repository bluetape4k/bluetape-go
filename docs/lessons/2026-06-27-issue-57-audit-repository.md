# Issue 57 audit repository 교훈

## 맥락

Issue #57은 #56 audit model 위에 durable adapter보다 먼저 storage-neutral repository와
history query contract를 추가한다.

## 교훈

- Audit history가 없을 때는 sentinel error가 아니라 boolean result를 반환한다. 그래야
  호출자가 absent data와 invalid input 또는 storage failure를 구분할 수 있다.
- `History`는 initial revision부터 full and contiguous해야 한다. Partial history query는
  repository filter가 history reconstruction invariant를 약화하지 않도록 `[]Entry`를
  반환한다.
- In-memory repository ordering은 문서화된 계약이 필요하다. 기본 append-order와
  `NewestFirst` reversal은 단순하고 deterministic하며 adapter 친화적이다.
- Reusable adapter conformance는 `audit` 내부 `_test.go` helper가 아니라 import 가능한
  `audit/audittest` package에 둔다. 미래 adapter package는 `audit` test에서만 compile되는
  helper를 import할 수 없다.
- Shared-state repository code에는 targeted behavior test와 `GoroutineStressTester`,
  `go test -race`가 모두 필요하다. Async job helper는 추가하지 않았으므로 #57에는
  `AsyncJobTester`가 적용되지 않는다.

## 후속 가드레일

- Durable SQL/Redis/Kafka/NATS adapter는 backend-specific behavior를 추가하기 전에
  `audittest.RunRepositoryConformance`를 실행해야 한다.
- Outbox/source transaction semantics는 repository query semantics와 별개이며,
  `MemoryRepository`가 이를 암시해서는 안 된다.
