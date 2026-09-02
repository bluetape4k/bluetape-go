# Issue #519 AWS KMS Envelope Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development` (recommended) or `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `encrypt/kms`가 caller-owned AWS KMS client로 AES-256 data key를 생성·복호화하고, local `encrypt` AES-GCM과 strict `BTKMS` envelope를 조합해 안전하게 큰 payload를 보호하도록 구현한다.

**Architecture:** 기존 `encrypt.Encryptor`에 nonce와 sealed bytes를 분리하는 detached 경계를 추가하고, 새 `encrypt/kms` package는 AWS SDK for Go v2의 `GenerateDataKey`/`Decrypt` method subset만 주입받는다. Provider는 immutable copied configuration을 사용하고, canonical metadata AAD·bounded strict parser·cooperative cancellation·best-effort DEK zeroing을 단일 흐름으로 적용한다. KMS client lifecycle, retry, credentials, IAM, rotation, cache, logging은 caller가 소유한다.

**Tech Stack:** Go `1.26.3`, standard library `crypto/aes`, `crypto/cipher`, `encoding/json`, `encoding/base64`, `crypto/rand`, AWS SDK for Go v2 `service/kms v1.42.1`, repository `go test`/`make` checks, fake-only tests (live AWS 없음).

---

## 변경 파일 지도

- Modify: `encrypt/encrypt.go` — 기존 random-nonce AES-GCM을 재사용하는 `EncryptDetached`/`DecryptDetached` 추가와 기존 facade 위임.
- Modify: `encrypt/encrypt_test.go` — detached round-trip, nonce/tag validation, zero-value 안전성 회귀 테스트.
- Modify: `encrypt/doc.go`, `encrypt/README.md`, `encrypt/README.ko.md` — detached provider 경계와 선택 규칙 문서화.
- Modify: `go.mod`, `go.sum` — AWS KMS service module `v1.42.1` direct requirement 추가.
- Create: `encrypt/kms/errors.go` — safe sentinel 및 `*Error` wrapping.
- Create: `encrypt/kms/errors_test.go` — externally constructed error redaction 회귀 테스트.
- Create: `encrypt/kms/doc.go` — package comment와 exported API documentation contract.
- Create: `encrypt/kms/envelope.go` — `BTKMS` wire object, canonical metadata/AAD, strict JSON token parser, size validation.
- Create: `encrypt/kms/provider.go` — caller-owned KMS client interface, immutable provider/options, Encrypt/Decrypt data flow, DEK zeroing/cancellation.
- Create: `encrypt/kms/provider_test.go` — mutex/deep-copy/context-aware fake와 table-driven provider 계약 테스트, oversized ciphertext zero-KMS guard.
- Create: `encrypt/kms/benchmark_test.go` — detached/envelope/provider fake benchmark matrix와 logical call count 검증.
- Create: `encrypt/kms/example_test.go` — live credential/network 없이 컴파일되는 context-aware caller-owned fake 사용 예.
- Create: `encrypt/kms/README.md`, `encrypt/kms/README.ko.md` — usage, limits, rotation, lifecycle, safe errors, unsupported live-AWS scope.
- Modify: `README.md`, `README.ko.md` — package index와 AWS/암호화 package documentation parity.
- Create: `docs/lessons/2026-09-02-issue-519-kms-envelope.md` — 2-R/TDD/6-R에서 재발 방지에 필요한 결정과 guard.
- Modify (managed source, separate durable surface): `/Users/debop/.local/share/chezmoi/private_dot_codex/private_skills/bluetape-go-patterns/SKILL.md` — RED에서 관측한 KMS/envelope/DEK lifetime/fake 계약을 재사용 가능한 규칙으로 보강한 뒤 `chezmoi apply`로 live skill 동기화.

Public `/Users/debop/work/bluetape4k/bluetape-skills` bundle은 별도 promotion/release 표면이며 이번 실행에서 자동 변경하지 않는다.

## 실행 게이트

- 구현 전: 승인된 설계 `docs/superpowers/specs/2026-09-02-issue-519-kms-envelope-design.md`의 P1=0 상태와 Step 3-R 계획 검토를 완료한다.
- 구현 중: 각 코드 변경은 RED 테스트 → 실패 확인 → 최소 구현 → targeted GREEN 순서로 수행한다. 기존 unrelated dirty/untracked state는 보존한다.
- 구현 후: Step 6-R 7-Tier 여섯 렌즈와 main integration에서 P0/P1=0을 확인하고, `go`/`make` 전체 검증 및 lesson/skill parity를 완료한다.
- 범위 밖: live AWS credential 호출, PR 생성, push, merge, tag, release, branch/worktree 삭제.

### Task 1: 기존 `encrypt` detached contract를 먼저 고정한다

**Files:**
- Test: `encrypt/encrypt_test.go`
- Modify: `encrypt/encrypt.go`
- Modify: `encrypt/doc.go`, `encrypt/README.md`, `encrypt/README.ko.md`

- [x] **Step 1: detached API failing tests를 추가한다.**

