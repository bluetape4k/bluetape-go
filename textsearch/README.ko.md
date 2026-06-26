# textsearch

[English](README.md) | [한국어](README.ko.md)

`textsearch`는 Go caller를 위한 deterministic multi-pattern search package입니다.
Dictionary를 immutable Aho-Corasick matcher로 compile하고 contains, first match,
all match, replacement, masking helper를 제공합니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/textsearch"
```

## 사용 예

```go
matcher, err := textsearch.Compile([]textsearch.Pattern{
    {ID: "greeting", Text: "hello"},
    {ID: "subject", Text: "world"},
}, textsearch.Config{IgnoreCase: true})
if err != nil {
    return err
}

matches := matcher.FindAll("Hello, world!")
_ = matches[0].Start // original input의 byte offset
```

Deterministic replacement에는 다음 helper를 사용합니다.

```go
masked := matcher.Mask("Hello, world!", '*')
_ = masked
```

## 동작

- `Matcher`는 `Compile` 이후 immutable이며 concurrent reader에 안전합니다.
- `FindAll`은 original byte offset 기준으로 match를 정렬합니다. 기본
  `OverlapAll` mode는 overlapping match와 duplicate-pattern match를 모두
  반환합니다.
- `OverlapLeftmostLongest`는 각 leftmost start에서 가장 긴 match를 선택해
  deterministic non-overlapping match를 반환합니다.
- `First`는 가장 이른 accepted match를 반환합니다. 동률이면 더 긴 pattern,
  그 다음 dictionary 순서를 우선합니다.
- 같은 pattern text를 여러 번 등록할 수 있습니다. Caller-owned dictionary
  metadata는 `Pattern.ID`로 보존합니다.
- `NormalizeNFC`와 `NormalizeNFKC`는 matching 전에 pattern과 input을
  normalize합니다. `Start`와 `End`는 여전히 original input의 byte offset입니다.
- `IgnoreCase`는 Go `strings.ToLower`를 사용합니다. Locale-specific casing은
  이 package의 범위 밖입니다.
- `BoundaryASCIIWord`와 `BoundaryUnicodeWord`는 word boundary match만
  허용합니다. Boundary check는 helper behavior일 뿐 moderation/security
  boundary가 아닙니다.
- Search complexity는 compilation 후 normalized input rune 수와 emitted match
  수에 선형입니다. Compilation은 dictionary size와 fail-link construction에
  선형입니다.
- Replacement와 masking은 matcher가 overlap 전체 보고로 설정되어 있어도
  leftmost-longest non-overlapping match를 사용합니다.

## 테스트

```bash
go test -count=1 ./textsearch
go test -race -count=1 ./textsearch
```
