# Issue #205 Foundation Contract Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden existing `core`, `collections`, `codec`, and `serialization` contracts before `0.6.3` adds new foundation parity APIs.

**Architecture:** Keep byte APIs binary and validate text APIs that can return errors. Add one shared `core.ErrInvalidUTF8` sentinel for invalid text contracts, use it from `core`, `codec`, and `serialization`, and prove the distinction with RED tests plus README notes. Existing no-error `Encode*String` helpers remain compatibility string-to-byte conversions and are documented as non-validating.

**Tech Stack:** Go 1.x, standard library only, existing `go test`, `make ci`, `git diff --check`.

---

## API Decision

Use `core.ErrInvalidUTF8` as the shared exported sentinel for invalid text
contracts. This intentionally makes `codec` and `serialization` depend on the
small `core` package for a common caller-visible error contract.

Rationale:

- Callers can use one stable check: `errors.Is(err, core.ErrInvalidUTF8)`.
- `core` already owns string helper behavior, and `codec` / `serialization`
  are foundation packages above it in this repository.
- This change must not introduce an import cycle. Final verification includes:

```bash
go list -deps ./codec ./serialization | rg '^github.com/bluetape4k/bluetape-go/core$'
go test -count=1 ./core ./collections ./codec ./serialization
```

No-error `Encode*String` helpers remain source-compatible string-to-byte
convenience wrappers. They cannot report invalid UTF-8 without changing their
signatures, so this plan documents them as non-validating and validates only
text APIs that can return errors.

## File Structure

- Create: `core/errors.go`
  - Exports `ErrInvalidUTF8` as the common text-contract sentinel.
- Modify: `core/string.go`
  - Reject invalid UTF-8 before truncation and document the sentinel in Go doc comments.
- Modify: `core/string_test.go`, `core/validate_test.go`, `core/number_test.go`
  - Add RED boundary tests for invalid UTF-8, zero/rune truncation, validation ranges, and hex spacing.
- Create: `codec/text.go`
  - Adds an unexported helper that converts decoded bytes to string only after UTF-8 validation.
- Modify: `codec/base58.go`, `codec/base62.go`, `codec/base64.go`, `codec/hex.go`
  - Route string decoders through the helper and update exported Go doc comments.
- Modify: `codec/codec_test.go`
  - Add invalid UTF-8 sentinel, malformed-input sentinel separation, Base64URL malformed input, hex invalid character, and binary-byte acceptance tests.
- Modify: `codec/codec_example_test.go`
  - Add migration examples for `errors.Is(err, core.ErrInvalidUTF8)` and byte helper fallback.
- Modify: `serialization/raw.go`
  - Validate `StringSerializer` marshal/unmarshal text with `core.ErrInvalidUTF8` and document it in Go doc comments.
- Modify: `serialization/serialization_test.go`
  - Add invalid UTF-8 sentinel, `BytesSerializer` binary/empty acceptance, nil input, empty string, and versioned format metadata tests.
- Modify: `serialization/serialization_example_test.go`
  - Add migration examples for `errors.Is(err, core.ErrInvalidUTF8)` and `BytesSerializer` fallback.
- Modify: `collections/slices_test.go`, `collections/maps_test.go`
  - Add nil/empty/callback regression tests.
- Modify: `codec/README.md`, `codec/README.ko.md`, `serialization/README.md`, `serialization/README.ko.md`, `core/README.md`, `core/README.ko.md`, `collections/README.md`, `collections/README.ko.md`
  - Document text-vs-binary contracts, non-validating no-error string encoders, and callback rejection in both languages.

## Task 1: Core Invalid UTF-8 Contract

**Files:**
- Create: `core/errors.go`
- Modify: `core/string.go`
- Test: `core/string_test.go`

- [ ] **Step 1: Write failing tests for invalid UTF-8 and boundary truncation**

Append to `core/string_test.go`:

