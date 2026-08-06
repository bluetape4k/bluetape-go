# Issue #592 교훈: Shared Redis Construction의 Compatibility Boundary는 좁다

## Context

`probabilistic/redis`에는 fixed prefix와 validated namespace에 대해 shared
`redis.KeyBuilder`와 정확히 일치하는 local Bloom/HyperLogLog structural key
formatting이 있었다.

## Learning

shared Redis key builder는 provider 자체 validation이 caller input을 수락한 뒤에만
재사용한다. provider가 별도 public diagnostic contract를 소유한다면 constructed key
value만 사용한다. shared construction이 validation error, redacted identifier, typed
operational error의 compatibility를 뜻하지는 않는다.

이 package에서는 shared builder의 24-hex `Key.RedactedID`가 기존 `redis-key:`와
12-hex probabilistic identifier를 대체하면 안 된다. provider의 namespace validation은
generic hash-tag validation을 넘어서는 sensitive-marker policy를 포함하므로 첫
caller-visible boundary로 남는다.

## Durable Checks

- colon-containing Cluster hash tag를 포함해 정확한 Redis key byte를 assert한다.
- output-parity test가 기존 formatting implementation에 대해 false-green이 될 수
  있으면 direct private-adapter RED test를 추가한다.
- locally validated input 뒤 shared builder failure는 opaque/unwrapped로 유지한다.
  provider를 통해 shared key-validation error type을 노출하지 않는다.
- 별도 public compatibility issue가 변경을 승인하지 않는 한 provider-specific
  `RedisError`와 script metadata sentinel mapping을 보존한다.
- construction-only migration의 benchmark work는 N/A로 표시한다. cross-provider
  performance conclusion은 issue #560 소관이며 result table, chart, written
  analysis가 함께 필요하다.
