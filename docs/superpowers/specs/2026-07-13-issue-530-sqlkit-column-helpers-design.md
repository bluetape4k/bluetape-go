# Issue #530 sqlkit JSON 및 암호화 컬럼 헬퍼 설계

## 배경

`sqlkit`은 caller-owned SQL과 `database/sql` 경계를 유지하는 작은 helper package다.
현재 transaction, row mapping, statement builder는 제공하지만 JSON 또는 암호화된 컬럼을
`sql.Scanner`와 `driver.Valuer`로 연결하는 first-party 타입은 없다. `encrypt` package는
이미 random-nonce AES-GCM, versioned envelope, URL-safe string encoding, caller-owned key와
associated data 계약을 제공한다.

Issue #530은 이 두 package를 조합해 JSON과 암호화 컬럼의 scan/value 경계를 명시적으로
제공한다. ORM, schema metadata, migration, model hook을 도입하지 않으며 SQL NULL과
payload-level null/empty 값을 구분한다.

## 목표

- root `sqlkit` package에 typed JSON `Scanner`/`Valuer`를 추가한다.
- `encrypt.Encryptor`를 caller가 제공하는 encrypted bytes 및 string 컬럼 타입을 추가한다.
- SQL NULL, JSON literal `null`, empty plaintext를 서로 구분한다.
- driver-owned source buffer를 보존하지 않고, 실패 후 이전 plaintext가 남지 않게 한다.
- source와 encoded output 크기를 제한해 비정상적인 allocation을 조기에 거절한다.
- malformed/type/size/encryption error를 `errors.Is`로 검사 가능하게 유지하되 error
  string에는 JSON, plaintext, ciphertext, key 또는 associated data를 포함하지 않는다.
- direct `database/sql`, `sqlkit`, generated query call site를 compile-checked example과
  영어/한국어 README에서 보여 준다.
- `bluetape-diagram` best-practices family를 사용해 helper contract map과 Scan/Value
  sequence를 PNG/SVG pair로 제공한다.

## 비목표

- ORM entity, schema metadata, migration, lifecycle callback, dirty tracking을 추가하지 않는다.
- JSON path query, patch/merge, schema validation 또는 canonical JSON을 제공하지 않는다.
- deterministic/searchable encryption, blind index 또는 encrypted-column query를 제공하지 않는다.
- key generation, persistence, rotation, KMS/envelope encryption 또는 tenant authorization을
  `sqlkit`이 소유하지 않는다.
- money/measure Scanner/Valuer를 같은 issue에 포함하지 않는다.
- 새로운 module이나 외부 dependency를 추가하지 않는다.
- database dialect별 column type이나 DDL을 추론하지 않는다.
- benchmark 또는 성능 우위 주장을 추가하지 않는다.

## 검토한 접근

### 접근 1: 목적별 concrete column 타입 (채택)

`JSONColumn[T]`, `EncryptedBytesColumn`, `EncryptedStringColumn`을 root `sqlkit`에 둔다.
각 타입이 자신의 SQL NULL 상태와 scan/value 계약을 소유하고 encrypted 타입은 기존
`encrypt.Encryptor`에 위임한다.

장점:

- `database/sql`의 `Scanner`/`Valuer` 사용법과 직접 대응한다.
- bytes와 TEXT 반환 타입, JSON null과 SQL NULL 의미가 타입별로 명확하다.
- codec registry나 ORM metadata 없이 bounded validation을 넣을 수 있다.
- generated query layer가 표준 interface로 그대로 전달할 수 있다.

단점:

- 세 타입 사이에 작은 상태/limit/error 처리 중복이 생긴다.
- encrypted 타입은 non-NULL 값을 처리하기 전에 constructor가 필요하다.

### 접근 2: function 기반 Scanner/Valuer adapter (제외)

