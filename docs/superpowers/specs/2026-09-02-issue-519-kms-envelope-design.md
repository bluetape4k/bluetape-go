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

두 method는 AES-GCM 표준 `cipher.NewGCM` AEAD를 사용하고 호출마다 `crypto/rand`로 12-byte nonce를 생성한다. `EncryptDetached`는 nonce와 nonce를 제외한 sealed bytes를 각각 독립 복사해 반환하고, `DecryptDetached`는 nonce 길이 12와 authentication tag 길이 16을 먼저 검사한다. 기존 `Encrypt`/`Decrypt`는 같은 `nonce|ciphertext+tag` 배열을 `BTENC` 뒤에 저장하므로 기존 wire 길이와 이미 발행된 ciphertext 복호화 호환성을 유지한다.

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

`Client`는 AWS SDK v2 method subset을 그대로 사용해 `*kms.Client`와 fake가 같은 interface를 만족하게 한다. 실제 SDK 적합성은 compile-only assertion으로 고정한다. `Provider`는 client를 닫거나 재구성하지 않고, AWS option function을 임의로 추가하지 않으며, caller가 설정한 retry/timeout/config를 그대로 존중한다. constructor가 받은 `keyID`와 context map은 복사해 provider 생성 뒤 외부 mutation이 동작을 바꾸지 않게 한다. nil context는 repository convention에 맞춰 `context.Background()`로 취급하고, nil client·interface 내부 typed-nil client·빈 key ID·nil option은 constructor에서 거부한다. typed-nil 검사는 reflection의 모든 nil 가능 kind(`Chan`, `Func`, `Interface`, `Map`, `Pointer`, `Slice`)를 포함해 panic과 KMS 호출을 막는다.

`WithEncryptionContext`는 하나의 option map을 추가하는 형태이며 nil map은 empty context로 취급한다. 여러 option이 같은 context key를 제공하면 `ErrInvalidOptions`로 거부하고 last-wins merge를 하지 않는다. context key 비교는 case-sensitive한 exact 문자열이며, wire entry field(`key`, `value`) 이름의 대소문자 변형은 parser가 거부한다. `MaxContextSize`는 모든 key/value의 UTF-8 byte 길이 합계로 계산하며 정확히 8 KiB는 허용하고 1 byte 초과는 거부한다. provider는 caller가 전달한 plaintext, associated data, envelope를 호출 중 변경하지 않는다는 계약을 요구하고, 반환 slice의 소유권은 caller에게 있지만 provider가 소비하는 KMS output buffer는 caller가 보관하거나 재사용하지 않아야 한다.

`Provider`는 생성 뒤 불변이며, 주입된 `Client`가 다음 계약을 준수하는 경우 concurrent `Encrypt`/`Decrypt` 호출에 안전하다. `Client` 구현은 method 동시 호출에 안전하고, `context.Context` 취소를 협력적으로 관찰해 반환해야 하며, 호출이 반환된 뒤 request input을 변경하거나 보관하지 않아야 한다. provider는 비협조적인 client 호출을 강제 종료하거나 별도 goroutine으로 감싸지 않는다. 호출마다 context map과 KMS input byte slices를 복사한다. provider는 내부 logger/global hook을 설치하지 않는다. 모든 operation error는 safe sentinel과 operation label만 `Error()`에 표시하고, caller가 그 error를 자체 logger로 기록한다.

caller instrumentation은 operation별 latency, 성공/실패, cancellation, provider logical KMS call을 SDK retry/network attempt와 분리해 기록해야 한다. key ID, encryption context, plaintext, raw envelope, associated data 또는 unwrapped AWS error text를 metric label이나 일반 로그에 넣지 않는 것은 caller의 운영 계약이다.

`keyID`는 non-blank UTF-8 문자열(최대 2 KiB)을 그대로 envelope에 저장하고 KMS input에 전달한다. ARN, key ID, alias를 모두 허용하지만 KMS output의 resolved `KeyId`는 저장하거나 비교하지 않는다. alias retarget는 historical envelope 복구 계약이 아니므로 장기 보존 데이터에는 immutable key ARN/ID를 사용하고, rotation/recovery는 caller가 소유한다. envelope의 `KeyID`와 provider 설정은 문자열 exact match이므로 alias와 ARN처럼 표현이 다르면 KMS 호출 전에 `ErrMetadataMismatch`가 된다.