다음 테스트를 `encrypt/encrypt_test.go`에 추가한다. 생성된 nonce는 12 bytes이고 ciphertext는 plaintext와 16-byte tag를 포함하며, detached decrypt는 동일한 associated data에서 원문을 복원해야 한다. 잘못된 nonce/tag와 zero-value encryptor는 기존 sentinel을 보존해야 한다.

```go
func TestDetachedRoundTripAndAuthentication(t *testing.T) {
	enc, err := encrypt.New(testKey(32, 20))
	if err != nil { t.Fatal(err) }
	nonce, sealed, err := enc.EncryptDetached([]byte("payload"), []byte("ad"))
	if err != nil { t.Fatal(err) }
	if len(nonce) != 12 || len(sealed) != len("payload")+16 { t.Fatalf("detached sizes: %d/%d", len(nonce), len(sealed)) }
	plain, err := enc.DecryptDetached(nonce, sealed, []byte("ad"))
	if err != nil || string(plain) != "payload" { t.Fatalf("decrypt = %q, %v", plain, err) }
	if _, err := enc.DecryptDetached(nonce[:len(nonce)-1], sealed, []byte("ad")); !errors.Is(err, encrypt.ErrMalformedCiphertext) { t.Fatalf("nonce error = %v", err) }
	if _, err := enc.DecryptDetached(nonce, sealed[:15], []byte("ad")); !errors.Is(err, encrypt.ErrMalformedCiphertext) { t.Fatalf("tag error = %v", err) }
	if _, err := enc.DecryptDetached(nonce, sealed, []byte("other")); !errors.Is(err, encrypt.ErrAuthenticationFailed) { t.Fatalf("AAD error = %v", err) }
}

func TestZeroValueEncryptorDetachedFailsSafely(t *testing.T) {
	var enc encrypt.Encryptor
	if _, _, err := enc.EncryptDetached(nil, nil); !errors.Is(err, encrypt.ErrInvalidKey) { t.Fatal(err) }
	if _, err := enc.DecryptDetached(make([]byte, 12), make([]byte, 16), nil); !errors.Is(err, encrypt.ErrInvalidKey) { t.Fatal(err) }
}
```

- [x] **Step 2: detached tests가 기존 코드에서 실패하는지 확인한다.**

Run: `go test -count=1 ./encrypt -run 'TestDetached|TestZeroValueEncryptorDetached'`

Expected: compile failure에 `EncryptDetached` 또는 `DecryptDetached` undefined가 포함된다. 이 실패는 새 public contract가 아직 없다는 RED 증거다.

- [x] **Step 3: 기존 AEAD를 재사용하는 최소 구현을 추가한다.**

`encrypt/encrypt.go`에 다음 흐름을 추가하고 기존 `Encrypt`/`Decrypt`는 이를 통해 동작하게 한다. 현재 `cipher.NewGCMWithRandomNonce`는 `NonceSize()==0`, `Overhead()==28`이므로 detached contract에 직접 사용할 수 없다. `New`는 `cipher.NewGCM`으로 12-byte nonce/16-byte tag AEAD를 만들고, `EncryptDetached`가 `crypto/rand.Reader`에서 새 nonce를 채운 뒤 `Seal(nil)`이 만든 새 ciphertext와 함께 반환한다. 두 slice는 입력과 분리된 caller-owned buffer이며 불필요한 전체 복사를 추가하지 않는다. `DecryptDetached`는 nonce/tag 길이를 먼저 검사한 뒤 `Open`하고 기존 authentication error를 wrapping한다. 기존 facade는 `header|nonce|ciphertext+tag` 배열을 유지해 이미 발행된 `BTENC` bytes도 복호화한다.

```go
func (e Encryptor) EncryptDetached(plaintext, associatedData []byte) ([]byte, []byte, error) {
	if e.aead == nil { return nil, nil, errorWith(ErrInvalidKey, "encrypt detached", nil) }
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return nil, nil, errorWith(ErrInvalidOptions, "encrypt detached", err) }
	ciphertext := e.aead.Seal(nil, nonce, plaintext, associatedData)
	return append([]byte(nil), nonce...), append([]byte(nil), ciphertext...), nil
}

func (e Encryptor) DecryptDetached(nonce, ciphertext, associatedData []byte) ([]byte, error) {
	if e.aead == nil { return nil, errorWith(ErrInvalidKey, "decrypt detached", nil) }
	if len(nonce) != e.aead.NonceSize() || len(ciphertext) < e.aead.Overhead() {
		return nil, errorWith(ErrMalformedCiphertext, "decrypt detached", nil)
	}
	plain, err := e.aead.Open(nil, nonce, ciphertext, associatedData)
	if err != nil { return nil, errorWith(ErrAuthenticationFailed, "decrypt detached", err) }
	return plain, nil
}
```

