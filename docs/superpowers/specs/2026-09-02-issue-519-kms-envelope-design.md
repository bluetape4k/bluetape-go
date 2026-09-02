# AWS KMS envelope provider 설계

## 상태와 독자

- 상태: 승인된 설계
- 대상: `bluetape-go` 유지보수자와 AWS KMS envelope를 호출하는 Go 애플리케이션 개발자
- 언어: Korean technical documentation. API 이름, 식별자, 명령, URL, AWS 공식 용어는 원문을 유지한다.
- 범위: parent issue [#517](https://github.com/bluetape4k/bluetape-go/issues/517)의 첫 미완료 자식 [#519](https://github.com/bluetape4k/bluetape-go/issues/519)
- 실행 경계: `feat/issue-519-kms-envelope` worktree에서 구현·검증한다. PR, merge, tag, GitHub Release, live AWS 호출은 이번 실행 범위가 아니다.

## 근거와 현재 상태

설계 기준은 `develop`의 exact head `60bff23462f90940b85a4f357d2ea2812b2ef5b5`와 다음 소스다.

| 근거 | 확인 내용 |
|---|---|
| `encrypt/encrypt.go`, `encrypt/errors.go` | 현재 `BTENC` versioned AES-GCM facade, key copy, safe `encrypt.Error`, `errors.Is` 계약 |
| `encrypt/README.md`, `encrypt/README.ko.md` | local encryption과 KMS provider를 분리한다는 현재 문서 경계 |
| `dynamodb/batchwrite/batchwrite.go` | AWS SDK for Go v2 client subset을 caller-owned interface로 주입하는 저장소 패턴 |
| `go.mod` | AWS SDK root `v1.42.1`, service 모듈과 Go `1.26.3` |
| [AWS SDK for Go v2 KMS package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/kms) | `GenerateDataKey`/`Decrypt`의 context와 variadic options 시그니처 |
| [`GenerateDataKey` API](https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html) | plaintext data key와 KMS-encrypted data key를 함께 반환하고 plaintext를 즉시 지우라는 AWS 계약 |
| [`Decrypt` API](https://docs.aws.amazon.com/kms/latest/APIReference/API_Decrypt.html) | encrypted data key를 복호화하고 동일한 encryption context를 요구하는 AWS 계약 |

현재 모듈에는 `service/kms` direct requirement가 없다. implementation 단계에서 현재 AWS SDK 계열과 호환되는 `github.com/aws/aws-sdk-go-v2/service/kms v1.42.1`을 추가하고 `go mod tidy` 결과를 검증한다. 더 최신 버전으로 올리는 변경은 이 설계의 범위가 아니며 별도 dependency review가 필요하다.

## 문제와 목표

애플리케이션이 큰 payload를 KMS에 직접 보내지 않고, KMS가 생성한 data key로 local AES-GCM을 수행하는 작은 provider가 필요하다. provider는 KMS credential, client lifecycle, key policy, IAM, rotation, retry, cache를 대신 소유하지 않아야 한다.

목표는 다음과 같다.

1. `encrypt/kms`에서 caller-owned KMS client를 주입받아 `GenerateDataKey`와 `Decrypt`만 사용한다.
2. AES-256 data key와 local AES-GCM payload를 `BTKMS` versioned envelope에 저장한다.
3. key ID, encryption context, algorithm, encrypted data key를 local AEAD associated data에 canonical하게 묶는다.
4. KMS와 local crypto의 성공·실패·취소 경계를 `errors.Is`로 관찰할 수 있게 한다.
5. KMS plaintext data key를 호출별로만 사용하고 모든 반환 경로에서 best-effort zero한다. Go runtime이나 AES 내부 expanded key가 완전 삭제된다고 주장하지 않는다.
6. fake KMS client로 live AWS 없이 계약을 재현하고, concurrent 호출을 race test로 검증한다.

## 비목표

- KMS credential/config 로딩, client 생성·종료, region 선택, IAM/key policy 관리
- KMS retry/backoff/circuit breaker, data-key cache, key rotation orchestration
- KMS `Encrypt`/`GenerateDataKeyWithoutPlaintext`, asymmetric key, Nitro Enclave, direct large-payload encryption
- streaming/file encryption, searchable/deterministic encryption, password hashing, JWT
- envelope format v2 또는 하위 호환 migration
- live AWS integration test와 publication/merge

## API 경계

### 기존 `encrypt`에 추가할 detached 경계

현재 `Encryptor`는 random nonce를 포함한 `BTENC`를 반환한다. provider가 nonce를 outer metadata에 보관할 수 있도록 다음 두 method를 추가한다.

```go
func (e Encryptor) EncryptDetached(plaintext, associatedData []byte) (nonce, ciphertext []byte, err error)
func (e Encryptor) DecryptDetached(nonce, ciphertext, associatedData []byte) ([]byte, error)
```

두 method는 기존 `cipher.NewGCMWithRandomNonce` AEAD를 재사용한다. `EncryptDetached`는 generated nonce와 nonce를 제외한 sealed bytes를 각각 복사해 반환하고, `DecryptDetached`는 nonce 길이와 authentication tag 길이를 먼저 검사한다. 기존 `Encrypt`/`Decrypt`의 `BTENC` wire bytes와 key-copy/concurrency 계약은 유지한다.

### `encrypt/kms` public API

```go
package kms

import awskms "github.com/aws/aws-sdk-go-v2/service/kms"

type Client interface {
	GenerateDataKey(context.Context, *awskms.GenerateDataKeyInput, ...func(*awskms.Options)) (*awskms.GenerateDataKeyOutput, error)
	Decrypt(context.Context, *awskms.DecryptInput, ...func(*awskms.Options)) (*awskms.DecryptOutput, error)
}

type Option func(*config) error

func WithEncryptionContext(values map[string]string) Option

func New(client Client, keyID string, options ...Option) (*Provider, error)

type Provider struct { /* immutable caller-owned client reference and copied config */ }

func (p *Provider) Encrypt(ctx context.Context, plaintext, associatedData []byte) ([]byte, error)
func (p *Provider) Decrypt(ctx context.Context, envelope, associatedData []byte) ([]byte, error)

type Algorithm string

const AlgorithmAES256GCM Algorithm = "AES-256-GCM"
const EnvelopeVersion uint8 = 1

type Envelope struct {
	Version            uint8
	Algorithm          Algorithm
	KeyID              string
	EncryptedDataKey   []byte
	EncryptionContext  map[string]string
	Nonce              []byte
	Ciphertext         []byte
}

func ParseEnvelope(data []byte) (Envelope, error)
func (e Envelope) MarshalBinary() ([]byte, error)
```

`Client`는 AWS SDK v2 method subset을 그대로 사용해 `*kms.Client`와 fake가 같은 interface를 만족하게 한다. `Provider`는 client를 닫거나 재구성하지 않고, AWS option function을 임의로 추가하지 않으며, caller가 설정한 retry/timeout/config를 그대로 존중한다. constructor가 받은 `keyID`와 context map은 복사해 provider 생성 뒤 외부 mutation이 동작을 바꾸지 않게 한다. nil context는 repository convention에 맞춰 `context.Background()`로 취급하고, nil client·빈 key ID·nil option은 constructor에서 거부한다.

`Provider`는 생성 뒤 불변이며 concurrent `Encrypt`/`Decrypt` 호출에 안전하다. 호출마다 context map과 KMS input byte slices를 복사한다. provider는 내부 logger/global hook을 설치하지 않는다. 모든 operation error는 safe sentinel과 operation label만 `Error()`에 표시하고, caller가 그 error를 자체 logger로 기록한다.

## `BTKMS` wire contract

serialized bytes는 ASCII magic `BTKMS` 뒤에 canonical JSON object를 붙인다. JSON의 `[]byte`는 표준 base64 표현을 사용한다.

```text
BTKMS | {"version":1,"algorithm":"AES-256-GCM","key_id":"...",
         "encrypted_data_key":"...","encryption_context":[...],
         "nonce":"...","ciphertext":"..."}
```

실제 wire object의 `encryption_context`는 map이 아니라 key 오름차순의 다음 entry 배열로 인코딩한다.

```json
[{"key":"service","value":"billing"},{"key":"tenant","value":"blue"}]
```

이 표현은 Go map iteration 순서에 의존하지 않으며 duplicate key와 순서 위반을 parse 단계에서 거부할 수 있다. `MarshalBinary`는 field validation과 context 정렬을 수행하고, `ParseEnvelope`는 `json.Decoder.DisallowUnknownFields`와 trailing byte 검사를 사용한다. version 또는 algorithm이 지원되지 않으면 전용 sentinel을 반환한다. envelope 전체와 context metadata는 bounded input으로 취급하며, 구현은 현재 범위에서 64 MiB envelope 상한, 2 KiB key ID 상한, 64개 context entry와 8 KiB context 합계 상한을 사용한다.

필수 검사는 다음과 같다.

| 필드 | 계약 |
|---|---|
| `Version` | `EnvelopeVersion`인 `1`만 허용 |
| `Algorithm` | `AlgorithmAES256GCM`인 `AES-256-GCM`만 허용 |
| `KeyID` | 공백만 있는 값 거부; provider 설정과 exact match 필요 |
| `EncryptedDataKey` | 비어 있지 않아야 함; KMS `CiphertextBlob` 그대로 저장 |
| `EncryptionContext` | non-secret key/value만 사용; constructor 값과 exact map match 필요 |
| `Nonce` | local `encrypt` AES-GCM nonce size인 12 bytes |
| `Ciphertext` | AES-GCM authentication tag 16 bytes 이상 |

`Envelope`가 반환하는 slice와 map은 parse 결과의 독립 복사본이다. caller가 그 값을 변경해도 이미 생성된 provider 상태나 다른 호출을 변경하지 않는다.

## 암호화 data flow

1. `ctx == nil`이면 background context로 정규화하고, `ctx.Err()`를 확인한다.
2. caller-owned client로 `GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{KeyId: keyID, KeySpec: kmstypes.DataKeySpecAes256, EncryptionContext: clone(context)})`를 한 번 호출한다.
3. output과 `Plaintext` 길이 32 bytes, `CiphertextBlob` non-empty를 검증한다. plaintext slice에는 즉시 `defer zeroBytes`를 예약한다.
4. `encrypt.New(output.Plaintext)`로 local AES-GCM facade를 만들고, KMS metadata의 canonical JSON과 caller associated data를 length-prefixed AAD로 결합한다.
5. `EncryptDetached`로 random 12-byte nonce와 sealed ciphertext를 만들고, metadata·nonce·ciphertext를 `Envelope`에 넣어 `MarshalBinary`한다.
6. context가 KMS 응답 뒤 취소됐으면 local encryption을 시작하지 않고 `context.Canceled` 또는 `context.DeadlineExceeded`를 반환한다. 반환값은 nil이다.

KMS plaintext key는 caller에게 반환하지 않는다. `encrypt.New` 내부의 key copy와 AES expanded key는 Go crypto implementation이 관리하므로, 문서는 삭제를 best-effort로 한정한다.

## 복호화 data flow

1. context를 정규화하고 `ParseEnvelope`로 magic, JSON, version, algorithm, size, field lengths를 검증한다.
2. envelope `KeyID`와 context가 provider 설정과 exact match인지 확인한다. 불일치면 KMS를 호출하지 않고 `ErrMetadataMismatch`를 반환한다.
3. `ctx.Err()`를 확인한 뒤 `Decrypt(ctx, &awskms.DecryptInput{CiphertextBlob: clone(encryptedDataKey), KeyId: keyID, EncryptionContext: clone(context)})`를 한 번 호출한다.
4. output과 32-byte `Plaintext`를 검증하고 즉시 zero defer를 예약한다. KMS output 이후 context가 취소됐으면 local decrypt를 실행하지 않고 cancellation error를 반환한다.
5. 같은 canonical metadata AAD와 envelope nonce로 `DecryptDetached`를 호출한다. nonce 변경, ciphertext 변경, associated data 변경, metadata 변경은 `encrypt.ErrAuthenticationFailed`를 보존한다.

## 오류와 cancellation 계약

provider는 다음 sentinel을 제공한다.

```go
var (
	ErrNilClient           = errors.New("kms: client must not be nil")
	ErrInvalidKeyID        = errors.New("kms: key ID must not be empty")
	ErrInvalidOptions      = errors.New("kms: invalid options")
	ErrMalformedEnvelope   = errors.New("kms: malformed envelope")
	ErrUnsupportedVersion  = errors.New("kms: unsupported envelope version")
	ErrUnsupportedAlgorithm = errors.New("kms: unsupported envelope algorithm")
	ErrMetadataMismatch    = errors.New("kms: envelope metadata mismatch")
	ErrInvalidDataKey      = errors.New("kms: invalid data key")
	ErrKMSOperation        = errors.New("kms: KMS operation failed")
	ErrAuthenticationFailed = encrypt.ErrAuthenticationFailed
)
```

`*Error`는 parent `encrypt.Error`와 동일하게 kind·operation·cause를 보관한다. `Error()`에는 sentinel과 고정 operation만 표시하고 key ID, context 값, plaintext, ciphertext, nonce, AWS request/response text를 표시하지 않는다. `Unwrap`/`Is`는 KMS 원인, `context.Canceled`, `context.DeadlineExceeded`, `encrypt.ErrAuthenticationFailed`를 보존한다. AWS SDK error 상세가 필요한 caller는 별도 `errors.As` 처리를 선택할 수 있지만, provider error string 자체는 바로 log할 수 있어야 한다.

| 단계 | 실패 조건 | KMS 호출 | 결과 |
|---|---|---:|---|
| constructor | nil client, blank key ID, invalid option/context | 0 | 해당 sentinel |
| preflight | 이미 취소/마감된 context | 0 | context error |
| GenerateDataKey/Decrypt | SDK error | 1 | `ErrKMSOperation` + 원인 |
| KMS output validation | nil output, missing encrypted/plaintext key, wrong DEK length | 1 | `ErrInvalidDataKey` |
| post-KMS cancellation | 응답 뒤 local crypto 전 context 취소 | 1 | context error, local crypto 0 |
| envelope parse | magic/JSON/size/field/unknown-field 오류 | 0 | `ErrMalformedEnvelope` 또는 version/algorithm sentinel |
| metadata validation | key ID/context mismatch | 0 | `ErrMetadataMismatch` |
| local AEAD | tamper, wrong AAD, wrong nonce, wrong DEK | 1 on decrypt path | `ErrAuthenticationFailed` 보존 |

모든 KMS output plaintext 경로는 성공·실패·cancellation·validation panic 경로를 포함해 best-effort zero defer를 사용한다. fake는 output slice를 기록하기 전에 자체 복사해 zero 검증과 input aliasing 검증을 분리한다.

## 테스트와 DoD

`encrypt/kms` fake client는 mutex로 호출을 기록하고, `GenerateDataKey`/`Decrypt` 입력을 deep-copy한다. 다음 table-driven 테스트를 작성한다.

- constructor: nil client, blank key ID, nil option, context copy/empty context
- round trip: AES-256 data key, KMS context, `BTKMS` parse/marshal, associated data
- envelope: deterministic metadata bytes, sorted context, unknown/trailing fields, size/length limits, unsupported version/algorithm
- metadata: provider key ID/context mismatch가 KMS 호출 전에 거부되는지
- KMS failures: GenerateDataKey/Decrypt error wrapping, nil output, wrong key lengths, empty encrypted blob
- local failures: nonce/ciphertext tamper, associated data mismatch, encrypted data key tamper, redaction of all sensitive fixtures
- cancellation: preflight, inside KMS, post-KMS-before-local-crypto, deadline; expected call counts and zero plaintext
- ownership: provider가 client를 close/reconfigure하지 않음, constructor/input/output slice와 map mutation isolation
- concurrency: shared immutable provider stress test와 `go test -race`; bounded timeout and no goroutine leak
- examples: package example compiles without live credentials or AWS network

검증 명령은 순서대로 `gofmt`, `go test -count=1 ./encrypt ./encrypt/kms`, `go test -race -count=1 ./encrypt ./encrypt/kms`, `go test ./...`, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, `make ci`다. Docker-backed 기존 패키지는 저장소 규칙에 따라 순차 실행한다.

완료 DoD는 provider code/test, detached parent contract, English/Korean package README와 root README parity, exact dependency diff, safe error/cancellation 증거, race 결과, skill GREEN/REFACTOR pressure 결과, lesson을 모두 포함한다. live AWS, PR, merge, publication은 명시적인 후속 gate로 남긴다.

## 대안과 결정

| 대안 | 결정 | 이유 |
|---|---|---|
| provider가 AES-GCM을 직접 구현 | 거부 | `encrypt`와 nonce/error 계약이 갈라지고 crypto drift가 생긴다. |
| opaque `BTENC`를 그대로 outer ciphertext로 저장 | 거부 | nonce가 독립 metadata가 아니며 parent wire format 내부 의존을 만든다. |
| custom first-party KMS request interface와 AWS adapter | 보류 | SDK coupling은 줄지만 adapter·request copy가 별도 API가 된다. 현재 저장소의 `dynamodb/batchwrite` 패턴과 #519의 “caller-provided KMS client interface”가 직접 SDK subset을 지지한다. |
| encrypted DEK cache/reuse | 거부 | key retention과 rotation/lifecycle이 provider 범위를 넘어가고 cancellation/zero proof가 약해진다. |

이 설계는 #519의 범위만 닫는다. S3, SQS/SNS, DynamoDB provider와 KMS cache/rotation은 parent #517의 별도 자식 issue로 남긴다.
