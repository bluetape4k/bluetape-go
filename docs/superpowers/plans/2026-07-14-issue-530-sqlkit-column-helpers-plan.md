# Issue #530 sqlkit Column Helpers 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 bounded JSON 및 encrypted byte/string `database/sql.Scanner` 및 `driver.Valuer` helpers to `sqlkit`, 함께 compile-checked example, bilingual documentation, 및 verified SVG/PNG diagrams.

**아키텍처:** 유지 three purpose-built values in root `sqlkit`. `JSONColumn[T]` owns typed JSON 및 SQL NULL state; `EncryptedBytesColumn` 및 `EncryptedStringColumn` compose 함께 a 호출자-owned immutable `encrypt.Encryptor` 및 copied associated data. A private 오류/limit layer preserves `errors.Is` without exposing payloads, 및 every `Scan` clears stale state 전에 work 및 publishes 만 후 complete validation.

**기술 스택:** Go 1.26, `database/sql`, `database/sql/driver`, `encoding/json`, 기존 `encrypt` 패키지, table-driven 테스트, `bluetape-writer`, `bluetape-diagram`, CairoSVG.

---

## 실행 제약

- Work 만 in `/Users/debop/work/bluetape4k/bluetape-go/.worktrees/feat-issue-530-sqlkit-columns` on `feat/issue-530-sqlkit-columns`.
- Design authority: `docs/superpowers/specs/2026-07-13-issue-530-sqlkit-column-helpers-design.md` at `8b6714d`.
- Load `test-driven-development` 전에 production edits 및 `verification-before-completion` 전에 delivery claims.
- No new module, dependency, schema, migration, ORM hook, deterministic encryption, KMS, money/measure helper, benchmark, 또는 workf낮음 YAML.
- 사용 `apply_patch` for edits. 실행 heavyweight checks serially.
- Public Go docs, 영문 README, commits, PR text, 및 diagram labels are 영문. 유지 `README.ko.md` source-equivalent 및 natural 한국어.
- 생성 one diagram asset at a time; rendered PNG evidence is authoritative.

## 파일 지도

| Path | 책임 |
|---|---|
| `sqlkit/column_errors.go` | Sentinels, safe wrapper, source-copy, limits, panic conversion |
| `sqlkit/json_column.go` | `JSONColumn[T]` Scanner/Valuer |
| `sqlkit/json_column_test.go` | JSON behavior 및 hardening |
| `sqlkit/encrypted_column.go` | Encrypted byte/string types 및 constructors |
| `sqlkit/encrypted_column_test.go` | Encryption behavior 및 hardening |
| `sqlkit/column_example_test.go` | Direct SQL, sqlkit, generated-query example |
| `sqlkit/README.md`, `sqlkit/README.ko.md` | Public column 계약 |
| `encrypt/README.md`, `encrypt/README.ko.md` | SQL integration 및 crypto boundaries |
| `docs/images/readme-diagrams/sqlkit-helper-contract-map.svg/.png` | Updated static responsibility map |
| `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg/.png` | New Scan/Value lifecycle |

### 작업 1: 구현 bounded JSON columns

**파일:**
- 생성: `sqlkit/column_errors.go`
- 생성: `sqlkit/json_column.go`
- 생성: `sqlkit/json_column_test.go`

- [ ] **단계 1: Write the failing JSON 테스트**

생성 external-패키지 테스트 함께 these fixtures:

```go
type jsonProfile struct {
    Name string `json:"name"`
}

type retainingJSON []byte

func (r *retainingJSON) UnmarshalJSON(data []byte) error {
    *r = data
    return nil
}

type panickingJSON struct{}

func (*panickingJSON) UnmarshalJSON([]byte) error { panic("unmarshal-secret") }
func (panickingJSON) MarshalJSON() ([]byte, error) { panic("marshal-secret") }

var _ sql.Scanner = (*sqlkit.JSONColumn[jsonProfile])(nil)
var _ driver.Valuer = sqlkit.JSONColumn[jsonProfile]{}
```

추가 these exact 테스트 및 assertions:

