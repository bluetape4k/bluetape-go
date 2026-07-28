# 7-Tier Review - Issue #125 Coverage Reports

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Add Go native coverage generation to local developer commands.
- Publish CI and Nightly coverage artifacts.
- Keep race tests and coverage tests as separate validation paths.

## 발견 사항

| Severity | Count | Notes |
|---|---:|---|
| P0 | 0 | No blocking correctness issue found. |
| P1 | 0 | No high-risk workflow or release-blocking issue found. |
| P2 | 0 | No medium-risk issue found. |
| P3 | 0 | No low-risk issue found. |

## Tier Notes

1. Stability: coverage uses the same Testcontainers-backed suite as `make test`.
2. Performance: coverage instrumentation is limited to CI/Nightly test jobs; race remains separate.
3. Operations: artifacts are uploaded with `if-no-files-found: warn` so cleanup steps do not mask the real test failure.
4. User: README command tables document local report generation.
5. Security: no new token, secret, or external upload target is introduced.
6. Integration: CI and Nightly share the same `make coverage` target and publish package subtotals in Step Summary.
7. Maintenance: lesson note records the Go-native choice and follow-up boundary.

## 판정

P0=0 and P1=0. Proceed to validation.
