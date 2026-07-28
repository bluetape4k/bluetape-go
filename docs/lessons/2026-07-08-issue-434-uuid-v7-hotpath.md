# Issue #434 UUID v7 hot-path 교훈

Issue: #434
Date: 2026-07-08

## Lesson

mutex 형태의 UUID v7 profile을 atomic logical tick rewrite의 충분한 증거로
간주하지 않는다.

fresh issue #434 benchmark에서 현재 shared-generator UUID v7 parallel row는 약
`192.6 ns/op`로 측정되었다. local atomic tick reservation 후보는 allocation 변화
없이 약 `193.2 ns/op`였다. 따라서 `id/uuid.go`에 concurrency complexity를
추가하지 않고 후보를 거절했다.

향후 UUID v7 작업은 먼저 string encoding/allocation 또는 명시적 caller-owned
pooling/sharding contract를 목표로 삼아야 한다. production code를 바꾸기 전에는
측정된 baseline-vs-candidate 비교를 유지한다.
