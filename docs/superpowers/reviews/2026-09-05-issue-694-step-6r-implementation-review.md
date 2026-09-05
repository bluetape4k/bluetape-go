# Issue #694 Step 6-R 구현 검토

## 검토 기준

- 이슈: [#694](https://github.com/bluetape4k/bluetape-go/issues/694)
- base: `7deddf7c17d673c6e62cc917b36a08afd3b38266`
- 범위: `web/echo` JWT dispatch, cancellation 회귀 테스트, 양국 README,
  설계 문서
- 방법: performance, stability, security, operator/Ops, developer/API,
  user/caller의 6개 독립 lane과 main-session 통합 검토
- 제한: exact-head GitHub CI와 live PR review는 PR 생성 뒤 CG-14에서 확인한다.

## 6개 관점 결과

| 관점 | P0 | P1 | 결론 | 지적과 처리 |
| --- | ---: | ---: | --- | --- |
| performance | 0 | 0 | APPROVE | 자동 승격 benchmark 공백은 비차단 P2로 유지했다. request별 type assertion P3는 `NewJWT` 생성 시 1회 검사로 수정했다. |
| stability | 0 | 0 | 수정 후 APPROVE | 자동 승격 경로의 in-flight cancellation·late-success 증거 P2를 새 회귀 테스트로 보강했다. |
| security | 0 | 0 | APPROVE | 인증 우회, token·원인 노출, fail-open 경로가 없음을 확인했다. legacy blocking 위험은 문서화된 caller-owned P2다. |
| operator/Ops | 0 | 0 | APPROVE | 새 goroutine과 로그 노출이 없고, stuck request는 `ContextParser` 이행과 server deadline으로 통제한다. |
| developer/API | 0 | 0 | 검증 후 APPROVE | public API/ABI, exactly-one parser 검증, typed-nil, explicit parser 우선순위를 보존했다. 상위 lane이 full/race/static 검증을 완료했다. |
| user/caller | 0 | 0 | APPROVE | 양국 README parity와 이행 안내가 일치한다. 설계 상태를 local PASS·exact-head CI PENDING으로 수정했다. |

## 보강한 회귀 계약

1. `Parser` 값이 `ContextParser`도 구현하면 `Parse`가 아니라
   `ParseContext`를 호출하고 request context를 전달한다.
2. 자동 승격 provider가 cancellation을 관찰한 뒤 성공 reader를 반환해도
   middleware는 post-cancel check로 reader를 publish하지 않고 redacted 401을
   반환한다.
3. legacy-only blocking parser는 adapter가 별도 goroutine으로 분리하지 않는다.
   호출 중 취소되면 provider 반환까지 기다린 뒤 active call이 0이 되고
   downstream은 호출하지 않는다.
4. explicit `JWTOptions.ContextParser`가 자동 승격보다 우선하며, 기존
   exactly-one parser와 typed-nil 검증은 유지한다.

## 검증 증적

- 신규 4개 cancellation 테스트: `-count=50` PASS
- 신규 4개 cancellation 테스트: `-race -count=10` PASS
- `go test -count=1 ./web/echo`: PASS
- `go test -race -count=1 ./web/echo`: PASS
- `go test -count=1 ./web/gin`: PASS
- `make test`: PASS
- `make race`: PASS
- `make tidy-check fmt-check vet lint check-bench-web-gin`: PASS
- Echo examples, `git diff --check`, 한국어 용어 audit: PASS
- code-quality checker: finding 0

첫 `make ci` 일반 test 구간에서 `ratelimit/redis` 세 테스트가 Redis 응답
timeout으로 실패했다. Colima와 Docker socket 상태가 정상임을 확인한 뒤 해당
package 단독 전체 PASS, `make test` 전체 재실행 PASS, `make race` 전체 PASS를
확보해 변경과 무관한 일시적 Testcontainers timing으로 격리했다.

## 남은 위험과 최종 판정

- legacy-only blocking provider는 반환할 때까지 request goroutine을 점유한다.
  이는 안전한 강제 종료가 불가능한 기존 호환 경계이며, I/O provider가
  `ContextParser`로 이행하도록 README와 설계에 명시했다.
- 자동 승격 경로 전용 benchmark는 없다. type assertion을 middleware 생성 시
  한 번만 수행하고 request 경로에 새 allocation·goroutine을 추가하지 않으므로
  이번 correctness fix의 merge blocker로 보지 않는다.
- exact-head GitHub CI와 live review/thread 상태는 아직 PENDING이다.

통합 판정은 **P0=0, P1=0, pre-PR APPROVE**다. PR 생성 뒤 exact-head CI가
통과하고 live blocker가 없을 때만 merge-ready로 전환한다.