closure로 marshal/unmarshal 또는 encrypt/decrypt 함수를 받으면 표면 API는 일반화된다.
그러나 scan 재사용 시 NULL 상태와 이전 값 제거가 caller closure로 분산되고, generated
query code에서 타입 계약을 읽기 어렵다. 작은 concrete 타입보다 lifecycle이 불명확하다.

### 접근 3: generic `Column[T]`와 public codec registry (제외)

하나의 `Column[T]`에 JSON/encryption codec을 주입하면 확장성은 높다. 대신 codec
composition, registry, option applicability, schema type 선택이 새로운 abstraction layer가
되어 ORM 방향으로 확장될 가능성이 크다. 현재 세 call site에는 과도하다.

## Package와 공개 API

새 타입은 기존 root `sqlkit` package에 둔다. `sqlkit`이 같은 module의 `encrypt`를
import하며 반대 방향 import는 없어 cycle이 없다.

```go
package sqlkit

const (
    DefaultJSONColumnMaxBytes                = 1 << 20
    DefaultEncryptedColumnMaxPlaintextBytes = 1 << 20
    DefaultEncryptedColumnMaxCiphertextBytes = 2 << 20
)

var (
    ErrInvalidColumnValue = errors.New("sqlkit: invalid column value")
    ErrColumnValueTooLarge = errors.New("sqlkit: column value too large")
)

type JSONColumn[T any] struct {
    Data     T
    Valid    bool
    MaxBytes int
}

func (c *JSONColumn[T]) Scan(src any) error
func (c JSONColumn[T]) Value() (driver.Value, error)

type EncryptedBytesColumn struct {
    Data               []byte
    Valid              bool
    MaxPlaintextBytes  int
    MaxCiphertextBytes int
    // encryptor and copied associated data are private.
}

func NewEncryptedBytesColumn(
    encryptor encrypt.Encryptor,
    associatedData []byte,
) EncryptedBytesColumn
func (c *EncryptedBytesColumn) Scan(src any) error
func (c EncryptedBytesColumn) Value() (driver.Value, error)

type EncryptedStringColumn struct {
    Data               string
    Valid              bool
    MaxPlaintextBytes  int
    MaxCiphertextBytes int
    // encryptor and copied associated data are private.
}

func NewEncryptedStringColumn(
    encryptor encrypt.Encryptor,
    associatedData []byte,
) EncryptedStringColumn
func (c *EncryptedStringColumn) Scan(src any) error
func (c EncryptedStringColumn) Value() (driver.Value, error)
```

`Max*Bytes == 0`은 해당 exported default를 사용한다. 음수 값은
`ErrInvalidColumnValue`다. Positive 값은 caller override이며 source 또는 output을 읽거나
생성한 직후 적용한다. Limit은 byte 수 기준이다.

`JSONColumn[T]` zero value는 SQL NULL을 나타내고 바로 사용할 수 있다. Encrypted 타입의
zero value도 SQL NULL의 `Value`/`Scan(nil)`에는 안전하지만, non-NULL scan/value는
constructor로 주입한 `encrypt.Encryptor`가 필요하다. 초기화되지 않은 encryptor의 기존
`encrypt.ErrInvalidKey`를 그대로 보존한다. 타입은 동시 mutation을 지원하지 않는다.
독립 row/call마다 별도 column 값을 사용한다. 공유되는 `encrypt.Encryptor` 자체의
goroutine-safe 계약은 유지된다.

모든 exported type, constant, constructor, method, sentinel에 English Go doc을 추가한다.
Compile-time assertion으로 다음 interface 구현을 고정한다.

```go
var _ sql.Scanner = (*JSONColumn[any])(nil)
var _ driver.Valuer = JSONColumn[any]{}
var _ sql.Scanner = (*EncryptedBytesColumn)(nil)
var _ driver.Valuer = EncryptedBytesColumn{}
var _ sql.Scanner = (*EncryptedStringColumn)(nil)
var _ driver.Valuer = EncryptedStringColumn{}
```

## 공통 상태 및 오류 계약

