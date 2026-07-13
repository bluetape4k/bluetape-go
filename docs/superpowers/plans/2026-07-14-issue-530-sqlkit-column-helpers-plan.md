# Issue #530 sqlkit Column Helpers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bounded JSON and encrypted byte/string `database/sql.Scanner` and `driver.Valuer` helpers to `sqlkit`, with compile-checked examples, bilingual documentation, and verified SVG/PNG diagrams.

**Architecture:** Keep three purpose-built values in root `sqlkit`. `JSONColumn[T]` owns typed JSON and SQL NULL state; `EncryptedBytesColumn` and `EncryptedStringColumn` compose with a caller-owned immutable `encrypt.Encryptor` and copied associated data. A private error/limit layer preserves `errors.Is` without exposing payloads, and every `Scan` clears stale state before work and publishes only after complete validation.

**Tech Stack:** Go 1.26, `database/sql`, `database/sql/driver`, `encoding/json`, existing `encrypt` package, table-driven tests, `bluetape-writer`, `bluetape-diagram`, CairoSVG.

---

## Execution constraints

- Work only in `/Users/debop/work/bluetape4k/bluetape-go/.worktrees/feat-issue-530-sqlkit-columns` on `feat/issue-530-sqlkit-columns`.
- Design authority: `docs/superpowers/specs/2026-07-13-issue-530-sqlkit-column-helpers-design.md` at `8b6714d`.
- Load `test-driven-development` before production edits and `verification-before-completion` before delivery claims.
- No new module, dependency, schema, migration, ORM hook, deterministic encryption, KMS, money/measure helper, benchmark, or workflow YAML.
- Use `apply_patch` for edits. Run heavyweight checks serially.
- Public Go docs, English README, commits, PR text, and diagram labels are English. Keep `README.ko.md` source-equivalent and natural Korean.
- Create one diagram asset at a time; rendered PNG evidence is authoritative.

## File map

| Path | Responsibility |
|---|---|
| `sqlkit/column_errors.go` | Sentinels, safe wrapper, source-copy, limits, panic conversion |
| `sqlkit/json_column.go` | `JSONColumn[T]` Scanner/Valuer |
| `sqlkit/json_column_test.go` | JSON behavior and hardening |
| `sqlkit/encrypted_column.go` | Encrypted byte/string types and constructors |
| `sqlkit/encrypted_column_test.go` | Encryption behavior and hardening |
| `sqlkit/column_example_test.go` | Direct SQL, sqlkit, generated-query examples |
| `sqlkit/README.md`, `sqlkit/README.ko.md` | Public column contract |
| `encrypt/README.md`, `encrypt/README.ko.md` | SQL integration and crypto boundaries |
| `docs/images/readme-diagrams/sqlkit-helper-contract-map.svg/.png` | Updated static responsibility map |
| `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg/.png` | New Scan/Value lifecycle |

### Task 1: Implement bounded JSON columns

**Files:**
- Create: `sqlkit/column_errors.go`
- Create: `sqlkit/json_column.go`
- Create: `sqlkit/json_column_test.go`

- [ ] **Step 1: Write the failing JSON tests**

Create external-package tests with these fixtures:

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

Add these exact tests and assertions:

| Test | Input | Required assertion |
|---|---|---|
| `TestJSONColumnRoundTrip` | valid `jsonProfile{Name:"Ada"}` | `Value` is `[]byte`; `Scan` returns equal data and `Valid=true` |
| `TestJSONColumnDistinguishesSQLNullAndJSONNull` | `nil` then `[]byte("null")` into `JSONColumn[*jsonProfile]` | SQL NULL is invalid; JSON null is valid with nil `Data` and re-encodes to `null` |
| `TestJSONColumnClearsStateOnFailure` | malformed JSON, trailing token, `int64` source | errors match `ErrInvalidColumnValue`; old data is zeroed and `Valid=false` |
| `TestJSONColumnEnforcesLimits` | source/output above 4 bytes and negative limit | oversize matches `ErrColumnValueTooLarge`; negative matches `ErrInvalidColumnValue` |
| `TestJSONColumnCopiesDriverSource` | `retainingJSON` with mutable `[]byte` source | mutating source after Scan cannot change retained data |
| `TestJSONColumnContainsCallbackPanics` | panicking unmarshal/marshal | no panic; error matches `ErrInvalidColumnValue` and omits both secret markers |
| `TestJSONColumnNilScanner` | nil `*JSONColumn[jsonProfile]` | returns `ErrInvalidColumnValue` |

