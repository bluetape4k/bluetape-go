# AWS Secrets Manager 및 SSM lookup provider 설계

## 상태와 범위

- 상태: 승인된 Type A 하위 설계
- 대상: `bluetape-go` 호출자와 애플리케이션 설정 조합 계층
- 이슈: [#538](https://github.com/bluetape4k/bluetape-go/issues/538)
- 부모 이슈: [#517](https://github.com/bluetape4k/bluetape-go/issues/517)
- 기준 head: `906a68fdb41551ccaa6ce1394a2370e654ade10e`
- 실행 경계: `feat/issue-538-secrets-ssm` worktree에서 fake-first 구현과 로컬 검증을 수행한다.

이 설계는 AWS SDK for Go v2의 `GetSecretValue`와 `GetParameter`에 필요한
method subset만 감싼다. AWS config, credentials, retry, endpoint, client
lifecycle, 환경 변수 precedence, KMS envelope, provisioning과 live credential
smoke는 호출자 또는 별도 이슈의 책임이다.

## 결정

1. Secrets Manager와 SSM은 응답 shape와 보안 의미가 달라 별도 top-level
   package(`secretsmanager`, `ssm`)로 제공한다.
2. 조회 결과는 `Value` wrapper로 반환한다. raw value는 `Bytes` 또는 `Text`
   메서드로 명시적으로 꺼내며 `String`, `GoString`, `%+v`는 항상
   `[REDACTED]`를 반환한다.
3. cache는 기존 `cache.LoadingCache[string, Value]`를 선택적으로 주입한다.
   `CacheTTL == 0`은 cache를 사용하지 않고, positive TTL만 성공 결과를
   저장한다. 오류, cancellation, stale value는 저장하지 않는다. cache가
   없고 positive TTL을 지정하면 process-local `cache.Memory`를 provider가
   생성한다.
4. 모든 외부 호출 전후에 `context.Context`를 검사한다. response와
   cancellation이 함께 도착하면 caller cancellation을 반환한다.
5. 오류 문자열에는 secret/parameter name, raw value, provider error text를
   포함하지 않는다. `errors.Is`를 위해 원인은 `Unwrap`으로만 보존한다.

## Public API

```go
package secretsmanager

type Client interface {
	GetSecretValue(context.Context, *awssm.GetSecretValueInput, ...func(*awssm.Options)) (*awssm.GetSecretValueOutput, error)
}

type Options struct {
	Client   Client
	Cache    cache.LoadingCache[string, Value]
	CacheTTL time.Duration
}

func New(options Options) (*Provider, error)
func (p *Provider) Get(context.Context, string) (Value, error)

type Value struct { /* immutable, redacted formatter */ }
func (v Value) Bytes() []byte
func (v Value) Text() string
func (v Value) IsBinary() bool
func (v Value) IsSet() bool
func (v Value) Len() int
```

`ssm`은 동일한 provider/cache/value 규약을 사용하고 `Options.WithDecryption`
및 `GetSecure(ctx, name)`을 추가한다. `Get`은 provider option의
`WithDecryption`을 사용하고 `GetSecure`는 항상 `WithDecryption=true`를
사용한다. plain/decrypted 결과가 cache에서 충돌하지 않도록 cache key에는
decryption mode를 포함한다.

`Client`에는 SDK concrete client가 compile-time assertion으로 연결된다.
`New`는 nil 및 모든 `reflect.IsNil` 가능 kind의 typed-nil client/cache,
negative TTL, nil option을 거부한다. 이름은 trim하거나 정규화하지 않고
valid UTF-8, non-blank, AWS의 2048-byte 상한만 검사한다.

## 응답 및 오류 계약

Secrets Manager 응답에서 `SecretString`과 `SecretBinary` 중 정확히 하나가
존재해야 한다. empty string과 non-nil empty binary는 유효한 값이다. 둘 다
없거나 둘 다 있으면 `ErrMissingValue` 또는 `ErrMalformedOutput`을 반환하고
cache/호출자에게 저장하지 않는다. SSM 응답에는 `Parameter`와 non-nil
`Parameter.Value`가 필요하다.

각 package는 다음 sentinel과 safe typed `Error`를 제공한다.

```go
var (
	ErrNilClient       = errors.New("...: client must not be nil")
	ErrInvalidOptions  = errors.New("...: invalid options")
	ErrInvalidName     = errors.New("...: name is invalid")
	ErrLookupFailed    = errors.New("...: lookup failed")
	ErrMalformedOutput = errors.New("...: malformed response")
	ErrMissingValue    = errors.New("...: value is missing")
	ErrCacheFailed     = errors.New("...: cache operation failed")
)
```

operation label은 allowlist로 제한한다. `Error()`와 `%+v`는 고정 sentinel과
operation만 출력하고, SDK 원문은 error chain에서만 접근한다. caller-owned
logger는 필요할 때 오류의 safe `Error()`를 기록할 수 있지만 provider는
global logger를 설치하거나 raw provider payload를 기록하지 않는다.

## 테스트 및 운영 경계

mutex-safe fake가 request를 deep-copy하고 call count/context를 기록한다.
constructor/typed-nil, request mapping, string/binary/empty value, malformed
output, transport error, error redaction, pre/post cancellation, cache hit/miss/
expiry/error non-cache, concurrent cache access를 table-driven으로 검증한다.
cache 공유 상태가 있으므로 `go test -race`와 bounded concurrent test를
실행한다. AWS credentials와 검증되지 않은 emulator는 기본 CI에 넣지 않는다.

## 수용 기준과 SPW gate

| 기준 | 증거 |
|---|---|
| 좁은 caller-owned SDK surface | `Client` interface와 concrete compile assertion |
| SecretString/SecretBinary 및 SecureString | fake request/response tests |
| cancellation 우선순위 | pre-dispatch/response-after-cancel tests |
| raw secret redaction | `String`, `%+v`, error formatting tests |
| positive TTL cache | hit/miss/expiry/no-error-cache/race tests |
| bilingual docs | package README pair와 root index |
| fake-first/no live AWS | default `go test`와 no credential dependency |

- SPW-01: PASS — live issue, parent issue, package/API sources와 unknown/live 범위를 고정했다.
- SPW-02: PASS — API, zero value, failure, cancellation, cache, compatibility, acceptance를 명시했다.
- SPW-03: PASS — Korean technical register checklist를 적용하고 API token은 보존한다.
- SPW-04: PASS — AWS SDK response contract와 local `cache`/provider pattern을 source ledger에 연결했다.
- SPW-05: PASS — 구현 read-back, targeted/full test, static check와 문서 parity를 `docs/review/2026-09-04-issue-538-secrets-ssm-implementation-review.md`에 기록했다.

## 롤백과 비목표

배포 전 rollback은 이 하위 PR commit revert로 한정한다. 환경 변수 precedence,
generic config framework, KMS envelope, secret rotation/refresh, IAM policy,
credential chain과 live service lifecycle은 추가하지 않는다.