| Test | Input | Required assertion |
|---|---|---|
| `TestJSONColumnRoundTrip` | valid `jsonProfile{Name:"Ada"}` | `Value` is `[]byte`; `Scan` returns equal data 및 `Valid=true` |
| `TestJSONColumnDistinguishesSQLNullAndJSONNull` | `nil` then `[]byte("null")` into `JSONColumn[*jsonProfile]` | SQL NULL is invalid; JSON null is valid 함께 nil `Data` 및 re-encodes to `null` |
| `TestJSONColumnClearsStateOnFailure` | malformed JSON, trailing token, `int64` source | 오류 match `ErrInvalidColumnValue`; old data is zeroed 및 `Valid=false` |
| `TestJSONColumnEnforcesLimits` | source/output above 4 bytes 및 negative limit | oversize matches `ErrColumnValueTooLarge`; negative matches `ErrInvalidColumnValue` |
| `TestJSONColumnCopiesDriverSource` | `retainingJSON` 함께 mutable `[]byte` source | mutating source 후 Scan cannot change retained data |
| `TestJSONColumnContainsCallbackPanics` | panicking unmarshal/marshal | 없음 panic; 오류 matches `ErrInvalidColumnValue` 및 omits both secret markers |
| `TestJSONColumnNilScanner` | nil `*JSONColumn[jsonProfile]` | returns `ErrInvalidColumnValue` |

사용 table-driven subtests for malformed/unsupported/limit cases 및 require default limit constants to equal `1 << 20`.

- [ ] **단계 2: 실행 RED**

```bash
go test -count=1 ./sqlkit -run '^TestJSONColumn'
```

예상: compile failure naming undefined `JSONColumn`, `ErrInvalidColumnValue`, 및 `ErrColumnValueTooLarge`.

- [ ] **단계 3: 구현 safe 공유 오류 및 limits**

생성 `sqlkit/column_errors.go`:

```go
package sqlkit

import (
    "errors"
    "fmt"
)

var (
    // ErrInvalidColumnValue reports an unsupported, malformed, or invalidly configured column value.
    ErrInvalidColumnValue = errors.New("sqlkit: invalid column value")
    // ErrColumnValueTooLarge reports a column source or encoded value above its byte limit.
    ErrColumnValueTooLarge = errors.New("sqlkit: column value too large")
)

type columnError struct {
    kind      error
    operation string
    cause     error
}

func (e *columnError) Error() string {
    if e == nil {
        return ErrInvalidColumnValue.Error()
    }
    kind := e.kind
    if kind == nil {
        kind = ErrInvalidColumnValue
    }
    if e.operation == "" {
        return kind.Error()
    }
    return fmt.Sprintf("%v: %s", kind, e.operation)
}

func (e *columnError) Unwrap() error {
    if e == nil {
        return nil
    }
    return e.cause
}

func (e *columnError) Is(target error) bool {
    return e != nil && (errors.Is(e.kind, target) || errors.Is(e.cause, target))
}

func newColumnError(kind error, operation string, cause error) error {
    return &columnError{kind: kind, operation: operation, cause: cause}
}

func recoverColumnPanic(operation string, errp *error) {
    if recover() != nil {
        *errp = newColumnError(ErrInvalidColumnValue, operation, nil)
    }
}

func effectiveColumnLimit(configured, fallback int, operation string) (int, error) {
    if configured < 0 {
        return 0, newColumnError(ErrInvalidColumnValue, operation, nil)
    }
    if configured == 0 {
        return fallback, nil
    }
    return configured, nil
}

func copiedColumnSource(src any, operation string) ([]byte, bool, error) {
    switch value := src.(type) {
    case nil:
        return nil, false, nil
    case []byte:
        return append([]byte(nil), value...), true, nil
    case string:
        return []byte(value), true, nil
    default:
        return nil, false, newColumnError(ErrInvalidColumnValue, operation, nil)
    }
}
```

- [ ] **단계 4: 구현 `JSONColumn[T]`**

생성 `sqlkit/json_column.go` 함께 `DefaultJSONColumnMaxBytes = 1 << 20`, fields `Data T`, `Valid bool`, `MaxBytes int`, 및 영문 Go docs. 구현 this exact order:

```go
func (c *JSONColumn[T]) Scan(src any) (err error) {
    if c == nil {
        return newColumnError(ErrInvalidColumnValue, "scan JSON", nil)
    }
    var zero T
    c.Data, c.Valid = zero, false
    defer recoverColumnPanic("scan JSON", &err)

    raw, present, err := copiedColumnSource(src, "scan JSON")
    if err != nil || !present {
        return err
    }
    limit, err := effectiveColumnLimit(c.MaxBytes, DefaultJSONColumnMaxBytes, "scan JSON limit")
    if err != nil {
        return err
    }
    if len(raw) > limit {
        return newColumnError(ErrColumnValueTooLarge, "scan JSON", nil)
    }
    var decoded T
    if err := json.Unmarshal(raw, &decoded); err != nil {
        return newColumnError(ErrInvalidColumnValue, "scan JSON", err)
    }
    c.Data, c.Valid = decoded, true
    return nil
}

func (c JSONColumn[T]) Value() (value driver.Value, err error) {
    if !c.Valid {
        return nil, nil
    }
    defer recoverColumnPanic("encode JSON", &err)

    limit, err := effectiveColumnLimit(c.MaxBytes, DefaultJSONColumnMaxBytes, "encode JSON limit")
    if err != nil {
        return nil, err
    }
    raw, err := json.Marshal(c.Data)
    if err != nil {
        return nil, newColumnError(ErrInvalidColumnValue, "encode JSON", err)
    }
    if len(raw) > limit {
        return nil, newColumnError(ErrColumnValueTooLarge, "encode JSON", nil)
    }
    return raw, nil
}
```

- [ ] **단계 5: 실행 GREEN 및 commit**

```bash
gofmt -w sqlkit/column_errors.go sqlkit/json_column.go sqlkit/json_column_test.go
go test -count=1 ./sqlkit -run '^TestJSONColumn'
go test -count=1 ./sqlkit
git add sqlkit/column_errors.go sqlkit/json_column.go sqlkit/json_column_test.go
git commit -m "feat: add bounded JSON column helper"
```

예상: 모든 테스트 PASS 및 the commit succeeds.

### 작업 2: 구현 encrypted byte columns

**파일:**
- 생성: `sqlkit/encrypted_column.go`
- 생성: `sqlkit/encrypted_column_test.go`

- [ ] **단계 1: Write failing byte-column 테스트**

사용 a fixed 32-byte 테스트 key 만 in 테스트:

```go
func newTestEncryptor(t *testing.T) encrypt.Encryptor {
    t.Helper()
    value, err := encrypt.New(bytes.Repeat([]byte{0x42}, 32))
    if err != nil {
        t.Fatalf("encrypt.New: %v", err)
    }
    return value
}

var _ sql.Scanner = (*sqlkit.EncryptedBytesColumn)(nil)
var _ driver.Valuer = sqlkit.EncryptedBytesColumn{}
```

추가 exact 테스트 coverage:

- `TestEncryptedBytesColumnRoundTripNullAndStorageType`: `Value` is `[]byte`; Scan restores plaintext; Scan(nil) clears; valid nil/empty plaintext remains non-NULL.
- `TestEncryptedBytesColumnUsesRandomCiphertext`: two Value calls differ 및 both decrypt.
- `TestEncryptedBytesColumnPreservesEncryptErrors`: malformed, tamper, wrong key, wrong AAD preserve the matching `encrypt` sentinel 및 `ErrInvalidColumnValue`.
- `TestEncryptedBytesColumnEnforcesLimits`: negative configuration, oversized stored source, decrypted plaintext, input plaintext, 및 encrypted output return the correct sqlkit sentinel.
- `TestEncryptedBytesColumnClearsPlaintextOnFailure`: 모든 failure branches leave nil `Data` 및 `Valid=false`.
- `TestEncryptedBytesColumnCopiesAADAndSource`: mutate 호출자 AAD 후 construction 및 driver bytes 후 Scan; behavior remains bound to the original copies.
- `TestEncryptedBytesColumnRedactsErrors`: marker plaintext/ciphertext/key/AAD never appears in `Error()`.
- `TestEncryptedBytesColumnZeroValue`: invalid Value is SQL NULL; non-NULL Scan/Value preserves `encrypt.ErrInvalidKey` without panic.
- `TestEncryptedBytesColumnNilScanner`: nil receiver returns `ErrInvalidColumnValue`.

