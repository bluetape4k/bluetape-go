# #522 Step Functions 실행 bridge 구현 리뷰

## 판정

- 리뷰 대상 head: `4d9c2b1` (`PENDING_REDRIVE` 관찰 의미를 문서 계약에 고정한다)
- 기준 head: `37825af2d5ec083f383bce583fd5fbd9e739c468` (`origin/develop`)
- 범위: `workflow/stepfunctions` 신규 package, AWS SDK `service/sfn` 의존성,
  EN/KO index·README, 실행 계획, 그리고 CI가 드러낸 기존 chart-timeout fixture
  경계 수정
- 리뷰 방식: Type-A 6개 관점(performance, stability, security, operator/Ops,
  developer/API, user/caller)과 main-session integration을 read-only로 대조했다.
  별도 독립 lane 결과는 이 세션에서 사용하지 않았고, repository AGENTS의
  main-session fallback으로 각 관점을 직접 수행했다.
- 아키텍처 상태: **WATCH** — blocking finding은 없지만 `PENDING_REDRIVE`를
  `Wait`가 오류 없이 반환하므로 caller는 반드시 `Execution.Status`를 분기해야
  한다. 이 정책과 redrive ownership을 package README에 명시했다.
- 최종 verdict: **APPROVE** (P0=0, P1=0)

## 기준과 추적성

