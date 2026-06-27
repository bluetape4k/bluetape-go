# Japanese Tokenizer

`textsearch/japanese` is an optional Kagome v2 adapter for the
`textsearch.Tokenizer` contract. Importing this package keeps the core
`textsearch` package dependency-free while letting Japanese callers opt in to
morphological tokenization.

## Install

```bash
go get github.com/bluetape4k/bluetape-go/textsearch/japanese
```

This package currently defaults to Kagome's IPA dictionary:

```go
tokenizer, err := japanese.NewTokenizer()
```

Use `WithDictionary` when a caller deliberately chooses another Kagome
dictionary. UniDic is intentionally documented as an opt-in/future path because
it has a larger deployment footprint and different dictionary metadata.

## Usage

```go
request, err := textsearch.NewTokenizeRequest("日本語を勉強します。", textsearch.TokenizeOptions{})
if err != nil {
    return err
}
response, err := tokenizer.Tokenize(request)
if err != nil {
    return err
}
for _, token := range japanese.FilterNouns(response.Tokens) {
    fmt.Println(token.Text, token.Metadata[japanese.MetadataPOS])
}
```

Returned spans are byte offsets into the original request text, so callers can
slice the original Go string with `token.Span.Start:token.Span.End`.

## Boundaries

- The package is for tokenization and search preparation, not a security
  classifier.
- Dictionary choice affects binary/module size, deployment cost, and token
  metadata. IPA is the default first slice.
- Kagome POS data is preserved in token metadata; `textsearch.PartOfSpeech`
  remains a coarse cross-language class.
- Use existing `textsearch.BlockwordDictionary` for masking or detection after
  selecting the token surfaces relevant to your use case.

## Verification

The package includes table-driven tokenization tests, byte-span checks,
normalization checks, dictionary option tests, examples, and
`GoroutineStressTester` coverage. Run:

```bash
go test -count=1 ./textsearch/japanese
go test -race -count=1 ./textsearch/japanese
```
