# Go-native Apache Fory Rediscoord Codecs 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 opt-in Go-native Apache Fory fast 및 compatible codecs for the inner `cache/rediscoord` result payload while preserving the 기존 coordination protocol.

**아키텍처:** 추가 an isolated `cache/rediscoord/fory` 패키지 that owns Fory v1.3.0 configuration, a `BTFY` profile envelope, one mutex-guarded registered runtime, bounded decode/encode, 및 sanitized typed 오류. 추가 an additive `MaxResultBytes` guard to `rediscoord` so the 기존 JSON/base64 outer envelope can be bounded 전에 JSON allocation; keep JSON as the default 및 do 아님 add a direct Redis value cache in this issue.

**기술 스택:** Go 1.26, Apache Fory Go `v1.3.0`, `encoding/binary`, `sync.Mutex`, `go-redis/v9`, Go 테스트/race, 기존 Testcontainers Redis fixtures.

---

### 작업 1: Pin Apache Fory 및 establish 패키지 boundaries

**파일:**
- Modify: `go.mod`
- Modify: `go.sum`
- 생성: `cache/rediscoord/fory/doc.go`

- [ ] **단계 1: 추가 the dependency 및 패키지 documentation.**

추가 `github.com/apache/fory/go/fory v1.3.0` as a direct dependency. 문서화 that
the 패키지 is Go-native, `TRUSTED_INTERNAL`, 아님 encryption, 및 separate from
xlang interoperability 및 direct Redis value storage.

- [ ] **단계 2: 실행 the 패키지 compile gate.**

실행:

```bash
go test ./cache/rediscoord/fory
```

예상: the 패키지 compiles; it may report 없음 테스트 files until 작업 2 adds
the first failing 테스트.

- [ ] **단계 3: 커밋 the dependency boundary.**

```bash
git add go.mod go.sum cache/rediscoord/fory/doc.go
git commit -m "chore: pin Apache Fory for Redis codecs"
```

### 작업 2: 정의 typed 오류 및 constructor validation through failing 테스트

**파일:**
- 생성: `cache/rediscoord/fory/codec_test.go`
- 생성: `cache/rediscoord/fory/errors.go`
- 생성: `cache/rediscoord/fory/codec.go`

- [ ] **단계 1: Write failing constructor 및 zero-value 테스트.**

Cover:

```go
func TestNewNativeFastRejectsInvalidOptions(t *testing.T)
func TestNewNativeFastRejectsUnsupportedRootShapes(t *testing.T)
func TestZeroCodecReturnsUninitializedError(t *testing.T)
func TestCodecErrorRedactsPayloadCauseAndRegistrationText(t *testing.T)
```

사용 a registered struct fixture, pointer/interface/function roots, zero 및
negative limits, missing registration, 및 a registration callback returning an
오류 containing a secret marker. 검증 `errors.As` exposes `CodecError`, the
reason is stable, 및 `Error()` excludes payload, registration, Fory-원인, key,
및 owner-token markers.

- [ ] **단계 2: 실행 the 테스트 및 verify the expected RED state.**

실행:

```bash
go test ./cache/rediscoord/fory -run 'Test(NewNativeFastRejects|ZeroCodec|CodecError)'
```

예상: FAIL because the 패키지 API 및 오류 implementation do 아님 exist.

- [ ] **단계 3: 구현 the typed 오류 및 immutable constructor skeleton.**

정의 exported `CodecError` 함께 `Operation()`, `Profile()`, `Reason()`,
`Error()`, 및 safe `Unwrap()`. 정의 `Options`, profile constants, validation
for 모든 six positive limits, root shape validation, 및 constructor-만 codec
state. A zero-value codec must return an `uninitialized` 오류.

- [ ] **단계 4: 실행 the focused 테스트 및 verify GREEN.**

```bash
go test ./cache/rediscoord/fory -run 'Test(NewNativeFastRejects|ZeroCodec|CodecError)'
```

예상: PASS 함께 없음 raw 원인 또는 marker leakage.

### 작업 3: 구현 native Fory runtime 및 `BTFY` envelope

**파일:**
- Modify: `cache/rediscoord/fory/codec.go`
- Modify: `cache/rediscoord/fory/codec_test.go`
- 생성: `cache/rediscoord/fory/envelope.go`
- 생성: `cache/rediscoord/fory/envelope_test.go`

- [ ] **단계 1: Write failing round-trip 및 envelope 테스트.**

Cover native-fast 및 native-compatible profiles 함께 a registered struct,
primitive scalar, string, 및 `[]byte`. 검증 the `BTFY` magic, version,
profile, length, exact round trip, 및 compatible added-field behavior. 추가
wrong-profile/version/magic, raw Fory, JSON, truncation, trailing bytes,
length mismatch, 및 oversize 테스트.

- [ ] **단계 2: 실행 the 테스트 및 verify RED.**

```bash
go test ./cache/rediscoord/fory -run 'Test(Native|Envelope|RoundTrip|Rejects)'
```

예상: FAIL on missing constructors, envelope, 및 codec methods.

- [ ] **단계 3: 구현 the mutex-guarded Fory runtime.**

Construct `fory.New` 함께 explicit `WithXlang(false)`, profile-specific
`WithCompatible`, `WithTrackRef(false)`, `WithMaxDepth`, 및 모든 explicit type
metadata limits. Apply registration once 전에 returning. `Marshal` locks,
serializes, rejects payloads above `MaxPayloadBytes`, copies Fory bytes, 및
unlocks 전에 building one `BTFY` wrapper allocation. `Unmarshal` validates
the wrapper 및 max size without copying its payload, locks 만 around Fory
decode, 및 returns a sanitized `CodecError` on every failure.