- [ ] **단계 2: 실행 RED**

```bash
go test -count=1 ./sqlkit -run '^TestEncryptedBytesColumn'
```

예상: compile failure for undefined encrypted byte APIs.

- [ ] **단계 3: 구현 the byte type**

생성 `sqlkit/encrypted_column.go` 함께:

```go
const (
    DefaultEncryptedColumnMaxPlaintextBytes  = 1 << 20
    DefaultEncryptedColumnMaxCiphertextBytes = 2 << 20
)

type encryptedColumnConfig struct {
    encryptor      encrypt.Encryptor
    associatedData []byte
}

type EncryptedBytesColumn struct {
    Data               []byte
    Valid              bool
    MaxPlaintextBytes  int
    MaxCiphertextBytes int
    config             encryptedColumnConfig
}

func NewEncryptedBytesColumn(encryptor encrypt.Encryptor, associatedData []byte) EncryptedBytesColumn {
    return EncryptedBytesColumn{config: encryptedColumnConfig{
        encryptor: encryptor,
        associatedData: append([]byte(nil), associatedData...),
    }}
}
```

추가 영문 Go docs 및 these complete methods:

```go
func (c *EncryptedBytesColumn) Scan(src any) error {
    if c == nil {
        return newColumnError(ErrInvalidColumnValue, "scan encrypted bytes", nil)
    }
    c.Data, c.Valid = nil, false
    raw, present, err := copiedColumnSource(src, "scan encrypted bytes")
    if err != nil || !present {
        return err
    }
    ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "scan encrypted bytes ciphertext limit")
    if err != nil {
        return err
    }
    plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "scan encrypted bytes plaintext limit")
    if err != nil {
        return err
    }
    if len(raw) > ciphertextLimit {
        return newColumnError(ErrColumnValueTooLarge, "scan encrypted bytes ciphertext", nil)
    }
    plaintext, err := c.config.encryptor.Decrypt(raw, c.config.associatedData)
    if err != nil {
        return newColumnError(ErrInvalidColumnValue, "scan encrypted bytes", err)
    }
    if len(plaintext) > plaintextLimit {
        return newColumnError(ErrColumnValueTooLarge, "scan encrypted bytes plaintext", nil)
    }
    c.Data, c.Valid = append([]byte(nil), plaintext...), true
    return nil
}

func (c EncryptedBytesColumn) Value() (value driver.Value, err error) {
    if !c.Valid {
        return nil, nil
    }
    defer recoverColumnPanic("encrypt bytes", &err)
    plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "encrypt bytes plaintext limit")
    if err != nil {
        return nil, err
    }
    if len(c.Data) > plaintextLimit {
        return nil, newColumnError(ErrColumnValueTooLarge, "encrypt bytes plaintext", nil)
    }
    ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "encrypt bytes ciphertext limit")
    if err != nil {
        return nil, err
    }
    ciphertext, err := c.config.encryptor.Encrypt(c.Data, c.config.associatedData)
    if err != nil {
        return nil, newColumnError(ErrInvalidColumnValue, "encrypt bytes", err)
    }
    if len(ciphertext) > ciphertextLimit {
        return nil, newColumnError(ErrColumnValueTooLarge, "encrypt bytes ciphertext", nil)
    }
    return ciphertext, nil
}
```

- [ ] **단계 4: 실행 GREEN 및 commit**

```bash
gofmt -w sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
go test -count=1 ./sqlkit -run '^TestEncryptedBytesColumn'
go test -count=1 ./sqlkit ./encrypt
git add sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
git commit -m "feat: add encrypted byte column helper"
```

### 작업 3: 구현 encrypted string columns

**파일:**
- Modify: `sqlkit/encrypted_column.go`
- Modify: `sqlkit/encrypted_column_test.go`

- [ ] **단계 1: Write failing string 테스트**

