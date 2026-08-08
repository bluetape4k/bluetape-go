Fixes #537.

## 요약

- 공식 `concurrency.Session` 및 `concurrency.Election` primitive를 기반으로
  하는 etcd v3 leader-election provider를 추가한다.
- 보호하는 경계: lease, exact-key watch, Proclaim, cancellation, response-loss, cleanup,
  caller-owned client shutdown 경계에서 fail-closed 동작을 보장한다.
- 검증 범위: digest-pinned real-etcd conformance, authorization, contention, resource,
  hard-stop, race coverage 검증을 추가한다.
- 영어/한국어 사용법, capacity, TLS/RBAC, migration, rollback,
  release-runbook 지침을 제공한다.
- 활성 dependency 및 rollback 보안 기준을 `x/crypto v0.52.0`과
  `x/net v0.55.0`으로 올린다.

## 계약

- Caller가 `*clientv3.Client` ownership을 유지한다.
- 모든 Campaign은 provider-owned Session을 생성한다. raw Session option,
  session adoption, restart-resume, fencing token API는 노출하지 않는다.
- `Campaign`은 synchronous하다. Caller는 보호할 각 work unit에 대해 실행
  가능한 `IsLeader` guard를 받고, 다시 획득하기 전에 작업을 중지하고 join해야
  한다.
- 별도의 healthy client가 정확한 candidate-range 부재와 etcd contender
  0개를 증명한 뒤에만 cleanup inventory를 비운다.
- 서로 신뢰하지 않는 tenant는 별도의 etcd cluster를 사용해야 한다.

## 검토

- Step 2-R 설계 검토, Step 3-P risk prediction, Step 3-R plan review,
  Step 6-R 7-tier review 산출물이 `docs/superpowers/` 아래에 포함되어 있다.
- Step 6-R 정확한 implementation head:
  `f5d24a83b08777cced3ede65c755af061417556b`.
- Step 6-R 검토 결과: `P0=0 P1=0 P2=0 P3=1`.
- 허용된 P3는 사용하지 않는 module-only `x/crypto/openpgp` advisory
  `GO-2026-5932`다. import/call path가 없고 고정된 module version도 없다.

## 검증

- PASS 검토한 implementation head에서 `make ci` — 578 seconds.
- PASS 전체 `leader/leadertest` 및 `leader/etcd` normal/race suite.
- PASS 다음 real etcd `v3.6.13` 검증: conformance, authorization, 32-contender resource,
  cleanup, hard-stop test.
- PASS proof success, proof failure, zero-contender failure를 포함한
  supervisor rollback ordering test.
- PASS reachable vulnerability 0개, imported-package vulnerability 0개인
  `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...`.
- PASS `make fmt-check`, `make tidy-check`, `make vet`, `make lint`.
- PASS 영어/한국어 문서 및 runbook contract 검사.
- PENDING 게시된 PR head의 GitHub CI.

## DoD Status

- [x] etcd leader provider와 constructor-only public contract를 구현했다.
- [x] exact ownership-loss 및 proof-gated cleanup semantics를 구현했다.
- [x] caller-owned client shutdown과 rollback ordering을 bounded하고
      fail-closed하게 만들었다.
- [x] real-server, contention, authorization, failure, normal, race test를
      추가했다.
- [x] capacity, TLS/RBAC, observability, migration, rollback, bilingual docs를
      추가했다.
- [x] Type A reusable lesson을 commit했다.
- [x] P0=0 P1=0 P2=0으로 Step 6-R six-lane review를 완료했다.
- [x] 최종 로컬 게이트를 완료했다.
- [ ] GitHub CI 및 Step 7-R exact-PR-head review 대기.
- [ ] 모든 remote 게이트가 통과한 뒤 fresh explicit approval이 있어야
      병합할 수 있다.
