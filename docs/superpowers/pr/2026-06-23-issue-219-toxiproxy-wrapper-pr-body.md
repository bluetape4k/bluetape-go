Part of #219

## 요약

- #219 messaging/HTTP/fault-injection prioritization matrix를 추가하고 첫
  high-value wrapper 범위로 Toxiproxy를 선택했다.
- `testcontainers/toxiproxy`에 `Start`, `StartServer`, `StartContainer`,
  `ProxiedEndpoint` 및 문서화된 `toxiproxy.control_uri` connection detail key를
  추가했다.
- latency sleep 없이 사용할 수 있는 failure injection을 증명하는
  Redis-through-Toxiproxy integration coverage를 추가했다.
- control URI export, dynamic-port 지침, bounded timeout 메모, 직렬 Docker
  test 지침, 명시적 보류 범위를 담은 English 및 Korean README를 추가했다.

## 범위 결정

- Toxiproxy는 messaging adapter를 선택하지 않고 resilience/failure-mode
  test를 직접 지원하므로 범위에 포함한다.
- RabbitMQ, Redpanda, Pulsar는 outbox adapter semantics가 broker를 선택할
  때까지 #58로 보류한다.
- WireMock, Nginx, Mailpit는 `httptest`로 충분하지 않음을 증명하는 #224
  또는 구체적인 recipe issue가 생길 때까지 보류한다.
- ElasticMQ/SQS/SNS 호환 fixture는 #220 및 #61-#64와 계속 조정한다.

## 검증

- `go test -p 1 -count=1 ./testcontainers/toxiproxy`
- `go test -race -p 1 -count=1 ./testcontainers/toxiproxy`
- `go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy`
- `go test -race -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy`
- `rg -n "toxiproxy.control_uri|BLUETAPE_TOXIPROXY_CONTROL_URI|dynamic|bounded|#58|#224|#220" testcontainers/toxiproxy/README.md testcontainers/toxiproxy/README.ko.md`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `make ci`
- `git diff --check`

## 검토 증거

- Step 2-R spec 검토:
  `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-2r-spec-review.md`
  `P0=0 P1=0`으로 기록했다.
- Step 3-R plan 검토:
  `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-3r-plan-review.md`
  `P0=0 P1=0`으로 기록했다.
- Step 6-R 코드 검토:
  `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-6r-code-review.md`
  `P0=0 P1=0`으로 기록했다.
- session 지침에 따라 7-Tier review lane에는 main integration fallback을
  사용했다. 여섯 관점은 추적 대상 review 산출물에 기록되어 있다.

## DoD Status

| 단계 | 상태 | 증거 |
|---|---|---|
| Step 1-R 이슈 분류 | PASS | 새 Testcontainers package와 dependency 범위를 추가하므로 #219를 Type A full-feature로 분류. |
| Step 2 spec | PASS | `docs/superpowers/specs/2026-06-23-issue-219-toxiproxy-wrapper-design.md`. |
| Step 2-R spec review | PASS | `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-2r-spec-review.md`, `P0=0 P1=0`. |
| Step 3 plan | PASS | `docs/superpowers/plans/2026-06-23-issue-219-toxiproxy-wrapper-plan.md`. |
| Step 3-R plan review | PASS | `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-3r-plan-review.md`, `P0=0 P1=0`. |
| Step 4 TDD | PASS | 구현 전에 Toxiproxy dependency/package가 없어 초기 package test가 실패했고, 구현 후 대상 test가 통과. |
| Step 5 implementation | PASS | `testcontainers/toxiproxy`, README pair, `go.mod`/`go.sum` 갱신. |
| Step 6 validation | PASS | 대상 test, race test, README grep, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, `make ci`, `git diff --check`. |
| Step 6-R code review | PASS | `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-6r-code-review.md`, `P0=0 P1=0`. |
| Step 7 PR readiness | PASS | assignee/milestone/label을 #219에서 복사; 이 PR body는 `## DoD Status`로 끝남. |