`Encrypt`는 `EncryptDetached` 결과를 `envelopeHeader|nonce|ciphertext`로 합치고, `Decrypt`는 header를 검증한 뒤 payload를 12-byte nonce와 나머지 ciphertext로 분리해 detached method를 호출한다. `parseEnvelope`의 최소 길이는 nonce 12 + tag 16으로 고정한다. 기존 BTENC 길이/bytes와 `EncryptString`/`DecryptString` 동작은 유지한다. 공개 Go doc comment는 한국어로 작성한다.

- [x] **Step 4: detached targeted GREEN과 기존 encrypt 회귀를 실행한다.**

Run: `gofmt -w encrypt/encrypt.go encrypt/encrypt_test.go && go test -count=1 ./encrypt`

Expected: detached tests와 기존 전체 `encrypt` tests가 PASS; wrong key/AAD/tamper는 `ErrAuthenticationFailed`, malformed input은 `ErrMalformedCiphertext`를 유지한다.

- [x] **Step 5: parent docs에 provider용 detached boundary를 명시한다.**

README 두 locale과 `doc.go`에 detached methods가 low-level provider composition surface이고 일반 caller는 기존 `Encrypt`/`Decrypt`를 계속 사용한다는 설명을 추가한다. 실행 범위 밖인 KMS client/config/lifecycle/rotation은 `encrypt/kms` README로 연결한다. `git diff --check`와 Korean terminology audit를 실행한다.

### Task 2: KMS dependency, error contract, envelope wire/parser를 TDD로 만든다

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `encrypt/kms/errors.go`, `encrypt/kms/envelope.go`, `encrypt/kms/provider_test.go`
- Create: `encrypt/kms/doc.go`

- [x] **Step 1: KMS package RED fixtures와 parser tests를 작성한다.**

`provider_test.go`에 fake 없는 순수 envelope table을 먼저 작성한다. tests는 canonical marshal bytes, sorted context, exact duplicate 및 top-level field case-variant/unknown/trailing/invalid-UTF-8/null rejection, canonical base64, top-level whitespace/field reorder/escaped-name/alternate-number mutation, unsupported version/algorithm, 6144-byte encrypted data key bound, nonce/tag/32 MiB plaintext-derived bound/64 MiB limits를 검증한다. `Envelope` values는 parse/marshal 후 slices/maps가 독립 복사됨을 확인한다. context byte limit은 모든 key/value의 UTF-8 byte 길이 합계로 계산하고, case-sensitive distinct key는 허용하며 여러 `WithEncryptionContext` option 사이 exact duplicate key는 deterministic error로 거부한다.

```go
func TestParseEnvelopeRejectsStrictInput(t *testing.T) {
	valid := mustEnvelopeBytes(t, Envelope{Version: EnvelopeVersion, Algorithm: AlgorithmAES256GCM,
		KeyID: "arn:aws:kms:ap-northeast-2:123456789012:key/demo",
		EncryptedDataKey: bytes.Repeat([]byte{1}, 32), EncryptionContext: map[string]string{"tenant":"blue"},
		Nonce: bytes.Repeat([]byte{2}, 12), Ciphertext: bytes.Repeat([]byte{3}, 16)})
	for _, tc := range []struct{name string; mutate func([]byte) []byte}{
		{"unknown", func(b []byte) []byte { return append(b[:len(b)-1], []byte(`,"extra":1}`)...)}},
		{"duplicate", func(b []byte) []byte { return bytes.Replace(b, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)}},
		{"case variant", func(b []byte) []byte { return bytes.Replace(b, []byte(`"version":1`), []byte(`"Version":1`), 1)}},
		{"trailing", func(b []byte) []byte { return append(append([]byte(nil), b...), 'x') }},
	} {
		t.Run(tc.name, func(t *testing.T) { if _, err := ParseEnvelope(tc.mutate(valid)); !errors.Is(err, ErrMalformedEnvelope) { t.Fatalf("error = %v", err) } })
	}
}
```

- [x] **Step 2: parser RED를 확인한다.**

Run: `go test -count=1 ./encrypt/kms -run 'TestParseEnvelope'`

Expected: package/files 또는 `ParseEnvelope` undefined compile failure. No AWS credentials/network are used.

- [x] **Step 3: `go.mod`에 선택된 SDK version만 추가한다.**

Run: `go get github.com/aws/aws-sdk-go-v2/service/kms@v1.42.1 && go mod tidy`

Expected: direct requirement `service/kms v1.42.1`가 추가되고 unrelated AWS root upgrade는 발생하지 않는다. `go list -m github.com/aws/aws-sdk-go-v2/service/kms`가 `v1.42.1`을 출력한다. Diff를 확인한 뒤 다음 단계로 이동한다.

- [x] **Step 4: safe error와 immutable envelope parser를 구현한다.**

