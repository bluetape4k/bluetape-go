# 이슈 #537 etcd 리더 선출 Step 6-R 코드 리뷰

이슈: #537 `feat: Add etcd leader election backend`

날짜: 2026-07-19

기준 및 merge base: `41663dea0a2a34cd459df24802f59882cff834aa`

검토한 구현 SHA: `f5d24a83b08777cced3ede65c755af061417556b`

브랜치: `feat/issue-537-etcd-leader`

게이트: 독립적인 여섯 관점과 메인 세션 통합 검토.

## 라이브 메타데이터

| 항목 | 라이브 결과 |
|---|---|
| 이슈 상태 | OPEN |
| 마일스톤 | `0.19.0` |
| 담당자 | `debop` |
| 레이블 | `type: task`, `area: leader`, `area: testing`, `priority: p2` |
| Pull request | 이 게이트에서는 아직 생성하지 않음 |
| 원격 CI/리뷰/스레드 | 승인된 PR을 생성하기 전까지 N/A |

## 수렴 이력

완성된 etcd provider에서 검토를 시작했고, 각 수정 후 영향을 받는 모든 관점을
반복해서 새로 검토했다. 정확한 보정 커밋은 다음과 같다.

| 커밋 | 결정 |
|---|---|
| `c024e2d` | campaign, 정리, 시간 제한, 진단 실패 경계를 격리했다. |
| `bb2da02` | cadence, 문서, rollback, 리뷰 증거를 일치시켰다. |
| `73f7f25` | fleet contender, lease, watcher, Proclaim의 명시적 용량 게이트를 추가했다. |
| `6f60d8a` | 연결된 lease의 권한 증거와 서버 버전별 재실행 요구 사항을 고정했다. |
| `c5c3c1d` | 공유 client 종료 위험을 제거하고 campaign join을 제한하며 정리 시도 이력을 보존했다. |
| `ac168e1` | 단위별 리더십 guard를 실행 가능하게 만들고 정확한 정리 증거와 미해결 상태를 분리했다. |
| `ae85445` | `x/crypto`와 `x/net`을 현재 및 rollback 보안 하한으로 올리고 취약점 검사를 고정했다. |
| `c948495` | 미해결 inventory와 provider 복원을 정상 client의 최종 정확한 부재 증명 뒤로 이동했다. |
| `f5d24a8` | contender가 0인 실패 분기를 직접적인 persist 및 복원 금지 회귀 테스트로 고정했다. |

서로 다른 principal의 `KeepAliveOnce`가 권한 검사를 우회한다는 최초 보안 가설은
고정된 etcd `v3.6.13`과 서버의 `checkLeaseRenew` 경로를 대조해 반증했다. 최종
테스트는 principal의 key 범위 밖에 연결된 lease에 대해 `Revoke`와
`KeepAliveOnce`가 모두 거부됨을 증명한다. 서로 신뢰하지 않는 tenant는 여전히
별도 cluster를 사용해야 한다.

최초 Developer/API 검토의 observer 우려는 승인된 비목표로 정리했다. 공개 계약은
의도적으로 제한된 `IsLeader` sampling을 사용하며 비동기 event API를 추가하지 않는다.

최종 lane은 모두 시간 안에 완료되어 메인 세션 fallback이 필요하지 않았다.

## 최종 정확한 HEAD 결과

| Tier | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | 성능 | PASS | 0 | 0 | 0 | 0 |
| 2 | 안정성 | PASS | 0 | 0 | 0 | 0 |
| 3 | 보안 | PASS | 0 | 0 | 0 | 1 |
| 4 | 운영/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | 개발자/API | PASS | 0 | 0 | 0 | 0 |
| 6 | 사용자/호출자 | PASS | 0 | 0 | 0 | 0 |
| 메인 | 통합 | PASS | 0 | 0 | 0 | 1 |

모든 최종 lane은 구현 SHA
`f5d24a83b08777cced3ede65c755af061417556b`를
`41663dea0a2a34cd459df24802f59882cff834aa` 기준으로 검토했다.

## 수용한 비차단 finding

보안 P3: `GO-2026-5932`는 `golang.org/x/crypto/openpgp`를 설계상 안전하지 않고
유지보수되지 않는 package로 표시한다. 이 저장소는 해당 package를 import하거나
호출하지 않고, advisory에 수정된 module 버전이 없으며, 고정된 저장소 검사는 도달
가능하거나 import된 package의 취약점이 0건이라고 보고한다. `openpgp` import를
추가하지 말고 승격 및 rollback 게이트에서 고정된 검사를 유지한다.

## 검증 증거

검토한 구현 SHA에서 새로 확보한 증거:

- `make ci` — 578초 만에 PASS. tidy, formatting, vet, lint, 저장소 일반 테스트,
  저장소 race 테스트, Testcontainers package를 포함한다.
- CI 내부 `make lint` — PASS, `0 issues`.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` — 도달 가능한
  취약점 및 import된 package 취약점 0건으로 PASS.
- `go mod verify` 및 `go mod tidy -diff` — PASS.
- 전체 `leader/leadertest` 및 `leader/etcd` 일반/race suite — PASS.
- supervisor 증명/rollback 테스트 — `-count=50`에서 PASS, 집중 race 실행은
  최대 `-race -count=20`에서 PASS.
- cadence 및 single-flight 테스트 — `-count=10`에서 PASS, 집중 race도 PASS.
- 실제 etcd `v3.6.13` conformance, 권한, 32-contender 자원 반환, hard-stop,
  exact-key watch, 정리 증명 — PASS.
- 영문/한국어 README 및 release runbook 계약 테스트 — PASS.
- `git diff --check` 및 clean-worktree 검증 — PASS.

수렴 과정에서 Docker 기반 etcd soak를 10회 실행해 281초 만에 완료했다. 최종
정확한 HEAD에 대한 신뢰는 새 전체 CI와 위의 최종 lane별 일반, race, 실제 서버
증거에서 얻었다.

## 메인 통합 판정

로컬 PR 준비 상태 PASS.

- P0 = 0
- P1 = 0
- P2 = 0
- P3 = 1, 도달 불가능한 module-only advisory로 수용
- 호출자가 소유하는 client, 공식 Session/Election primitive, exact-key 소유권
  monitoring, 증명 기반 정리, hard-stop 조정, 용량 게이트, 보안 하한, 이중 언어
  운영 지침, Type A lesson이 승인된 설계 및 계획과 일치한다.
- 구현은 이미 승인된 push 및 PR 생성 준비가 되었다.
- PR의 정확한 head가 원격 CI와 Step 7-R을 통과하기 전에는 merge-ready가 아니다.

## DoD

| 항목 | 상태 |
|---|---|
| 라이브 이슈 및 마일스톤 상태 갱신 | 완료 |
| 독립적인 여섯 관점 검토 | 완료 |
| 동일한 정확한 구현 SHA 검토 | 완료: `f5d24a83b08777cced3ede65c755af061417556b` |
| 메인 통합 검토 | 완료 |
| P0/P1/P2 정규화 | 완료: `P0=0 P1=0 P2=0` |
| 수용한 P3 기록 | 완료: 도달 불가능한 module-only advisory 1건 |
| 집중, race, 실제 서버, 정적, 취약점, 전체 CI 증거 | 완료 |
| Type A 재사용 lesson | 완료: `docs/lessons/2026-07-19-issue-537-etcd-leader.md` |
| 원격 CI/리뷰/스레드 | N/A: PR을 아직 생성하지 않음 |
| Merge 부수 효과 | 미승인: 새로운 명시적 merge 승인이 필요함 |
