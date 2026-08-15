# Issue #543 Gin adapter 구현 교훈

## 재사용 가능한 규칙

1. **framework 경계를 먼저 고정한다.** Gin import는 `web/gin` 패키지에만
   두고, 공통 계약은 기존 `web`, `resilience`, `ratelimit`, `jwt` API를
   호출한다. import-boundary 테스트를 구현 초기에 두면 우연한 framework
   결합을 빠르게 발견할 수 있다.
2. **요청 상태는 시도 단위로 저장하고 복원한다.** `*http.Request`, body,
   header, `gin.Context`의 keys/index/params/errors와 response header를
   attempt snapshot으로 다룬다. request pointer를 middleware 진입 전 값으로
   복원하는 `defer`도 반드시 둔다.
3. **이미 커밋된 응답의 오류는 성공으로 위장하지 않는다.** response body를
   덮어쓰거나 재시도하지 않고 `resilience.NonRetryable` marker로 감싸
   caller와 circuit 상태에 실패를 전달한다.
4. **취소 가능성은 parser 계약으로 표현한다.** context-aware parser는
   `ParseContext`를 사용하고, legacy `Parse`만 있는 parser는 pre/post
   `ctx.Err()`를 확인하는 best-effort 경로로 명시한다. blocking legacy
   parser를 별도 goroutine으로 감싸 취소를 가장하는 선택은 잔류 goroutine을
   만들 수 있으므로 피한다.
5. **보안 오류는 구조화된 문제로만 외부에 보낸다.** JWT token, parser
   error, backend raw error는 callback request와 Problem body에서 제거한다.
   typed-nil interface는 constructor에서 거부하고, options/policy slice는
   방어적으로 복사한다.
6. **benchmark evidence는 데이터 계약으로 검증한다.** raw output parser는
   누락·미지 benchmark·중복·비유한 값·실패 행을 거부해야 한다. 반복 sample의
   동일 행은 capture metadata의 `benchmark_count > 1`일 때만 허용하고,
   첫 canonical metadata 값은 Go output의 machine header가 덮어쓰지 못하게
   한다.
7. **수치와 no-regression 결론을 분리한다.** clean tree, fixture identity,
   capture SHA를 함께 보존하고 clean provenance는
   `capture_eligibility=eligible`로 표시하되, 비교 baseline SHA가 없으면
   숫자를 근거로 회귀 없음이라고 주장하지 않고 `no_regression=N/A`로
   기록한다. dirty tree capture도 같은 규칙을 따른다.
8. **문서 언어와 API 문서 규칙을 검증한다.** public README는 한국어/영어
   parity를 유지하고, Go exported identifier doc은 lint가 인식할 수 있도록
   identifier로 시작한 뒤 한국어 설명을 둔다. `revive`, `noctx`가 요구하는
   context 전달과 오류 원인 비교(`errors.Is`)도 테스트 코드까지 적용한다.

## 이번 작업에서 확인한 실패 원인

- benchmark parser가 Go benchmark output의 `cpu: Apple M5` 행을 configured
  CPU metadata보다 나중 값으로 채택하면 결과가 잘못된 CPU에 귀속된다. 첫
  canonical metadata를 보존하도록 parser를 수정했다.
- 반복 benchmark row를 일반 duplicate로 거부하면 `BENCH_COUNT=5` capture를
  읽을 수 없다. 반대로 single-sample duplicate를 허용하면 손상된 output을
  통과시키므로 capture count를 명시적인 허용 조건으로 사용했다.
- `make ci`에서 Testcontainers PostgreSQL renewal 테스트가 한 번 timeout된
  뒤 동일 테스트의 `-race -p 1 -count=1` 재실행은 통과했다. 전체 `make ci`
  재실행도 통과했으므로 기능 결함으로 확정하지 않았지만, CI에서 같은 증상이
  반복되면 renewal timing과 container lifecycle을 별도 조사해야 한다.

## 다음 수정자가 피해야 할 선택

- Gin 타입을 framework-neutral `web` 또는 `webtest`에 역수입하지 않는다.
- callback 편의를 위해 Authorization header나 raw parser/backend error를
  그대로 전달하지 않는다.
- committed response 뒤에 2차 response를 쓰거나 retry를 강제하지 않는다.
- parser/capture 결과를 수동으로 편집하거나 chart 숫자를 raw output과
  분리하지 않는다. canonical artifact는 capture script가 원자적으로 만든
  동일 SHA 묶음으로 갱신한다.
- local benchmark snapshot을 baseline 비교 결과처럼 표현하지 않는다.

## 근거와 재현 명령

- canonical capture: `make bench-web-gin`
- contract fixtures: `make check-bench-web-gin`
- chart self-test: `node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs --self-test`
- 통합 검증: `make fmt-check`, `make tidy-check`, `make vet`, `make lint`,
  `make test`, `make race`, `make ci`
