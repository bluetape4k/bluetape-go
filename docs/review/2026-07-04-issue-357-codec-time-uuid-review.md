# Issue #357 Codec Time UUID Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-04

Scope:

- `codec/url62_uuid.go`
- `codec/codec_test.go`
- `codec/codec_example_test.go`
- `codec/doc.go`
- `codec/README.md`
- `codec/README.ko.md`
- `id/README.md`
- `id/README.ko.md`

## 증거

- Issue #357 asks for URL62, time, and UUID parity gaps from
  `bluetape4k-core` without importing JVM DSL shape.
- Kotlin source references reviewed:
  - `codec/Base58.kt`
  - `codec/Base62.kt`
  - `codec/Url62.kt`
  - `support/UuidSupport.kt`
  - `javatimes/Quarter.kt`
  - `javatimes/YearQuarter.kt`
  - `javatimes/DurationSupport.kt`
- Current Go package boundaries reviewed:
  - `codec` already owns byte/string encoders and URL62 as a Base62 byte alias.
  - `core` already owns UUID text validation and small `Quarter`/`YearQuarter`
    time helpers.
  - `id` owns UUID/ULID/KSUID/Snowflake generation.
- Time helpers already cover the idiomatic Go subset. Broad duration
  parser/formatter wrappers and Java-time DSL aliases remain non-goals.

## 7-Tier 관점

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Initial P1 found unbounded pre-decode work for oversized UUID URL62 input. Fixed by rejecting strings longer than the 22-character 128-bit Base62 UUID bound before decode. |
| Stability | Pass | UUID URL62 helpers validate canonical UUID text on encode, reject blank/malformed/oversized decode input, and wrap invalid caller input with `core.ErrInvalidArgument`. |
| Security | Pass | Initial P1 found non-canonical compact aliases such as `00` and `01`. Fixed by re-encoding decoded UUIDs and rejecting non-canonical URL62 text. |
| Operator/Ops | Pass | No runtime configuration, logging, goroutine, or external service behavior changed. Error messages identify encode/decode direction and invalid value class. |
| Developer/API | Pass | Compact UUID URL62 rendering lives in `codec`; UUID generation remains in `id`; UUID text validation remains in `core`. No `google/uuid.UUID` concrete type is exposed. |
| User/Caller | Pass | README pairs and executable examples document Kotlin `Url62` compatibility, high-order zero normalization, and the byte-helper divergence. |
| Integration | Pass | Full local gate passed after the P1 fixes. |

## 검증

- `git diff --check`: PASS
- `go test -count=1 ./codec`: PASS
- `go test -race -count=1 ./codec`: PASS
- `go test -count=1 ./core ./codec ./id`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `make vet`: PASS
- `make lint`: PASS
- `make test`: PASS
- `make race`: PASS

## 발견 사항

- P0: 0
- P1: 0

## 잔여 위험

The UUID URL62 helpers intentionally normalize compact UUID text as numeric
128-bit values. Callers that need arbitrary byte payload round trips should keep
using `EncodeURL62` and `DecodeURL62`, which preserve leading zero bytes.
