# #522 Step Functions 실행 bridge 교훈

## 맥락

workflow/stepfunctions는 AWS SDK for Go v2의 StartExecution,
DescribeExecution, 선택적 StopExecution만 주입받아 외부 execution을
관찰한다. State machine 정의·배포, credential·IAM·retry·redrive policy와
execution lifecycle은 caller/operator가 소유한다. 기본 CI는 fake client만
사용한다.

## 실패한 가정과 증거

저장소 전체 make ci의 첫 실행에서 Go normal/race와 package 검증은
통과했지만 check-bench-web-gin의 1초 chart-timeout fixture가 간헐적으로
chart renderer timeout stderr를 남기지 못했다. 실패 fixture를 반복 실행해
재현했고, scripts/capture-gin-adapter-benchmark.sh가 SECONDS 정수 초
값으로 경과 시간을 비교해 child가 stderr를 쓰기 전에 timeout을 판정할 수
있음을 확인했다.

즉, “1초 timeout이면 정수 초 시계로도 충분하다”는 가정이 fixture 경계에서
틀렸다. 이 문제는 Step Functions API 자체의 장애가 아니라 외부 실행을
polling하는 검증 harness의 경계 오차였다.

## 결정

chart-timeout 로직의 동작 범위를 넓히지 않고 chart_elapsed_ms 50ms 누적
counter를 사용하도록 최소 수정했다(scripts/capture-gin-adapter-benchmark.sh:371-392).
출력 크기 제한, process 종료, failure metadata와 publication rollback은
그대로 유지했다. 수정 후 make check-bench-web-gin을 5회 연속 실행하고
전체 make ci를 재실행해 모든 normal/race/package/chart guard가 통과했다.

동시에 외부 execution polling에는 다음 불변 조건을 적용했다.

- positive bridge timeout 또는 명시된 caller deadline 중 하나가 wait budget을
  소유한다.
- time.NewTimer와 context select로 대기를 취소하고 timer를 정리한다.
- custom/default backoff는 max interval으로 cap하며 음수는 거부한다.
- 알려진 terminal status만 완료로 해석하고 unknown status는 fail closed 한다.
- cancellation을 이유로 자동 StopExecution이나 retry를 실행하지 않는다.
- provider response 직후 parent context를 다시 확인해 늦은 성공을 반환하지 않는다.
- fake가 request 복사, 호출 순서, delay/backoff, terminal failure, timeout,
  cancellation과 no-live-credential 경계를 검증한다.

이 조건은 bluetape-go-patterns의 GO-HARD-07/08에 반영했다. GO-HARD-08은
외부 실행 adapter가 유한한 대기 예산 또는 caller deadline을 명시하고,
cancellable timer, capped backoff, terminal allowlist, unknown-status 정책,
implicit side effect 금지와 late response 차단을 fake/race 증거로 고정하도록
한다.

## 결과와 후속 guard

- workflow/stepfunctions의 fake-first 성공/실패/malformed/timeout/
  cancellation/backoff/race 계약은 유지된다.
- EN/KO README에 AWS STANDARD/EXPRESS idempotency, 90일 이름 재사용,
  DescribeExecution eventual consistency, StopExecution 제한과
  PENDING_REDRIVE의 비-성공 의미를 함께 기록했다.
- Describe는 요청 ARN과 다른 response ARN을 malformed로 거부해 caller
  identity를 보존한다. AWS가 허용하는 order.v1 같은 response name은
  손실시키지 않는다.
- 다음 외부 provider adapter에서는 정수 초/분 단위 timeout을 사용하지 말고
  monotonic 또는 sub-second-safe 측정과 경계 fixture를 먼저 검증한다.
- PENDING_REDRIVE를 전용 오류로 노출할지는 redrive API를 추가하는 후속
  issue에서 caller compatibility를 검토한다. 현재 bridge는 상태를 보존해
  반환하고 redrive side effect를 하지 않는다.

## 검증

go test -count=1 ./workflow/stepfunctions                  PASS
go test -race -count=1 ./workflow/stepfunctions           PASS
go vet ./workflow/stepfunctions                           PASS
golangci-lint run ./workflow/stepfunctions/...             0 issues
make fmt-check                                             PASS
make tidy-check                                            PASS
make vet                                                   PASS
make lint                                                  PASS
make test                                                  PASS
make race                                                  PASS
make check-bench-web-gin (5회)                             PASS
make ci                                                    PASS

Live AWS credential, state-machine provisioning/deploy, remote CI와 real-cloud
latency는 이 lesson의 대상이 아니며 별도 gate로 남긴다.