```go
func TestTruncateUTF8BytesRejectsInvalidUTF8(t *testing.T) {
	invalidShort := string([]byte{0xff})
	if _, err := core.TruncateUTF8Bytes(invalidShort, len(invalidShort)); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("TruncateUTF8Bytes invalid short error = %v, want ErrInvalidUTF8", err)
	}

	invalidAroundBoundary := "ok" + string([]byte{0xff}) + "tail"
	if _, err := core.TruncateUTF8Bytes(invalidAroundBoundary, 3); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("TruncateUTF8Bytes invalid boundary error = %v, want ErrInvalidUTF8", err)
	}
}

func TestTruncateUTF8BytesBoundaries(t *testing.T) {
	got, err := core.TruncateUTF8Bytes("세계", 0)
	if err != nil {
		t.Fatalf("TruncateUTF8Bytes zero limit returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("TruncateUTF8Bytes zero limit = %q, want empty", got)
	}

	got, err = core.TruncateUTF8Bytes("세계", len("세"))
	if err != nil {
		t.Fatalf("TruncateUTF8Bytes exact rune boundary returned error: %v", err)
	}
	if got != "세" {
		t.Fatalf("TruncateUTF8Bytes exact rune boundary = %q, want %q", got, "세")
	}
}
```

Also add `errors` to the test imports.

- [ ] **Step 2: Run RED test**

Run:

```bash
go test -count=1 ./core
```

Expected: FAIL because `core.ErrInvalidUTF8` is undefined or invalid UTF-8 is not rejected.

- [ ] **Step 3: Add the sentinel**

Create `core/errors.go`:

```go
package core

import "errors"

var (
	// ErrInvalidUTF8 reports text input that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("invalid UTF-8 text")
)
```

- [ ] **Step 4: Validate text before truncation**

Update `core/string.go`:

```go
func TruncateUTF8Bytes(value string, maxBytes int) (string, error) {
	if maxBytes < 0 {
		return "", fmt.Errorf("maxBytes[%d] must be non-negative", maxBytes)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("truncate UTF-8 bytes: %w", ErrInvalidUTF8)
	}
	if len(value) <= maxBytes {
		return value, nil
	}

	for maxBytes > 0 && !utf8.RuneStart(value[maxBytes]) {
		maxBytes--
	}
	return value[:maxBytes], nil
}
```

- [ ] **Step 5: Run GREEN test**

Run:

```bash
go test -count=1 ./core
```

Expected: PASS.

## Task 2: Codec Text Decoders

**Files:**
- Create: `codec/text.go`
- Modify: `codec/base58.go`, `codec/base62.go`, `codec/base64.go`, `codec/hex.go`
- Test: `codec/codec_test.go`

- [ ] **Step 1: Write failing codec tests**

Add `errors` to `codec/codec_test.go` imports and append:

Keep existing imports and add only the missing packages:

```go
import (
	"errors"

	"github.com/bluetape4k/bluetape-go/core"
)
```