- Issue: [#522](https://github.com/bluetape4k/bluetape-go/issues/522), parent #517
- 설계: `docs/superpowers/specs/2026-09-03-issue-522-step-functions-design.md`
- 계획: `docs/superpowers/plans/2026-09-03-issue-522-step-functions-plan.md`
- 조사 gate: [bluetape-go AWS research gate](https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-aws-research-gate.md)
- 외부 계약: [AWS `StartExecution`](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StartExecution.html),
  [`DescribeExecution`](https://docs.aws.amazon.com/step-functions/latest/apireference/API_DescribeExecution.html),
  [`StopExecution`](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StopExecution.html)
- 적용한 Go hardening: `bluetape-go-patterns` GO-HARD-07(외부 provider 경계)와
  GO-HARD-08(외부 execution polling). GO-HARD-08은 이번 작업의 finite timeout/
  caller deadline, cancellable timer, capped backoff, status allowlist, no implicit
  stop/retry, late-response cancellation, fake sequence 증거를 요구하도록
  skill source에도 반영했다.

## 6개 관점 리뷰

### 1. Performance

- `workflow/stepfunctions/bridge.go:357-367`에서 positive `Timeout`일 때만 child
  context를 만들며, polling은 `workflow/stepfunctions/bridge.go:519-549`의 단일
  `time.NewTimer`를 사용한다. ticker/goroutine을 누적하지 않는다.
- `workflow/stepfunctions/bridge.go:397-412`의 backoff는 callback 결과를
  `MaxPollInterval`로 cap하고, 기본 backoff overflow도 `bridge.go:509-517`에서
  포화시킨다.
- 입력과 provider payload는 `bridge.go:462-487`에서 사전 byte bound와 복사로
  제한된다. 불필요한 JSON re-encode나 범용 workflow abstraction은 추가하지
  않았다.
- 판정: **PASS**. 기본/사용자 backoff가 bounded이고 race 실행에서 공유 상태
  경합이 관찰되지 않았다.

### 2. Stability

- 모든 public IO 경계는 dispatch 전후 context를 검사한다
  (`bridge.go:167-195`, `218-238`, `296-332`). `Wait`는 response 직후 parent
  cancellation을 재검사해 늦은 성공을 publish하지 않는다(`bridge.go:379-399`).
- `Wait`는 `RUNNING`만 재조회하고 terminal status를 명시적으로 allowlist한다
  (`bridge.go:390-425`). unknown status는 `ErrUnknownStatus`로 fail closed 한다.
- nil/malformed 필수 응답과 Describe response identity mismatch는
  `bridge.go:200-205`, `239-250`, `333-341`에서 성공으로 통과하지 않는다.
- parent cancellation은 bridge timeout보다 우선하며(`errors.go:66-84`,
  `bridge.go:551-558`), owned timeout만 `ErrWaitTimeout`으로 분류한다.
- 판정: **PASS**. targeted normal/race, full `make race`, timeout/cancel/late
  response tests가 통과했다.

### 3. Security

- `Client`/`StopClient`는 SDK method subset만 주입하고 client, credential, region,
  endpoint, IAM, retry, logger 수명은 caller-owned로 유지한다
  (`bridge.go:27-37`, README).
- Start input, ARN, name, trace header와 Stop error/cause를 provider 호출 전에
  byte/UTF-8/문자 집합으로 제한한다(`bridge.go:450-477`, `568-627`).
- `*Error`는 allowlisted kind/operation/status와 원인 wrapping만 노출하며
  provider message, payload, ARN, credential, trace header를 문자열에 넣지 않는다
  (`errors.go:41-154`). `%+v` redaction test도 동일 계약을 확인한다.
- 판정: **PASS**. live credential/network를 요구하는 경로, raw provider log,
  global client/credential state가 없다.

### 4. Operator/Ops

- `Stop`은 client capability가 있을 때만 실행되고(`bridge.go:309-341`), `Wait`는
  자동 stop/retry를 하지 않는다(`bridge.go:340-345`). IAM, retry, timeout,
  redrive와 execution lifecycle은 운영자가 명시적으로 선택한다.
- README EN/KO에 eventual consistency, EXPRESS의 Describe/Stop 제한,
  STANDARD idempotency와 90일 이름 재사용, payload/error/cause bounds,
  polling timeout/backoff를 함께 기록했다.
- CI에서 발견한 `SECONDS` 정수 초 경계 race는
  `scripts/capture-gin-adapter-benchmark.sh:371-392`의 50ms elapsed counter로
  최소 범위만 보정했다. `make check-bench-web-gin` 5회 반복과 이후 full CI
  출력이 모두 PASS다.
- 판정: **PASS**. live AWS smoke는 의도적으로 N/A이며 caller/operator가
  endpoint와 운영 policy를 소유한다.

### 5. Developer/API

- `Options{Client, MaxInputSize}`, 좁은 `Client`, 선택적 `StopClient`,
  `StartRequest`/`StopRequest`/`WaitOptions`와 typed sentinels가
  `bridge.go:27-83`, `errors.go:8-38`에 명확히 정의되어 있다.
- zero-value `Bridge`와 typed-nil client는 panic 없이 명시적 오류를 반환한다
  (`bridge.go:149-162`, `439-447`). input slice와 response payload는 독립 복사본이다.
- Describe는 caller가 요청한 execution ARN과 provider response ARN이 다르면
  malformed로 거부하고(`bridge.go:243-247`), AWS가 허용하는 `order.v1` 같은
  response name은 보존한다(`bridge.go:587-603`).
- 기존 `workflow` runner API와 provisioning/deploy surface를 변경하지 않고
  `service/sfn`만 직접 의존한다(`go.mod:5-18`). public example과 EN/KO README
  index가 compile/read-back 되었다.
- 판정: **PASS**. Go doc, `go vet`, `golangci-lint`가 통과했다.

### 6. User/Caller

- `Start`는 nil/empty input을 `{}`로 보내고 caller bytes를 재정렬하지 않는다
  (`bridge.go:450-477`). `Describe`/`Wait`는 status, input/output, Error/Cause,
  start/stop time을 caller가 관찰할 수 있게 한다(`bridge.go:252-287`).
- terminal `FAILED`/`TIMED_OUT`/`ABORTED`는 마지막 `Execution`과 상태별 typed
  error를 함께 반환한다(`bridge.go:414-421`). `PENDING_REDRIVE`는 알려진 관찰
  결과로 반환되며 성공과 동일하지 않다는 점을 문서화했다.
- fake는 request를 deep-copy하고 call sequence/context를 기록한다
  (`bridge_test.go:19-124`). 성공, 실패, timeout, cancellation, backoff,
  no-implicit-stop, redaction, concurrent isolation을 검증한다.
- 판정: **PASS**, 단 `PENDING_REDRIVE` caller branching은 WATCH 항목으로 남긴다.

## Findings

| 우선순위 | 상태 | 위치 | 내용 및 조치 |
| --- | --- | --- | --- |
| P0 | 0건 | - | 데이터 손실·보안·전체 장애를 유발할 blocking finding 없음 |
| P1 | 0건 | - | cancellation, timeout, identity, error wrapping, unbounded polling finding 없음 |
| P2 | 1건(WATCH) | `workflow/stepfunctions/bridge.go:422-423` | `PENDING_REDRIVE`는 `nil` error로 반환되므로 caller가 nil을 성공으로 오해할 수 있다. AWS 상태와 redrive ownership을 EN/KO README에 명시했고, 후속 API 확장 때 전용 sentinel 도입 여부를 재검토한다. |
| P3 | 0건 | - | 현재 변경에서 별도 polish-only finding 없음 |

## GO-HARD-08 증거

| 요구 | 증거 |
| --- | --- |
| timeout/deadline ownership | `WaitOptions.Timeout` positive child context, zero는 caller context/deadline only (`bridge.go:357-367`, README) |
| cancellable timer | `time.NewTimer` + parent/child `select`와 drain (`bridge.go:519-549`) |
| capped backoff | custom/default callback, negative reject, max cap (`bridge.go:397-412`, tests `472-507`) |
| terminal allowlist/unknown policy | `isKnownStatus`/switch (`bridge.go:390-425`, `630-637`), unknown test `bridge_test.go:333-347` |
| no implicit stop/retry | `Wait` path has no Stop call; test `bridge_test.go:509-533` |
| response-boundary cancellation | Start/Describe/Stop/Wait post-response checks and tests `235-248`, `447-460`, `641-654` |
| fake sequencing/resource proof | deep-copy fake (`bridge_test.go:19-124`), backoff call order (`472-507`), `go test -race`, full `make race` |
| bounded CI fixture | millisecond-safe chart timeout (`scripts/capture-gin-adapter-benchmark.sh:371-392`), five guard repetitions and full `make ci` |

## SPW-01..05 및 검증

- SPW-01: PASS — Go caller/operator 독자, issue/spec/research/AWS source, no-live-
  AWS/provisioning 범위를 기록했다.
- SPW-02: PASS — API 경계, failure/cancellation, limits, tests, compatibility,
  rollback과 DoD를 계획/README에 포함했다.
- SPW-03: PASS — reader-facing review/lesson/README는 Korean prose를 사용하고
  code/API/commands/URLs/status token은 보존했다.
- SPW-04: PASS — issue #522 및 parent #517과 AWS API contract, local workflow
  boundary, GO-HARD-07/08를 exact file/line에 연결했다.
- SPW-05: PASS — EN/KO headings, code block, limits, URLs를 read-back했고
  `git diff --check`가 통과했다.

실행한 검증:

```text
go test -count=1 ./workflow/stepfunctions                  PASS
go test -race -count=1 ./workflow/stepfunctions           PASS
go vet ./workflow/stepfunctions                           PASS
golangci-lint run ./workflow/stepfunctions/...             0 issues
go test -run '^Example' -count=1 ./workflow/stepfunctions PASS
make fmt-check                                             PASS
make tidy-check                                            PASS
make vet                                                   PASS
make lint                                                  PASS
make test                                                  PASS
make race                                                  PASS
make check-bench-web-gin (5회)                             PASS
make ci                                                    PASS
```

Live AWS credentials, state-machine provisioning/deploy, and remote PR CI are
별도 gate이므로 이 로컬 리뷰에서는 N/A/PENDING으로 남긴다. PR exact-head CI와
fresh merge approval 없이는 merge verdict를 확정하지 않는다.
