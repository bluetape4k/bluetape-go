# Issue #360 collection helper 범위

일자: 2026-07-03

Kotlin `bluetape4k-core`는 mutable list extension, sequence DSL, synchronized
container, primitive-array adapter, permutation utility까지 넓은 collection surface를
제공한다. Go package에는 이미 stack, ring buffer, page, permutation, chunk, group,
distinct, error-aware map에 대한 bounded contract가 있었다.

교훈: Kotlin collection helper를 Go에 맞출 때는 feature parity가 아니라 실제 service-code
readability gap에서 시작한다. `Sliding`, `PadTo`, `SafeSubslice`, `Count`,
`ZipWithIndex`, `ForEachErr`처럼 반복 local code보다 contract가 명확한 좁은 helper는
허용된다. Synchronized container facade, 넓은 sequence DSL, Java stream parity,
primitive-array conversion helper는 미래 issue가 구체적인 production demand를 증명하기
전까지 명시적인 non-goal로 둔다.

예방: collection helper를 더 추가하기 전에는 repo에서 중복 production logic을 찾고,
그 helper가 `slices`, `maps`, 짧은 local loop보다 왜 명확한지 문서화한다.
