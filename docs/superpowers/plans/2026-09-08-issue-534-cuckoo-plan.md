# #534 Cuckoo 구현 계획

> 실행 지침: `executing-plans`로 이 세션에서 순차 구현한다. 사용자 요청에 따라 구현을 subagent에 위임하지 않는다. 독립 subagent는 읽기 전용 검토만 담당한다.

**목표:** 승인된 Cuckoo spec을 기존 `probabilistic/redis`에 구현하고 이슈별 PR을 생성한다.

**구조:** caller-owned `Do` client, 기존 namespace key builder, 안전한 `redis.OpError`를 재사용한다. Cuckoo 구현체는 비공개이며 단일 명령의 결과를 엄격히 해석한다.

**기술:** Go 1.26, 기존 go-redis v9.20.0, testify, 기존 Redis Testcontainers helper. 새 Go 의존성 없음.

기준 spec: `docs/superpowers/specs/2026-09-08-issue-534-cuckoo-design.md`, commit `1218112`.
사용자 written-spec 승인: 2026-09-08 현재 대화의 `승인`.
대상: `bluetape4k/bluetape-go`, head `feat/issue-534-cuckoo`, base `develop`.
PR 생성은 승인 범위이며 머지·배포·삭제는 제외한다.

## 파일과 책임

| 파일 | 책임 |
|---|---|
| `probabilistic/redis/cuckoo.go` | 공개 인터페이스·옵션·생성자·명령 전송과 결과 해석 |
| `probabilistic/redis/cuckoo_test.go` | fake client, 요청·입력·RESP·오류·취소 단위 회귀 |
| `probabilistic/redis/cuckoo_integration_test.go` | plain unsupported, opt-in Redis 8 양성·포화·동시성·응답 유실 |
| `probabilistic/redis/cuckoo_example_test.go` | 네트워크 없는 compile-checked caller 예제 |
| `probabilistic/redis/README.md`, `README.ko.md` | 동일한 API·제한·실행 명령 |
| `docs/review/2026-09-08-issue-534-plan-review.md` | 계획 검토 및 수용 기준 대응 |
| `docs/review/2026-09-08-issue-534-code-review.md` | 최종 변경 검토 및 실제 검증 결과 |
| `docs/lessons/2026-09-08-issue-534-cuckoo.md` | RESP·retry·review 판단 오류의 재발 방지 |

main만 위 파일을 편집한다. 기존 Bloom/HLL·module·CI 설정은 변경하지 않는다.
새 module·JVM 등록·Spring·Kover·diagram·성능 benchmark는 해당 변경이나 주장이 없어 N/A다.
Docker는 main에서 한 suite씩 실행한다. 검토 lane에는 테스트·네트워크·쓰기 권한이 없다.

## 작업 1: 생성자와 전송 전 검증 (중간 복잡도)

- [ ] `cuckoo_test.go`에 아래 첫 RED를 추가한다. 같은 표에서 nil/typed-nil client,
  빈 namespace, 129 byte, 공백·brace·민감 marker를 검증하고 호출 수가 0인지 확인한다.

```go
func TestCuckooConstructor(t *testing.T) {
    _, err := NewCuckoo(CuckooOptions{Namespace: "tenant"})
    require.ErrorIs(t, err, ErrInvalidOptions)
}
```

- [ ] `go test -p 1 -count=1 ./probabilistic/redis -run '^TestCuckooConstructor$'`를 실행해
  `undefined: NewCuckoo`를 확인한다. 무관한 compile 오류면 RED로 계산하지 않는다.
- [ ] `cuckoo.go`에 아래 공개 계약을 추가하고 생성자에서 reflect의 nil 가능한 모든
  kind를 검사한다. `keyBuilderForNamespace`와 `structuralKeyValue`로 key를 생성하며
  validation 오류는 고정 문자열 `ErrInvalidOptions`만 반환한다. 내부 validator의 입력
  포함 오류 문자열은 외부에 그대로 노출하지 않는다.

```go
type CuckooClient interface { Do(context.Context, ...any) *redis.Cmd }
type CuckooOptions struct { Client CuckooClient; Namespace string }
type CuckooReserveOptions struct {
    Capacity int64
    BucketSize uint8
    MaxIterations uint16
    Expansion uint16
}
type Cuckoo interface {
    Reserve(context.Context, CuckooReserveOptions) error
    Add(context.Context, string) error
    Exists(context.Context, string) (bool, error)
    Count(context.Context, string) (int64, error)
    Delete(context.Context, string) (bool, error)
}
```