`errors.go`는 다음 sentinel을 선언한다: `ErrNilClient`, `ErrInvalidKeyID`, `ErrInvalidProvider`, `ErrInvalidOptions`, `ErrInputTooLarge`, `ErrMalformedEnvelope`, `ErrUnsupportedVersion`, `ErrUnsupportedAlgorithm`, `ErrMetadataMismatch`, `ErrInvalidDataKey`, `ErrKMSOperation`, 그리고 `ErrAuthenticationFailed = encrypt.ErrAuthenticationFailed`. `*Error.Error()`는 sentinel과 고정 operation만 출력하고 `Unwrap`/`Is`로 cause와 sentinel을 보존한다. `doc.go`와 모든 exported type/field/constant/sentinel/method에는 한국어 Go doc comment를 작성해 lint/revive 계약을 고정한다.

`envelope.go`는 `BTKMS` prefix와 ordered wire struct를 사용한다. `MarshalBinary`는 version/algorithm/key/context/nonce/tag/blob 및 UTF-8/limit을 검증하고 context entry를 key 오름차순으로 정렬하며, 큰 base64 field는 streaming encoder로 bounded output buffer에 직접 쓴다. `ParseEnvelope`는 전체 길이와 base64 encoded length를 먼저 검사하고, canonical JSON byte scanner로 exact field/array order와 literal field names를 확인해 duplicate/case-variant/unknown/trailing/whitespace/invalid UTF-8/null을 거부한다. context entry object도 같은 strict scanner를 사용하며 sorted order를 요구한다. 문자열은 canonical JSON escape와 byte-for-byte 형태를 확인하고, base64는 padded alphabet/unused-bit canonicality를 destination buffer에 직접 decode한다. 이 lexical 검증으로 별도 전체-envelope re-marshal 없이 field reorder, escaped field name, alternate numeric form을 거부해 `RawMessage` 중복 할당을 피한다. `EncryptedDataKey` decoded bytes는 `MaxEncryptedDataKeySize=6144`를 넘으면 KMS 전 `ErrMalformedEnvelope`다. ciphertext에서 GCM tag 16 bytes를 제외한 plaintext 길이가 `MaxPlaintextSize`를 넘는 경우도 parse 단계에서 KMS 전 `ErrInputTooLarge`로 거부한다. Encryption context key는 case-sensitive exact 문자열 정책이므로 case만 다른 distinct key는 허용하고 exact duplicate만 거부한다.

canonical metadata/AAD는 다음 helpers로 고정한다.

```go
func canonicalMetadata(e Envelope) ([]byte, error) // version, algorithm, key_id, encrypted_data_key, sorted context only
func associatedData(metadata, callerAD []byte) ([]byte, error) {
	// "BTKMS-AAD\x01" | u32be(metadata length) | metadata | u32be(callerAD length) | callerAD
}
```

`nonce`와 `ciphertext`는 AEAD nonce/tag가 검증하므로 metadata JSON에 넣지 않는다. Caller AD는 `MaxAssociatedDataSize`를 초과하면 `ErrInputTooLarge`다. `go vet`/gofmt를 실행하고 parser tests를 GREEN으로 만든다.

- [x] **Step 5: envelope targeted GREEN과 strict parser mutation tests를 실행한다.**

Run: `gofmt -w encrypt/kms/*.go && go test -count=1 ./encrypt/kms -run 'TestParseEnvelope|TestMarshalEnvelope|TestCanonical'`

Expected: all pure wire/AAD tests PASS; malformed/unsupported input never calls a client (fake is not needed for this task).

### Task 3: caller-owned KMS provider encryption flow와 zeroing을 구현한다

**Files:**
- Create: `encrypt/kms/provider.go`
- Modify: `encrypt/kms/provider_test.go`

- [x] **Step 1: fake client와 Encrypt RED tests를 작성한다.**

Fake는 `sync.Mutex`로 logical `GenerateDataKey`/`Decrypt` calls, deep-copied inputs, optional output/error, blocking channel, and context observation을 기록한다. Tests는 constructor validation/option map copy, zero-value provider, successful Encrypt envelope, key spec/context/key ID assertions, KMS error wrapping, nil/wrong-length/empty blob output, preflight cancellation, post-KMS cancellation, output+error zeroing, input aliasing, and plaintext/AD bound with zero calls를 포함한다.

```go
func TestProviderEncryptUsesOneDataKeyCallAndZeroesSDKPlaintext(t *testing.T) {
	plainKey := bytes.Repeat([]byte{7}, 32)
	fake := newFakeClient(plainKey, []byte("encrypted-data-key"))
	p := mustProvider(t, fake, "arn:aws:kms:ap-northeast-2:123456789012:key/demo", map[string]string{"tenant":"blue"})
	out, err := p.Encrypt(context.Background(), []byte("payload"), []byte("record-v1"))
	if err != nil { t.Fatal(err) }
	if len(out) == 0 || fake.generateCalls() != 1 { t.Fatalf("envelope/calls = %d/%d", len(out), fake.generateCalls()) }
	fake.assertLastGeneratedInput(t, "arn:aws:kms:ap-northeast-2:123456789012:key/demo", map[string]string{"tenant":"blue"})
	if !bytes.Equal(fake.lastReturnedPlaintext(), make([]byte, len(plainKey))) { t.Fatal("KMS plaintext was not zeroed") }
}
```

