# Issue #519 KMS envelope provider lesson

## 결정

`encrypt/kms`는 caller-owned AWS SDK for Go v2 KMS client의
`GenerateDataKey`/`Decrypt` 두 method만 주입받는다. KMS client의 credential,
retry, timeout, lifecycle, key policy, rotation, cache, logging은 provider가
소유하지 않는다. Payload는 KMS에 직접 보내지 않고 `encrypt`의 standard
AES-GCM detached 경계를 재사용한다.

기존 `BTENC`와 새 `BTKMS`는 서로 다른 wire contract다. 자동 migration은 하지
않으며, rollout이 필요하면 caller가 reader-first/writer-later 또는 dual-read/
dual-write, historical re-encryption, rollback을 설계한다.

## 재발 방지 guard

- AES-GCM은 `cipher.NewGCM`과 `crypto/rand.Reader`의 12-byte nonce, 16-byte tag를
  사용한다. `cipher.NewGCMWithRandomNonce`의 zero nonce size API를 detached
  provider 경계에 재사용하지 않는다. `BTENC`의 기존 `header|nonce|ciphertext+tag`
  bytes는 계속 읽는다.
- `BTKMS` parser는 `BTKMS` prefix, strict token walk, exact/lowercase duplicate
  field 검사, unknown/null/invalid UTF-8/trailing 거부, canonical re-marshal byte
  비교, padded base64 canonical 검사를 모두 통과해야 한다. Context key는
  case-sensitive exact 문자열이고 case만 다른 distinct key는 허용한다.
- `MaxPlaintextSize=32 MiB`, `MaxAssociatedDataSize=64 KiB`, encrypted data key
  6144 bytes, envelope 64 MiB의 사전 한도를 유지한다. ciphertext에서 tag를 뺀
  plaintext 길이는 parse 전에 검사해 oversized input이 KMS에 도달하지 않는다.
- KMS output이 non-nil이면 error/길이 검증보다 먼저 plaintext와 ciphertext blob에
  zero defer를 예약한다. Metadata에 쓸 blob과 local AES key는 별도 복사하고,
  local key도 모든 반환 경로에서 best-effort zero한다. Go GC와 AES expanded key
  삭제는 주장하지 않는다.
- Context는 preflight, parse/metadata 뒤, KMS 뒤, 최종 publish 전에 확인한다.
  결과를 반환하지 못하는 cancellation은 deterministic context double로 검증하고,
  provider가 KMS retry나 비협조적인 goroutine 강제 종료를 추가하지 않는다.
- Error 문자열은 safe sentinel과 고정 operation label만 포함한다. AWS cause는
  `Unwrap`/`Is`로만 선택적으로 조회하며 key ID, context, plaintext, raw envelope를
  metric label이나 일반 log에 넣지 않는다.

## 검증과 release bookkeeping

모든 provider test는 live credential/network 없이 mutex/deep-copy/context-aware
fake를 사용한다. Fake는 block 중 mutex를 잡지 않고, 호출마다 fresh output을
반환하며 `t.Cleanup`으로 block을 해제한다. Concurrency 검증은 `go test -race`와
독립적인 GenerateDataKey/Decrypt logical call count를 사용한다. Benchmark는
1 KiB, 1 MiB, 32 MiB fixture를 나누고 최대 fixture serial `-benchtime=1x`, 작은
fixture parallel time-based 실행을 분리한다. 현재 baseline이 없으면
`no_regression=N/A`로 기록하고 live AWS RTT나 SDK retry attempt를 주장하지 않는다.

이번 변경의 public release bookkeeping은 `CHANGELOG.md`의 `[Unreleased]`에
기록했다. `WIP.md`의 release train, tag, GitHub Release, PR/merge/publish 상태는
별도 release gate에서 갱신하며, 이 issue 작업에서는 외부 mutation을 수행하지
않는다.