- [ ] item 65536 byte 허용/65537 거부, Capacity 4와 2^30 경계, bucket 0→2,
  iterations 0→20, capacity <2*bucket, expansion 32768/32769를 표로 검증한다.
  nil context는 ErrInvalidOptions, pre-cancel은 context 원인만 보존하고 전송하지 않는다.
- [ ] 공개 선언 직후 `gofmt -w probabilistic/redis/cuckoo*.go`와
  `golangci-lint run ./probabilistic/redis`를 실행한다. Go doc은 식별자 다음 ASCII 공백과 한국어를 사용한다.
- [ ] constructor/input 회귀 GREEN 후 생성자·테스트를 함께 Korean Lore commit한다.

## 작업 2: 명령과 오류 경계 (높은 복잡도, 작업 1 이후)

- [ ] mutex로 요청 slice를 복사하는 fake Do를 추가한다. 응답은 `redis.NewCmd(ctx)`에
  `SetVal`/`SetErr`로 설정하고 nil command·output-plus-error·후행 cancel도 주입한다.
  fake는 전체 인자를 복사하며 item은 immutable string을 그대로 보존한다.

```go
type cuckooFake struct {
    mu sync.Mutex
    calls [][]any
    result any
    err error
    after func()
    nilCommand bool
}
func (f *cuckooFake) Do(ctx context.Context, args ...any) *redis.Cmd {
    f.mu.Lock()
    f.calls = append(f.calls, append([]any(nil), args...))
    f.mu.Unlock()
    if f.after != nil { f.after() }
    if f.nilCommand { return nil }
    cmd := redis.NewCmd(ctx, args...)
    cmd.SetVal(f.result)
    cmd.SetErr(f.err)
    return cmd
}
```

- [ ] 성공 응답과 argv를 표로 먼저 고정하고 아직 실패하는 결과를 확인한다.

| 메서드 | argv (key 이후) | 허용 응답 |
|---|---|---|
| Reserve | capacity BUCKETSIZE bucket MAXITERATIONS iterations EXPANSION expansion | string OK |
| Add | NOCREATE ITEMS item | `[]any{int64(1)}` 또는 `[]any{true}` |
| Exists | item | int64 0/1 또는 bool |
| Count | item | 비음수 int64 |
| Delete | item | int64 0/1 또는 bool |

- [ ] 모든 메서드는 같은 전송 함수에서 pre-check → Do → Result → ctx.Err 검사 순서를
  공유한다. nil command와 잘못된 응답은 ErrCuckooReply, Add의 단일 -1은 ErrCuckooFull이다.
  응답 오류가 있으면 결과를 반환하지 않고, 오류 경로에서 bool/count는 항상 zero다.
- [ ] 오류 판정 순서를 아래대로 구현한다. error-plus-output와 후행 취소에서는
  unsupported 예외를 적용하지 않는다. 모든 원인은 errors.Join으로 보존한다.

```go
// 응답 수신 직후 사용할 분류 순서다.
late := ctx.Err()
cause := errors.Join(providerErr, late)
// unsupported는 순수 redis.Error이면서 해당 CF 명령의 unknown command일 때만 성립한다.
// bool 결과나 output-plus-error, timeout을 capability 미지원으로 숨기지 않는다.
if mutation && cause != nil && !definiteUnsupported {
    cause = errors.Join(cause, btredis.ErrCommitUnknown)
}
```

- [ ] typed `redis.Error`의 unknown command, 일반 error의 같은 문자열, ACL, WRONGTYPE,
  EOF, nil command, output-plus-error, canceled/deadline, nested 오류를 표로 검증한다.
  unsupported 문구가 포함된 일반 transport 오류는 unsupported로 매핑하지 않는다.
  다섯 메서드 각각 pre-cancel(0회), Do 중 cancel, 유효 응답 직후 cancel(1회)을 검증한다.
  Reserve/Add/Delete는 전송 후 취소에 commit-unknown을 보존하고 Exists/Count는
  이를 추가하지 않는다. 읽기 결과는 false/0이며 errors.Is로 취소 원인을 확인한다.
- [ ] Reserve OK 이외, Count string/음수/float, Exists/Delete int64 2, Add 빈 배열·2개 배열·
  bool false·string·nested error를 거부한다. mutation malformed는 commit-unknown이다.
- [ ] key·namespace·item·provider secret이 `%v`/`%+v`에 없고 `errors.Is/As`에는
  원인이 남는지 검증한다. unknown 결과를 성공 횟수에 넣지 않는 caller 예제를 검증한다.