다음 입력 한도는 KMS 호출 전과 envelope 직렬화/parse 단계에서 동일하게 적용한다.

```go
const (
    MaxEnvelopeSize         = 64 << 20 // 64 MiB, serialized BTKMS bytes
    MaxPlaintextSize        = 32 << 20 // conservative bound that fits worst-case envelope metadata
    MaxAssociatedDataSize   = 64 << 10
    MaxKeyIDSize            = 2 << 10
    MaxContextEntries       = 64
    MaxContextSize          = 8 << 10
    MaxEncryptedDataKeySize = 6144 // AWS KMS Decrypt CiphertextBlob limit
)
```

`MaxPlaintextSize`는 32 MiB로 고정한다. AES-GCM tag와 표준 padded base64, 최대 key ID,
context, encrypted data key, JSON metadata를 포함한 worst-case envelope가
`MaxEnvelopeSize` 안에 들어가도록 계산한 보수적 사전 한도다. plaintext가 정확히
`MaxPlaintextSize`이면 boundary 성공을 검증하고, `+1`이면 `ErrInputTooLarge`와 KMS
호출 0회를 검증한다. `ParseEnvelope`/`Decrypt`는 ciphertext에서 GCM tag 16 bytes를
제외한 plaintext 길이도 KMS 호출 전에 같은 한도로 검사한다. fake benchmark는 logical
SDK method call과 local allocation/latency만 측정하며, caller-owned SDK retry에 따른
network attempt나 실제 AWS RTT를 주장하지 않는다.

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

이 표현은 Go map iteration 순서에 의존하지 않으며 duplicate key와 순서 위반을 parse 단계에서 거부한다. `MarshalBinary`는 field validation과 context 정렬을 수행하고, `ParseEnvelope`는 token-level object walk로 unknown field, duplicate field, 대소문자만 다른 field, invalid UTF-8, `null` 필수값, trailing byte를 거부한다. decoded field name은 exact/lowercase 집합으로 중복을 검사하고, parser가 재직렬화한 canonical bytes가 입력 bytes와 일치하는지 확인해 top-level whitespace, field reorder, escaped name, alternate numeric form도 거부한다. `encoding/json`의 case-insensitive field matching이나 duplicate overwrite에 의존하지 않는다. byte field는 표준 padded base64의 canonical re-encoding과 일치해야 하며 whitespace/비정규 표기는 거부한다. version 또는 algorithm이 지원되지 않으면 전용 sentinel을 반환한다. envelope 전체와 context metadata는 bounded input으로 취급한다. `EncryptedDataKey`는 JSON base64 길이를 먼저 검사한 뒤 decode하며, decode된 raw `CiphertextBlob`이 `MaxEncryptedDataKeySize`를 넘으면 KMS 호출 전에 `ErrMalformedEnvelope`를 반환한다. 모든 한도는 `MaxEnvelopeSize`, `MaxKeyIDSize`, `MaxContextEntries`, `MaxContextSize` 상수로 고정한다.

필수 검사는 다음과 같다.

| 필드 | 계약 |
|---|---|
| `Version` | `EnvelopeVersion`인 `1`만 허용 |
| `Algorithm` | `AlgorithmAES256GCM`인 `AES-256-GCM`만 허용 |
| `KeyID` | valid UTF-8, 공백만 있는 값과 2 KiB 초과 거부; provider 설정과 exact match 필요 |
| `EncryptedDataKey` | 비어 있지 않아야 하며 decoded raw blob은 6144 bytes 이하; KMS `CiphertextBlob` 그대로 저장 |
| `EncryptionContext` | valid UTF-8 non-secret key/value만 사용; sorted entry, exact duplicate key와 64개 초과 거부; key는 case-sensitive하므로 `tenant`와 `Tenant`처럼 case만 다른 distinct key는 허용; `MaxContextSize`는 모든 key/value의 UTF-8 byte 길이 합계(구분자·JSON escaping 제외)로 계산하고 8 KiB 초과 거부; constructor 값과 exact map match 필요 |
| `Nonce` | local `encrypt` AES-GCM nonce size인 12 bytes |
| `Ciphertext` | AES-GCM authentication tag 16 bytes 이상 |

