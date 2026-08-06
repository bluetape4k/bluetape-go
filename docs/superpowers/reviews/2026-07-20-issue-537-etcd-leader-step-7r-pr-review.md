# 이슈 #537 etcd 리더 선출 Step 7-R PR 리뷰

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #537 `feat: Add etcd leader election backend`

PR: #613 `feat: add etcd leader election backend`

날짜: 2026-07-20

기준 및 merge base: `41663dea0a2a34cd459df24802f59882cff834aa`

검토한 정확한 PR HEAD: `636fa0f059aa786f414253c984d0b9cbab150a16`

검토한 구현 SHA: `f5d24a83b08777cced3ede65c755af061417556b`

게이트: 독립적인 여섯 관점과 메인 세션 통합 검토.

## 라이브 메타데이터

| 항목 | 라이브 결과 |
|---|---|
| PR 상태 | OPEN, non-draft |
| Base / head | `develop` / `feat/issue-537-etcd-leader` |
| 담당자 | `debop` |
| 마일스톤 | `0.19.0` |
| 레이블 | `type: task`, `area: leader`, `area: testing`, `priority: p2` |
| Merge 가능성 | MERGEABLE |
| 리뷰 / 코멘트 / 미해결 thread | 0 / 0 / 0 |
| 정확한 HEAD CI | SUCCESS: run `29693917829`, 13분 11초 |

## PR 리뷰 수렴 이력

최초 PR HEAD `c02f75c357e583d31d17e78c8c54c8f35d71f3f1`의 원격 CI는 성공했다.
Step 7-R은 PR 공개 표면까지 검토하면서 다음 finding을 발견하고 수정 후 모든 관점을
새 HEAD에서 다시 검토했다.

| 커밋 | 해소한 finding |
|---|---|
| `24a0eff` | 공개 rollback 문서를 실행 가능한 supervisor 계약과 같은 `exact proof → zero contenders → restore` 순서로 고치고 순서 회귀 테스트를 추가했다. |
| `68aa0d8` | 영문 rollback 문법을 보정하고 blocked contender의 campaign wait watch까지 capacity inventory에 포함했다. |
| `636fa0f` | zero-contender 이전 restore를 금지하는 음성 회귀를 추가하고, 명시적 `KeyPrefix`, 재현 가능한 candidate root 계산, `RunWithConfig` fail-stop/outer-timeout 계약을 공개 문서와 compile-checked example에 고정했다. |

최종 HEAD `636fa0f059aa786f414253c984d0b9cbab150a16`은 로컬 HEAD, 원격 branch,
PR `headRefOid`가 모두 일치하는 상태에서 검토했다. Worktree는 clean이었다.

## 최종 여섯 관점 결과

| Tier | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | 성능 | PASS | 0 | 0 | 0 | 0 |
| 2 | 안정성 | PASS | 0 | 0 | 0 | 0 |
| 3 | 보안 | PASS | 0 | 0 | 0 | 1 |
| 4 | 운영/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | 개발자/API | PASS | 0 | 0 | 0 | 0 |
| 6 | 사용자/호출자 | PASS | 0 | 0 | 0 | 0 |
| 메인 | 통합 | PASS | 0 | 0 | 0 | 1 |

### 성능

- 최소 100ms cadence와 synchronous single-flight `Proclaim` 계약을 다시 확인했다.
- Blocked contender당 predecessor campaign wait watch 최대 1개와 published group당
  ownership exact-key watch 1개를 capacity envelope에 포함했다.
- 32 contender 자원 회수, cadence, single-flight, targeted normal/race 검증이 통과했다.

### 안정성

- Campaign publication/cancellation, generation 단위 cleanup 직렬화, monitor/session join,
  hard-stop, exact-key proof 경계를 다시 확인했다.
- Rollback은 exact-range 부재 증명 후 zero contenders를 확인하고 그 다음에만 이전
  provider를 복원한다.
- 공개 문서의 이 순서는 음성 회귀 테스트로 고정돼 조기 restore/복원을 거부한다.

### 보안

