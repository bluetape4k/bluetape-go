# 이슈 #559 DynamoDB 조건부 쓰기 리더 선출 Step 6-R 코드 리뷰

> 리뷰 경계: 판정과 설명은 한국어로 작성하고, severity 토큰, 상태 토큰, 파일 경로, 라인 번호, 명령, API 식별자는 검증 가능한 증거 앵커로 보존한다.

## 범위와 정확한 기준

- Base: `origin/develop` = `352c0bdbbef7ef41362027e3ecb591ed38be1c32`
- Reviewed implementation head: `e9b003caaec937b06719a87fdda8a6ba69b84387`
- Branch: `feat/issue-559-dynamodb-leader`
- Scope: caller-owned DynamoDB client 위의 conditional Put/Update/Delete/Get,
  lease/TTL schema, bounded reconciliation, fake/unit coverage, bilingual
  README와 issue #559 계약.
- Initial code-review lane은 P1 두 건(사용하지 않는 `#key` alias, 무제한
  ownership probe)을 발견했다. 후속 변경은 같은 base에서 `b308175`와 최종
  `e9b003c`에 반영하고, 늦은 response·fresh takeover deadline·Resign probe까지
  회귀 테스트로 닫았다.
- Architect 독립 lane은 현재 native slot 부족으로 생성할 수 없었다. 따라서
  architect 관점은 main-session architectural fallback으로 수행했으며,
  독립 architect 승인으로 간주하지 않는다.

## 최종 판정 요약

`P0=0`, `P1=0`, `P2=2`, `P3=0`이다. P2 두 건은 live provider와 공통
conformance 범위의 명시적 조건부 gate로 남겨 두었고, 구현 자체의 PR 준비를
막는 P0/P1은 없다. 최종 verdict는 `PASS for PR / PENDING conditional gates`다.

## 6개 관점과 메인 통합

| 관점 | 판정 | 증거와 경계 |
|---|---|---|
| 성능 | PASS | `elector.go:161-240`에서 Put/Update와 reconciliation probe가 `attemptBudget`으로 bounded다. strong `GetItem`은 오류 후 한 번만 수행하고, TTL은 cleanup 전용이다. 추가 AWS latency/throughput 수치는 측정하지 않았다. |
| 안정성 | PASS after fix | `elector.go:173-177,199-240`은 child attempt timeout 뒤 늦은 output을 성공으로 신뢰하지 않고 bounded probe로 재조정한다. `elector_test.go:472-507` late response, `510-554` contention 뒤 deadline 재계산, `598-631` Resign probe bounded 회귀가 통과했다. |
| 보안 | PASS after fix | `takeoverInput`, `renewInput`, `deleteInput`의 expression alias는 실제 표현식만 선언한다(`elector.go:348-397`). operation error는 raw AWS text/table/group/token을 노출하지 않으며 tests가 redaction을 확인한다. IAM은 caller-owned least privilege 범위다. |
| 운영/Ops | PASS with PENDING | table schema, explicit TTL cleanup, capacity/throttling, credential/region/retry ownership, cleanup-pending/`ErrCommitUnknown` runbook이 양쪽 README에 있다(`leader/dynamodb/README.ko.md:26-96`). Floci/live AWS readiness와 실제 IAM/TTL 동작은 명시적 opt-in 전까지 실행하지 않았다. |
| 개발자/API | PASS | `Client`는 `PutItem`, `UpdateItem`, `DeleteItem`, `GetItem`만 요구하고 constructor I/O/dependency 추가가 없다. nil context와 compile-checked fake/example, package `go doc` 및 EN/KO README를 검증했다. `KeyPrefix`는 검증하되 table namespace 경계 밖에서 자동 인코딩하지 않는 계약을 문서화했다. |
| 사용자/호출자 | PASS with PENDING | Campaign/renew/resign의 commit ambiguity, retry 가능한 cleanup, 다른 owner 보호, caller가 제공하는 context와 AWS lifecycle이 명시돼 있다. `leadertest.Harness`는 DynamoDB의 conditional semantics와 backend clock이 공통 harness와 달라 직접 adapter로 연결하지 않았고, 동등 fake 시나리오를 실행했다. |
| 메인 통합 | PASS | base/head를 고정하고 P0/P1을 재검토했다. 구현, tests, examples, docs, issue scope는 일치하며 PR 생성 가능 상태다. 독립 architect lane은 위와 같이 unavailable/fallback으로 기록한다. |

