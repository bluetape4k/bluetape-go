# encrypt

[English](README.md) | [한국어](README.ko.md)

`encrypt` provides a small standard-library AES-GCM facade for local service
data. It hides nonce management by using Go's random-nonce GCM AEAD and wraps
ciphertext in a versioned envelope.

## Diagrams

![encrypt class contract map](../docs/images/readme-diagrams/encrypt-class-contract-map.png)

![encrypt envelope sequence](../docs/images/readme-diagrams/encrypt-envelope-sequence.png)

![sqlkit column scan and value sequence](../docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/encrypt"
```

## Usage

```go
key := loadPersistedAESKey()
ad := []byte("tenant=blue:entity=invoice:column=payload")

ciphertext, err := encrypt.Encrypt(key, []byte("secret payload"), ad)
if err != nil {
    return err
}

plaintext, err := encrypt.Decrypt(key, ciphertext, ad)
```

For UTF-8 text, the string helpers return URL-safe raw base64:

```go
encryptor, err := encrypt.New(key)
if err != nil {
    return err
}

token, err := encryptor.EncryptString("secret text", ad)
text, err := encryptor.DecryptString(token, ad)
```

String helpers reject invalid UTF-8. Use byte helpers for arbitrary binary
payloads.

## Key Material

- Keys must be AES-128, AES-192, or AES-256 material: 16, 24, or 32 bytes.
- Callers own key generation, persistence, rotation, backup, and access control.
- Do not generate a fresh process-local key for ciphertext that must survive
  restarts or be read by another process.
- `New` copies the key before constructing the encryptor. It does not retain the
  caller's slice.

## Associated Data

Associated data binds ciphertext to context without encrypting that context. Use
stable values such as tenant, entity, column, message type, or protocol version.
The exact same associated data must be supplied for decryption; wrong associated
data returns `ErrAuthenticationFailed`.

## SQL Column Integration

`sqlkit.NewEncryptedBytesColumn` stores the binary `BTENC` envelope in a
BYTEA/BLOB column. `sqlkit.NewEncryptedStringColumn` stores the same envelope
as raw URL-safe base64 in a TEXT/VARCHAR column. Both constructors copy their
associated data.

Use stable associated data such as tenant, entity, column, and protocol version
so ciphertext cannot be moved silently to a different context. Callers still
own key persistence, access control, rotation, and the strategy for decrypting
historical rows after rotation.

Random nonces deliberately prevent equality, ordering, and filtering queries
over ciphertext. When search is a real requirement, design and review a
separate blind-index system; do not replace the random nonce with a fixed one.
Cloud KMS and envelope encryption remain separate provider concerns.

## Envelope

Byte ciphertext uses this envelope:

```text
BTENC | version=0x01 | algorithm=0x01 | random-nonce AES-GCM ciphertext
```

The AES-GCM payload is produced by `cipher.NewGCMWithRandomNonce`. The standard
library prepends a random 96-bit nonce to each ciphertext and adds a 16-byte
authentication tag, for 28 bytes of AEAD overhead before the plaintext length.

String ciphertext is raw URL-safe base64 of the same byte envelope.

## Errors

Errors support `errors.Is` against:

- `ErrInvalidKey`
- `ErrMalformedCiphertext`
- `ErrAuthenticationFailed`
- `ErrInvalidOptions`

Error strings are safe to log and do not include plaintext, ciphertext, key
bytes, or associated data.

## Boundaries

Use this package for local byte/string authenticated encryption. Use a different
tool when the problem is:

- Direct stdlib: when the caller needs a lower-level AEAD contract and owns all
  nonce/envelope compatibility details.
- Tink: when keysets, registry-managed primitives, or deterministic AEAD are
  first-class requirements.
- age: when encrypting files or streams for recipients or password-derived
  identities.
- KMS/envelope encryption: when data keys, encryption context, cloud policy,
  audit, and retry behavior belong to a cloud adapter.
- Password hashing: use a password-hashing package, not reversible encryption.
- MAC/digest: use integrity-only primitives when plaintext secrecy is not
  required.
- JWT: use `jwt` for signed tokens and key rotation.

## Test

```bash
go test -count=1 ./encrypt
go test -race -count=1 ./encrypt
```
