# AWS RDS IAM authentication token helper 설계

## 상태와 범위

- 상태: 승인된 Type A 하위 설계
- 대상: `database/sql` 호출자와 PostgreSQL/MySQL connection 설정 계층
- 이슈: [#539](https://github.com/bluetape4k/bluetape-go/issues/539)
- 부모 이슈: [#517](https://github.com/bluetape4k/bluetape-go/issues/517)
- 기준 head: `906a68fdb41551ccaa6ce1394a2370e654ade10e`
- 실행 경계: `feat/issue-539-rds-iam-auth` worktree에서 fake credential provider 기반 구현과 로컬 검증을 수행한다.

이 설계는 AWS SDK for Go v2 `feature/rds/auth.BuildAuthToken`을 호출하는
작은 validation wrapper만 추가한다. credentials, region/config, IAM policy,
database driver, connection/pool lifecycle, token refresh와 live AWS 호출은
caller 또는 운영 계층의 책임이다.

## 결정

1. package 경계는 `rds/auth`로 둔다. RDS service API나 일반 SQL provider를
   만들지 않고 IAM token 생성에 필요한 SDK function만 사용한다.
2. `Token`은 redacted formatter를 가진 immutable wrapper로 반환한다. raw token은
   `Text` 또는 `Bytes` 메서드로 명시적으로 꺼내고 `String`, `GoString`, `%+v`는
   항상 `[REDACTED]`를 반환한다.
3. token cache, refresh goroutine, database/sql DSN builder는 추가하지 않는다.
   AWS SDK의 15분 signing lifetime은 README와 Go doc에 기록하고 갱신 시점은
   caller가 소유한다.
4. endpoint, region, username을 SDK 호출 전에 검증하고, endpoint는 scheme,
   path, query, fragment가 없는 host:port 형식이어야 한다. IPv6 bracket 표기는
   허용하고 port는 1..65535로 제한한다.
5. 모든 외부/credential 경계 전후에 context를 검사해 caller cancellation이
   token 성공보다 우선하도록 한다.

## Public API

```go
package auth

type Credentials interface {
	aws.CredentialsProvider
}

type Request struct {
	Endpoint string
	Region   string
	Username string
}

func BuildAuthToken(context.Context, Request, aws.CredentialsProvider) (Token, error)

type Token struct { /* immutable, redacted formatter */ }
func (t Token) Text() string
func (t Token) Bytes() []byte
func (t Token) IsSet() bool
func (t Token) Len() int
```

`aws.CredentialsProvider`는 SDK가 요구하는 caller-owned credential boundary다.
`BuildAuthToken`은 nil 및 typed-nil credentials provider를 거부하고, nil
context는 repository convention대로 `context.Background()`으로 정규화한다.
credential provider가 SDK 내부에서 context를 관찰하며, wrapper는 자체
goroutine/refresh를 만들지 않는다.

SDK `BuildAuthToken`은 endpoint host, region, username과 credentials로
15-minute `X-Amz-Expires=900` 서명 토큰을 만든다. helper는 token을 수정,
파싱, 저장하거나 DSN에 삽입하지 않는다. README에는 PostgreSQL의
`password=<token>`과 MySQL의 password handoff를 예제로만 보여주되 실제
driver/pool 호출은 caller 코드로 남긴다.

## 오류 계약

```go
var (
	ErrInvalidRequest = errors.New("rds/auth: invalid request")
	ErrNilCredentials = errors.New("rds/auth: credentials provider must not be nil")
	ErrBuildFailed    = errors.New("rds/auth: auth token build failed")
	ErrMalformedToken = errors.New("rds/auth: malformed auth token")
)
```

safe typed `Error`는 fixed kind/operation만 `Error()`와 `%+v`에 출력한다.
endpoint, username, region, token, credential details와 SDK provider message는
public error 문자열 또는 log에 넣지 않는다. 원인은 `Unwrap`으로만 보존해
`errors.Is`와 caller 진단을 지원한다. SDK가 빈 token을 반환하면
`ErrMalformedToken`으로 fail closed한다.

## 테스트 및 운영 경계

fake `aws.CredentialsProvider`가 호출 수와 context를 기록하고 configured
credentials/error/blocking behavior를 제공한다. valid host/IPv4/IPv6/port,
invalid scheme/path/port/blank fields, typed-nil credentials, pre/post
cancellation, SDK error, empty token, redaction, output independence와
PostgreSQL/MySQL token handoff examples를 검증한다. live AWS credentials와
database connection은 기본 CI에 사용하지 않는다.

## 수용 기준과 SPW gate

| 기준 | 증거 |
|---|---|
| RDS auth API만 감싼 좁은 경계 | `BuildAuthToken` source와 no DB client API |
| strict request validation | table-driven endpoint/region/user tests |
| cancellation 우선순위 | credential fake context test |
| token redaction | formatter/error `%+v` tests |
| token lifetime/refresh ownership | Go doc와 EN/KO README |
| SDK compile-check | `feature/rds/auth` concrete API test/build |

- SPW-01: PASS — live issue, parent issue, SDK API와 non-goal을 고정했다.
- SPW-02: PASS — API, validation, failure, cancellation, token lifetime, acceptance를 명시했다.
- SPW-03: PASS — Korean technical register checklist를 적용하고 code token은 보존한다.
- SPW-04: PASS — generated SDK contract와 caller-owned database boundary를 source ledger에 연결했다.
- SPW-05: PASS — 구현 read-back, targeted/full test, static check와 문서 parity를 `docs/review/2026-09-04-issue-539-rds-iam-auth-implementation-review.md`에 기록했다.

## 롤백과 비목표

배포 전 rollback은 하위 PR commit revert로 한정한다. credential resolution,
IAM provisioning, driver dependency, connection pool, token cache/refresh,
general configuration framework와 live smoke는 이 범위에 포함하지 않는다.