- [ ] **단계 4: 실행 focused 테스트 및 verify GREEN.**

```bash
go test ./cache/rediscoord/fory -run 'Test(Native|Envelope|RoundTrip|Rejects)'
```

예상: PASS for both profiles, value shapes, limits, 및 fail-closed
envelope behavior.

- [ ] **단계 5: 커밋 the codec implementation.**

```bash
git add cache/rediscoord/fory
git commit -m "feat: add native Fory rediscoord codecs"
```

### 작업 4: 추가 concurrency 및 compile-checked example

**파일:**
- Modify: `cache/rediscoord/fory/codec_test.go`
- 생성: `cache/rediscoord/fory/example_test.go`

- [ ] **단계 1: Write failing concurrency 및 example.**

추가 a 높음-contention 테스트 that concurrently marshals 및 unmarshals distinct
registered values through one codec 및 asserts every result. 추가
`ExampleNewNativeFast` 및 `ExampleNewNativeCompatible` 함께 deterministic
registration 및 output-free compile-checked usage.

- [ ] **단계 2: 실행 the race gate.**

```bash
go test -race ./cache/rediscoord/fory
```

예상: initially RED 전에 the runtime is complete; 후 implementation,
PASS 함께 없음 data races 또는 buffer reuse corruption.

- [ ] **단계 3: 검증 normal 패키지 테스트.**

```bash
go test ./cache/rediscoord/fory
```

예상: PASS, including example.

### 작업 5: Bound the outer `rediscoord` JSON envelope

**파일:**
- Modify: `cache/rediscoord/options.go`
- Modify: `cache/rediscoord/stampede_cache.go`
- Modify: `cache/rediscoord/stampede_cache_test.go`
- Modify: `cache/rediscoord/operation_error_test.go`

- [ ] **단계 1: Write failing outer-limit 테스트.**

추가 테스트 proving `Options.MaxResultBytes` accepts zero as unlimited, rejects
negative values, rejects oversized result bytes 전에 Redis publication, 및
rejects oversized Redis result bytes 전에 `json.Unmarshal`. 검증 기존
operation/context 오류 behavior remains intact.

- [ ] **단계 2: 실행 the focused 테스트 및 verify RED.**

```bash
go test ./cache/rediscoord -run 'Test.*ResultBytes|Test.*OperationError'
```

예상: FAIL because `MaxResultBytes` does 아님 exist.

- [ ] **단계 3: 구현 the additive guard.**

Normalize `MaxResultBytes` as zero-또는-positive. Check encoded result length in
`storeResult` 전에 `client.Set`, 및 check raw Redis bytes in
`readOwnerResult` 전에 `decodeResult`. Return a sanitized typed operation
오류 using 기존 `btredis.OpError`; preserve zero-default behavior.

- [ ] **단계 4: 실행 focused 및 full 패키지 테스트.**

```bash
go test ./cache/rediscoord -run 'Test.*ResultBytes|Test.*OperationError'
go test -p 1 ./cache/rediscoord
```

예상: PASS 함께 기존 lock, TTL, context, redaction, 및 coordination
테스트 unchanged in behavior.

- [ ] **단계 5: 커밋 the outer guard.**

```bash
git add cache/rediscoord/options.go cache/rediscoord/stampede_cache.go cache/rediscoord/stampede_cache_test.go cache/rediscoord/operation_error_test.go
git commit -m "feat: bound rediscoord result envelopes"
```

### 작업 6: Documentation 및 rollout runbook

**파일:**
- Modify: `cache/rediscoord/README.md`
- Modify: `cache/rediscoord/README.ko.md`
- Modify: `cache/rediscoord/doc.go`

- [ ] **단계 1: 추가 영문 및 한국어 usage documentation.**

문서화 both constructors, explicit native mode, supported root shapes,
registration-전에-concurrency, the six resource limits 및 starting values,
`CodecError` reason labels, `MaxResultBytes`, `TRUSTED_INTERNAL`, 및 the fact
that Redis can observe bytes because Fory is 아님 encryption.

- [ ] **단계 2: 추가 the rollout/rollback runbook.**

Show a namespace format containing profile 및 schema generation. State that
모든 processes sharing a namespace must use one codec/profile/registration set;
mixed JSON/Fory 및 fast/compatible deployments are unsupported. 문서화
drain time `LockTTL + ResultTTL + safety margin`, rollback to the prior
codec/namespace pair, bounded `SCAN MATCH` cleanup, 및 없음 `KEYS`.

- [ ] **단계 3: 검증 documentation example against example.**

```bash
go test ./cache/rediscoord/fory -run '^Example'
git diff --check
```

예상: PASS; README code matches compile-checked example 및 contains 없음
unsupported fallback 또는 interoperability claim.

### 작업 7: Final verification 및 review preparation

**파일:**
- No new files; review 모든 changed files 및 issue-linked docs.

- [ ] **단계 1: 실행 focused unit 및 race checks.**

```bash
go test -p 1 ./cache/rediscoord/fory
go test -p 1 -race ./cache/rediscoord/fory
go test -p 1 ./cache/rediscoord
go test -p 1 -race ./cache/rediscoord
git diff --check
```

- [ ] **단계 2: 실행 repository gates.**

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
```

실행 Testcontainers-backed 패키지 checks serially 및 use the repository's
standard `make ci` gate 전에 PR creation. 다음을 하지 않는다: claim benchmark improvement;
#599 owns benchmark table, Chart, 및 analysis.

- [ ] **단계 3: 리뷰 the final diff against #597 및 the spec.**

Confirm 없음 xlang/default codec/persistence migration slipped into the diff,
모든 공개 APIs have Go doc comments, both README files are synchronized, 및
모든 P0/P1 review findings are zero 전에 opening a PR.
