# Graph I/O 경계

일자: 2026-06-30
이슈: #49
마일스톤: 0.10.0

## 교훈

Graph import/export는 filesystem 또는 backend ownership이 아니라 bounded record
stream에서 시작해야 한다. NDJSON과 paired CSV만으로 vertex/edge interchange behavior를
증명할 수 있으며, GraphML, compression, encryption, atomic file replacement,
repository/session API, traversal semantics는 첫 public contract 밖에 둘 수 있다.

## 적용된 계약

- `graph/graphio` finite helper는 vertex를 edge보다 먼저 쓴다.
- Streaming CSV reader는 caller가 vertex를 edge보다 먼저 소비해야 한다.
- Reader는 기본적으로 duplicate vertex와 missing endpoint에 fail closed한다.
- CSV record byte limit은 `encoding/csv`가 logical record를 parse하기 전에 적용한다.
- CSV formula escaping은 caller-facing export의 기본값이다. Raw output은 명시적으로
  선택해야 한다.

## 후속 작업

Backend adapter evaluation (#50)과 domain example (#51)은 repository, session, schema
contract를 성급히 도입하지 말고 record contract를 직접 재사용해야 한다.