- `Scan`은 시작 시 `Data`를 타입의 fresh zero value로 만들고 `Valid=false`로 설정한다.
- `Scan(nil)`은 성공하며 cleared SQL NULL 상태를 유지한다.
- non-NULL scan은 decode/decrypt와 모든 limit 검증이 성공한 뒤에만 `Data`와
  `Valid=true`를 publish한다.
- 실패하면 이전 JSON 또는 plaintext가 남지 않는다.
- retained data는 driver-owned `[]byte` backing array를 참조하지 않는다.
- unsupported source type과 malformed payload는 `ErrInvalidColumnValue`에 match한다.
- source/output limit 위반은 `ErrColumnValueTooLarge`에 match한다.
- JSON 및 encrypt cause는 안전한 내부 wrapper로 보존해 `errors.Is`/`errors.As`를
  사용할 수 있게 한다. Wrapper의 `Error()`는 operation과 sentinel만 출력하며 source,
  JSON field value, plaintext, ciphertext, key, associated data 또는 cause text를 출력하지
  않는다.
- `Scan`과 `Value`는 user-defined JSON callback 때문에 panic하지 않는다.
  `json.Unmarshaler`/`json.Marshaler` panic은 recover하여 `ErrInvalidColumnValue`로
  반환하며 panic value를 error text에 넣지 않는다.
- nil `*JSONColumn`, `*EncryptedBytesColumn`, `*EncryptedStringColumn`에 직접 `Scan`하면
  panic 대신 `ErrInvalidColumnValue`를 반환한다.

## JSONColumn 계약

### Scan

- 허용 source는 `nil`, `string`, `[]byte`다. 그 외 driver source는 거절한다.
- source 크기를 unmarshal 전에 effective `MaxBytes`와 비교한다.
- `[]byte` source는 unmarshal 전에 복사한다. 따라서 custom `json.Unmarshaler`가 전달된
  slice를 보관하더라도 driver-owned backing array를 retain하지 않는다.
- decode 대상은 기존 `Data`가 아닌 fresh `var decoded T`다.
- `encoding/json.Unmarshal`을 사용하므로 trailing token, malformed JSON, type mismatch를
  그대로 실패 처리한다.
- JSON literal `null`은 SQL NULL이 아니다. Fresh zero `T`를 `Data`에 넣고
  `Valid=true`로 설정한다. 따라서 pointer/slice/map `T`는 nil data와 valid state를 함께
  가질 수 있다.
- String source도 private byte slice로 변환한 뒤 decode하며 source representation을
  retained state에 직접 저장하지 않는다.

### Value

- `Valid=false`이면 `(nil, nil)`을 반환한다.
- `Valid=true`이면 `encoding/json.Marshal(Data)` 결과를 `[]byte` driver value로 반환한다.
- marshal output이 effective `MaxBytes`보다 크면 ciphertext/JSON bytes를 반환하지 않고
  `ErrColumnValueTooLarge`를 반환한다.
- `Data`가 nil pointer/slice/map이고 `Valid=true`이면 JSON bytes `null`을 반환한다.
- canonical ordering 또는 semantic normalization을 약속하지 않는다. 출력은 현재
  `encoding/json` 계약이다.

## EncryptedBytesColumn 계약

- constructor는 associated data를 즉시 복사한다. 이후 caller가 원본 slice를 변경해도
  decrypt/encrypt context가 바뀌지 않는다.
- `Scan`은 `nil`, `[]byte`, `string`을 허용한다. String은 raw bytes로 바꾸며 둘 모두
  `encrypt.Encryptor.Decrypt`의 versioned binary envelope로 해석한다.
- stored source를 decrypt 전에 effective `MaxCiphertextBytes`와 비교한다.
- decrypted plaintext를 publish하기 전에 effective `MaxPlaintextBytes`와 비교한다.
- `Value`는 `Valid=false`이면 SQL NULL, `Valid=true`이면
  `encrypt.Encryptor.Encrypt(Data, copiedAAD)`가 만든 `[]byte`를 반환한다.