- [ ] `go test -p 1 -count=1 -timeout=3m ./probabilistic/redis -run '^TestCuckoo'`
  GREEN, `go test -race -p 1 -count=1 -timeout=3m ./probabilistic/redis -run '^TestCuckoo'`
  GREEN 후 Korean Lore commit한다. 의도하지 않은 재시도는 호출 수 assertion으로 탐지한다.
  작업 1의 생성자 반환형이 Cuckoo이므로 메서드 선언은 작업 1에서 함께 추가하되
  미구현 메서드는 고정 ErrCuckooReply를 반환하게 한다. 작업 2 RED에서 이 거부가
  실제 성공·명령·오류 계약으로 대체되는 것을 확인한다.

## 작업 3: 실제 서버 계약 (높은 복잡도, 작업 2 이후)

- [ ] 기존 `newRedisClient(t)`로 plain Redis 7.4의 CF.RESERVE 실패를 먼저 확인한다.
  `errors.Is(err, ErrCuckooUnsupported)`와 commit-unknown 부재를 assertion한다.
- [ ] opt-in fixture는 아래 값만 사용한다. `BLUETAPE_CUCKOO_INTEGRATION=1`이 아니면
  명시적인 opt-in 안내로 테스트를 건너뛰되 양성 PASS로 보고하지 않는다.

```go
const cuckooImage = "redis@sha256:3eafabb4c93fcb8b36b666e07a43f096cb157bc6b07dce4b2492b895c63cf37f"
```

- [ ] 기존 `testcontainers/redis.StartServer`는 image를 주입할 수 없어 이 양성 fixture에는
  기존 패키지의 `tcredis.Run(ctx, cuckooImage)` 패턴을 사용한다. `internal/testcleanup`
  으로 container를 정리하고 client Close를 t.Cleanup에 등록한다. container 생성 직후
  cleanup을 등록하여 endpoint 조회·PING 실패도 정리한다. startup90초와 operation30초를
  분리하고 client IO timeout, MaxRetries -1을 사용한다.
  PING에 이어 실제 Reserve로 readiness/capability를 증명한다. shared fixture FlushDB는 쓰지 않는다.
- [ ] RESP2와 RESP3 각각 Reserve/Add duplicate/Count/Delete, missing key Add 거부,
  reserve 충돌, namespace 격리를 검증한다. missing/WRONGTYPE Exists=false도 검증한다.
- [ ] Capacity4·BucketSize2·Expansion0에 같은 item을 최대128회 추가해 포화를 찾고
  ErrCuckooFull과 count 불변을 확인한다. 포화가 관측되지 않으면 FAIL한다.
- [ ] 별도 namespace에서 Capacity512·BucketSize64·Expansion0으로 64개 goroutine이
  같은 item을 한 번씩 추가한다. WaitGroup 이후 모든 오류 nil, Count64를 assertion한다.
- [ ] 응답 유실은 #611 별도 worktree의 `ratelimit/sql/provider_cutover_test.go`에서
  조사한 읽기 후 EOF 방식을 #534 테스트 내부 타입으로 독립 구현한다. 미머지 파일을
  import하거나 실행 시 참조하지 않는다. net.Conn을 embedding하여 go-redis의 Close가
  원래 연결을 닫도록 하고 테스트 종료에는 client Close를 등록한다. 서버 명령 수행 뒤
  응답을 숨기며 기본 retry client는 중복 삽입,
  MaxRetries -1 client는 한 번 삽입임을 독립 관찰 client Count로 확인한다.
  응답이 최종 유실된 mutation만 ErrCommitUnknown이며 안전한 자동 replay를 주장하지 않는다.
  두 client는 다른 namespace를 사용한다. MaxRetries -1은 실제 삽입1회와 EOF 및
  ErrCommitUnknown을 확인한다. 기본 retry는 첫 응답만 유실시켜 성공+중복 삽입을
  관찰하는 negative control이며 adapter의 안전한 사용으로 간주하지 않는다.
- [ ] 아래 명령을 직렬 실행한다. 실패는 원인을 설명하고 수정한 뒤 같은 명령으로 재검증한다.

```bash
BLUETAPE_CUCKOO_INTEGRATION=1 go test -p 1 -count=1 -timeout=5m ./probabilistic/redis -run '^TestCuckoo'
BLUETAPE_CUCKOO_INTEGRATION=1 go test -race -p 1 -count=1 -timeout=5m ./probabilistic/redis -run '^TestCuckoo'
```

- [ ] fixture·통합 회귀를 Korean Lore commit한다. Docker 관련 다른 suite와 병렬 실행하지 않는다.

