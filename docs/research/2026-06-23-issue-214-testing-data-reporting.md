# Issue #214 testing data 및 reporting 연구

Date: 2026-06-23
Milestone: 0.6.4
Issue: #214
Parent: #209

## 결정

0.6.4에서는 faker, random-data, parameter-source, Mermaid reporting dependency를
추가하지 않는다.

Go testing surface는 table-driven test, fuzz test, compile-checked example,
fixture, golden file, 작은 deterministic data builder 중심으로 유지한다. faker
dependency는 #222로 defer하고, focused assertion test-data와 fixture example이
구체적인 consumer를 증명한 뒤 다시 판단한다.

## Acceptance Criteria 대응

| Requirement | Decision |
|---|---|
| Compare standard table tests, fuzzing, fixtures, and small data builders before dependencies. | Prefer standard table tests, `testing.F` fuzz targets, package fixtures, examples, and deterministic builders. |
| Faker/random dependency recommendation includes maintenance, license, determinism, and CI notes. | No dependency now. Candidate packages are documented below with maintenance/license/determinism risks for later review. |
| Reporting recommendation explains `go test -json` fit. | Keep `go test -json` as the machine-readable source; do not generate Mermaid as a library feature. |
| Include `testing/assertions`, random/faker support, parameter sources, mock web servers, Spring/Ktor test data patterns. | Assertions/test-data examples route to #222; HTTP/mock servers route to #219/#224; Spring/Ktor-style fixtures become typed Go builders, not reflection injection. |
| Decide later package needs. | Database/audit/graph/AWS/golden needs should start with deterministic builders and checked-in fixtures; randomized text/token data remains opt-in research only. |

## Go 기준선

- Table-driven test가 이미 repository의 주된 형태이며, helper API 없이도
  parameter-source 요구를 충족한다.
- Go fuzz target(`func FuzzXxx(f *testing.F)`)은 example이나 table만으로 너무
  좁은 parser, codec, wildcard, boundary-input coverage에 맞는 형태다.
- Compile-checked example은 이미 caller-facing documentation과 exact-output
  verification을 제공한다.
- `go test -json`은 verbose test output과 같은 정보를 machine-readable format으로
  방출한다. Downstream reporting은 custom test runner를 추가하기보다 이 stream을
  소비해야 한다.
- `testing.T.TempDir`, `testing.T.Setenv`, #212의 scoped helper면 temp output,
  env restoration, golden-file write target에 충분하다.

## 후보 Dependency Snapshot

Live metadata was collected with `gh repo view` and `go list -m -versions` on
2026-06-23.

| Package | License | Activity / version signal | Determinism and CI notes | Decision |
|---|---|---:|---|---|
| `github.com/brianvoe/gofakeit/v7` | MIT | Active repo; latest observed module line `v7.15.0`; zero dependencies; supports seeded faker instances and custom random sources. | Best candidate if a future package needs broad realistic text/domain data. Use local `Faker` instances only; avoid global seeding in parallel tests. | Defer to #222. |
| `github.com/go-faker/faker/v4` | MIT | Active repo; latest observed module line `v4.8.0`; struct-tag oriented. | Reflection/tag surface is broader than current needs, can panic on unsupported/private fields, and makes fixtures less explicit. | Reject for 0.6.4. |
| `github.com/jaswdr/faker/v2` | MIT | Active repo; latest observed module line `v2.9.1`; zero-dependency faker-style API. | Useful for ad hoc realistic values but includes APIs that can create image/temp files and broad random behavior; determinism must be wrapped carefully. | Defer. |
| `github.com/Pallinder/go-randomdata` | MIT | Older release line `v1.2.0`; last push observed in 2023. | Small API, but weaker maintenance signal and less explicit determinism than local builders. | Reject. |

## Parameter Source

JUnit-style field-source와 parameter-source API는 generic Go helper로 port하지 않는다.
Go에서는 table literal이 더 단순하고, type-checked이며, 이름 붙이기 쉽고,
subtest와도 잘 합성된다.

권장 패턴:

```go
tests := []struct {
	name string
	in   string
	want string
}{
	{name: "empty", in: "", want: ""},
	{name: "trim", in: " value ", want: "value"},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		// exercise behavior
	})
}
```

Generics helper는 한 package 안의 반복 assertion loop를 제거할 때만 허용한다.
최소 세 package에서 같은 typed pattern이 반복되고 table literal이 더 이상 명확하지
않다는 증거가 나오기 전에는 public `testing` parameter-source API를 추가하지 않는다.

## Test Data Builder

dependency보다 example을 먼저 추가한다. #222 follow-up은 다음 local deterministic
builder에서 시작해야 한다.

- stable ID, timestamp, 작고 읽기 쉬운 이름을 가진 database/audit/graph domain fixture
- fuzz target 또는 parser boundary가 더 다양한 input을 필요로 할 때만 쓰는 randomized
  text/token data
- emulator integration보다 먼저 checked-in JSON 또는 typed builder로 제공하는 AWS payload fixture
- path는 #212 `TempOutputPath`로 만들되 canonical expected file은 package-local
  `testdata`에 유지하는 golden-file helper

## Mock Web Server 및 Spring/Ktor Pattern

Spring/Ktor test data pattern은 Go에서 explicit builder, `httptest.Server` fixture,
package-local client/server helper로 매핑한다. 0.6.4에서는 WireMock-style general
dependency를 추가하지 않는다.

HTTP mock과 fault-injection work는 이미 #219에 속하고, integration recipe
documentation은 #224에 속한다. shared mock server wrapper가 real package consumer로
정당화되는지는 해당 issue들이 결정해야 한다.

## Reporting

Reporting은 public `testing` helper API 밖에 둔다.

- structured event stream에는 `go test -json`을 사용한다.
- coverage와 package summary는 기존 CI/doc tooling으로 저장한다.
- Mermaid/timeline output은 향후 issue가 가치를 증명하면 external script 또는 docs
  artifact가 될 수 있지만, library dependency나 test runner가 되면 안 된다.

`go test -json`을 변환하는 reporting helper는 package, test, action, elapsed time,
output, failure text를 보존해야 한다. diagram-only view는 merge evidence로 충분하지
않다.

## 후속 매핑

| Need | Follow-up |
|---|---|
| Focused assertion test-data and deterministic fixture examples | #222 |
| HTTP mock and fault-injection Testcontainers wrappers | #219 |
| End-to-end integration recipes for corrected 0.6.x packages | #224 |
| Generic parameter-source public API | Non-goal until repeated need is proven. |
| Mermaid/timeline reporting library | Non-goal; use `go test -json` consumers if needed. |

## 출처

- #214 issue requirements.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`.
- Go tool help for `go test -json` and fuzz/example test functions.
- `https://github.com/brianvoe/gofakeit`
- `https://github.com/go-faker/faker`
- `https://github.com/jaswdr/faker`
- `https://github.com/Pallinder/go-randomdata`