Use table-driven subtests for malformed/unsupported/limit cases and require default limit constants to equal `1 << 20`.

- [ ] **Step 2: Run RED**

```bash
go test -count=1 ./sqlkit -run '^TestJSONColumn'
```

Expected: compile failure naming undefined `JSONColumn`, `ErrInvalidColumnValue`, and `ErrColumnValueTooLarge`.

- [ ] **Step 3: Implement safe shared errors and limits**

Create `sqlkit/column_errors.go`:

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

- [ ] **Step 4: Implement `JSONColumn[T]`**

Create `sqlkit/json_column.go` with `DefaultJSONColumnMaxBytes = 1 << 20`, fields `Data T`, `Valid bool`, `MaxBytes int`, and English Go docs. Implement this exact order:

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

- [ ] **Step 5: Run GREEN and commit**

```bash
gofmt -w sqlkit/column_errors.go sqlkit/json_column.go sqlkit/json_column_test.go
go test -count=1 ./sqlkit -run '^TestJSONColumn'
go test -count=1 ./sqlkit
git add sqlkit/column_errors.go sqlkit/json_column.go sqlkit/json_column_test.go
git commit -m "feat: add bounded JSON column helper"
```

Expected: all tests PASS and the commit succeeds.

### Task 2: Implement encrypted byte columns

**Files:**
- Create: `sqlkit/encrypted_column.go`
- Create: `sqlkit/encrypted_column_test.go`

- [ ] **Step 1: Write failing byte-column tests**

Use a fixed 32-byte test key only in tests:

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

Add exact test coverage:

- `TestEncryptedBytesColumnRoundTripNullAndStorageType`: `Value` is `[]byte`; Scan restores plaintext; Scan(nil) clears; valid nil/empty plaintext remains non-NULL.
- `TestEncryptedBytesColumnUsesRandomCiphertext`: two Value calls differ and both decrypt.
- `TestEncryptedBytesColumnPreservesEncryptErrors`: malformed, tamper, wrong key, wrong AAD preserve the matching `encrypt` sentinel and `ErrInvalidColumnValue`.
- `TestEncryptedBytesColumnEnforcesLimits`: negative configuration, oversized stored source, decrypted plaintext, input plaintext, and encrypted output return the correct sqlkit sentinel.
- `TestEncryptedBytesColumnClearsPlaintextOnFailure`: all failure branches leave nil `Data` and `Valid=false`.
- `TestEncryptedBytesColumnCopiesAADAndSource`: mutate caller AAD after construction and driver bytes after Scan; behavior remains bound to the original copies.
- `TestEncryptedBytesColumnRedactsErrors`: marker plaintext/ciphertext/key/AAD never appears in `Error()`.
- `TestEncryptedBytesColumnZeroValue`: invalid Value is SQL NULL; non-NULL Scan/Value preserves `encrypt.ErrInvalidKey` without panic.
- `TestEncryptedBytesColumnNilScanner`: nil receiver returns `ErrInvalidColumnValue`.

- [ ] **Step 2: Run RED**

```bash
go test -count=1 ./sqlkit -run '^TestEncryptedBytesColumn'
```

Expected: compile failure for undefined encrypted byte APIs.

- [ ] **Step 3: Implement the byte type**

Create `sqlkit/encrypted_column.go` with:

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

Add English Go docs and these complete methods:

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

- [ ] **Step 4: Run GREEN and commit**

```bash
gofmt -w sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
go test -count=1 ./sqlkit -run '^TestEncryptedBytesColumn'
go test -count=1 ./sqlkit ./encrypt
git add sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
git commit -m "feat: add encrypted byte column helper"
```

### Task 3: Implement encrypted string columns

**Files:**
- Modify: `sqlkit/encrypted_column.go`
- Modify: `sqlkit/encrypted_column_test.go`

- [ ] **Step 1: Write failing string tests**

Add interface assertions and these tests:

- `TestEncryptedStringColumnRoundTripNullAndStorageType`: Value is `string`; both string/`[]byte` Scan work; nil clears.
- `TestEncryptedStringColumnEmptyPlaintextIsNotSQLNull`: empty valid string encrypts and scans back as `Valid=true`.
- `TestEncryptedStringColumnPreservesMalformedAuthenticationAndUTF8Errors`: malformed base64, tamper, wrong key/AAD, and encrypted invalid UTF-8 preserve `encrypt` sentinels.
- `TestEncryptedStringColumnEnforcesLimitsAndClearsState`: all source/plaintext/output/negative cases return the correct sentinel and clear old text.
- `TestEncryptedStringColumnCopiesAADAndRedactsErrors`.
- `TestEncryptedStringColumnZeroValueAndNilScanner`.

- [ ] **Step 2: Run RED**

```bash
go test -count=1 ./sqlkit -run '^TestEncryptedStringColumn'
```

Expected: compile failure for undefined string APIs.

- [ ] **Step 3: Implement the string type**

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

Add English Go docs and these complete methods:

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

- [ ] **Step 4: Run GREEN, targeted race, and commit**

```bash
gofmt -w sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
go test -count=1 ./sqlkit -run '^TestEncrypted(String|Bytes)Column'
go test -count=1 ./sqlkit ./encrypt
go test -race -count=1 ./sqlkit ./encrypt
git add sqlkit/encrypted_column.go sqlkit/encrypted_column_test.go
git commit -m "feat: add encrypted string column helper"
```

Expected: every command PASS. Race is required because immutable Encryptor sharing is part of the composed contract.

### Task 4: Add compile-checked integration examples

**Files:**
- Create: `sqlkit/column_example_test.go`

- [ ] **Step 1: Add three compile-only examples**

Implement:

- `ExampleJSONColumn_databaseSQL`: local closures using `QueryRowContext(...).Scan(&column)` and `ExecContext(..., column)`.
- `ExampleEncryptedBytesColumn_sqlkit`: `QueryOne` mapper constructs the byte column, calls `rows.Scan(&column)`, and returns a copied plaintext.
- `ExampleEncryptedStringColumn_generatedQuery`: a local generated-query interface takes a params struct whose payload is `driver.Valuer`; construct/set/pass the column.

Do not call a nil database. Assign closures to `_` and omit Output comments so the examples compile without executing external I/O.

- [ ] **Step 2: Verify and commit**

```bash
gofmt -w sqlkit/column_example_test.go
go test -run Example -count=1 ./sqlkit
git add sqlkit/column_example_test.go
git commit -m "test: add sqlkit column integration examples"
```

### Task 5: Update bilingual public documentation

**Files:**
- Modify: `sqlkit/README.md`
- Modify: `sqlkit/README.ko.md`
- Modify: `encrypt/README.md`
- Modify: `encrypt/README.ko.md`

- [ ] **Step 1: Add the sqlkit contract in both locales**

Insert `JSON and Encrypted Columns` / `JSON 및 암호화 컬럼` after Usage with the same selection rows:

| Stored value | Helper | SQL NULL | Non-NULL empty/null |
|---|---|---|---|
| JSON/JSONB | `JSONColumn[T]` | `Valid=false` | JSON `null` is valid |
| BYTEA/BLOB | `EncryptedBytesColumn` | `Valid=false` | empty/nil plaintext is encrypted |
| TEXT/VARCHAR | `EncryptedStringColumn` | `Valid=false` | empty string is encrypted |

Both locales must cover: 1 MiB JSON/plaintext defaults; 2 MiB ciphertext default; zero/custom/negative limit; source/AAD copy; failure clearing; both sqlkit sentinels; preserved encrypt sentinels; random ciphertext; non-searchability; direct/sqlkit/generated examples.

- [ ] **Step 2: Add encrypt integration in both locales**

Insert `SQL Column Integration` / `SQL 컬럼 연동` after Associated Data. State exact storage formats, AAD context/copy, caller-owned key persistence/rotation/history, random-nonce query limitation, blind-index separation, and KMS non-goal.

- [ ] **Step 3: Run parity checks and commit**