- [x] **Step 2: Encrypt RED를 실행한다.**

Run: `go test -count=1 ./encrypt/kms -run 'TestProviderEncrypt'`

Expected: compile failure for `Client`, `New`, `Provider.Encrypt`, or fake support. This is the first provider behavior RED checkpoint.

- [x] **Step 3: immutable provider/config and Encrypt flow를 최소 구현한다.**

`Client`는 AWS SDK v2 exact subset을 선언하고 실제 SDK client 적합성을 compile-only assertion(`var _ Client = (*awskms.Client)(nil)`)으로 고정한다.

```go
type Client interface {
	GenerateDataKey(context.Context, *awskms.GenerateDataKeyInput, ...func(*awskms.Options)) (*awskms.GenerateDataKeyOutput, error)
	Decrypt(context.Context, *awskms.DecryptInput, ...func(*awskms.Options)) (*awskms.DecryptOutput, error)
}
```

`New`는 nil client와 typed-nil client(reflection으로 `Chan`, `Func`, `Interface`, `Map`, `Pointer`, `Slice` 모든 nil 가능 kind를 판별), blank/invalid UTF-8/oversized key ID, nil Option을 거부하고 key/context를 복사한다. `WithEncryptionContext`는 nil을 empty context로 취급하되 key/value UTF-8과 64-entry/8 KiB(sum of key/value UTF-8 bytes) bounds를 적용한다. 여러 option이 같은 key를 제공하면 `ErrInvalidOptions`로 거부하고 조용히 덮어쓰지 않는다. zero-value 또는 nil receiver는 `ErrInvalidProvider`다. typed-nil constructor 테스트는 panic과 KMS call이 모두 0인지 확인한다. KMS output의 `Plaintext`/`CiphertextBlob` buffer는 provider가 복사한 뒤 반환 즉시 best-effort zero하므로 caller-owned client는 반환 후 해당 slice를 공유·재사용·보관하지 않아야 하며 fake가 이 ownership 계약을 검사한다.

`Encrypt` 순서는 (1) nil context→background와 즉시 cancellation, (2) plaintext/AD bounds(boundary 성공, +1 실패), (3) `GenerateDataKey` 한 번 호출, (4) non-nil output 직후 `defer zeroBytes(output.Plaintext)`와 `defer zeroBytes(output.CiphertextBlob)`를 `err`/length 검증보다 먼저 예약, (5) `CiphertextBlob` empty 또는 `MaxEncryptedDataKeySize+1`이면 즉시 `ErrInvalidDataKey`로 반환하고 metadata에 사용할 blob은 fresh deep copy 직후 zero defer, (6) 별도 `localKey` copy 생성 직후 zero defer, (7) `ErrKMSOperation`/`ErrInvalidDataKey` 분기, (8) `encrypt.New` 내부 key copy도 생성 직후 zero, (9) exact canonical AAD + `EncryptDetached`, (10) envelope marshal/size, (11) 최종 context check와 cancellation 시 nil 결과다. transient KMS error fake는 retry 없이 `GenerateDataKey=1`을 검증한다. KMS output `KeyId`는 alias resolution 차이 때문에 비교하지 않고 configured keyID를 metadata에 유지한다. Provider는 retry/close/reconfigure/logger를 하지 않는다.

- [x] **Step 4: Encrypt GREEN과 failure/cancellation/zeroing tests를 실행한다.**

Run: `gofmt -w encrypt/kms/provider.go encrypt/kms/provider_test.go && go test -count=1 ./encrypt/kms -run 'TestProviderEncrypt|TestProviderConstructor|TestZero|TestCancellation'`

Expected: success, KMS failures, wrong output lengths, `(output, err)`, pre/post cancellation, max plaintext/AD and call-count assertions PASS; fake returned plaintext is all zero on every output path.

### Task 4: provider Decrypt flow, metadata binding, strict redaction을 완성한다

**Files:**
- Modify: `encrypt/kms/provider.go`, `encrypt/kms/envelope.go`
- Modify: `encrypt/kms/provider_test.go`

- [x] **Step 1: Decrypt RED tests를 작성한다.**

Round-trip envelope로 success를 고정하고, provider key/context mismatch가 KMS calls 0인지, ciphertext-derived plaintext `MaxPlaintextSize+1`과 oversized caller associated data가 KMS calls 0인지, encrypted data key/nonce/ciphertext/metadata/AAD tamper가 authentication error인지, KMS decrypt error/nil/wrong plaintext output/empty blob이 safe sentinel인지, pre-canceled malformed input이 context error 우선인지 검증한다. alias/ARN 표현이 달라진 envelope는 KMS 전에 `ErrMetadataMismatch`인지 확인하고, encrypted data key tamper/KMS `InvalidCiphertext`는 `ErrKMSOperation` 또는 `ErrInvalidDataKey`로 구분한다. KMS fake가 반환한 plaintext key의 zeroing과 local plaintext의 final-cancel zeroing을 검사한다. `err.Error()` 및 `fmt.Sprintf("%+v", err)`가 key ID/context/plaintext/ciphertext/nonce/AWS error text를 포함하지 않는지 확인한다.