추가 interface assertions 및 these 테스트:

- `TestEncryptedStringColumnRoundTripNullAndStorageType`: Value is `string`; both string/`[]byte` Scan work; nil clears.
- `TestEncryptedStringColumnEmptyPlaintextIsNotSQLNull`: empty valid string encrypts 및 scans back as `Valid=true`.
- `TestEncryptedStringColumnPreservesMalformedAuthenticationAndUTF8Errors`: malformed base64, tamper, wrong key/AAD, 및 encrypted invalid UTF-8 preserve `encrypt` sentinels.
- `TestEncryptedStringColumnEnforcesLimitsAndClearsState`: 모든 source/plaintext/output/negative cases return the correct sentinel 및 clear old text.
- `TestEncryptedStringColumnCopiesAADAndRedactsErrors`.
- `TestEncryptedStringColumnZeroValueAndNilScanner`.

- [ ] **단계 2: 실행 RED**

```bash
go test -count=1 ./sqlkit -run '^TestEncryptedStringColumn'
```

예상: compile failure for undefined string APIs.

- [ ] **단계 3: 구현 the string type**

Append:

```go
type EncryptedStringColumn struct {
    Data               string
    Valid              bool
    MaxPlaintextBytes  int
    MaxCiphertextBytes int
    config             encryptedColumnConfig
}

func NewEncryptedStringColumn(encryptor encrypt.Encryptor, associatedData []byte) EncryptedStringColumn {
    return EncryptedStringColumn{config: encryptedColumnConfig{
        encryptor: encryptor,
        associatedData: append([]byte(nil), associatedData...),
    }}
}
```

추가 영문 Go docs 및 these complete methods:

```go
func (c *EncryptedStringColumn) Scan(src any) error {
    if c == nil {
        return newColumnError(ErrInvalidColumnValue, "scan encrypted string", nil)
    }
    c.Data, c.Valid = "", false
    raw, present, err := copiedColumnSource(src, "scan encrypted string")
    if err != nil || !present {
        return err
    }
    ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "scan encrypted string ciphertext limit")
    if err != nil {
        return err
    }
    plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "scan encrypted string plaintext limit")
    if err != nil {
        return err
    }
    if len(raw) > ciphertextLimit {
        return newColumnError(ErrColumnValueTooLarge, "scan encrypted string ciphertext", nil)
    }
    plaintext, err := c.config.encryptor.DecryptString(string(raw), c.config.associatedData)
    if err != nil {
        return newColumnError(ErrInvalidColumnValue, "scan encrypted string", err)
    }
    if len(plaintext) > plaintextLimit {
        return newColumnError(ErrColumnValueTooLarge, "scan encrypted string plaintext", nil)
    }
    c.Data, c.Valid = plaintext, true
    return nil
}

func (c EncryptedStringColumn) Value() (value driver.Value, err error) {
    if !c.Valid {
        return nil, nil
    }
    defer recoverColumnPanic("encrypt string", &err)
    plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "encrypt string plaintext limit")
    if err != nil {
        return nil, err
    }
    if len(c.Data) > plaintextLimit {
        return nil, newColumnError(ErrColumnValueTooLarge, "encrypt string plaintext", nil)
    }
    ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "encrypt string ciphertext limit")
    if err != nil {
        return nil, err
    }
    ciphertext, err := c.config.encryptor.EncryptString(c.Data, c.config.associatedData)
    if err != nil {
        return nil, newColumnError(ErrInvalidColumnValue, "encrypt string", err)
    }
    if len(ciphertext) > ciphertextLimit {
        return nil, newColumnError(ErrColumnValueTooLarge, "encrypt string ciphertext", nil)
    }
    return ciphertext, nil
}
```

- [ ] **단계 4: 실행 GREEN, targeted race, 및 commit**

```bash
gofmt -w sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
go test -count=1 ./sqlkit -run '^TestEncrypted(String|Bytes)Column'
go test -count=1 ./sqlkit ./encrypt
go test -race -count=1 ./sqlkit ./encrypt
git add sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
git commit -m "feat: add encrypted string column helper"
```

