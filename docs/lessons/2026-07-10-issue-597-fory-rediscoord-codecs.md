# Issue #597 교훈: Serialization Bound는 모든 Allocation Layer를 덮어야 한다

## Context

`rediscoord`는 codec payload를 binary profile wrapper 안에 넣고, 다시 JSON/base64
owner-result envelope 안에 넣은 뒤 Redis에 저장한다. Apache Fory도 mutable runtime
state가 소유한 marshal byte를 반환한다.

## Learning

provider decode 단계의 payload limit만으로는 부족하다. owner path는 provider byte를
copy/wrap하기 전에 oversized provider byte를 거절하고, 큰 allocation 전에 outer
JSON/base64 size를 preflight하며, JSON decode 전에 Redis read response를 bound해야
한다. runtime-owned Fory byte는 같은 mutex가 runtime을 보호하는 동안 copy해야 한다.

Error redaction도 같은 layered property를 갖는다. `Unwrap()`이 raw registration 또는
provider text를 노출한다면 sanitized `Error()`만으로는 부족하다. stable typed
operation/profile/reason label은 유지하되 untrusted cause는 safe sentinel cause로
대체한다.

## Durable Checks

- wire profile, wrapper version, 모든 provider metadata limit을 pin한다.
- wire length field보다 큰 limit을 거절한다.
- root-kind whitelist를 사용하고 모든 public profile의 nil/empty/zero semantics를
  증명한다.
- shared runtime ownership을 panic-safe하게 만들고 `go vet -copylocks`로 value
  copy를 탐지한다.
- namespace, codec/profile, registration set, 모든 size limit을 하나의 rollout
  tuple로 취급한다.
- implementation PR에는 benchmark claim을 넣지 않는다. benchmark work에는
  same-condition result table, Chart, written analysis가 필요하다.