## 작업 4: 사용 예제·문서·전체 검증 (중간 복잡도, 작업 3 이후)

- [ ] `cuckoo_example_test.go`의 fake-only `ExampleNewCuckoo`는 Reserve 후 Add 성공만
  장부에 기록한다. Exists는 근사 membership이며 unknown 뒤 Delete/replay하지 않는다.
  `go test -p 1 -count=1 ./probabilistic/redis -run '^ExampleNewCuckoo$'`로 출력까지 검증한다.
- [ ] 두 README에 API, Namespace/item byte 계약, Reserve 신뢰 설정/quota, EXPANSION0,
  근사값·삭제 전제, 모듈 요구, no-retry client, timeout·Close 소유권, redacted error와
  unwrap 로깅 금지, 두 opt-in 명령을 동일하게 추가한다. root README는 기존 package index이므로 변경하지 않는다.
- [ ] `make fmt-check`, `make tidy-check`, `go vet ./probabilistic/redis`,
  `golangci-lint run ./probabilistic/redis`, 해당 package 전체 일반/race를 순서대로 실행한다.
- [ ] `make ci`를 main의 별도 폴링 process로 실행한다. 실패 package와 원인을 기록한다.
  무관한 환경 실패는 targeted proof와 분리하며 전체 PASS로 바꾸지 않는다.
- [ ] `git diff --check`, 변경 파일·untracked 검사, public Go doc·README parity를 확인한다.
  SPW-01..05와 KO-01..07을 plan/review/lesson별로 기록한다.
- [ ] 여섯 관점과 main 코드 검토를 수행하고 P0–P3 지적을 모두 처리한다.
  native 실행 불가면 실패 근거를 보존하고 inline fallback으로 표시한다.

## 작업 5: 교훈·개별 PR 전달 (작업 4 이후)

- [ ] 교훈에 CF helper의 zero 생략·RESP2/3, 오류와 cancellation 우선순위,
  알려진 성공 삭제에 대한 과도한 리뷰 주장의 철회, native receipt 형식 오류의 예방을 기록한다.
- [ ] spec/plan/test/code/README/review/lesson만 commit하고 Git 상태를 재확인한다.
- [ ] CG-12A 지침과 linked issue metadata를 갱신 확인한 후 승인된 head를 push한다.
  remote SHA 일치 확인 후 develop 대상 개별 PR을 생성한다. debop, milestone0.23.0,
  원래 labels를 설정하고 body 마지막 `## DoD Status`를 live-read한다.
- [ ] actual PR diff의 6관점+main 검토, exact-head CI 성공, 최신 reviews/threads 확인을
  완료한다. CI 중에는 다음 이슈의 허용된 로컬 단계를 진행할 수 있으나 Docker 병렬은 금지한다.
- [ ] PR과 남은 이슈 상태를 보고한다. 머지·배포·정리는 실행하지 않는다.

## 위험·되돌리기·추적

| 위험/신호 | 예방과 검증 | 재개 지점 |
|---|---|---|
| Redis 미지원, RESP 형태 차이 | plain/RESP2/RESP3 분리, strict parser | 작업 2·3 |
| 응답 유실 후 중복 변경 | no-replay 필수 전제, EOF와 실제 Count | 작업 2·3 |
| 취소가 성공에 가려짐 | 모든 전송 전후 checkpoint, zero result | 작업 2 |
| 포화·충돌·자원 소모 | 고정 quota 전제, 같은 item 포화·bounded64 | 작업 1·3 |
| namespace/item/오류 정보 노출 | argv 분리, 기존 keybuilder, OpError | 작업 1·2 |
| 기존 Bloom/HLL 회귀 | 기존 파일 무변경, 전체 package와 make ci | 작업 4 |

코드 되돌리기는 새 Cuckoo 파일과 README 추가분만 되돌린다. 서버 key 삭제나
호출자 client 설정 변경을 자동 복구로 실행하지 않는다. source parity는 기존
Go package 관례의 adapt이며 JVM wrapper·generic Hasher·batch는 비목표다.

수용 기준 대응: 생성자/크기/namespace→작업1, 모든 명령·RESP·오류·취소→작업2,
module/중복/삭제/포화/동시성/retry→작업3, example/양쪽README/정적·일반·race→작업4,
review/lesson/exact-headCI/개별PR→작업5. 성능 우위나 real Cluster 검증은 주장하지 않는다.

## 계획 상태

구현 전 계획 검토 대상이다. 체크되지 않은 실행 항목은 아직 실행하지 않았다.