- [x] **Step 2: Decrypt RED를 실행한다.**

Run: `go test -count=1 ./encrypt/kms -run 'TestProviderDecrypt|TestMetadata|TestRedaction'`

Expected: missing `Provider.Decrypt` or incomplete metadata/AAD behavior failures.

- [x] **Step 3: Decrypt flow를 구현한다.**

`Decrypt`는 즉시 ctx check → caller associated data byte length preflight(`MaxAssociatedDataSize` 초과는 parse/KMS 전에 `ErrInputTooLarge`) → strict ParseEnvelope → ciphertext-derived plaintext bound 확인 → exact key/context match → parse/metadata/KMS 직전 ctx checks → `Decrypt` one call → request `CiphertextBlob` fresh copy 직후 zero defer → output non-nil 즉시 zero defer → output/error validation → post-KMS ctx check → 별도 `localKey` copy와 즉시 zero defer → `encrypt.New` 내부 key copy zero → same canonical metadata/AAD와 nonce를 이용한 `DecryptDetached` → final ctx check에서 cancellation이면 local plaintext zero 후 nil 반환 순서를 지킨다. KMS input context는 매 호출 deep copy한다. Metadata mismatch/parse/size failure는 KMS 0회다. KMS operation errors는 `errorWith(ErrKMSOperation, "decrypt", err)`로 감싸고 AWS cause는 `Unwrap`에서만 접근한다.

- [x] **Step 4: Decrypt GREEN과 mutation/redaction 테스트를 실행한다.**

Run: `gofmt -w encrypt/kms/provider.go encrypt/kms/envelope.go encrypt/kms/provider_test.go && go test -count=1 ./encrypt/kms`

Expected: entire package tests PASS; tamper maps to `encrypt.ErrAuthenticationFailed`; metadata mismatch and oversized blob happen before KMS; no sensitive fixture appears in error formatting.

### Task 5: concurrency, benchmarks, example, and public README parity를 추가한다

**Files:**
- Create: `encrypt/kms/benchmark_test.go`, `encrypt/kms/example_test.go`, `encrypt/kms/README.md`, `encrypt/kms/README.ko.md`
- Modify: `encrypt/kms/provider_test.go`, `README.md`, `README.ko.md`

- [x] **Step 1: shared provider stress/race RED를 보강한다.**

8 workers × 64 rounds fake test에서 immutable provider를 공유해 exact Generate/Decrypt logical call count를 각각 독립 검증하고 no goroutine leak/bounded timeout를 검증한다. Fake는 state mutex를 block 구간에서 놓고, 매 호출 fresh output/deep-copy와 context observation을 제공한다. Fake는 concurrent-safe/context-aware contract를 명시적으로 구현하고, provider guarantee가 that contract를 조건으로 한다는 문서 assertion을 추가한다. Thread-unsafe client를 provider가 serialize하거나 종료한다고 주장하지 않는다.

- [x] **Step 2: benchmark harness를 추가한다.**

`BenchmarkDetached`, `BenchmarkEnvelopeMarshalParse`, `BenchmarkProviderEncrypt`, `BenchmarkProviderDecrypt`, `BenchmarkProviderRoundTrip`, `BenchmarkProviderRoundTripParallel`를 1 KiB, 1 MiB, `MaxPlaintextSize` fixture에 대해 작성한다. setup, fake output, fixture, result buffer는 timer 밖에 두고 timed loop에는 대상 연산만 둔다. 최대 fixture는 serial benchmark에서 `-benchtime=1x`를 사용하고, 작은 fixture의 parallel benchmark는 별도 time-based 실행으로 실제 겹침을 측정한다. 모든 benchmark는 `b.ReportAllocs()`와 pre-timer correctness를 유지한다. Parallel benchmark는 `b.RunParallel`, isolated buffers/results, start gate를 사용한다. 각 sub-benchmark counter를 reset하고 serial Encrypt/Decrypt는 `GenerateDataKey=1`/`Decrypt=1`, RoundTrip은 각각 `1/1`을 독립 검증한다. parallel variant는 기대값을 `b.N`으로 검증한다. Provider benchmark fake는 fresh output을 매 호출 반환하고 network RTT/retry attempt를 측정하지 않는다.

Run serial bounded fixtures: `go test -timeout=10m -run '^$' -bench 'Benchmark(Detached|Envelope|Provider)(/1KiB|/1MiB|/MaxPlaintextSize)$' -benchmem -benchtime=1x -count=3 -cpu=1,2,4 ./encrypt/kms`.

Run parallel small fixtures separately: `go test -timeout=10m -run '^$' -bench 'BenchmarkProviderRoundTripParallel/(1KiB|1MiB)$' -benchmem -benchtime=200ms -count=3 -cpu=1,2,4 ./encrypt/kms`.

