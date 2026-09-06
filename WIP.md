# 진행 상황

기준 시각: 2026-09-07 KST
범위: `v0.22.0` foundation 구현 및 후속 delivery gate.

## Issue #555 graph backend conformance

- `[x]` `go test -race -count=10 ./graph/graphtest`
- `[x]` 독립 process 3회 `go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -v -timeout=10m`
- `[x]` legacy Neo4j/Memgraph integration과 shared suite의 같은 tree parity
- `[x]` migration 뒤 `go test -count=1 ./graph/neo4j -timeout=10m`
- `[x]` `make ci`
- `[x]` exact-head Testcontainers Nightly
- `[x]` Step 6-R 7-Tier review `P0=0 P1=0 P2=0 P3=0`
- Migration commit: `5d73079cd2ce2f8403cb068e7816b399c823e76d`
- 구현·리뷰 수정 source HEAD: `ec98e00276ffacd98e559ea4177f16fb58df31d0`
- local canonical validation SHA: `76707c2fd255c24cf27e3f96d5b75fad20c8e1a8`
- pre-integration PR head: `87f9bc3778461a59964de9b1354db3f245e3f205`
- Base/head: `develop` / `feat/issue-555-graph-conformance`
- PR number/URL: #736 / https://github.com/bluetape4k/bluetape-go/pull/736
- Required CI: run `34026850947`, `SUCCESS`, head
  `e59992cfb6fc9b0931fc7a2e3ab8a6767e147806`, completed
  `2026-09-06T10:29:38Z`
- `origin/develop` integration commit: `5a461a28f56fa4b049240092e9baa3eba4a4cefe`
- Post-integration local `make ci`: `PASS` (exit 0)
- Testcontainers Nightly: run `34027924828`, `SUCCESS`, head
  `e59992cfb6fc9b0931fc7a2e3ab8a6767e147806`, completed
  `2026-09-06T10:49:48Z`
- Merge commit: `0407766723625cd08951c4b92e89ccafc69b231c`

세 독립 process의 전체 suite 시간은 각각 16.77초, 16.34초, 16.33초였다.
Neo4j와 Memgraph는 digest-pinned image에서 strict core와 traversal을 skip 없이
통과했다. `callback join → fixture cleanup → adapter close → Run 반환 → container
terminate` 순서와 redacted provider 진단을 확인했다. Legacy/shared parity 뒤 중복
integration body를 제거했지만 `benchmark_test.go`가 공유하는
`waitForMemgraphConnectivity`와 `memgraphBoltPort`는 보존했다.

Step 6-R은 startup deadline 뒤 늦게 반환된 adapter 누수 P1 한 건과 startup failure
진단 및 caller 문서 P2 네 건을 발견해 모두 수정했다. Delta verifier와 main
integration review의 최종 판정은 `P0=0 P1=0 P2=0 P3=0`이다. Exact head
`76707c2fd255c24cf27e3f96d5b75fad20c8e1a8`에서 `make ci`가 exit 0으로
통과했다.

## 현재 대상 릴리스

`v0.22.0`은 좌표/Geohash와 graph backend conformance foundation을 묶는
릴리스입니다. Issue #548의 외부 dependency 없는 `geo` package는 PR #735로
merge됐습니다. Tag와 publication은 milestone open issue가 0이 된 뒤 별도
gate에서 진행합니다.

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

- #548은 PR #735로 merge됐고 `develop`의
  `c48c5c8e0f4edfb12436849ac52d3f04793f6ab6`에 반영됐습니다.
- #555는 PR #736으로 merge됐고 exact-head CI와 Testcontainers Nightly가
  모두 통과했습니다.
- `#551` PostGIS는 `b8e79534`로, `#552` MySQL/MariaDB GIS는 `629b589e`로,
  `#554` Nominatim reverse geocoding은 `5cbc183e`로 구현했습니다.
- `#547` FalkorDB OpenCypher adapter는 `4a1fdc82`로, `#561` remote
  Gremlin/TinkerPop adapter는 `c896e254`로 구현했습니다. Gremlin 중첩 결과
  상한 보정은 `94aa3ef`, lint·errcheck·staticcheck 계약 보정은 `d956c97`에
  반영했습니다. 각 slice는 caller-owned client/credential/lifecycle,
  bounded result/error, context 경계와 digest-pinned local fixture를 유지합니다.
- 다섯 구현 slice와 fixture는 `feat/milestone-0.22.0-integration` 한 통합
  branch에서 한 번의 PR/squash merge를 목표로 합니다. 현재 package-level
  테스트, race, vet, lint(`0 issues.`), 실제 PostGIS/MySQL/MariaDB/FalkorDB/
  TinkerPop fixture 검증과 전체 `make test`, `make race`, `make ci`가
  통과했습니다. PR exact-head GitHub CI·review/thread read-back, merge와
  post-merge sync는 아직 남아 있습니다.
- PR #738의 첫 exact-head CI run `34046283255`는 coverage 단계에서
  `graph/gremlin`의 TinkerPop factory가 TCP port만 열린 순간 연결을 시도해
  실패했습니다. `7053dae`에서 `Channel started at port 8182.` log까지 기다리는
  readiness를 추가했고, 로컬 동일 경계 5회 반복 테스트가 통과했습니다. 수정
  head를 push한 뒤 원격 CI를 다시 확인해야 합니다.
- 최종 검증 중 기존 `leader/etcd`의
  `TestBlockedOfficialCampaignCleanupRequiresClientHardStop`이 full-suite에서 한 번
  실패했습니다. Exact test 5회와 전체 `leader/etcd` package 3회가 연속 통과했고,
  이어서 실행한 full `make ci`도 통과했습니다. 이번 diff는 `leader/etcd`를 변경하지
  않으며 Go 기본값과 같은 10분 timeout을 명시하므로 기존 2초 관찰 timing flake로
  분류했습니다.
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
