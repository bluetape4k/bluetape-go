# 진행 상황

기준 시각: 2026-09-06 KST
범위: `v0.22.0` foundation 구현.

## 현재 대상 릴리스

`v0.22.0`은 좌표/Geohash와 graph backend conformance foundation을 묶는
릴리스입니다. Issue #548은 외부 dependency 없는 `geo` package delivery를
담당하며 tag와 publication은 milestone open issue가 0이 된 뒤 별도 gate에서
진행합니다.

## v0.21.0 이력 경계

`v0.20.0`의 `main` projection tree에는 #541~#545, #688, #689의 구현이
이미 포함되어 있습니다. 그러나 `v0.20.0` 변경 기록은 해당 web API helper
범위를 별도 릴리스 항목으로 설명하지 않았습니다. `v0.21.0`은 이 기능군을
공식 릴리스 범위로 명시하고, `v0.20.0..develop`의 실제 source delta인
Echo 후속 #692~#694를 함께 배포합니다. `v0.20.0` 사용자는 Echo 후속 수정이
필요할 때 `v0.21.0`으로 올리면 됩니다. Milestone #33은 `open_issues=0`으로
닫혔고, annotated tag `v0.21.0`은
`c51be7c604a07a131fa39932e0251f67b3c457e6`을 가리킵니다. GitHub Release는
2026-09-05 13:59:26 UTC에 게시됐습니다.

## 현재 상태

- #548은 `geo`의 `Point`, `Bounds`, Haversine distance, canonical lowercase
  Geohash API와 영어/한국어 package 문서를 구현하고 있습니다.
- #548의 완료 gate는 `go test`, race, benchmark 관찰, formatter, tidy, vet,
  lint와 전체 직렬 test입니다. Testcontainers와 diagram은 이 순수 계산
  package에 N/A입니다.
- `v0.22.0` release preparation, tag와 GitHub Release는 아직 실행하지
  않았습니다.

## 0.22.0 / #548

### Benchmark ledger

- Benchmarked SHA: `0dc2035bf32494df1c10e6bf3498a52cd0a9d960`
- Post-benchmark changes: benchmark evidence commit과 Go doc의 `revive`
  공백 수정만 뒤따랐으며 실행문은 바뀌지 않았다.
- Working tree: clean
- Go version: `go1.27.1 darwin/arm64`
- OS: `Darwin 25.6.0 arm64`
- GOMAXPROCS: `default`
- Command: `go test -run '^$' -bench
  'Benchmark(NewPoint|BoundsContains|DistanceMeters|Encode|Decode)$' -benchmem
  -count=3 -benchtime=1s ./geo`
- Fixture/order: `NewPoint`, ordinary/antimeridian `BoundsContains`,
  `DistanceMeters`, `Encode` precision 1/12, `Decode` precision 1/12 순서다.
- Metric direction: allocation은 낮을수록 좋다. `ns/op`은 환경 관찰값이며
  기능 gate로 사용하지 않는다.
- Three-run verdict: 세 반복 모두 `NewPoint`, `BoundsContains`,
  `DistanceMeters`, `Decode`는 `0 allocs/op`, `Encode`는 `1 allocs/op` 이하로
  계획의 `2 allocs/op` 상한을 충족했다.
- Raw output:

```text
0dc2035bf32494df1c10e6bf3498a52cd0a9d960
go version go1.27.1 darwin/arm64
Darwin debop-m4-pro.local 25.6.0 Darwin Kernel Version 25.6.0: Fri Jul 31 19:17:26 PDT 2026; root:xnu-12377.161.14~5/RELEASE_ARM64_T6041 arm64
GOMAXPROCS=default
goos: darwin
goarch: arm64
pkg: github.com/bluetape4k/bluetape-go/geo
cpu: Apple M4 Pro
BenchmarkNewPoint-12                         632558580   1.910 ns/op    0 B/op   0 allocs/op
BenchmarkNewPoint-12                         599988624   2.008 ns/op    0 B/op   0 allocs/op
BenchmarkNewPoint-12                         599008015   2.002 ns/op    0 B/op   0 allocs/op
BenchmarkBoundsContains/ordinary-12          272564817   4.534 ns/op    0 B/op   0 allocs/op
BenchmarkBoundsContains/ordinary-12          264264574   4.439 ns/op    0 B/op   0 allocs/op
BenchmarkBoundsContains/ordinary-12          276224308   4.394 ns/op    0 B/op   0 allocs/op
BenchmarkBoundsContains/antimeridian-12      289196144   4.171 ns/op    0 B/op   0 allocs/op
BenchmarkBoundsContains/antimeridian-12      283608799   4.252 ns/op    0 B/op   0 allocs/op
BenchmarkBoundsContains/antimeridian-12      287523888   4.246 ns/op    0 B/op   0 allocs/op
BenchmarkDistanceMeters-12                   40226552    30.71 ns/op    0 B/op   0 allocs/op
BenchmarkDistanceMeters-12                   38827882    30.93 ns/op    0 B/op   0 allocs/op
BenchmarkDistanceMeters-12                   38723260    30.59 ns/op    0 B/op   0 allocs/op
BenchmarkEncode/precision-1-12               136655691   8.883 ns/op    0 B/op   0 allocs/op
BenchmarkEncode/precision-1-12               138612745   8.877 ns/op    0 B/op   0 allocs/op
BenchmarkEncode/precision-1-12               134620063   9.033 ns/op    0 B/op   0 allocs/op
BenchmarkEncode/precision-12-12              11539750    102.6 ns/op   16 B/op   1 allocs/op
BenchmarkEncode/precision-12-12              11471758    105.9 ns/op   16 B/op   1 allocs/op
BenchmarkEncode/precision-12-12              11467683    102.6 ns/op   16 B/op   1 allocs/op
BenchmarkDecode/precision-1-12               91436986    13.18 ns/op    0 B/op   0 allocs/op
BenchmarkDecode/precision-1-12               91056540    13.16 ns/op    0 B/op   0 allocs/op
BenchmarkDecode/precision-1-12               92889141    13.07 ns/op    0 B/op   0 allocs/op
BenchmarkDecode/precision-12-12              13884676    171.0 ns/op   0 B/op   0 allocs/op
BenchmarkDecode/precision-12-12              13565989    88.40 ns/op   0 B/op   0 allocs/op
BenchmarkDecode/precision-12-12              13375285    88.18 ns/op   0 B/op   0 allocs/op
PASS
ok github.com/bluetape4k/bluetape-go/geo 36.909s
```

## 비범위

- downstream consumer의 `go.mod` 업데이트는 이번 요청에 포함하지 않습니다.
- `v0.22.0` release preparation, tag, publication과 downstream handoff는
  feature 구현 뒤 별도 gate로 진행합니다.