Expected: command compiles/runs without AWS credentials, emits `ns/op`, `B/op`, `allocs/op` for each size, and records raw output in the lesson/DoD with commit SHA, dirty-tree state, Go/OS/CPU/GOMAXPROCS, fixture identity, metric direction, and `no_regression=N/A` when no prior baseline exists. No fixed machine-dependent threshold is claimed; regression guard is bounded input plus per-method logical call counts.

- [x] **Step 3: example과 package README 두 locale를 작성한다.**

README usage는 `kms.New(callerOwnedClient, keyARN, kms.WithEncryptionContext(...))`, context cancellation/deadline, `errors.Is`, `Max*` bounds, alias-retarget caveat, caller-owned credential/client/retry/rotation/lifecycle/IAM/logging/instrumentation을 설명한다. instrumentation은 operation별 latency/error/cancellation과 provider logical call을 기록하되 SDK retry/network attempt와 구분하고 key ID/context/plaintext/envelope를 metric label이나 로그에 넣지 않는 caller 지침을 포함한다. `BTENC`와 `BTKMS`는 wire 호환이 아니며 자동 migration이 없다. reader-first/writer-later rollout, rollback 시 신규 BTKMS 쓰기 중단, 기존 reader와 key 보존, caller-owned dual-read/dual-write·re-encryption 절차를 명시한다. 동일한 key ID 문자열 표현(alias/ARN)을 암복호화에 재사용해야 하며 표현이 다르면 `ErrMetadataMismatch`가 된다. key ID/context는 envelope와 AWS audit log에 노출될 수 있고 secret/PII를 넣지 않으며 raw envelope/associated data를 로그에 남기지 않는다는 경고를 둔다. 최소 IAM permission은 대상 key에 대한 `kms:GenerateDataKey`와 `kms:Decrypt`이며 policy/region/account 조건은 caller provisioning 범위다. associated data는 envelope에 저장되지 않으므로 caller가 복호화 때 동일하게 재공급해야 한다. Live AWS 호출은 예제·test·CI에서 하지 않는다. English/Korean headings/claims/commands/links를 동일하게 유지한다.

- [x] **Step 4: root README package index와 documentation 목록을 동기화한다.**

`encrypt/kms` row와 AWS/암호화 documentation bullet을 English/Korean root README에 같은 위치와 의미로 추가한다. `encrypt` README의 KMS boundary는 새 package 링크를 가리킨다. Run `git diff --check`와 `node /Users/debop/.codex/skills/bluetape-writer/scripts/audit-korean-terms.mjs --json <all changed Korean docs>`; findings=0이어야 한다.

### Task 6: bluetape-go-patterns를 RED evidence로 개선하고 managed/live parity를 검증한다

**Files:**
- Modify managed source: `/Users/debop/.local/share/chezmoi/private_dot_codex/private_skills/bluetape-go-patterns/SKILL.md`
- Apply live target: `/Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`
- Evidence: `.omx/skill-green-evidence.json`, `.omx/skill-refactor-evidence.json` (ignored runtime files)

- [x] **Step 1: RED pressure 결과와 현재 skill source를 대조한다.**

초기 RED에서 관측한 KMS client lifecycle/SDK coupling, cancellation, DEK zeroing/expanded-key 한계, safe error/redaction, canonical metadata/fake deep-copy, README parity 누락을 current managed skill의 기존 guidance와 대조한다. 중복 문구 대신 한 번 재사용 가능한 hardening rule로 합친다.

- [x] **Step 2: managed source에 최소 규칙을 추가한다.**

영문 LLM-facing skill 문체로 다음 규칙을 추가한다: caller-owned AWS/KMS client/config/credential/lifecycle/retry/cache/IAM boundary와 minimal SDK subset; non-nil KMS plaintext output 즉시 zero defer(성공/실패/cancel/panic, local copies 포함) 및 best-effort 한계; canonical version/algorithm/key/context/encrypted-DEK AAD와 strict duplicate/unknown/invalid-UTF8 parsing; JSON decode 전 raw string bound와 legacy wire fixture compatibility; pre/post/final cancellation checkpoints와 no-goroutine-leak; safe operation errors/no internal logger; mutex/deep-copy/context-aware fake, metadata mismatch pre-KMS, race/redaction/benchmark proof; package README locale parity. Add KMS/envelope trigger to hardening-lessons routing without broad unrelated rules.

- [x] **Step 3: `chezmoi apply`와 source/live parity를 검증한다.**

Run `chezmoi source-path /Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`, `chezmoi diff -- /Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`, `chezmoi apply /Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`; then compare SHA-256 of managed source and live target. `private_dot_claude` 및 public `bluetape-skills`는 변경하지 않는다.

- [x] **Step 4: GREEN pressure test를 fresh native lane에서 실행한다.**

Fresh `analyst`/`verifier` lane에게 combined issue-519 pressure scenario를 주고 updated live `bluetape-go-patterns`를 읽게 한다. Agent가 선택한 client ownership, zeroing order, AAD strictness, cancellation, fake observability, docs/benchmark gates를 RED baseline과 비교해 `.omx/skill-green-evidence.json`에 기록한다. No code edits.