- Empty plaintext와 nil byte slice는 `Valid=true`이면 각각 유효한 encrypted payload다.
  SQL NULL은 오직 `Valid=false`다.
- 각 `Value` 호출은 random nonce 때문에 동일 plaintext/AAD에도 다른 ciphertext를 만든다.
  equality/filter/order query를 지원하지 않는다.

## EncryptedStringColumn 계약

- constructor, 상태 제거, limit, associated-data copy 계약은 bytes 타입과 같다.
- `Scan`은 `nil`, `string`, `[]byte`를 허용한다. Non-NULL source는 raw URL-safe base64
  envelope이며 `encrypt.Encryptor.DecryptString`으로 처리한다.
- `Value`는 `Valid=false`이면 SQL NULL, `Valid=true`이면
  `encrypt.Encryptor.EncryptString(Data, copiedAAD)`가 만든 `string`을 반환한다.
- Empty string과 SQL NULL은 `Valid`로 구분한다.
- `encrypt`의 UTF-8, malformed base64, authentication, wrong-key/wrong-AAD sentinel을
  `errors.Is`로 보존한다.

Encrypted bytes는 BYTEA/BLOB 계열, encrypted string은 TEXT/VARCHAR 계열의 명시적
boundary로 문서화한다. `Scan`에서 두 common driver representation을 허용하는 것은 driver
호환성일 뿐 schema type 추론이나 자동 conversion을 의미하지 않는다.

## 보안 및 운영 경계

- Associated data는 secret이 아니며 tenant/entity/column/protocol version처럼 안정적인
  context를 caller가 canonical하게 구성한다.
- Caller는 key 생성, 안전한 저장, 접근 제어, backup, rotation 및 과거 key decrypt 전략을
  소유한다. 이 helper는 key ID나 rotation metadata를 ciphertext 옆에 추가하지 않는다.
- Error와 README 예제에는 실제 key, plaintext, ciphertext 또는 tenant secret을 넣지 않는다.
- Encrypted column을 검색 가능하게 만들기 위해 nonce를 고정하거나 deterministic mode를
  추가하지 않는다.
- Size limit은 allocation 방어이며 database column DDL limit을 대체하지 않는다.
- `Value` 호출은 암호화 side effect가 없는 local computation이지만 매번 새 nonce를
  소비한다. Driver retry 또는 repeated `Value` 호출은 다른 ciphertext를 만들 수 있으나
  같은 plaintext 의미를 유지한다.

## 테스트 전략

### JSON

- compile-time `Scanner`/`Valuer` assertion.
- struct, slice, pointer의 Scan/Value round trip.
- SQL NULL과 JSON literal `null` 구분.
- empty string, malformed/trailing JSON, unsupported source type.
- source 및 marshaled output oversize.
- failed scan 뒤 previous data/valid state 제거.
- caller/driver source slice 변경이 decoded state에 영향 없음.
- custom `json.Unmarshaler`/`json.Marshaler` error 및 panic이 safe inspectable error가 됨.
- custom unmarshaler가 source slice를 retain해도 이후 driver buffer mutation과 격리됨.
- zero/default/custom/negative limit.

### Encryption

- bytes와 string의 Scan/Value round trip 및 empty plaintext.
- SQL NULL과 valid empty bytes/string 구분.
- BYTEA 반환은 `[]byte`, TEXT 반환은 `string`인지 확인.
- same plaintext의 repeated Value가 다른 ciphertext이며 모두 decrypt되는지 확인.
- malformed envelope/base64, wrong key, wrong AAD, tamper에서 encrypt sentinel 보존.
- ciphertext source 및 decrypted plaintext oversize.
- failed scan 뒤 plaintext/valid state 제거.
- constructor 이후 AAD source mutation과 scan source slice alias 회귀.
- zero/unconfigured encryptor가 panic하지 않고 `encrypt.ErrInvalidKey`를 반환.
- error string에 test plaintext/ciphertext/key/AAD marker가 없는지 확인.