`Envelope`가 반환하는 slice와 map은 parse 결과의 독립 복사본이다. caller가 그 값을 변경해도 이미 생성된 provider 상태나 다른 호출을 변경하지 않는다.

local AEAD associated data는 다음 고정 record의 canonical JSON bytes를 metadata로
사용한다. field 순서는 `version`, `algorithm`, `key_id`, `encrypted_data_key`,
`encryption_context`이며, `encryption_context`는 위의 sorted entry 배열이다. `nonce`와
`ciphertext`는 각각 AEAD nonce와 authentication tag에 의해 이미 검증되므로 metadata
record에 중복하지 않는다. 최종 AAD bytes는 다음과 같이 domain과 두 length prefix를
big-endian `uint32`로 이어 붙인다.

```text
"BTKMS-AAD\x01" |
u32be(len(metadataJSON)) | metadataJSON |
u32be(len(callerAssociatedData)) | callerAssociatedData
```

`metadataJSON`의 byte field는 wire와 같은 표준 padded base64이고, caller associated
data는 최대 `MaxAssociatedDataSize`다. 따라서 key ID, algorithm, encrypted data key,
정렬된 context, caller AD 중 하나라도 바뀌면 동일한 data key를 복호화하더라도 local
AEAD authentication이 실패한다.

## 암호화 data flow

1. `ctx == nil`이면 background context로 정규화하고, 즉시 `ctx.Err()`를 확인한다.
2. plaintext와 caller associated data의 byte 한도를 확인한다. 초과하면 KMS 호출 없이 `ErrInputTooLarge`를 반환한다. `MaxPlaintextSize` boundary는 통과하고 `+1`은 실패해야 한다.
3. caller-owned client로 `GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{KeyId: keyID, KeySpec: kmstypes.DataKeySpecAes256, EncryptionContext: clone(context)})`를 한 번 호출한다. client는 context를 협력적으로 관찰하고 provider는 retry를 추가하지 않는다.
4. output이 non-nil이면 `err`와 길이를 검사하기 전에 `defer zeroBytes(output.Plaintext)`와 `defer zeroBytes(output.CiphertextBlob)`를 예약한다. `(output, err)` 반환과 panic을 포함해 SDK slice를 zero한다. `CiphertextBlob`이 비어 있거나 `MaxEncryptedDataKeySize`를 넘으면 `ErrInvalidDataKey`를 반환하고, metadata/AAD에 사용할 blob은 별도 deep copy를 만든다.
5. `localKey := append([]byte(nil), output.Plaintext...)`를 만든 직후 `defer zeroBytes(localKey)`를 예약한 뒤 `encrypt.New(localKey)`로 local AES-GCM facade를 만든다. output slice와 local copy는 모든 반환 경로에서 각각 zero한다. metadata JSON과 caller associated data는 length-prefixed AAD로 결합한다.
6. `ctx.Err()`를 확인한 뒤 `EncryptDetached`로 random 12-byte nonce와 sealed ciphertext를 만들고, metadata·nonce·ciphertext를 `Envelope`에 넣어 `MarshalBinary`한다. 직렬화 결과가 `MaxEnvelopeSize`를 넘으면 `ErrInputTooLarge`를 반환한다.
7. 결과 publish 직전에 `ctx.Err()`를 다시 확인한다. 취소된 경우 결과 bytes를 폐기하고 nil과 `context.Canceled` 또는 `context.DeadlineExceeded`를 반환한다. provider는 local crypto 중간을 강제 중단하지 않지만 최종 publish 경계에서 취소된 결과를 노출하지 않는다.

KMS plaintext key는 caller에게 반환하지 않는다. `encrypt.New` 내부의 key copy와 AES expanded key는 Go crypto implementation이 관리하므로, 문서는 삭제를 best-effort로 한정한다.

## 복호화 data flow