```go
func TestStringDecodersRejectInvalidUTF8(t *testing.T) {
	invalidBytes := []byte{0xff, 0xfe}
	tests := map[string]struct {
		encoded string
		decode  func(string) (string, error)
	}{
		"base58": {encoded: codec.EncodeBase58(invalidBytes), decode: codec.DecodeBase58String},
		"base62": {encoded: codec.EncodeBase62(invalidBytes), decode: codec.DecodeBase62String},
		"base64": {encoded: codec.EncodeBase64(invalidBytes), decode: codec.DecodeBase64String},
		"hex":    {encoded: codec.EncodeHex(invalidBytes), decode: codec.DecodeHexString},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := tt.decode(tt.encoded); !errors.Is(err, core.ErrInvalidUTF8) {
				t.Fatalf("decode invalid UTF-8 error = %v, want ErrInvalidUTF8", err)
			}
		})
	}
}

func TestByteDecodersAcceptArbitraryBinary(t *testing.T) {
	input := []byte{0xff, 0xfe, 0x00, 0x61}
	tests := map[string]struct {
		encoded string
		decode  func(string) ([]byte, error)
	}{
		"base58":    {encoded: codec.EncodeBase58(input), decode: codec.DecodeBase58},
		"base62":    {encoded: codec.EncodeBase62(input), decode: codec.DecodeBase62},
		"url62":     {encoded: codec.EncodeURL62(input), decode: codec.DecodeURL62},
		"base64":    {encoded: codec.EncodeBase64(input), decode: codec.DecodeBase64},
		"base64url": {encoded: codec.EncodeBase64URL(input), decode: codec.DecodeBase64URL},
		"hex":       {encoded: codec.EncodeHex(input), decode: codec.DecodeHex},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.decode(tt.encoded)
			if err != nil {
				t.Fatalf("byte decoder returned error: %v", err)
			}
			if !bytes.Equal(got, input) {
				t.Fatalf("%s byte decode = %v, want %v", name, got, input)
			}
		})
	}
}

func TestBase64URLRejectsMalformedInput(t *testing.T) {
	for _, value := range []string{"++", "__=", "abc="} {
		if _, err := codec.DecodeBase64URL(value); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
			t.Fatalf("malformed Base64URL error = %v, want non-UTF8 codec error", err)
		}
	}
}

func TestHexRejectsInvalidCharacters(t *testing.T) {
	if _, err := codec.DecodeHex("zz"); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("invalid Hex error = %v, want non-UTF8 codec error", err)
	}
}

func TestMalformedStringDecodersDoNotUseInvalidUTF8Sentinel(t *testing.T) {
	tests := map[string]struct {
		input  string
		decode func(string) (string, error)
	}{
		"base58": {input: "0", decode: codec.DecodeBase58String},
		"base62": {input: "abc-123", decode: codec.DecodeBase62String},
		"base64": {input: "not valid base64", decode: codec.DecodeBase64String},
		"hex":    {input: "zz", decode: codec.DecodeHexString},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := tt.decode(tt.input); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
				t.Fatalf("%s malformed input error = %v, want non-UTF8 codec error", name, err)
			}
		})
	}
}
```

Also add `github.com/bluetape4k/bluetape-go/core` to imports.

- [ ] **Step 2: Run RED test**

Run:

```bash
go test -count=1 ./codec
```

Expected: FAIL in `TestStringDecodersRejectInvalidUTF8` because string decoders do not reject invalid UTF-8. The malformed-input and binary-byte checks are regression tests and may already pass.

- [ ] **Step 3: Add codec text validation helper**

Create `codec/text.go`:

```go
package codec

import (
	"fmt"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/core"
)

func stringFromUTF8Bytes(operation string, data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("%s: %w", operation, core.ErrInvalidUTF8)
	}
	return string(data), nil
}
```

- [ ] **Step 4: Route string decoders through the helper**

In each string decoder, replace `return string(decoded), nil` with:

```go
return stringFromUTF8Bytes("decode Base58 string", decoded)
```

Use operation names matching the helper:

- `decode Base58 string`
- `decode Base62 string`
- `decode Base64 string`
- `decode Hex string`

- [ ] **Step 5: Run GREEN test**

Run:

```bash
go test -count=1 ./codec
```

Expected: PASS.

## Task 3: Serialization Text and Metadata Contracts

**Files:**
- Modify: `serialization/raw.go`
- Test: `serialization/serialization_test.go`

- [ ] **Step 1: Write failing serialization tests**

Add `strings` and `github.com/bluetape4k/bluetape-go/core` to the existing
`serialization/serialization_test.go` import block. Keep existing `bytes`,
`errors`, `testing`, and `serialization` imports.

```go
import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
	"github.com/bluetape4k/bluetape-go/serialization"
)
```

Append:

```go
type formatSerializer[T any] struct {
	format string
}

func (s formatSerializer[T]) Format() string { return s.format }
func (s formatSerializer[T]) Marshal(value T) ([]byte, error) { return nil, nil }
func (s formatSerializer[T]) Unmarshal(data []byte) (T, error) {
	var zero T
	return zero, nil
}

func TestStringSerializerRejectsInvalidUTF8(t *testing.T) {
	serializer := serialization.StringSerializer{}
	invalid := string([]byte{0xff})

	if _, err := serializer.Marshal(invalid); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("Marshal invalid UTF-8 error = %v, want ErrInvalidUTF8", err)
	}
	if _, err := serializer.Unmarshal([]byte{0xff}); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("Unmarshal invalid UTF-8 error = %v, want ErrInvalidUTF8", err)
	}
}

func TestBytesSerializerAcceptsArbitraryBinary(t *testing.T) {
	serializer := serialization.BytesSerializer{}
	input := []byte{0xff, 0xfe, 0x00}

	data, err := serializer.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal binary failed: %v", err)
	}
	got, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal binary failed: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("BytesSerializer binary = %v, want %v", got, input)
	}
}

func TestRawSerializersAcceptEmptyNonNilInput(t *testing.T) {
	emptyBytes := []byte{}
	bytesValue, err := (serialization.BytesSerializer{}).Unmarshal(emptyBytes)
	if err != nil {
		t.Fatalf("BytesSerializer empty input failed: %v", err)
	}
	if bytesValue == nil || len(bytesValue) != 0 {
		t.Fatalf("BytesSerializer empty input = %#v, want empty non-nil slice", bytesValue)
	}

	stringValue, err := (serialization.StringSerializer{}).Unmarshal([]byte{})
	if err != nil {
		t.Fatalf("StringSerializer empty input failed: %v", err)
	}
	if stringValue != "" {
		t.Fatalf("StringSerializer empty input = %q, want empty string", stringValue)
	}

	data, err := (serialization.StringSerializer{}).Marshal("")
	if err != nil {
		t.Fatalf("StringSerializer empty string marshal failed: %v", err)
	}
	if data == nil || len(data) != 0 {
		t.Fatalf("StringSerializer empty string marshal = %#v, want empty non-nil bytes", data)
	}
}

func TestRawSerializersRejectNilUnmarshalInput(t *testing.T) {
	if _, err := (serialization.BytesSerializer{}).Unmarshal(nil); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("BytesSerializer nil input error = %v, want non-UTF8 error", err)
	}
	if _, err := (serialization.StringSerializer{}).Unmarshal(nil); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("StringSerializer nil input error = %v, want non-UTF8 error", err)
	}
}

func TestVersionedSerializerRejectsInvalidFormatMetadata(t *testing.T) {
	if _, err := serialization.NewVersionedSerializer[string](formatSerializer[string]{format: ""}, 1); err == nil {
		t.Fatal("expected empty format to fail")
	}

	tooLong := strings.Repeat("x", 256)
	if _, err := serialization.NewVersionedSerializer[string](formatSerializer[string]{format: tooLong}, 1); err == nil {
		t.Fatal("expected overlong format to fail")
	}
}
```

- [ ] **Step 2: Run RED test**

Run:

```bash
go test -count=1 ./serialization
```

Expected: FAIL in `TestStringSerializerRejectsInvalidUTF8` because `StringSerializer` does not reject invalid UTF-8. Empty/nil/metadata tests are regression tests and may already pass.

- [ ] **Step 3: Validate `StringSerializer` text**

Update `serialization/raw.go` imports:

```go
import (
	"fmt"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/core"
)
```

Update methods:

```go
func (StringSerializer) Marshal(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("marshal string: %w", core.ErrInvalidUTF8)
	}
	return []byte(value), nil
}

func (StringSerializer) Unmarshal(data []byte) (string, error) {
	if data == nil {
		return "", fmt.Errorf("unmarshal string: input must not be nil")
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("unmarshal string: %w", core.ErrInvalidUTF8)
	}
	return string(data), nil
}
```

- [ ] **Step 4: Run GREEN test**

Run:

```bash
go test -count=1 ./serialization
```

Expected: PASS.

## Task 4: Collections and Core Regression Coverage

**Files:**
- Modify: `collections/slices_test.go`, `collections/maps_test.go`
- Modify: `core/validate_test.go`, `core/number_test.go`