예상: every command PASS. Race is required because immutable Encryptor sharing is part of the composed 계약.

### 작업 4: 추가 compile-checked integration example

**파일:**
- 생성: `sqlkit/column_example_test.go`

- [ ] **단계 1: 추가 three compile-만 example**

구현:

- `ExampleJSONColumn_databaseSQL`: local closures using `QueryRowContext(...).Scan(&column)` 및 `ExecContext(..., column)`.
- `ExampleEncryptedBytesColumn_sqlkit`: `QueryOne` mapper constructs the byte column, calls `rows.Scan(&column)`, 및 returns a copied plaintext.
- `ExampleEncryptedStringColumn_generatedQuery`: a local generated-query interface takes a params struct whose payload is `driver.Valuer`; construct/set/pass the column.

다음을 하지 않는다: call a nil database. Assign closures to `_` 및 omit Output comments so the example compile without executing external I/O.

- [ ] **단계 2: 검증 및 commit**

```bash
gofmt -w sqlkit/column_example_test.go
go test -run Example -count=1 ./sqlkit
git add sqlkit/column_example_test.go
git commit -m "test: add sqlkit column integration examples"
```

### 작업 5: 업데이트 bilingual 공개 documentation

**파일:**
- Modify: `sqlkit/README.md`
- Modify: `sqlkit/README.ko.md`
- Modify: `encrypt/README.md`
- Modify: `encrypt/README.ko.md`

- [ ] **단계 1: 추가 the sqlkit 계약 in both locales**

Insert `JSON and Encrypted Columns` / `JSON 및 암호화 컬럼` 후 Usage 함께 the same selection rows:

| Stored value | Helper | SQL NULL | Non-NULL empty/null |
|---|---|---|---|
| JSON/JSONB | `JSONColumn[T]` | `Valid=false` | JSON `null` is valid |
| BYTEA/BLOB | `EncryptedBytesColumn` | `Valid=false` | empty/nil plaintext is encrypted |
| TEXT/VARCHAR | `EncryptedStringColumn` | `Valid=false` | empty string is encrypted |

Both locales must cover: 1 MiB JSON/plaintext defaults; 2 MiB ciphertext default; zero/custom/negative limit; source/AAD copy; failure clearing; both sqlkit sentinels; preserved encrypt sentinels; random ciphertext; non-searchability; direct/sqlkit/generated example.

- [ ] **단계 2: 추가 encrypt integration in both locales**

Insert `SQL Column Integration` / `SQL 컬럼 연동` 후 Associated Data. State exact storage formats, AAD context/copy, 호출자-owned key persistence/rotation/history, random-nonce query limitation, blind-index separation, 및 KMS non-goal.

- [ ] **단계 3: 실행 parity checks 및 commit**

```bash
rg -n 'JSONColumn|EncryptedBytesColumn|EncryptedStringColumn|DefaultJSONColumnMaxBytes|DefaultEncryptedColumnMaxPlaintextBytes|DefaultEncryptedColumnMaxCiphertextBytes|ErrInvalidColumnValue|ErrColumnValueTooLarge' sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md
go test -run Example -count=1 ./sqlkit
git diff --check -- sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md
git add sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md
git commit -m "docs: document sqlkit column helpers"
```

예상: identifiers/claims are source-equivalent, 한국어 is practical engineer-to-engineer prose, example PASS, diff clean.

### 작업 6: 업데이트 the helper 계약 map 함께 `bluetape-diagram`

**파일:**
- Modify: `docs/images/readme-diagrams/sqlkit-helper-contract-map.svg`
- Regenerate: `docs/images/readme-diagrams/sqlkit-helper-contract-map.png`

- [ ] **단계 1: Open sources 및 references**

Read implemented column code 및 both sqlkit README sections. Open full-size:

- Best-practices: `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/graph-graph-core-architecture-01.png`
- Repo-local: `docs/images/readme-diagrams/sqlkit-helper-contract-map.png`

Reader question: “Which responsibilities remain 호출자/database-owned, 및 which bounded transformations do column helpers own?”

- [ ] **단계 2: Edit exactly one SVG**

Expand to `1800x1150` 및 model these exact groups:

