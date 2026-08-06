# Issue #315 AES-GCM Facade Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 목표

Add the narrow default encryption package selected by #71: a dependency-free
byte/string AEAD facade backed by Go's standard-library
`cipher.NewGCMWithRandomNonce`.

## Package Path

Use root package `encrypt`.

Rejected alternatives:

- `crypto/encrypt`: visually close to the standard-library `crypto/*` tree and
  adds path depth without a second crypto subpackage.
- `codec`: encoding helpers must not grow encryption responsibilities.
- `jwt`: token signing and key rotation are separate from local reversible data
  encryption.

## Public Surface

- `New(key []byte, options ...Option) (Encryptor, error)`.
- `Encryptor` value methods:
  - `Encrypt(plaintext, associatedData []byte) ([]byte, error)`.
  - `Decrypt(ciphertext, associatedData []byte) ([]byte, error)`.
  - `EncryptString(plaintext string, associatedData []byte) (string, error)`.
  - `DecryptString(ciphertext string, associatedData []byte) (string, error)`.
- Convenience package functions with explicit key material:
  - `Encrypt`, `Decrypt`.
- Sentinel errors:
  - `ErrInvalidKey`.
  - `ErrMalformedCiphertext`.
  - `ErrAuthenticationFailed`.
  - `ErrInvalidOptions`.

## Envelope

Byte ciphertext format:

```text
BTENC | version=0x01 | algorithm=0x01 | random-nonce AES-GCM ciphertext
```

The AEAD payload is the output of `cipher.NewGCMWithRandomNonce`, which prepends
a random 96-bit nonce and appends a 16-byte tag. String ciphertext is raw
URL-safe base64 of the byte envelope.

## Required Semantics

- Accept only 16, 24, or 32 byte AES keys and copy key material in `New`.
- Never expose caller-managed nonces.
- Preserve associated data as an explicit parameter.
- Use safe error strings that do not include plaintext, ciphertext, key bytes,
  or associated data.
- Treat tamper, wrong key, and wrong associated data as
  `ErrAuthenticationFailed`.
- Treat unknown envelope version/algorithm, short data, bad magic, and bad
  string base64 as `ErrMalformedCiphertext`.
- Keep `Encryptor` immutable and goroutine-safe.
- Reject invalid UTF-8 in string helpers; byte helpers own arbitrary binary.

## Test Requirements

- Byte and string round trips.
- AES-128, AES-192, and AES-256 key acceptance.
- Wrong key, wrong associated data, and tamper rejection.
- Malformed envelope and malformed base64 rejection.
- Nil/short/unsupported key rejection.
- Invalid option rejection.
- Key-copy regression.
- Safe logging regression.
- `testing/concurrency.GoroutineStressTester` and `AsyncJobTester` coverage.
- `go test -count=1 ./encrypt` and `go test -race -count=1 ./encrypt`.