- [ ] **Step 1: Add collections callback and nil/empty tests**

Append to `collections/slices_test.go`:

```go
func TestCollectionSliceHelpersNilAndEmptyContracts(t *testing.T) {
	if got, err := collections.ChunkBy[int](nil, func(int) bool { return false }); err != nil || got != nil {
		t.Fatalf("ChunkBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.ChunkBy([]int{}, func(int) bool { return false }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("ChunkBy empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.DistinctBy[int, int](nil, func(value int) int { return value }); err != nil || got != nil {
		t.Fatalf("DistinctBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.DistinctBy([]int{}, func(value int) int { return value }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("DistinctBy empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.MapErr[int, int](nil, func(value int) (int, error) { return value, nil }); err != nil || got != nil {
		t.Fatalf("MapErr nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.MapErr([]int{}, func(value int) (int, error) { return value, nil }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("MapErr empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.FilterErr[int](nil, func(value int) (bool, error) { return true, nil }); err != nil || got != nil {
		t.Fatalf("FilterErr nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.FilterErr([]int{}, func(value int) (bool, error) { return true, nil }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FilterErr empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.FilterMap[int, int](nil, func(value int) (int, bool) { return value, true }); err != nil || got != nil {
		t.Fatalf("FilterMap nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.FilterMap([]int{}, func(value int) (int, bool) { return value, true }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FilterMap empty = %#v, %v; want empty slice, nil", got, err)
	}
}

func TestCollectionSliceHelpersRejectNilCallbacks(t *testing.T) {
	if _, err := collections.ChunkBy[int](nil, nil); err == nil {
		t.Fatal("ChunkBy should reject nil predicate before nil input")
	}
	if _, err := collections.ChunkBy([]int{}, nil); err == nil {
		t.Fatal("ChunkBy should reject nil predicate before empty input")
	}
	if _, err := collections.DistinctBy[int, int]([]int{1}, nil); err == nil {
		t.Fatal("DistinctBy should reject nil key function")
	}
	if _, err := collections.DistinctBy[int, int](nil, nil); err == nil {
		t.Fatal("DistinctBy should reject nil key function before nil input")
	}
	if _, err := collections.DistinctBy[int, int]([]int{}, nil); err == nil {
		t.Fatal("DistinctBy should reject nil key function before empty input")
	}
	if _, err := collections.MapErr[int, int]([]int{1}, nil); err == nil {
		t.Fatal("MapErr should reject nil mapper")
	}
	if _, err := collections.MapErr[int, int](nil, nil); err == nil {
		t.Fatal("MapErr should reject nil mapper before nil input")
	}
	if _, err := collections.MapErr[int, int]([]int{}, nil); err == nil {
		t.Fatal("MapErr should reject nil mapper before empty input")
	}
	if _, err := collections.FilterErr[int]([]int{1}, nil); err == nil {
		t.Fatal("FilterErr should reject nil predicate")
	}
	if _, err := collections.FilterErr[int](nil, nil); err == nil {
		t.Fatal("FilterErr should reject nil predicate before nil input")
	}
	if _, err := collections.FilterErr[int]([]int{}, nil); err == nil {
		t.Fatal("FilterErr should reject nil predicate before empty input")
	}
	if _, err := collections.FilterMap[int, int]([]int{1}, nil); err == nil {
		t.Fatal("FilterMap should reject nil mapper")
	}
	if _, err := collections.FilterMap[int, int](nil, nil); err == nil {
		t.Fatal("FilterMap should reject nil mapper before nil input")
	}
	if _, err := collections.FilterMap[int, int]([]int{}, nil); err == nil {
		t.Fatal("FilterMap should reject nil mapper before empty input")
	}
}
```

Append to `collections/maps_test.go`:

```go
func TestMapHelpersNilEmptyAndCallbackContracts(t *testing.T) {
	if _, err := collections.GroupBy[string, int]([]string{"a"}, nil); err == nil {
		t.Fatal("GroupBy should reject nil key function")
	}
	if _, err := collections.GroupBy[string, int](nil, nil); err == nil {
		t.Fatal("GroupBy should reject nil key function before nil input")
	}
	if _, err := collections.GroupBy[string, int]([]string{}, nil); err == nil {
		t.Fatal("GroupBy should reject nil key function before empty input")
	}
	if got, err := collections.CountBy[string, int](nil, func(value string) int { return len(value) }); err != nil || got != nil {
		t.Fatalf("CountBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.CountBy([]string{}, func(value string) int { return len(value) }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("CountBy empty = %#v, %v; want empty map, nil", got, err)
	}
	if _, err := collections.CountBy[string, int](nil, nil); err == nil {
		t.Fatal("CountBy should reject nil key function before nil input")
	}
	if _, err := collections.CountBy[string, int]([]string{}, nil); err == nil {
		t.Fatal("CountBy should reject nil key function before empty input")
	}
}
```

- [ ] **Step 2: Add core boundary tests**

Append to `core/validate_test.go`:

```go
func TestRequireRangeBoundaries(t *testing.T) {
	if err := core.RequireInRange("value", 1, 1, 5); err != nil {
		t.Fatalf("RequireInRange lower boundary returned error: %v", err)
	}
	if err := core.RequireInRange("value", 5, 1, 5); err != nil {
		t.Fatalf("RequireInRange upper boundary returned error: %v", err)
	}
	if err := core.RequireInRange("value", 1, 5, 1); err == nil {
		t.Fatal("RequireInRange should reject invalid range")
	}
	if err := core.RequireInOpenRange("value", 1, 1, 5); err != nil {
		t.Fatalf("RequireInOpenRange lower boundary returned error: %v", err)
	}
	if err := core.RequireInOpenRange("value", 1, 5, 5); err == nil {
		t.Fatal("RequireInOpenRange should reject empty range")
	}
}
```

Add cases to `core/number_test.go` invalid format list:

```go
for _, value := range []string{"1234", "0x", "#", "0xxyz", "+0xff", "0x 12"} {
	if core.IsHexFormat(value) {
		t.Fatalf("%q should not be hex format", value)
	}
}
```

Keep the existing valid list unchanged; trimmed negative prefixes such as
`" -0xff "` remain valid because `IsHexFormat` already trims surrounding
whitespace before handling a leading minus sign.

- [ ] **Step 3: Run regression tests**

Run:

```bash
go test -count=1 ./core ./collections
```

Expected: PASS. If any regression test fails, inspect whether the plan captured the wrong current contract before editing production code.

## Task 5: Go Doc, README, and Example Contract Notes

**Files:**
- Modify: `core/errors.go`, `core/string.go`
- Modify: `codec/base58.go`, `codec/base62.go`, `codec/base64.go`, `codec/hex.go`
- Modify: `serialization/raw.go`
- Modify: `codec/codec_example_test.go`
- Modify: `serialization/serialization_example_test.go`
- Modify: `codec/README.md`, `codec/README.ko.md`
- Modify: `serialization/README.md`, `serialization/README.ko.md`
- Modify: `core/README.md`, `core/README.ko.md`
- Modify: `collections/README.md`, `collections/README.ko.md`

- [ ] **Step 1: Update exported Go doc comments**

Update public comments so pkg.go.dev exposes the sentinel and binary
alternatives:

```go
// ErrInvalidUTF8 reports text input that is not valid UTF-8.
```

```go
// TruncateUTF8Bytes truncates value to at most maxBytes without splitting a UTF-8 rune.
//
// It returns an error wrapping ErrInvalidUTF8 when value is not valid UTF-8.
```

For each codec `Decode*String` helper, add:

```go
// It returns an error wrapping core.ErrInvalidUTF8 when decoded bytes are not valid UTF-8.
// Use the corresponding byte decoder for binary payloads.
```

For each no-error codec `Encode*String` helper, add:

```go
// It converts the string to bytes before encoding and cannot report invalid UTF-8.
```

