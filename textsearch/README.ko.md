# textsearch

[English](README.md) | [한국어](README.ko.md)

`textsearch`는 Go caller를 위한 deterministic multi-pattern search, tokenizer
core interface, blockword masking package입니다. Dictionary를 immutable
Aho-Corasick matcher로 compile하고 contains, first match, all match, replacement,
masking, tokenization, dictionary, blockword response helper를 제공합니다.

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

Dependency-free lexical tokenization에는 다음 helper를 사용합니다.

```go
tokenizer := textsearch.NewSimpleTokenizer()
request, err := textsearch.NewTokenizeRequest("Hello 세계 123!", textsearch.TokenizeOptions{
    Normalize: textsearch.NormalizeNFC,
})
if err != nil {
    return err
}

response, err := tokenizer.Tokenize(request)
if err != nil {
    return err
}
_ = response.Tokens[0].Span.Start // original input의 byte offset
```

Moderation-like blockword processing에는 static dictionary를 compile하고, entry가
바뀔 때 새 dictionary를 rebuild해 교체합니다.

```go
dictionary, err := textsearch.NewBlockwordDictionary([]textsearch.BlockwordEntry{
    {ID: "ko", Text: "욕설", Severity: textsearch.SeverityHigh},
    {ID: "ja", Text: "ホモ", Severity: textsearch.SeverityMiddle},
}, textsearch.Config{Normalize: textsearch.NormalizeNFC})
if err != nil {
    return err
}

request, err := textsearch.NewBlockwordRequest("욕설 그리고 ホモ", textsearch.BlockwordOptions{
    Mask:        "*",
    MinSeverity: textsearch.SeverityMiddle,
})
if err != nil {
    return err
}

response, err := dictionary.Process(request)
if err != nil {
    return err
}
_ = response.MaskedText // "** 그리고 **"
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
- `Token` span도 original input의 byte offset을 사용합니다.
  `Token.Normalized`는 `Token.Text`와 길이가 다를 수 있습니다.
- `Tokenizer`와 `TokenizerFunc`는 core extension point입니다. 언어별 optional
  dependency는 core package 밖에 둡니다. Kagome 기반 일본어 tokenizer가 필요하면
  [`textsearch/japanese`](japanese/README.ko.md)를, Lingua-Go language detection이
  필요하면 [`textsearch/language`](language/README.ko.md)를 명시적으로 import하세요.
- `SimpleTokenizer`는 deterministic dependency-free tokenizer입니다. Test와
  simple lexical workflow를 위해 Unicode letter/mark, digit, whitespace,
  punctuation, symbol을 묶으며, language-aware POS tagger가 아닙니다.
- `DictionaryProvider`, `StaticDictionaryProvider`, `DictionarySet`은 storage,
  tokenizer model size, runtime reload policy를 강제하지 않고 dictionary loading과
  immutable lookup을 모델링합니다.
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
- `BlockwordDictionary`는 `NewBlockwordDictionary` 이후 immutable이며 concurrent
  reader에 안전합니다. Runtime dictionary mutation은 의도적으로 rebuild/swap
  workflow로 표현합니다.
- `BlockwordEntry`는 severity와 caller-owned metadata를 보존합니다.
  `BlockwordOptions`는 deterministic non-overlapping masking 전에 minimum severity
  기준으로 match를 필터링합니다.
- `NewBlockwordRequest`는 blank input과 `MaxBlockwordTextLength`보다 긴 입력을
  거부합니다. Error message에는 raw input을 넣지 않고 길이만 보고합니다.
- `NewTokenizeRequest`는 blank input과 `MaxTokenizeTextLength`보다 긴 입력을
  거부합니다. Error message에는 raw input을 넣지 않고 길이만 보고합니다.
- Blockword masking은 service code를 위한 helper transform입니다. 완전한
  moderation policy, authorization check, security boundary가 아닙니다.

## 테스트

```bash
go test -count=1 ./textsearch
go test -race -count=1 ./textsearch
go test -count=1 ./textsearch/japanese
go test -race -count=1 ./textsearch/japanese
go test -count=1 ./textsearch/language
go test -race -count=1 ./textsearch/language
```
