# AWS Secrets Manager 및 SSM lookup provider 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** caller-owned AWS SDK client로 Secrets Manager와 SSM 값을 조회하는 두 개의 좁은 provider를 추가하고 redaction, cancellation, optional positive TTL cache와 bounded cache ownership을 fake-first 테스트로 고정한다.

**Architecture:** `secretsmanager`와 `ssm`을 별도 package로 두고 각각 AWS SDK method subset만 주입한다. 결과는 formatter가 비밀값을 숨기는 immutable `Value`로 반환하며 기존 generic `cache.LoadingCache`를 선택적으로 사용한다. positive TTL에서는 bounded caller-owned cache를 요구하고 provider는 unbounded process-local cache를 만들지 않는다. provider는 retry, credentials, precedence, refresh와 logger를 소유하지 않는다.

**Tech Stack:** Go `1.26.3`, AWS SDK for Go v2 `service/secretsmanager v1.47.0`, `service/ssm v1.76.0`, `context`, `reflect`, `errors`, existing `cache.Memory`, `go test -race`.

## 파일 구조

| 경로 | 책임 |
|---|---|
| `go.mod`, `go.sum` | Secrets Manager/SSM service modules direct dependency |
| `secretsmanager/*.go` | client/options/provider/value/error 및 fake-first tests/example |
| `ssm/*.go` | client/options/provider/value/error 및 fake-first tests/example |
| `secretsmanager/README.md`, `.ko.md` | API, redaction, cache, IAM/lifecycle 경계 |
| `ssm/README.md`, `.ko.md` | `WithDecryption`/`GetSecure`와 cache key 경계 |
| `README.md`, `README.ko.md` | root package index와 AWS section link |
| `docs/review/...risk-prediction.md` | implementation 전 위험과 mitigation |
| `docs/review/...implementation-review.md` | 7-Tier implementation evidence |
| `docs/lessons/...md` | 재사용 가능한 provider/cache lesson |

## Task 1: dependency 및 RED contract

- [x] `go.mod`에 `github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.47.0`와 `github.com/aws/aws-sdk-go-v2/service/ssm v1.76.0`을 direct requirement로 추가한다.
- [x] fake가 구현할 `Client`, `Value` formatter, constructor/lookup/cache/cancellation 테스트를 먼저 작성한다.
- [x] `go test ./secretsmanager ./ssm`을 실행해 package/심볼 부재에 따른 예상 RED를 확인한다.

## Task 2: Secrets Manager 구현

- [x] `secretsmanager/doc.go`, `errors.go`, `value.go`, `provider.go`에 `Client`, `Options`, `Provider`, `Value`, safe `Error`를 구현한다.
- [x] `Get`은 identifier 검증, pre/post context check, `GetSecretValue` request mapping, string/binary output 선택을 수행한다.
- [x] positive TTL cache는 caller-supplied `cache.LoadingCache.GetOrLoad`를 사용하고 성공만 저장한다. cache key와 raw value를 error에 넣지 않으며, cache가 없으면 constructor가 거부한다.
- [x] targeted normal/race tests와 `go vet ./secretsmanager`를 실행한다.

## Task 3: SSM 구현

- [x] `ssm/doc.go`, `errors.go`, `value.go`, `provider.go`에 `Client`, `Options.WithDecryption`, `Provider.Get`, `Provider.GetSecure`를 구현한다.
- [x] request의 `Name`과 effective `WithDecryption`을 검증/전달하고 mode별 cache key를 분리한다.
- [x] parameter missing value, transport error, cancellation, cache hit/expiry, positive TTL cache 누락과 redaction을 검증한다.
- [x] targeted normal/race tests와 `go vet ./ssm`를 실행한다.

## Task 4: docs/index 및 review

- [x] 두 package README를 EN/KO source-equivalent로 작성하고 credential/IAM/retry/precedence/rotation/live smoke 비목표를 명시한다.
- [x] root README 두 locale의 package table/AWS section을 갱신한다.
- [x] risk prediction을 implementation evidence로 갱신하고 `docs/review/...implementation-review.md`와 `docs/lessons/...md`를 작성한다.
- [x] `git diff --check`와 Korean terminology audit를 실행한다.

## Task 5: repository verification

- [x] `gofmt -w` 후 `make fmt-check`, `make vet`, `make lint`를 실행한다. `make tidy-check`는 미커밋 dependency diff로 예상 실패했으며 clean-tree 재실행이 필요하다.
- [x] `go test -count=1 ./secretsmanager ./ssm`, `go test -race -count=1 ./secretsmanager ./ssm`, `go test -run '^Example' -count=1 ./secretsmanager ./ssm`을 실행한다.
- [x] 변경 영향 범위가 root module이므로 `go test -count=1 ./...`를 실행했고 모든 package가 통과했다.

## Rollback/rerun

각 task는 독립적으로 revert할 수 있는 파일 집합을 유지한다. AWS live call,
credential setup, provisioning과 secret rotation은 실행하지 않는다. SDK type
drift가 발생하면 generated type을 먼저 다시 확인하고 API 범위를 넓히지 않는다.

## 계획 self-review

- Spec의 API/zero value/errors/cache/cancellation/docs 항목은 Task 1~5에 모두 매핑된다.
- placeholder/TBD/TODO 없이 파일, 함수, 명령, 기대 증거를 구체화했다.
- `Options`, `Provider.Get`, `GetSecure`, `Value`, sentinel 명칭을 spec과 동일하게 유지한다.
- SPW-01/02/03/04/05: PASS — 구현 read-back과 fresh verification은 implementation review에 기록했다.
