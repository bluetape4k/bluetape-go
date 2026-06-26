# textsearch

[English](README.md) | [한국어](README.ko.md)

`textsearch` provides deterministic multi-pattern search for Go callers. It
compiles a dictionary into an immutable Aho-Corasick matcher and exposes
contains, first-match, all-match, replacement, and masking helpers.

## Import

```go
import "github.com/bluetape4k/bluetape-go/textsearch"
```

## Usage

```go
matcher, err := textsearch.Compile([]textsearch.Pattern{
    {ID: "greeting", Text: "hello"},
    {ID: "subject", Text: "world"},
}, textsearch.Config{IgnoreCase: true})
if err != nil {
    return err
}

matches := matcher.FindAll("Hello, world!")
_ = matches[0].Start // byte offset in the original input
```

For deterministic replacement:

```go
masked := matcher.Mask("Hello, world!", '*')
_ = masked
```

## Behavior

- `Matcher` is immutable after `Compile` returns and is safe for concurrent
  readers.
- `FindAll` returns matches sorted by original byte offset. The default
  `OverlapAll` mode returns overlapping and duplicate-pattern matches.
- `OverlapLeftmostLongest` returns deterministic non-overlapping matches by
  selecting the longest match at each leftmost start.
- `First` returns the earliest accepted match; ties prefer longer patterns,
  then dictionary order.
- Duplicate pattern text is allowed. Use `Pattern.ID` to preserve caller-owned
  dictionary metadata.
- `NormalizeNFC` and `NormalizeNFKC` normalize patterns and input before
  matching. Match `Start` and `End` still refer to byte offsets in the original
  input.
- `IgnoreCase` uses Go's `strings.ToLower`; locale-specific casing is outside
  this package's scope.
- `BoundaryASCIIWord` and `BoundaryUnicodeWord` filter matches to word
  boundaries. Boundary checks are helper behavior, not a moderation or security
  boundary.
- Search complexity is linear in normalized input runes plus emitted matches
  after compilation. Compilation is linear in dictionary size plus fail-link
  construction.
- Replacement and masking use leftmost-longest non-overlapping matches even
  when the matcher is configured to report all overlaps.

## Test

```bash
go test -count=1 ./textsearch
go test -race -count=1 ./textsearch
```