### Examples 및 commands

- direct `database/sql`: query row scan과 Exec argument로 JSON/encrypted column 사용.
- `sqlkit`: `QueryOne` mapper와 built `Statement` argument 사용.
- generated query code: 표준 `Scanner` destination과 `driver.Valuer` parameter를 받는
  compile-checked stub으로 sqlc/Jet runtime dependency 없이 integration shape 증명.
- RED/GREEN 이후 최소 검증:
  - `go test -count=1 ./sqlkit ./encrypt`
  - `go test -run Example -count=1 ./sqlkit`
  - `go test -race -count=1 ./sqlkit ./encrypt`
  - `make fmt-check`, `make tidy-check`, `make vet`, `make lint`
  - 최종 authoritative gate `make ci`

## 문서 및 diagram

- `sqlkit/README.md`와 `sqlkit/README.ko.md`에 타입 선택표, NULL/limit/error 계약,
  direct/sqlkit/generated usage를 source-equivalent하게 추가한다.
- `encrypt/README.md`와 `encrypt/README.ko.md`에 sqlkit column integration, storage type,
  AAD/key/rotation/query non-goal을 source-equivalent하게 추가한다.
- 기존 `docs/images/readme-diagrams/sqlkit-helper-contract-map.svg/.png`는 JSON/encrypted
  boundary를 포함하도록 architecture/component diagram으로 갱신한다.
- 새 `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg/.png`는 SQL NULL,
  JSON decode 또는 decrypt, size/error branch, successful publish, Value encode/encrypt를
  시간 순서로 보여 준다.
- Diagram label은 English로 작성하고 양 locale README가 같은 PNG를 embed한다.
- `bluetape-diagram`의 common, architecture, sequence reference를 적용한다. Sequence는
  best-practices catalog와 nearest approved repo-local full-size PNG 두 개를 먼저 열고,
  architecture는 current approved family를 기준으로 한다.
- 각 asset을 한 번에 하나씩 SVG edit -> `xmllint` -> CairoSVG `-s 2` render ->
  connector/geometry/endpoint/mixed-corner/type audit -> full-size PNG inspection 순서로
  검증한다. PNG가 최종 authority다.

## 변경 범위와 delivery

예상 production scope는 root `sqlkit`의 error/JSON/encrypted column 파일과 해당 tests다.
기존 `encrypt` production API는 변경하지 않는다. 추가되는 나머지 파일은 compile-checked
examples, locale README, diagram SVG/PNG 및 이 spec/후속 plan이다. 새로운 module,
dependency, schema, workflow YAML, migration 또는 benchmark harness는 없다.

Issue #530의 assignee `debop`, milestone `0.19.0`, 현재 labels를 PR에 mirror한다. PR body의
마지막 `##` heading은 `## DoD Status`로 유지한다. Local/CI verification과 P0/P1=0 이후
PR을 생성하고, 사용자가 요청한 대로 CI 완료 시점까지 관찰하되 merge는 별도 명시된
승인 경계에서만 수행한다.

## 수용 기준

- 세 타입의 NULL/data/limit/source/output/error 계약이 table-driven tests로 고정된다.
- `errors.Is`로 sqlkit size/type sentinel과 encrypt authentication/malformed/key sentinel을
  검사할 수 있다.
- 실패한 Scan 뒤 stale JSON/plaintext가 없고 source/AAD alias가 없다.
- public examples가 direct `database/sql`, `sqlkit`, generated query 형태로 compile된다.
- sqlkit/encrypt English/Korean README가 source-equivalent하다.
- contract map과 Scan/Value sequence의 SVG/PNG pair가 source-backed이며 diagram ledger의
  모든 required audit와 full-size inspection을 통과한다.
- targeted tests, race, formatter/tidy/vet/lint와 `make ci`가 fresh PASS다.
- final review 결과 P0=0, P1=0이며 issue-linked PR metadata/body/CI를 live로 검증한다.