1. context를 정규화하고 즉시 `ctx.Err()`를 확인한다. pre-canceled context는 malformed envelope나 metadata mismatch보다 우선한다.
2. caller associated data의 byte 길이를 먼저 확인한다. `MaxAssociatedDataSize` 초과는 envelope parse나 KMS 호출보다 앞서 `ErrInputTooLarge`와 KMS 호출 0회를 보장한다. 이어서 `ParseEnvelope`로 magic, JSON, version, algorithm, size, field lengths와 strict token 규칙을 검증한다.
3. envelope `KeyID`와 context가 provider 설정과 exact match인지 확인한다. 불일치면 KMS를 호출하지 않고 `ErrMetadataMismatch`를 반환한다.
4. parse/metadata validation 뒤와 KMS 호출 직전에 `ctx.Err()`를 다시 확인한다. 취소되면 KMS 호출은 0회다.
5. `Decrypt(ctx, &awskms.DecryptInput{CiphertextBlob: clone(encryptedDataKey), KeyId: keyID, EncryptionContext: clone(context)})`를 한 번 호출한다. output이 non-nil이면 `err`와 길이를 검사하기 전에 `defer zeroBytes(output.Plaintext)`를 예약하고, `(output, err)`·wrong-length·panic을 포함해 SDK slice를 zero한다. `err`/nil output/32 bytes가 아닌 plaintext는 각각 `ErrKMSOperation`/`ErrInvalidDataKey`다.
6. KMS output 이후 `ctx.Err()`를 확인한다. 취소되면 local decrypt를 실행하지 않고 cancellation error를 반환한다. `localKey`를 별도 복사하고 즉시 zero defer를 예약한다.
7. 같은 canonical metadata AAD와 envelope nonce로 `DecryptDetached`를 호출한다. local decrypt 뒤 최종 `ctx.Err()`를 확인하고, 취소 시 local plaintext를 zero한 뒤 결과를 폐기한다. nonce 변경, ciphertext 변경, associated data 변경, metadata 변경은 `encrypt.ErrAuthenticationFailed`를 보존한다.

## 오류와 cancellation 계약

provider는 다음 sentinel을 제공한다.