- `KeyPrefix`와 `Group`의 base64url-no-padding 계산, exact RBAC range, TLS hostname 검증,
  cross-principal revoke/keepalive denial, 신뢰하지 않는 tenant의 cluster 격리를 확인했다.
- 명시적 production `KeyPrefix`와 runbook의 candidate root 계산이 실제 provider 경로와
  일치한다.
- `govulncheck@v1.6.0`은 reachable/imported-package 취약점 0건을 보고했다.

### 운영/Ops

- Capacity, watcher budget, bounded-cardinality telemetry, quorum recovery, hard-stop,
  unresolved inventory, migration/rollback gate가 영문/한국어 문서에서 일치한다.
- Runbook은 `ETCD_KEY_PREFIX`와 `ETCD_GROUP`을 필수로 받고 exact candidate root를
  재현한다.

### 개발자/API

- Constructor-only `New`, caller-owned client, 기존 `leader.Elector` 호환성,
  context/cancellation, typed error, opaque owner token 계약을 확인했다.
- `RunWithConfig`는 기존 `Run` wrapper와 호환되고 `CaseTimeout`, `Abort`, outer
  `go test -timeout`의 fail-stop 경계를 Go doc과 example로 노출한다.

### 사용자/호출자

- Production 예제는 명시적 `KeyPrefix`를 사용하고 운영자는 공개 문서만으로 RBAC 및
  cleanup 범위를 재현할 수 있다.
- Failure recovery, protected-work join, rollback, bilingual usage, conformance timeout
  guidance가 호출자 관점에서 일치한다.

## 수용한 비차단 finding

보안 P3 `GO-2026-5932`는 `golang.org/x/crypto/openpgp`의 module-only advisory다.
저장소에 import/call 경로가 없고 수정된 module 버전도 없으며 pinned scan은 reachable 및
imported-package 취약점 0건을 보고한다. `openpgp` import를 추가하지 않고 승격 및 rollback
게이트에서 pinned scan을 유지한다.

## 검증 증거

- 전체 `leader/leadertest` 및 `leader/etcd` 일반 테스트 — PASS.
- 전체 `leader/leadertest` 및 `leader/etcd` race 테스트 — PASS.
- 문서, rollback, runbook, example 계약 테스트 20회 — PASS.
- 같은 문서/example 범위 race 테스트 5회 — PASS.
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint` — PASS.
- `go mod verify` — PASS.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` — reachable 0,
  imported-package 0.
- `git diff --check` — PASS.
- GitHub Actions run `29693917829` — SUCCESS, 13분 11초. Formatting, tidy,
  vet, lint, Testcontainers+coverage, race Testcontainers를 모두 포함한다.

## 메인 통합 판정

코드·문서·공개 계약은 PASS이며 `P0=0 P1=0 P2=0 P3=1`이다. 마지막 P3는 비도달
module-only advisory로 수용했다. 검토한 정확한 HEAD의 GitHub CI도 성공했다. 이 리뷰
기록은 후속 artifact-only commit으로 publish하며, 그 최종 PR HEAD의 CI와 리뷰/thread
상태를 다시 확인한 뒤 merge-ready를 선언한다.

## DoD

| 항목 | 상태 |
|---|---|
| 로컬/원격/PR exact HEAD 일치 | 완료: `636fa0f059aa786f414253c984d0b9cbab150a16` |
| 독립적인 여섯 관점 최종 검토 | 완료 |
| P0/P1/P2 정규화 | 완료: `P0=0 P1=0 P2=0` |
| 수용한 P3 기록 | 완료: 도달 불가능한 module-only advisory 1건 |
| 로컬 normal/race/static/docs 검증 | 완료 |
| 정확한 검토 HEAD 원격 CI | 완료: run `29693917829` SUCCESS |
| 리뷰/코멘트/미해결 thread | 완료: 0 / 0 / 0 |
| Step 7-R artifact publish | 완료: 이 문서를 포함한 후속 artifact-only commit |
| Merge 부수 효과 | 미승인: 새로운 명시적 merge 승인이 필요함 |
