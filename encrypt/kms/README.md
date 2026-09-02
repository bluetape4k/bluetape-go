# encrypt/kms

[한국어](README.ko.md)

`encrypt/kms` combines a caller-owned AWS SDK for Go v2 KMS client with the
local `encrypt` AES-GCM facade. `GenerateDataKey` supplies one AES-256 data key;
the provider encrypts the payload locally and stores the encrypted data key,
canonical metadata, nonce, and sealed bytes in a bounded `BTKMS` envelope.

## Usage

```go
client := kms.NewFromConfig(cfg) // caller owns cfg, credentials, retries, and lifecycle
provider, err := kmsenvelope.New(
    client,
    "arn:aws:kms:ap-northeast-2:123456789012:key/example",
    kmsenvelope.WithEncryptionContext(map[string]string{
        "service": "billing",
        "tenant":  "blue",
    }),
)
if err != nil {
    return err
}

envelope, err := provider.Encrypt(ctx, plaintext, []byte("invoice:v1"))
if err != nil {
    return err
}
plaintext, err = provider.Decrypt(ctx, envelope, []byte("invoice:v1"))
```

The example above is a composition sketch; tests and the package example use a
fake client and never require AWS credentials or network access.

## Contracts and limits

- The provider calls only `GenerateDataKey` and `Decrypt`, once per logical
  operation, and never adds retries, logging, client closing, or reconfiguration.
- The injected client must be safe for concurrent calls and must observe the
  supplied `context.Context`. A non-cooperative call is not force-stopped by the
  provider.
- `KeyID` is stored and compared as the exact caller string. Alias and ARN
  spellings are different metadata; use an immutable key ARN/ID for long-lived
  data when alias retargeting must not change recovery behavior.
- Encryption-context keys and values are non-secret, valid UTF-8 strings. Keys
  are case-sensitive: `tenant` and `Tenant` are distinct keys, while an exact
  duplicate is rejected. The context is visible to KMS/CloudTrail and must not
  contain credentials, PII, or payload data.
- Plaintext is limited to `MaxPlaintextSize` (32 MiB), associated data to
  `MaxAssociatedDataSize` (64 KiB), encrypted data keys to
  `MaxEncryptedDataKeySize` (6144 bytes), and serialized envelopes to
  `MaxEnvelopeSize` (64 MiB). `BTKMS` parsing is strict and canonical; unknown,
  duplicate, case-variant, non-canonical, or trailing JSON is rejected.

`BTKMS` is intentionally incompatible with the local `BTENC` wire format from
`encrypt`. There is no automatic migration. A caller that needs a rollout must
own a reader-first/writer-later or dual-read/dual-write plan, then explicitly
re-encrypt historical values and retain a rollback path.

## Key, IAM, and operations

The caller owns KMS client construction, credentials, retry and timeout policy,
key policy, rotation, encrypted-data-key caching, retention, and lifecycle. The
minimum permissions for this provider are `kms:GenerateDataKey` and
`kms:Decrypt` on the selected symmetric key. Use stable caller instrumentation
for latency, success/failure, cancellation, and logical provider calls, but do
not put key IDs, context values, plaintext, associated data, raw envelopes, or
unwrapped AWS error text into ordinary logs or metric labels.

Errors expose safe sentinels such as `ErrKMSOperation`, `ErrInvalidDataKey`,
`ErrMetadataMismatch`, and `ErrAuthenticationFailed`. `Error()` contains only a
sentinel and fixed operation label; inspect the wrapped cause deliberately with
`errors.Is`/`errors.As` when a caller-owned diagnostic policy permits it.

KMS plaintext output and the provider's local key copy are best-effort zeroed on
success, error, cancellation, malformed output, and panic paths. Go's garbage
collector and the crypto implementation's expanded AES key are outside this
guarantee. The injected client must not retain or reuse output buffers after the
method returns.

## Verification

```bash
go test -count=1 ./encrypt/kms
go test -race -count=1 ./encrypt/kms
```

Tests use only deterministic fakes. Benchmarks measure local crypto, envelope
work, and logical fake calls; they do not claim live AWS latency or retry
attempts.