```go
var (
	ErrNilClient           = errors.New("kms: client must not be nil")
	ErrInvalidKeyID        = errors.New("kms: key ID must not be empty")
	ErrInvalidProvider     = errors.New("kms: invalid provider")
	ErrInvalidOptions      = errors.New("kms: invalid options")
	ErrInputTooLarge       = errors.New("kms: input exceeds limit")
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
| zero-value provider | nil receiver 또는 zero-value `Provider` | 0 | `ErrInvalidProvider` |
| preflight | 이미 취소/마감된 context | 0 | context error |
| GenerateDataKey/Decrypt | SDK error | 1 | `ErrKMSOperation` + 원인 |
| KMS output validation | nil output, missing encrypted/plaintext key, wrong DEK length, encrypted blob > 6144 bytes | 1 | `ErrInvalidDataKey` |
| input bound | plaintext/associated data/envelope/blob/context 한도 초과 | 0 | `ErrInputTooLarge` 또는 `ErrMalformedEnvelope` |
| post-KMS cancellation | 응답 뒤 local crypto 전 context 취소 | 1 | context error, local crypto 0 |
| envelope parse | magic/JSON/size/field/unknown-field 오류 | 0 | `ErrMalformedEnvelope` 또는 version/algorithm sentinel |
| metadata validation | key ID/context mismatch | 0 | `ErrMetadataMismatch` |
| local AEAD | tamper, wrong AAD, wrong nonce, wrong DEK | 1 on decrypt path | `ErrAuthenticationFailed` 보존 |

모든 KMS output plaintext 경로는 output이 non-nil인 즉시 best-effort zero defer를 예약하고
성공·실패·cancellation·validation panic 경로를 포함한다. `output.Plaintext` 자체와
provider가 별도로 만드는 mutable local copy를 모두 zero한다. Go compiler/GC와
`encrypt.New` 내부 AES expanded key까지 삭제된다고 주장하지 않는다. fake는 output slice를
반환 전에 자체 복사해 zero 검증과 input aliasing 검증을 분리한다.

## 테스트와 DoD

`encrypt/kms` fake client는 state를 보호할 때만 mutex를 잡고, blocking channel을 기다리는 동안에는 mutex를 보유하지 않는다. 매 호출마다 SDK output의 `Plaintext`와 `CiphertextBlob`을 fresh deep copy로 반환하며 `t.Cleanup`이 모든 block channel을 해제한다. Provider는 복사 후 SDK `Plaintext`와 `CiphertextBlob`을 모두 best-effort zero하고, `GenerateDataKey`/`Decrypt` 입력도 deep-copy한다. 다음 table-driven 테스트를 작성한다.

- constructor: nil client, blank/oversized/invalid-UTF-8 key ID, nil option, context copy/empty context, zero-value provider
- round trip: AES-256 data key, KMS context, `BTKMS` parse/marshal, associated data
- envelope: deterministic metadata bytes, sorted context, unknown/trailing/duplicate/case-variant/invalid-UTF-8 fields, canonical base64, top-level whitespace/reorder/escaped-name/alternate-number mutations, size/length limits, unsupported version/algorithm, oversized encrypted data key rejected before KMS
- metadata: provider key ID/context mismatch가 KMS 호출 전에 거부되는지
- KMS failures: GenerateDataKey/Decrypt error wrapping, nil output, wrong key lengths, empty encrypted blob
- local failures: nonce/ciphertext tamper, associated data mismatch, encrypted data key tamper, redaction of all sensitive fixtures
- cancellation: preflight-before-parse, inside KMS, post-KMS-before-local-crypto, local-crypto/final-publish deadline; a deterministic Context test double flips `Err()` at a documented checkpoint (timer-only evidence is insufficient), with expected call counts and zero plaintext; blocking fake cleanup must always unblock
- ownership: provider가 client를 close/reconfigure하지 않음, constructor/input/output slice와 map mutation isolation
- concurrency: shared immutable provider stress test와 `go test -race` under a concurrent-safe, context-aware fake; `GenerateDataKey` and `Decrypt` counts are asserted independently; bounded timeout and no goroutine leak
- bounds/performance: plaintext and associated-data preflight with zero KMS calls; fake logical call count plus detached AEAD, envelope marshal/parse, and provider round-trip `-benchmem` matrix for 1 KiB/1 MiB/`MaxPlaintextSize` (boundary accepted, +1 rejected); fake outputs are fresh each iteration; setup/fake/fixture stay outside timers. Use `-benchtime=1x` for the maximum fixture, `-count=3 -cpu=1,2,4` for repeatability, and `-timeout=10m`; reset and assert `GenerateDataKey=1` and `Decrypt=1` independently per sub-benchmark, record raw output plus commit SHA, Go/OS/CPU/GOMAXPROCS, fixture identity, dirty-tree state, and metric direction. If no prior baseline exists, `no_regression=N/A`; do not claim live AWS RTT or retry attempts.
- examples: package example compiles without live credentials or AWS network

검증 명령은 순서대로 `gofmt`, `go test -count=1 ./encrypt ./encrypt/kms`, `go test -race -count=1 ./encrypt ./encrypt/kms`, `go test -timeout=10m -p 1 -count=1 ./...`, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, `make ci`다. Docker-backed 기존 패키지는 저장소 규칙에 따라 순차 실행한다.

완료 DoD는 provider code/test, detached parent contract, English/Korean package README와 root README parity, exact dependency diff, safe error/cancellation 증거, race 결과, bounded benchmark evidence, skill GREEN/REFACTOR pressure 결과, lesson을 모두 포함한다. live AWS, PR, merge, publication은 명시적인 후속 gate로 남긴다.

## 대안과 결정

| 대안 | 결정 | 이유 |
|---|---|---|
| provider가 AES-GCM을 직접 구현 | 거부 | `encrypt`와 nonce/error 계약이 갈라지고 crypto drift가 생긴다. |
| opaque `BTENC`를 그대로 outer ciphertext로 저장 | 거부 | nonce가 독립 metadata가 아니며 parent wire format 내부 의존을 만든다. |
| custom first-party KMS request interface와 AWS adapter | 보류 | SDK coupling은 줄지만 adapter·request copy가 별도 API가 된다. 현재 저장소의 `dynamodb/batchwrite` 패턴과 #519의 “caller-provided KMS client interface”가 직접 SDK subset을 지지한다. |
| encrypted DEK cache/reuse | 거부 | key retention과 rotation/lifecycle이 provider 범위를 넘어가고 cancellation/zero proof가 약해진다. |

이 설계는 #519의 범위만 닫는다. S3, SQS/SNS, DynamoDB provider와 KMS cache/rotation은 parent #517의 별도 자식 issue로 남긴다.