For `StringSerializer.Marshal` and `StringSerializer.Unmarshal`, add:

```go
// It returns an error wrapping core.ErrInvalidUTF8 when the string is not valid UTF-8.
```

or:

```go
// It returns an error wrapping core.ErrInvalidUTF8 when data is not valid UTF-8.
```

- [ ] **Step 2: Add migration examples**

Add one codec example that shows invalid UTF-8 detection and byte fallback:

```go
func ExampleDecodeBase64String_invalidUTF8() {
	encoded := codec.EncodeBase64([]byte{0xff, 0xfe})
	if _, err := codec.DecodeBase64String(encoded); errors.Is(err, core.ErrInvalidUTF8) {
		fmt.Println("invalid text")
	}

	decoded, err := codec.DecodeBase64(encoded)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(decoded))

	// Output:
	// invalid text
	// 2
}
```

Add one serialization example that shows invalid UTF-8 detection and
`BytesSerializer` fallback:

```go
func ExampleStringSerializer_invalidUTF8() {
	if _, err := (serialization.StringSerializer{}).Unmarshal([]byte{0xff}); errors.Is(err, core.ErrInvalidUTF8) {
		fmt.Println("invalid text")
	}

	decoded, err := (serialization.BytesSerializer{}).Unmarshal([]byte{0xff})
	if err != nil {
		panic(err)
	}
	fmt.Println(len(decoded))

	// Output:
	// invalid text
	// 1
}
```

Add the required `errors`, `fmt`, and `core` imports without removing existing
example imports.

- [ ] **Step 3: Update codec README behavior**

In English, add behavior bullets:

```markdown
- Decode string helpers are UTF-8 text helpers and return an error wrapping
  `core.ErrInvalidUTF8` when the decoded bytes are not valid UTF-8.
- No-error encode string helpers convert strings to bytes before encoding and
  cannot report invalid UTF-8.
- Binary payloads should use byte helpers such as `DecodeBase64`,
  `DecodeBase64URL`, `DecodeHex`, `DecodeBase58`, `DecodeBase62`, or
  `DecodeURL62`.
```

In Korean, add equivalent bullets:

```markdown
- Decode string helper는 UTF-8 text helper이며 decoded byte가 valid UTF-8이
  아니면 `core.ErrInvalidUTF8`을 wrapping한 error를 반환합니다.
- Error를 반환하지 않는 encode string helper는 string을 byte로 변환한 뒤
  encoding하며 invalid UTF-8을 보고할 수 없습니다.
- Binary payload는 `DecodeBase64`, `DecodeBase64URL`, `DecodeHex`,
  `DecodeBase58`, `DecodeBase62`, `DecodeURL62` 같은 byte helper를 사용해야
  합니다.
```

- [ ] **Step 4: Update serialization README behavior**

In English, add:

```markdown
- `StringSerializer` is a UTF-8 text serializer and returns an error wrapping
  `core.ErrInvalidUTF8` for invalid UTF-8 input.
- Binary payloads should use `BytesSerializer`.
```

In Korean, add:

```markdown
- `StringSerializer`는 UTF-8 text serializer이며 invalid UTF-8 input에는
  `core.ErrInvalidUTF8`을 wrapping한 error를 반환합니다.
- Binary payload는 `BytesSerializer`를 사용해야 합니다.
```

- [ ] **Step 5: Update core README behavior**

In English, change the `TruncateUTF8Bytes` bullet to:

```markdown
- `TruncateUTF8Bytes` truncates at rune boundaries and rejects negative limits
  or invalid UTF-8 input.
```

In Korean, change it to:

```markdown
- `TruncateUTF8Bytes`는 rune boundary에서 자르고 negative limit 또는 invalid
  UTF-8 input을 거부합니다.
```

- [ ] **Step 6: Update collections README behavior**

In English, replace the callback bullets with:

```markdown
- `ChunkBy`, `DistinctBy`, `GroupBy`, and `CountBy` reject nil key or predicate
  functions.
- `MapErr`, `FilterErr`, and `FilterMap` reject nil mapper or predicate
  functions.
```