## 닫힌 주요 finding

1. `UpdateItem`/`DeleteItem`의 사용하지 않는 `#key` alias를 제거했다. DynamoDB가
   `ValidationException: Value provided in ExpressionAttributeNames unused`를
   반환하던 stale takeover/renew/resign 경로를 `TestUpdateAndDeleteRequestsDoNotDeclareUnusedKeyAlias`로 고정했다.
2. renewal probe뿐 아니라 Campaign late response, takeover, Resign probe까지
   bounded context를 적용했다. caller-owned SDK가 child timeout 뒤 성공 output을
   돌려줘도 ownership을 직접 확정하지 않으며, own token이면 복구하고 no-owner면
   재시도한다. probe 실패는 cleanup pending과 `ErrCommitUnknown`이다.
3. takeover의 lease deadline을 initial attempt 시각이 아닌 conditional update
   직전에 다시 계산했다. contention 동안 시간이 경과해도 lease가 과거 시각으로
   기록되지 않는다.

## 수용된 P2 및 검증 공백

- Floci/live AWS credentials, IAM policy, DynamoDB TTL 지연과 실제 throttling은
  환경·권한을 요구하므로 실행하지 않았다(`PENDING`).
- `leader/leadertest.Harness` 15-case 직접 adapter는 backend clock 및
  conditional-write 의미가 달라 연결하지 않았다. fake에서 late response,
  cancellation, renewal/resign, redaction, stale takeover equivalent를
  검증했으며, 실제 provider conformance adapter는 후속 작업으로 남긴다.
- `gopls`/LSP 증적은 수집하지 않았다(`PENDING`).

## 검증 증거

```text
go test -count=1 ./leader/dynamodb                         PASS
go test -race -count=1 ./leader/dynamodb                   PASS
go test -run '^Example' -count=1 ./leader/dynamodb          PASS
go vet ./leader/dynamodb                                    PASS
golangci-lint run ./leader/dynamodb                         PASS (0 issues)
go test -count=1 ./leader/...                               PASS
make fmt-check tidy-check vet lint                          PASS (lint 0 issues)
git diff --check origin/develop...HEAD                      PASS
```

Docker-backed Redis는 이 이슈의 변경 범위가 아니며, #573에서 별도로 `-p 1`로
검증했다. 전체 repository `make test`의 첫 실행은 변경하지 않은 `lock/redis`
두 timeout에서 중단됐지만, `go test -count=1 ./lock/redis` isolated retry는
통과했다. 해당 baseline은 #559 판정의 성공 증거로 합산하지 않는다.

## PR 전 DoD

| 항목 | 상태 |
|---|---|
| live issue #559/milestone/assignee 확인 | 완료: 0.20.0, `debop`, task/leader/testing/database/p2 |
| 정확한 base/head 및 six lenses/main 통합 | 완료: 위 SHA, P0/P1 zero |
| 구현·fake·examples·문서·redaction | 완료 |
| focused/race/examples/vet/lint/leader suite | 완료 |
| Floci/live AWS 및 직접 leadertest adapter | `PENDING` |
| PR 생성/원격 CI/Step 7-R/merge | `PENDING`: PR 생성 후 fresh exact-head gate 필요 |

## 최종 상태

`PASS for PR / PENDING conditional gates`. PR을 생성할 수 있으나, live AWS와
직접 conformance adapter를 실행했다고 주장하지 않으며, 원격 CI와 merge는 별도
게이트다.
