# Issue #456 JSON repeated collection profile 교훈

일자: 2026-07-07
범위: `serialization.JSONSerializer`

## 교훈

Default JSON decode는 trailing JSON value를 거부하기 위해 반드시 `json.Decoder`를 쓸 필요가
없다. `json.Unmarshal`은 이미 trailing non-whitespace data를 거부하고 decoder refill buffer
allocation을 피한다. `json.Decoder`는 `DisallowUnknownFields`처럼 필요한 option에만 둔다.

## 패턴

- Allocation count가 높은 benchmark row를 최적화하기 전에 profile한다.
- 변경 전후 raw `-benchmem` output과 `pprof -alloc_space` top output을 보존한다.
- Corrupt input, empty input, trailing JSON, unknown-field rejection에 대한 strict
  behavior test를 유지한다.
- Decoded JSON object graph에는 tests가 caller-visible aliasing 또는 race risk가 없음을
  증명하기 전까지 pooling을 추가하지 않는다.

## 후속 작업

#455는 compression allocation을 별도로 profile해야 한다. #456 결과는
`compression.Default()`나 zstd policy를 바꾸지 않는다.