In Korean, use:

```markdown
- `ChunkBy`, `DistinctBy`, `GroupBy`, `CountBy`는 nil key/predicate function을
  거부합니다.
- `MapErr`, `FilterErr`, `FilterMap`은 nil mapper/predicate function을
  거부합니다.
```

- [ ] **Step 7: Run documentation checks**

Run:

```bash
git diff --check
rg -n "ErrInvalidUTF8|BytesSerializer|String helper|StringSerializer|TruncateUTF8Bytes|MapErr" codec/README.md codec/README.ko.md serialization/README.md serialization/README.ko.md core/README.md core/README.ko.md collections/README.md collections/README.ko.md
rg -n "ErrInvalidUTF8|binary payload|DecodeURL62|cannot report invalid UTF-8" core codec serialization
go test -run Example -count=1 ./codec ./serialization
```

Expected: no whitespace errors; grep shows updated English and Korean behavior notes.

## Task 6: Final Verification and Review Prep

**Files:**
- All touched files from Tasks 1-5

- [ ] **Step 1: Format changed Go files**

Run:

```bash
gofmt -w core/errors.go core/string.go core/string_test.go core/validate_test.go core/number_test.go codec/text.go codec/base58.go codec/base62.go codec/base64.go codec/hex.go codec/codec_test.go codec/codec_example_test.go serialization/raw.go serialization/serialization_test.go serialization/serialization_example_test.go collections/slices_test.go collections/maps_test.go
```

Expected: no output.

- [ ] **Step 2: Run targeted tests**

Run:

```bash
go test -count=1 ./core ./collections ./codec ./serialization
```

Expected: PASS for all four packages.

- [ ] **Step 3: Run example and dependency checks**

Run:

```bash
go test -run Example -count=1 ./codec ./serialization
go list -deps ./codec ./serialization | rg '^github.com/bluetape4k/bluetape-go/core$'
```

Expected: examples pass and dependency output confirms the intentional
`codec`/`serialization` dependency on `core`.

- [ ] **Step 4: Run bounded race smoke**

Run:

```bash
go test -race -count=1 ./codec
```

Expected: PASS. The final `make ci` runs full repo race; this pre-CI smoke is
limited to `codec` because its existing tests include concurrent round-trip
stress.

- [ ] **Step 5: Run repository gate**

Run:

```bash
make ci
```

Expected: PASS. Run `make ci` once. If it fails, preserve exact output and do
not loop the full suite. Rerun only the failing package/test with `-p 1`,
`-count=1`, and targeted `-run` where possible. After a code or docs fix, run
`make ci` once as the final repository gate.

- [ ] **Step 6: Run diff and API review checks**

Run:

```bash
git diff --check
git diff --stat
git diff -- core codec serialization collections
rg -n "ErrInvalidUTF8|cannot report invalid UTF-8|DecodeURL62|errors.Is" core codec serialization
```

Expected: no whitespace errors; diff shows no new parity primitive APIs and only `core.ErrInvalidUTF8` as the new exported error contract.

- [ ] **Step 7: Prepare Step 6-R**

Before PR creation, run Step 5 verifier and Step 6-R 7-Tier code review against the final diff. Step 6-R must converge to `P0=0 P1=0` and save a review artifact under `docs/superpowers/reviews`.

## Self-Review

- Spec coverage: Tasks 1-5 cover invalid UTF-8 text contracts, binary byte contracts, collections callback/nil/empty behavior, core validation/truncation boundaries, malformed codec vectors, README sync, and final validation commands.
- Placeholder scan: no `TBD`, `TODO`, or “implement later” placeholders are intentionally present.
- Type consistency: shared sentinel is `core.ErrInvalidUTF8`; codec and serialization wrap it so callers can use `errors.Is`.
- Performance note: UTF-8 validation is O(n), expected to add no allocations beyond existing byte/string conversions, and does not need a benchmark unless implementation or docs later claim large-payload throughput.