```bash
rg -n 'JSONColumn|EncryptedBytesColumn|EncryptedStringColumn|DefaultJSONColumnMaxBytes|DefaultEncryptedColumnMaxPlaintextBytes|DefaultEncryptedColumnMaxCiphertextBytes|ErrInvalidColumnValue|ErrColumnValueTooLarge' sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md
go test -run Example -count=1 ./sqlkit
git diff --check -- sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md
git add sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md
git commit -m "docs: document sqlkit column helpers"
```

Expected: identifiers/claims are source-equivalent, Korean is practical engineer-to-engineer prose, examples PASS, diff clean.

### Task 6: Update the helper contract map with `bluetape-diagram`

**Files:**
- Modify: `docs/images/readme-diagrams/sqlkit-helper-contract-map.svg`
- Regenerate: `docs/images/readme-diagrams/sqlkit-helper-contract-map.png`

- [ ] **Step 1: Open sources and references**

Read implemented column code and both sqlkit README sections. Open full-size:

- Best-practices: `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/graph-graph-core-architecture-01.png`
- Repo-local: `docs/images/readme-diagrams/sqlkit-helper-contract-map.png`

Reader question: “Which responsibilities remain caller/database-owned, and which bounded transformations do column helpers own?”

- [ ] **Step 2: Edit exactly one SVG**

Expand to `1800x1150` and model these exact groups:

```text
Caller owned: *sql.DB / *sql.Tx, SQL, schema/migrations, keys/AAD
sqlkit control: WithTx, QueryAll/QueryOptional/QueryOne, Statement builders
Column boundary: JSONColumn[T], EncryptedBytesColumn, EncryptedStringColumn
Primitives: encoding/json, encrypt.Encryptor
Driver output: nil, []byte JSON, []byte BTENC envelope, raw URL-safe string
Non-goals: ORM metadata/hooks, key rotation/KMS, searchable encryption
```

Use horizontal responsibility columns, existing palette/fonts/legend, 14x14 primary and 10x10 static arrowheads, explained dashed dependencies, rounded orthogonal routes, and balanced margins.

- [ ] **Step 3: Render/audit/inspect and commit**

```bash
xmllint --noout docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
cairosvg docs/images/readme-diagrams/sqlkit-helper-contract-map.svg -o docs/images/readme-diagrams/sqlkit-helper-contract-map.png -s 2
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/sqlkit-helper-contract-map.svg
git diff --check -- docs/images/readme-diagrams/sqlkit-helper-contract-map.svg docs/images/readme-diagrams/sqlkit-helper-contract-map.png
```

Open the final PNG full-size. Require readable text, balanced whitespace, explained connector styles, perpendicular endpoints, no card intrusion/crossing, no sharp mixed corners, and correct heads. A weak/zero generic count needs a targeted card/path invariant.

```bash
git add docs/images/readme-diagrams/sqlkit-helper-contract-map.svg docs/images/readme-diagrams/sqlkit-helper-contract-map.png
git commit -m "docs: update sqlkit helper contract map"
```

### Task 7: Add the Scan/Value sequence and README embeds

**Files:**
- Create: `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg`
- Create: `docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png`
- Modify: four package README files

- [ ] **Step 1: Open sequence references**

Open full-size:

- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png`
- `docs/images/readme-diagrams/sqlkit-tx-query-sequence.png`

Read both column source files and the new README sections.

- [ ] **Step 2: Create the sequence SVG**

Use `1800x1750`; participants: `Caller`, `Column helper`, `encoding/json or encrypt`, `database/sql driver`. Include four lifelines, activations, transparent alt frames, explicit per-color 16x16 heads, and 14 visible ordered labels:

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

Muted blue=call, olive=success, amber=encode/decode, teal=return, muted red=error. Line, marker, label, and badge colors must match.

- [ ] **Step 3: Render/audit/inspect**

```bash
xmllint --noout docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
cairosvg docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg -o docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png -s 2
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg
```

Inspect full-size after the last coordinate change. Require 4 participants/lifelines, activations, 14 visible labels, transparent frames, continuous lanes, no overlap, palette/marker parity, and clear footer whitespace.

- [ ] **Step 4: Embed and commit**

Embed the same image in all four README files:

```markdown
![sqlkit column Scan and Value sequence](../docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png)
```

Add these source-equivalent sentences beside the embed:

- English: “Scan clears the previous value before decoding, so malformed or unauthenticated input cannot leave stale plaintext. Value returns nil, JSON bytes, a binary BTENC envelope, or raw URL-safe text according to the concrete column type.”
- Korean: “Scan은 디코딩 전에 이전 값을 지우므로 잘못된 입력이나 인증에 실패한 입력이 오래된 평문을 남기지 않습니다. Value는 구체 컬럼 타입에 따라 nil, JSON 바이트, 바이너리 BTENC envelope 또는 패딩 없는 URL-safe base64 문자열을 반환합니다.”

```bash
test "$(rg -l 'sqlkit-column-scan-value-sequence.png' sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md | wc -l | tr -d ' ')" = 4
git diff --check -- sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png
git add sqlkit/README.md sqlkit/README.ko.md encrypt/README.md encrypt/README.ko.md docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.svg docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png
git commit -m "docs: add sqlkit column lifecycle diagram"
```

### Task 8: Run authoritative verification and 7-Tier review

**Files:**
- Review `8b6714d..HEAD`
- Modify only files needed to repair findings

- [ ] **Step 1: Run focused proof**

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

- [ ] **Step 2: Run authoritative gate**

```bash
make ci
```

Expected exit 0. If lint cites a deleted worktree, run `golangci-lint cache clean && make lint` and rerun `make ci` from scratch.

- [ ] **Step 3: Perform the 7-Tier review**

Review six lanes plus main integration:

- performance: pre-decode/decrypt limits, bounded copies;
- stability: NULL reuse, failure clearing, callback panic, method sets, zero state;
- security: nonce/AAD/key ownership, redaction, tamper/wrong key/AAD;
- Ops: defaults/storage types/KMS-query-migration boundaries;
- developer/API: Go docs, errors.Is/As, three integration shapes, no ORM abstraction;
- user/caller: locale parity, selection rules, null/empty semantics, diagram readability;
- integration: imports, scope, tests/CI, diagram ledger, final verdict.

Repair P0/P1, rerun downstream evidence, and record exact `P0=0 P1=0`. Commit only real repairs with `fix: harden sqlkit column contracts`; do not create an empty commit.

### Task 9: Create and verify the PR, then stop before merge

- [ ] **Step 1: Verify and push**

```bash
git status --short
git log --oneline --decorate 8b6714d^..HEAD
git push -u origin feat/issue-530-sqlkit-columns
```

- [ ] **Step 2: Create the PR**

Target `develop`, assign `debop`, mirror issue #530 milestone `0.19.0` and relevant labels, include `Closes #530`. English body sections: Why, What changed, Validation, diagrams/docs, risks/non-goals, with final Markdown heading `## DoD Status`.

- [ ] **Step 3: Verify live metadata/body/checks**

```bash
pr_number="$(gh pr view --json number --jq .number)"
gh pr view "$pr_number" --json number,url,state,baseRefName,headRefName,assignees,labels,milestone,body,reviews,statusCheckRollup
```

Confirm the live body is non-empty and its final `##` heading is `## DoD Status`. Monitor CI to conclusion, then reread reviews and unresolved threads.

- [ ] **Step 4: Stop at the authorized boundary**

Report PR URL, CI, review/thread state, commit range, diagram ledger, `P0=0 P1=0`, and DoD counts. Do not merge, sync `develop`, delete the branch, or remove the worktree until explicit post-CI approval.

## Final acceptance ledger

- [ ] Three Scanner/Valuer method sets compile.
- [ ] SQL NULL, JSON null, empty/nil bytes, and empty string semantics are tested.
- [ ] Limits, copies, failure clearing, panic containment, random ciphertext, and redacted inspectable errors are proven.
- [ ] Direct SQL, sqlkit, and generated-query examples compile.
- [ ] sqlkit/encrypt locale pairs are source-equivalent.
- [ ] Both diagrams have paired SVG/PNG, audit ledger, and full-size inspection.
- [ ] Targeted tests, examples, race, format, tidy, vet, lint, and `make ci` pass freshly.
- [ ] Final review reports `P0=0 P1=0`.
- [ ] PR metadata/body/checks/reviews are verified live and execution stops before merge.
