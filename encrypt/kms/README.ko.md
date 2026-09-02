# encrypt/kms

[English](README.md)

`encrypt/kms`는 caller-owned AWS SDK for Go v2 KMS client와 local `encrypt`
AES-GCM facade를 조합합니다. `GenerateDataKey`로 AES-256 data key 하나를 받고,
payload는 local에서 암호화한 뒤 encrypted data key, canonical metadata, nonce,
sealed bytes를 bounded `BTKMS` envelope에 저장합니다.

## 사용 예

```go
client := kms.NewFromConfig(cfg) // cfg, credential, retry, lifecycle은 caller 소유
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

위 코드는 조합 경계를 보여주는 예이며, package example과 test는 fake client만
사용하므로 AWS credential이나 network가 필요하지 않습니다.

## 계약과 한도

- Provider는 `GenerateDataKey`와 `Decrypt`만 사용하며 logical operation마다 한 번
  호출합니다. Retry, logging, client close, reconfiguration은 추가하지 않습니다.
- 주입한 client는 동시 호출을 지원하고 전달받은 `context.Context`를 관찰해야 합니다.
  비협조적인 호출을 provider가 강제로 중단하지 않습니다.
- `KeyID`는 caller가 준 문자열을 그대로 저장하고 비교합니다. Alias와 ARN 표기는
  다른 metadata입니다. Alias retarget가 복구 동작을 바꾸지 않아야 하는 장기 데이터에는
  immutable key ARN/ID를 사용하십시오.
- Encryption context key/value는 secret이 아닌 valid UTF-8 문자열이어야 합니다. Key는
  case-sensitive하므로 `tenant`와 `Tenant`는 서로 다른 key이고, exact duplicate만
  거부합니다. Context는 KMS/CloudTrail에 보일 수 있으므로 credential, PII, payload를
  넣지 마십시오.
- Plaintext는 `MaxPlaintextSize`(32 MiB), associated data는
  `MaxAssociatedDataSize`(64 KiB), encrypted data key는
  `MaxEncryptedDataKeySize`(6144 byte), serialized envelope는
  `MaxEnvelopeSize`(64 MiB)까지입니다. `BTKMS` parser는 strict canonical 규칙을
  적용하며 unknown, duplicate, case-variant, non-canonical, trailing JSON을 거부합니다.

`BTKMS`는 `encrypt`의 local `BTENC` wire format과 의도적으로 호환되지 않습니다.
자동 migration도 하지 않습니다. Rollout이 필요하면 caller가 reader-first/writer-later
또는 dual-read/dual-write 계획, 명시적 historical re-encryption, rollback 경로를
소유해야 합니다.

## Key, IAM, 운영

KMS client 생성, credential, retry와 timeout policy, key policy, rotation, encrypted
data key cache, retention, lifecycle은 caller 책임입니다. 최소 IAM 권한은 선택한 symmetric
key에 대한 `kms:GenerateDataKey`와 `kms:Decrypt`입니다. Latency, 성공/실패, cancellation,
logical provider call을 caller instrumentation으로 기록하되 key ID, context 값,
plaintext, associated data, raw envelope, unwrap된 AWS error text를 일반 log나 metric
label에 넣지 마십시오.

Error는 `ErrKMSOperation`, `ErrInvalidDataKey`, `ErrMetadataMismatch`,
`ErrAuthenticationFailed` 같은 safe sentinel을 제공합니다. `Error()`에는 sentinel과
고정 operation label만 포함됩니다. Caller-owned 진단 정책이 허용할 때만
`errors.Is`/`errors.As`로 wrapped cause를 확인하십시오.

KMS plaintext output과 provider local key copy는 성공, 오류, cancellation, malformed
output, panic 경로에서 best-effort zero합니다. Go garbage collector와 crypto 구현의
expanded AES key는 이 보장 범위 밖입니다. 주입한 client는 method 반환 뒤 output buffer를
보관하거나 재사용하지 않아야 합니다.

## 검증

```bash
go test -count=1 ./encrypt/kms
go test -race -count=1 ./encrypt/kms
```

Test는 deterministic fake만 사용합니다. Benchmark는 local crypto, envelope 작업,
logical fake call을 측정하며 live AWS latency나 retry attempt를 주장하지 않습니다.
