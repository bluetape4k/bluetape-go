# Issue 71 Encryption Facade Research Scope

Issue #71은 #42가 KMS를 여기로 라우팅하고 #43이
AEAD/DAEAD/keyset/MAC/digest 경계를 여기로 라우팅한 뒤, bluetape-go가 Go
service encryption facade를 노출해야 하는지 결정한다. 결론은 좁다.
Default byte/string AEAD facade는 Go standard library로 구현하고,
keyset/KMS/streaming variant는 concrete caller가 필요로 할 때까지 보류한다.

## 소스 인벤토리

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-projects/io/tink`

- `TinkAead`는 associated data를 포함한 AEAD byte 및 UTF-8 string encryption을
  감싼다.
- `TinkDeterministicAead`는 searchable field용 deterministic AEAD를 감싸고
  equality leakage를 문서화한다.
- `VersionedKeysetStore`는 current/find/rotate/rotate-if-due operation을
  제공한다.
- `VersionedCiphertextSupport`는 ciphertext 앞에 8-byte version을 붙인다.
- Redis-backed store는 generic cache나 config wrapper가 아니라 protected
  shared keyset boundary를 제공한다.
- MAC과 digest helper는 encryption이 아니라고 명시적으로 문서화되어 있다.

Relevant bluetape-go evidence:

- Go module target은 `go 1.26.3`이고 local toolchain은 `go1.26.4`다.
- `jwt`는 이미 sentinel errors, safe key material copying, repository
  rotation, Redis/Mongo storage boundaries, stress/race expectations를 가진다.
- `docs/research/2026-06-25-issue-42-aws-scope.md`는 KMS envelope
  compatibility를 여기로 라우팅하고 generic AWS KMS wrapper를 기각한다.
- `docs/research/2026-06-25-issue-43-io-codec-protocol-scope.md`는
  AEAD/DAEAD/keysets/MAC/digest를 여기로 라우팅하고 broad I/O wrapper를
  기각한다.
- `bluetape4k-exposed` PR #300은 persisted encrypted column의 unsafe
  generated-keyset default를 제거했다. durable ciphertext에는 explicit
  persisted/versioned key material이 필요하다는 증거다.

## 외부 근거

- Go 1.26의 `crypto/cipher`는 random 96-bit nonce를 ciphertext 앞에 붙이고
  nonce size가 zero인 AEAD를 caller에게 주는 `NewGCMWithRandomNonce`를
  노출한다. 이는 common API가 caller-managed nonce를 노출하면 안 된다는
  #71 요구를 직접 지원한다.
- Tink deterministic encryption documentation은 AES256-SIV를 권장하지만,
  deterministic ciphertext가 repeated plaintext equality를 드러낼 수 있다고
  경고한다. 또한 실제 애플리케이션에서 cleartext keyset을 쓰지 말라고
  경고한다.
- Tink key-management documentation은 KMS/KEK로 보호되는 encrypted keyset을
  권장하고 AWS KMS URI support를 문서화한다. 하지만 이는 작은 default
  encryption facade가 아니라 operational keyset story다.
- `filippo.io/age`는 password-derived identities를 포함한
  recipient/identity-based file 또는 stream encryption에 credible하지만,
  general service byte/string AEAD facade가 아니라 file/recipient shaped이다.
- AWS KMS `GenerateDataKey`는 local envelope encryption을 위한 plaintext data
  key와 encrypted data-key blob을 반환하고, decrypt에는 encryption-context
  matching이 필요하며, local use 이후 plaintext data key를 지우라고 말한다.
- AWS Encryption SDK for Go는 존재하고 Go 1.23+를 요구하지만, 이를 채택하면
  AWS/KMS policy가 optional이 아니라 default package의 중심이 된다.

Sources:

- https://pkg.go.dev/crypto/cipher
- https://developers.google.com/tink/deterministic-encryption
- https://developers.google.com/tink/key-management-overview
- https://pkg.go.dev/filippo.io/age
- https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html
- https://docs.aws.amazon.com/encryption-sdk/latest/developer-guide/go.html

## 후보 순위

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Stdlib AES-GCM byte/string facade | High | Medium | Implement in #315. |
| Versioned ciphertext envelope | High | Medium | Include in #315 so future keys/KMS can evolve safely. |
| Explicit associated data | High | Medium | Include in #315 as a first-class input. |
| Deterministic AEAD/searchable encryption | Medium | High | Defer; needs explicit searchable-field consumer and leakage docs. |
| Tink Go keyset integration | Medium/high | High | Defer until versioned keyset store/KMS owner is needed. |
| Redis keyset store | Medium | High | Defer; do not store encryption keys in Redis without a protected secret-store contract. |
| AWS KMS envelope support | Medium/high | High | Defer to a future adapter; keep caller-owned AWS SDK clients and encryption context. |
| AWS Encryption SDK | Medium | High | Defer; too broad for the default facade. |
| age stream/file encryption | Medium | Medium/high | Defer until a file/archive or recipient workflow needs it. |
| MAC/digest helpers | Medium | Medium | Document boundaries; no package now. |
| JWT signing/password hashing | Low | High | Out of scope; existing packages and specialized libraries own these. |

## 구현

Focused stdlib-backed package용 #315를 만든다.

- `[]byte`와 UTF-8 `string`을 encrypt/decrypt한다.
- Authenticated encryption만 사용한다.
- Go 1.26 `cipher.NewGCMWithRandomNonce`를 사용해 nonce management를 숨긴다.
- Explicit caller-provided key material 또는 narrow key provider를 요구한다.
- Tenant/entity/column/message context binding을 위해 associated data를 받는다.
- Invalid key, malformed envelope, authentication failure, wrong key, tamper,
  invalid options에 대해 typed/sentinel error를 반환한다.
- Later key IDs, deterministic variants, KMS envelope data를 initial API 변경
  없이 추가할 수 있도록 versioned envelope를 사용한다.
- Safe logging을 문서화한다. Plaintext, ciphertext, key bytes, associated
  data를 error에 포함하지 않는다.

## 보류

- Concrete searchable-field package 또는 database example이 필요로 할 때까지
  Deterministic AEAD를 보류한다.
- Keyset rotation, encrypted keyset storage, KMS-backed keyset protection이
  first-class feature가 될 때까지 Tink Go를 보류한다.
- Key material owner가 정의되고 protected storage requirement가 명확해질
  때까지 Redis/Mongo keyset repository를 보류한다.
- Cloud adapter issue가 data-key generation, encryption context, redaction,
  caching, retry behavior를 소유할 때까지 AWS KMS envelope encryption을
  보류한다.
- File/stream/recipient workflow가 생길 때까지 age를 보류한다.
- Later integrity-only API가 repeated caller value를 증명하기 전까지
  MAC/digest helper를 보류한다.

## 후속 이슈

- #315: implement the stdlib AES-GCM encryption facade.

이 pass에서는 Tink, age, AWS KMS, deterministic AEAD, Redis keysets, MAC,
digest에 대한 추가 follow-up issue를 만들지 않는다.

## 검증 계획

- Documentation-only PR: `git diff --check` and targeted `rg`.
- #71과 #315 issue body에 #71 research outcome이 들어 있는지 확인한다.
- External evidence는 `bluetape4k-wiki`에 보존하고 `gno update`,
  `gno embed --collection bluetape4k-wiki`, representative `gno search`로
  검증한다.
- Go code change가 없으므로 이 PR에는 Go test가 필요하지 않다.

## 후속 권고

이 research PR로 #71을 닫고, 이후 0.12.0 delivery slice에서 #315를 구현한다.
#315가 standard-library envelope로 documented test를 만족할 수 없음을
발견하기 전까지 default package는 dependency-free로 유지한다.
