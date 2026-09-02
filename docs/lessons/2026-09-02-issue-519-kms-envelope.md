# Issue #519 KMS envelope provider 교훈

Issue: [#519](https://github.com/bluetape4k/bluetape-go/issues/519)
Parent: [#517](https://github.com/bluetape4k/bluetape-go/issues/517)
범위: caller-owned AWS SDK for Go v2 KMS client를 사용하는 `encrypt/kms`와
재사용 가능한 `encrypt` detached 경계

## 맥락과 결정

`encrypt/kms`는 caller-owned AWS SDK for Go v2 KMS client의
`GenerateDataKey`/`Decrypt` 두 method만 주입받는다. KMS client의 credential,
retry, timeout, lifecycle, key policy, rotation, cache, logging은 provider가
소유하지 않는다. Payload는 KMS에 직접 보내지 않고 `encrypt`의 standard
AES-GCM detached 경계를 재사용한다.

기존 `BTENC`와 새 `BTKMS`는 서로 다른 wire contract다. 자동 migration은 하지
않으며, rollout이 필요하면 caller가 reader-first/writer-later 또는 dual-read/
dual-write, historical re-encryption, rollback을 설계한다.

## 발견과 수정

초기 구현은 KMS envelope라는 외형을 먼저 만들면 충분하다고 가정했다. RED와
7-Tier 렌즈는 다음 누락을 드러냈다.

- KMS client lifecycle, credential, retry, IAM, logging을 provider가 가져가면
  caller의 운영 정책과 SDK 계약을 숨기게 된다. 두 SDK method만 받는
  caller-owned interface로 경계를 고정했다.
- KMS output을 길이 검사한 뒤 지우면 `(output, err)`, malformed output,
  cancellation 경로에서 plaintext data key가 남는다. non-nil output을 받은
  직후 plaintext와 encrypted blob에 zero `defer`를 예약하고 local key와
  metadata blob copy도 별도로 지운다. 이는 Go GC나 AES expanded key 삭제를
  보장하지 않는 best-effort 계약이다.
- metadata를 일반 JSON decoder에 맡기면 duplicate/case-variant/unknown/null/
  whitespace/trailing 입력을 묵인할 수 있다. `BTKMS` parser는 bounded token
  scanner로 exact field/order와 canonical string/number를 확인하고, base64를
  destination에 직접 decode한다. `MarshalBinary`도 큰 base64 값을 streaming
  encoder로 써서 canonical envelope 전체를 다시 만들지 않는다.
- 32 MiB payload에서 `RawMessage`와 canonical re-marshal을 겹치면 allocation이
  650--973 MB/op까지 증폭했다. lexical parser와 streaming marshal로 바꾼 뒤
  최대 `ProviderRoundTrip`은 약 145 MB/op로 내려갔다. 이후 `EncryptDetached`가
  `Seal(nil)`의 새 ciphertext와 새 nonce를 다시 복사하던 P2도 제거했다.
- fake가 mutex를 block 구간에 잡거나 output slice를 재사용하면 cancellation,
  aliasing, concurrent contract를 검증하지 못한다. fake는 호출마다 deep copy와
  fresh output을 만들고, block 해제와 child goroutine 종료를 `t.Cleanup` 및
  bounded wait로 보장한다.

## 재발 방지 guard

- AES-GCM은 `cipher.NewGCM`과 `crypto/rand.Reader`의 12-byte nonce, 16-byte tag를
  사용한다. `cipher.NewGCMWithRandomNonce`의 zero nonce size API를 detached
  provider 경계에 재사용하지 않는다. `BTENC`의 기존 `header|nonce|ciphertext+tag`
  bytes는 계속 읽는다.
- `BTKMS` parser는 `BTKMS` prefix, bounded lexical token walk, exact/lowercase
  duplicate field 검사, unknown/null/invalid UTF-8/trailing 거부, canonical
  string/number 검사, padded base64 canonical 검사를 모두 통과해야 한다. 전체
  envelope를 다시 marshal해 비교하지 않으며, context key는 case-sensitive exact
  문자열이고 case만 다른 distinct key는 허용한다.
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

## Benchmark evidence

이번 benchmark 측정은 production ranking이 아닌 local allocation/latency와 fake의
logical KMS call count를 확인하는 자료다. 이전 benchmark baseline은 없으므로
`no_regression=N/A`로 기록한다.

| 항목 | 값 |
|---|---|
| benchmark commit | `12a1e0601aa3f42e5ef9c0f4cd3772d415da6e3a` |
| benchmark dirty tree | `false` |
| 환경 | `darwin/arm64`, Apple M4 Pro, 12 logical CPU, Go `go1.27.1`, `go.mod` `go 1.26.3`, `GOMAXPROCS` unset |
| fixtures | `1KiB`, `1MiB`, `MaxPlaintextSize=32 MiB`; fake outputs와 setup은 timer 밖 |
| 범위 | live AWS credential/network, SDK retry/network attempt, production throughput은 측정하지 않음 |

실행 명령:

```bash
go test -timeout=10m -run '^$' \
  -bench '^Benchmark(Detached|EnvelopeMarshalParse|Provider(Encrypt|Decrypt|RoundTrip))/(1KiB|1MiB|MaxPlaintextSize)' \
  -benchmem -benchtime=1x -count=3 -cpu=1,2,4 ./encrypt/kms
```

대표 raw rows는 다음과 같다. `-count=3 -cpu=1,2,4` 전체 출력은 이 실행에서
터미널로 읽었고, 아래 값은 같은 실행의 최대 fixture와 parallel 경로를 보존한
요약이다.

```text
BenchmarkEnvelopeMarshalParse/MaxPlaintextSize-4   1  80734625 ns/op  33569456 B/op  32 allocs/op
BenchmarkProviderEncrypt/MaxPlaintextSize-4        1  15805542 ns/op  78314712 B/op  39 allocs/op
BenchmarkProviderDecrypt/MaxPlaintextSize-4        1 229348834 ns/op  67126296 B/op  45 allocs/op
BenchmarkProviderRoundTrip/MaxPlaintextSize-4      1 245781416 ns/op 145442296 B/op  90 allocs/op
BenchmarkProviderRoundTripParallel/1MiB-4          109 2099372 ns/op   4577016 B/op  71 allocs/op
```

직렬 benchmark는 `GenerateDataKey`/`Decrypt` logical call count를 각각 `b.N`과
비교하고, parallel benchmark는 두 count 모두 `b.N`과 비교한다. 최대 fixture의
입력·envelope 한도와 KMS 전 조기 거부도 별도 test로 유지한다.

작은 fixture parallel 실행도 별도로 확인했다.

```bash
go test -timeout=10m -run '^$' \
  -bench '^BenchmarkProviderRoundTripParallel/(1KiB|1MiB)' \
  -benchmem -benchtime=200ms -count=3 -cpu=1,2,4 ./encrypt/kms
```

```text
BenchmarkProviderRoundTripParallel/1KiB-4  62632  3826 ns/op  13673 B/op  63 allocs/op
BenchmarkProviderRoundTripParallel/1MiB-4    109  2099372 ns/op  4577016 B/op  71 allocs/op
```

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
