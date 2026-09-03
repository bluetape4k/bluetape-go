# AWS RDS IAM authentication token helper 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AWS SDK v2 RDS auth token signing을 엄격히 검증하고 redacted immutable token으로 반환하는 caller-owned helper를 추가한다.

**Architecture:** `rds/auth`는 `feature/rds/auth.BuildAuthToken`과 `aws.CredentialsProvider`만 경계로 노출한다. `Request` preflight와 response validation, typed redacted error/token을 제공하며 SQL 연결·DSN mutation·credential lifecycle은 소유하지 않는다.

**Tech Stack:** Go `1.26.3`, AWS SDK for Go v2 `feature/rds/auth v1.7.1`, `aws`, `context`, `net`, `net/url`, `reflect`, `errors`, `go test -race`.

## 파일 구조

| 경로 | 책임 |
|---|---|
| `go.mod`, `go.sum` | RDS auth feature module direct dependency |
| `rds/auth/doc.go` | caller-owned credentials/database와 15-minute token contract |
| `rds/auth/errors.go` | validation/build/malformed sentinels 및 safe Error |
| `rds/auth/token.go` | immutable redacted token wrapper |
| `rds/auth/auth.go` | Request validation과 SDK BuildAuthToken bridge |
| `rds/auth/auth_test.go` | credential fake, validation/cancel/redaction/token tests |
| `rds/auth/example_test.go` | PostgreSQL/MySQL DSN/password handoff compile examples |
| `rds/auth/README.md`, `.ko.md` | API, token lifetime, DSN handoff, ownership boundary |
| `README.md`, `README.ko.md` | root package index/AWS section link |
| `docs/review/...risk-prediction.md` | implementation 전 위험/mitigation |
| `docs/review/...implementation-review.md` | 7-Tier evidence |
| `docs/lessons/...md` | RDS auth boundary lesson |

## Task 1: dependency와 RED tests

- [x] `go.mod`에 `github.com/aws/aws-sdk-go-v2/feature/rds/auth v1.7.1`을 direct requirement로 추가한다.
- [x] `Request`, `Token`, `BuildAuthToken`, sentinel 및 fake credential provider를 참조하는 failing tests를 먼저 작성한다.
- [x] `go test ./rds/auth`로 심볼 부재에 따른 RED를 확인한다.

## Task 2: validation/token/error 구현

- [x] `doc.go`, `errors.go`, `token.go`와 `auth.go`를 작성한다.
- [x] endpoint는 `net.SplitHostPort`와 strict DNS/IP host grammar로 IPv4/IPv6 host:port를 검사하고 scheme/path/query/fragment/userinfo/percent escape/backslash, malformed label, blank host와 port 범위를 거부한다. region/username은 valid UTF-8/nonblank/byte bound를 적용한다.
- [x] dispatch 전후 context를 검사하고 `rdsauth.BuildAuthToken`을 한 번만 호출한다. output empty는 malformed로 거부하고 SDK error는 safe typed error로 감싼다.
- [x] token copy/formatter/redaction과 `errors.Is`를 검증한다.

## Task 3: examples/docs/review

- [x] compile-checked examples에서 PostgreSQL/MySQL password handoff만 보여주고 driver/pool/client construction은 caller code로 둔다.
- [x] EN/KO README와 root index를 갱신해 15-minute token, refresh/credentials/IAM/DB lifecycle ownership, no-live-test를 명시한다.
- [x] risk prediction 및 implementation review/lesson artifact를 작성하고 SPW-05 read-back을 기록한다.
- [x] `git diff --check`와 Korean terminology audit를 실행한다.

## Task 4: verification

- [x] `gofmt -w`, `make fmt-check`, `make vet`, `make lint`를 실행한다. `make tidy-check`는 미커밋 dependency diff로 예상 실패했으며 clean-tree 재실행이 필요하다.
- [x] `go test -count=1 ./rds/auth`, `go test -race -count=1 ./rds/auth`, `go test -run '^Example' -count=1 ./rds/auth`를 실행한다.
- [x] `go test -count=1 ./...`로 root module 영향을 확인했고 모든 package가 통과했다.

## Rollback/rerun

구현은 package와 docs/index 및 dependency 변경으로 한정한다. live credentials,
RDS network, `database/sql`, driver dependency와 refresh goroutine은 실행하지
않는다. generated SDK signature drift가 있으면 API를 확장하지 말고 현재
module source를 재확인한다.

## 계획 self-review

- Spec의 validation/token/error/context/docs/no-live 항목은 Task 1~4에 모두 매핑된다.
- placeholder/TBD/TODO 없이 정확한 파일과 함수/명령을 지정했다.
- `Request`, `Token`, `BuildAuthToken`, sentinel 명칭은 spec과 동일하다.
- SPW-01/02/03/04/05: PASS — 구현 read-back과 fresh verification은 implementation review에 기록했다.