- [ ] **Step 5: REFACTOR self-review와 self-audit를 실행한다.**

규칙 중복/모호한 “secure/safe/handle errors” 표현/placeholder/과도한 library-specific claim을 제거하고 description이 trigger-only인지 확인한다. `$self-audit` skill을 fresh read한 뒤 live/managed skill hashes, `git diff --check`, `chezmoi diff`, ownership audit를 실행한다. Managed chezmoi repository commit/push는 source/live 검증 후 별도 commit gate로 기록하며 public bundle promotion은 N/A로 남긴다.

### Task 7: lessons, Step 6-R, full verification, and local completion evidence

**Files:**
- Create: `docs/lessons/2026-09-02-issue-519-kms-envelope.md`
- Modify: no production files after Step 6-R except fixes explicitly mapped to findings.

- [ ] **Step 1: Korean lesson artifact를 작성하고 SPW-01..05를 완료한다.**

Lesson은 context/decision/outcome/verification을 요약하고, RED shortcut(“KMS wrap이면 충분”), security/stability P1(즉시 zero defer, exact AAD, cancellation precedence, client contract), performance P1(blob bound), JSON decode 전 raw string bound, public error redaction, legacy BTENC fixture, strict parser/limit correction, BTENC↔BTKMS rollout/rollback과 deferred CHANGELOG/WIP release bookkeeping을 failed assumption→evidence→future guard 형식으로 기록한다. live AWS/PR/merge/public bundle은 범위 밖이라는 N/A 근거를 명시한다. `bluetape-writer` Korean naturalness checklist와 terminology audit 후 `git diff --check`를 실행한다.

- [ ] **Step 2: 구현된 exact head에서 7-Tier Step 6-R를 수행한다.**

performance, stability, security, operator/Ops, developer/API, user/caller 여섯 독립 lane을 최대 3개씩 bounded wave로 dispatch하고 main session이 통합한다. 모든 lane은 exact local head/SHA와 changed-path scope를 고정한다. P0/P1이면 해당 파일을 수정하고 영향받은 테스트/렌즈를 재실행한다. Solo-developer 범위의 independent human review는 N/A로 기록하되 security, exact-head, CI, external-mutation, credential gates는 유지한다.

- [ ] **Step 3: targeted → standard → full verification을 순서대로 실행한다.**

Run in order:

```bash
gofmt -w encrypt/encrypt.go encrypt/encrypt_test.go encrypt/kms/*.go
go test -count=1 ./encrypt ./encrypt/kms
go test -race -count=1 ./encrypt ./encrypt/kms
go test -timeout=10m -p 1 -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

Expected: each command exits 0; full Go suite uses `-p 1 -count=1 -timeout=10m` because Testcontainers resources are shared. `make race`/`make ci` output is read to terminal completion, skipped Docker surfaces are reported as N/A only with concrete scope evidence, and no failed/old-SHA result is treated as pass. Record first failure and independent rerun separately; a retry-only PASS cannot erase the first failure. Re-run `go mod tidy` only if `tidy-check` identifies drift, then verify clean second run.

- [ ] **Step 4: final diff, dependency, docs, skill, and worktree audit를 수행한다.**

Run `git diff --check`, `git status --short`, `git diff --stat`, `git diff -- go.mod go.sum`, `go list -m all | rg 'aws-sdk-go-v2/service/kms'`, README locale parity checks, Korean terminology audit, managed/live skill SHA comparison, and `git log -1 --format=%H`. Confirm no live AWS request, credential file, unrelated file, or public skill bundle change exists.

- [ ] **Step 5: Lore commit으로 수렴된 scoped branch를 기록한다.**

Commit intent and trailers must be Korean and follow repository protocol:

```text
Issue #519 KMS envelope 경계를 caller-owned provider로 구현한다.

Constraint: AWS KMS data key와 local encrypt 조합, fake-only CI, PR·merge·publish 제외 범위를 지킨다.
Rejected: provider 내부 client lifecycle/retry/cache와 direct large-payload KMS | caller 정책과 보안 경계를 침범한다.
Confidence: high
Scope-risk: moderate
Directive: 후속 modifier는 BTKMS v1, exact AAD, input bounds, Client concurrency/context 계약을 유지해야 한다.
Tested: targeted, race, full go test, make fmt/tidy/vet/lint/test/race/ci, docs/skill parity.
Not-tested: live AWS credential/network, PR/merge/publish.
```

- [ ] **Step 6: workflow completion-check와 DoD report를 작성한다.**

Machine receipt에서 all lanes terminal, required checks, component evidence, main verification을 확인한다. Final report는 plan item status, changed outputs, exact head SHA, command evidence/counts, N/A/blocked items (`Required checks: X/Y; N/A: N; Blocked: N`), known risks, user actions(없으면 명시), next steps(없으면 명시), final `DONE`/`PENDING`/`BLOCKED`를 Korean으로 정리한다. PR/merge/publish는 명시적 후속 gate로 남긴다.
