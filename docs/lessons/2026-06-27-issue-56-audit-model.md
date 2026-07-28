# Issue #56 audit model 교훈

일자: 2026-06-27

## 교훈

- Raw byte cloning은 중립적으로 유지한다. Event payload default는 event별로 다를 수
  있지만 shared clone helper가 nil payload를 valid JSON으로 바꾸면 안 된다. Snapshot과
  decode 경로는 누락된 필수 field를 거부할 수 있어야 한다.
- Audit data validation error는 caller value를 되풀이하지 않으면서 sentinel과 field
  evidence를 보존해야 한다. Audit payload, metadata, author, idempotency key에는
  민감한 운영 데이터가 들어갈 가능성이 높다.
- In-memory pending event는 retry aid일 뿐 durability가 아니다. Recorder example은
  source write와 audit commit에 shared durable transaction, rollback, outbox 또는
  reconciliation 경로가 필요하다고 말해야 한다.
- 새 package API는 repository/outbox package가 의존하기 전에 Go-idiomatic하게 정리한다.
  `audit.Entry`, `audit.NewEntry`, `audit.DecodeEntryJSON`, `audit.ErrInvalidEntry`는
  stuttered name을 고정하지 않게 해 준다.
