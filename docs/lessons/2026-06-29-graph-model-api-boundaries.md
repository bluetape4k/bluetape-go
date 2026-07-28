# Graph model API 경계

일자: 2026-06-29
이슈: #48
마일스톤: 0.10.0

## 결정

첫 `graph` package는 model-only package다. Vertex, edge, path, label, ID, shallow
property, JSON round trip을 위한 validated graph value를 정의하지만 repository,
session, schema, query, transaction, backend, algorithm, capability interface는
의도적으로 제외한다.

## 이유

0.10.0 graph milestone은 더 넓은 contract를 안정화하기 전에 #49 graph I/O helper,
#50 backend adapter evaluation, #51 domain example의 증거가 필요하다. #48에서
abstraction을 추가하면 shared behavior가 알려지기 전에 추측을 고정하게 된다.

## API 경계

- Edge endpoint는 named struct를 사용해 directed role이 callsite에서 보이게 한다.
- `PathStep` constructor는 vertex와 edge value를 검증해 invalid step shape가 public
  helper로 조용히 생성되지 않게 한다.
- `Path`는 step value와 aggregate weight만 검증한다. Endpoint continuity나 traversal
  correctness는 검증하지 않으며, 이후 algorithm과 adapter가 그 invariant를 소유한다.
- Struct field는 unexported로 유지하고 accessor는 shallow defensive copy를 반환한다.
- `Properties`는 map boundary만 copy한다. Nested mutable value는 caller-owned로 남으며,
  미래 I/O/backend adapter가 trust boundary 전에 copy하거나 sanitize해야 한다.
- `ValidationError`는 typed category, field, redacted summary, cause를 보존한다. Raw
  value를 저장하면 안 된다.
- `ErrUnsupportedCapability`는 미래 capability boundary를 위해 예약하며 #48 constructor는
  이를 반환하지 않는다.

## 검증 메모

구현은 PR creation 전에 `go test -count=1 ./graph`, `go test -race -count=1 ./graph`,
`go doc ./graph`, README parity, Step 6-R P0/P1 gate를 green으로 유지해야 한다.