```text
Caller owned: *sql.DB / *sql.Tx, SQL, schema/migrations, keys/AAD
sqlkit control: WithTx, QueryAll/QueryOptional/QueryOne, Statement builders
Column boundary: JSONColumn[T], EncryptedBytesColumn, EncryptedStringColumn
Primitives: encoding/json, encrypt.Encryptor
Driver output: nil, []byte JSON, []byte BTENC envelope, raw URL-safe string
Non-goals: ORM metadata/hooks, key rotation/KMS, searchable encryption
```

사용 horizontal responsibility columns, 기존 palette/fonts/legend, 14x14 primary 및 10x10 static arrowheads, explained dashed dependencies, rounded orthogonal routes, 및 balanced margins.

- [ ] **단계 3: Render/audit/inspect 및 commit**

```bash
xmllint --noout docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
cairosvg docs/images/readme-diagrams/sqlkit-helper-contract-map.svg -o docs/images/readme-diagrams/sqlkit-helper-contract-map.png -s 2
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
git diff --check -- docs/images/readme-diagrams/sqlkit-helper-contract-map.svg docs/images/readme-diagrams/sqlkit-helper-contract-map.png
```

Open the final PNG full-size. Require readable text, balanced whitespace, explained connector styles, perpendicular endpoints, 없음 card intrusion/crossing, 없음 sharp mixed corners, 및 correct heads. A weak/zero generic count needs a targeted card/path invariant.

```bash
git add docs/images/readme-diagrams/sqlkit-helper-contract-map.svg docs/images/readme-diagrams/sqlkit-helper-contract-map.png
git commit -m "docs: update sqlkit helper contract map"
```

### 작업 7: 추가 the Scan/Value sequence 및 README embeds

**파일:**
- 생성: `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg`
- 생성: `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png`
- Modify: four 패키지 README files

- [ ] **단계 1: Open sequence references**

Open full-size:

- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workf낮음-sample.png`
- `docs/images/readme-diagrams/sqlkit-tx-query-sequence.png`

Read both column source files 및 the new README sections.

- [ ] **단계 2: 생성 the sequence SVG**

사용 `1800x1750`; participants: `Caller`, `Column helper`, `encoding/json or encrypt`, `database/sql driver`. Include four lifelines, activations, transparent alt frames, explicit per-color 16x16 heads, 및 14 visible ordered labels:

```text
1 set Data + Valid
2 invoke Value
3 resolve limits
4 alt invalid -> SQL NULL
5 marshal/encrypt
6 alt callback/crypto/size error -> safe error
7 return []byte JSON / []byte BTENC / base64 string
8 driver executes
9 driver returns nil/string/[]byte
10 Scan clears state
11 copy and stored-size check
12 unmarshal/decrypt
13 alt malformed/auth/plaintext-size -> remain cleared
14 publish Data + Valid
```

Muted blue=call, olive=success, amber=encode/decode, teal=return, muted red=오류. Line, marker, label, 및 badge colors must match.

- [ ] **단계 3: Render/audit/inspect**

```bash
xmllint --noout docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
cairosvg docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg -o docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png -s 2
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
```

Inspect full-size 후 the last coordinate change. Require 4 participants/lifelines, activations, 14 visible labels, transparent frames, continuous lanes, 없음 overlap, palette/marker parity, 및 clear footer whitespace.

- [ ] **단계 4: Embed 및 commit**

Embed the same image in 모든 four README files:

```markdown
![sqlkit column Scan and Value sequence](../docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png)
```

추가 these source-equivalent sentences beside the embed:

- 영문: “Scan clears the previous value 전에 decoding, so malformed 또는 unauthenticated input cannot leave stale plaintext. Value returns nil, JSON bytes, a binary BTENC envelope, 또는 raw URL-safe text according to the concrete column type.”
- 한국어: “Scan은 디코딩 전에 이전 값을 지우므로 잘못된 입력이나 인증에 실패한 입력이 오래된 평문을 남기지 않습니다. Value는 구체 컬럼 타입에 따라 nil, JSON 바이트, 바이너리 BTENC envelope 또는 패딩 없는 URL-safe base64 문자열을 반환합니다.”

```bash
test "$(rg -l 'sqlkit-column-scan-value-sequence.png' sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md | wc -l | tr -d ' ')" = 4
git diff --check -- sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png
git add sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png
git commit -m "docs: add sqlkit column lifecycle diagram"
```

### 작업 8: 실행 authoritative verification 및 7-Tier review

**파일:**
- 리뷰 `8b6714d..HEAD`
- Modify 만 files needed to repair findings

- [ ] **단계 1: 실행 focused proof**

```bash
git diff --check 8b6714d..HEAD
make fmt-check
make tidy-check
make vet
go test -count=1 ./sqlkit ./encrypt
go test -run Example -count=1 ./sqlkit
go test -race -count=1 ./sqlkit ./encrypt
```

All exit codes must be 0. Missing process/exit evidence requires a fresh rerun.

- [ ] **단계 2: 실행 authoritative gate**

```bash
make ci
```

Expected exit 0. If lint cites a deleted worktree, run `golangci-lint cache clean && make lint` 및 rerun `make ci` from scratch.

- [ ] **단계 3: Perform the 7-Tier review**

리뷰 six lanes plus main integration:

- 성능: pre-decode/decrypt limits, bounded copies;
- 안정성: NULL reuse, failure clearing, callback panic, method sets, zero state;
- 보안: nonce/AAD/key ownership, redaction, tamper/wrong key/AAD;
- Ops: defaults/storage types/KMS-query-migration boundaries;
- 개발자/API: Go docs, 오류.Is/As, three integration shapes, 없음 ORM abstraction;
- 사용자/호출자: locale parity, selection rules, null/empty semantics, diagram readability;
- integration: imports, scope, 테스트/CI, diagram ledger, final verdict.

Repair P0/P1, rerun downstream evidence, 및 record exact `P0=0 P1=0`. 커밋 만 real repairs 함께 `fix: harden sqlkit column contracts`; do 아님 create an empty commit.

### 작업 9: 생성 및 verify the PR, then stop 전에 merge

- [ ] **단계 1: 검증 및 push**

```bash
git status --short
git log --oneline --decorate 8b6714d^..HEAD
git push -u origin feat/issue-530-sqlkit-columns
```

- [ ] **단계 2: 생성 the PR**

Target `develop`, assign `debop`, mirror issue #530 milestone `0.19.0` 및 relevant labels, include `Closes #530`. 영문 body sections: Why, What changed, 검증, diagrams/docs, risks/non-goals, 함께 final Markdown heading `## DoD Status`.

- [ ] **단계 3: 검증 live metadata/body/checks**

```bash
pr_number="$(gh pr view --json number --jq .number)"
gh pr view "$pr_number" --json number,url,state,baseRefName,headRefName,assignees,labels,milestone,body,reviews,statusCheckRollup
```

Confirm the live body is non-empty 및 its final `##` heading is `## DoD Status`. Monitor CI to conclusion, then reread reviews 및 unresolved threads.

- [ ] **단계 4: Stop at the authorized boundary**

Report PR URL, CI, review/thread state, commit range, diagram ledger, `P0=0 P1=0`, 및 DoD counts. 다음을 하지 않는다: merge, sync `develop`, delete the branch, 또는 remove the worktree until explicit post-CI approval.

## 최종 인수 원장

- [ ] Three Scanner/Valuer method sets compile.
- [ ] SQL NULL, JSON null, empty/nil bytes, 및 empty string semantics are tested.
- [ ] Limits, copies, failure clearing, panic containment, random ciphertext, 및 redacted inspectable 오류 are proven.
- [ ] Direct SQL, sqlkit, 및 generated-query example compile.
- [ ] sqlkit/encrypt locale pairs are source-equivalent.
- [ ] Both diagrams have paired SVG/PNG, audit ledger, 및 full-size inspection.
- [ ] Targeted 테스트, example, race, format, tidy, vet, lint, 및 `make ci` pass freshly.
- [ ] Final review reports `P0=0 P1=0`.
- [ ] PR metadata/body/checks/reviews are verified live 및 execution stops 전에 merge.
