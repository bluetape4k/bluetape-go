# SerDe benchmark runner 범위

Issue #400은 production API를 바꾸지 않고 #399 fixture matrix를 runnable Go benchmark entry
point로 만든다.

## 교훈

- Package가 reusable fixture API를 소유하지 않는다면 benchmark fixture는 `_test.go`에 둔다.
  Cross-repo fixture name은 documentation contract이지 production symbol을 export할 이유가
  아니다.
- Codec exclusion은 command 옆에 문서화한다. 큰 Base58/Base62 byte-array row는 현실적인
  SerDe transport path보다 현재 division-based alphabet implementation을 더 크게 측정한다.
- Artifact-producing command는 artifact format이 최종화되기 전에도 명시한다.
  `tee docs/research/outputs/issue-400/...`는 metadata를 너무 일찍 고정하지 않으면서도
  #401에 구체적인 retention target을 제공한다.

## 증거

- `serialization/serialization_benchmark_test.go`
- `codec/codec_benchmark_test.go`
- `compression/compression_benchmark_test.go`
- `docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md`
