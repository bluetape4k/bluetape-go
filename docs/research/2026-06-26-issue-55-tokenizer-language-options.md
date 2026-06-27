# Issue #55 Tokenizer And Language Detection Options

Issue #55 decides which dependency-backed text features should follow the
first-party `textsearch` core. It compares the Go ecosystem against
`bluetape4k-text` source parity and keeps large NLP dependencies out of core.

## Source Parity Target

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-text`

| Source module | Source capability | Go parity implication |
|---|---|---|
| `tokenizer-korean` | Korean normalization, 26-class POS tokenization, noun tokenization, phrase extraction, stemming, sentence splitting, detokenization, runtime noun/blockword dictionary updates, and thread-safe facade methods. | Full parity is a large NLP project, not an adapter around `textsearch.Tokenizer`. |
| `tokenizer-japanese` | Kuromoji IPAdic tokenization, POS helpers, noun/verb filtering, compound blockword detection, masking, and dynamic blockword dictionary management. | Japanese has a credible Go-native adapter path through Kagome, but dictionary/model cost must stay optional. |
| `lingua` | Lingua-backed language detection, mixed-language detection, detector builders, Unicode script helpers, and detector reuse guidance. | Lingua-Go is the closest parity path; lightweight detectors can be fallback-only. |

## External Evidence

Sources checked on 2026-06-26:

- Kagome: <https://github.com/ikawaha/kagome>, <https://pkg.go.dev/github.com/ikawaha/kagome/v2/tokenizer>
- Kagome Korean dictionary: <https://github.com/ikawaha/kagome-dict-ko>
- Lingua-Go: <https://github.com/pemistahl/lingua-go>, <https://github.com/pemistahl/lingua-go/discussions/28>
- Whatlanggo: <https://github.com/abadojack/whatlanggo>, <https://pkg.go.dev/github.com/abadojack/whatlanggo>
- gocld3: <https://github.com/jmhodges/gocld3>
- MeCab wrapper comparison point: <https://github.com/bluele/mecab-golang>

Repository metadata from `gh repo view`:

| Candidate | License | Latest release | Stars | Notes |
|---|---|---:|---:|---|
| `github.com/ikawaha/kagome/v2` | MIT | `v2.11.0` on 2026-03-03 | 969 | Pure Go Japanese morphological analyzer, embedded dictionaries, IPA/UniDic support, segmentation modes, user dictionary support. |
| `github.com/ikawaha/kagome-dict-ko` | MIT | `v.0.2.5` on 2025-04-01 | 6 | MeCab-ko dictionary package for Kagome; low adoption signal and different feature fields from Japanese dictionaries. |
| `github.com/pemistahl/lingua-go` | Apache-2.0 | `v1.4.0` on 2023-09-05 | 1352 | Closest parity to source `lingua`, including mixed-language scope and detector reuse. |
| `github.com/abadojack/whatlanggo` | MIT | no GitHub release; Go module `v1.0.1` | 689 | Pure Go, no external dependencies, 84 languages and script recognition, but older module release and less parity. |
| `github.com/jmhodges/gocld3` | Apache-2.0 | `v1.0.0` on 2025-08-22 | 25 | Small Go wrapper around CLD3-style detection; lower adoption and likely non-core fallback only. |
| `github.com/bluele/mecab-golang` | MIT | no GitHub release; no tagged Go module | 42 | cgo wrapper around MeCab; deployment cost is too high for bluetape-go core. |

Module cache size sample from `go get` and `du -sh`:

| Module | Cache size |
|---|---:|
| `github.com/ikawaha/kagome/v2@v2.11.0` | 31M |
| `github.com/ikawaha/kagome-dict/ipa@latest` | 22M |
| `github.com/ikawaha/kagome-dict/uni@latest` | 87M |
| `github.com/ikawaha/kagome-dict-ko@v.0.2.5` | 90M |
| `github.com/pemistahl/lingua-go@v1.4.0` | 123M |
| `github.com/abadojack/whatlanggo@v1.0.1` | 388K |
| `github.com/jmhodges/gocld3@v1.0.0` | 2.8M |

This size evidence makes dependency isolation mandatory. None of these should
be pulled into the base `textsearch` package.

## Comparison

| Area | Best candidate | Quality | Dictionary/model story | Deployment cost | Decision |
|---|---|---:|---|---|---|
| Japanese tokenizer | Kagome v2 + IPA dictionary | High | Pure Go, embedded IPA/UniDic dictionaries, user dictionary support, POS features. | Medium/high: 31M + 22M IPA or 87M UniDic. | Adopt in optional `textsearch/japanese` package. |
| Korean tokenizer | None for full parity | Low/medium | Kagome ko dict exists but has tiny adoption signal, 90M cache size, and does not cover source normalization/stemming/sentence/dictionary facade parity. MeCab wrappers add cgo/deployment cost. | High. | Defer full Korean tokenizer. No core dependency. |
| Language detection parity | Lingua-Go | High | Short-text and mixed-language oriented, detector reuse, high/low accuracy modes, shared model memory. | High: 123M cache size and large runtime memory risk. | Adopt in optional `textsearch/language` package with subset builder. |
| Lightweight language/script detection | Whatlanggo | Medium | 84 languages, script recognition, pure Go/no dependencies. | Low: 388K cache size. | Compare as fallback only; not parity replacement. |
| CLD3-style detection | gocld3 | Low/medium | Small module but low adoption; may need external native-style model assumptions depending on implementation. | Low/medium. | Defer unless Lingua-Go is rejected by benchmark/memory proof. |

## Decisions

### Implement / Adopt

1. **Japanese tokenizer adapter**: create an optional package around Kagome v2
   and `textsearch.Tokenizer`. Start with IPA dictionary, POS mapping,
   byte-span preservation, noun/verb filters, and blockword examples. Keep
   UniDic as a documented opt-in if needed later.
2. **Language detection adapter**: create an optional package around Lingua-Go
   with subset construction, detector reuse, mixed-language helper, Unicode
   script helper, and memory guidance. Add Whatlanggo comparison tests only if
   the memory budget forces a lighter detector path.

### Defer

- Full Korean tokenizer parity: defer normalization, POS, stemming, phrase
  extraction, sentence splitting, detokenization, and runtime noun dictionary
  updates. Existing `textsearch` blockword matching plus examples are enough
  until a mature Go-native Korean NLP path appears.
- Kagome Korean dictionary adoption: do not select it now. Its 90M cache size,
  tiny adoption signal, and narrower dictionary-only surface do not justify a
  public Korean tokenizer package.
- MeCab cgo wrappers: keep out of bluetape-go package scope because they add
  system-level runtime dependencies and platform deployment cost.

### Will Not Attempt In Go Core

- No direct Kotlin/JVM facade port.
- No large NLP model/dictionary dependency in `textsearch`.
- No runtime mutable dictionary contract unless a future package proves
  lock-free swap or synchronized mutation through `GoroutineStressTester` and
  `go test -race`.
- No claim that blockword masking or language detection is a security policy.

## Follow-up Implementation Issues

- #336 Japanese Kagome adapter: selected. `type: task`, `priority: p1`,
  `area: text`, milestone `0.8.0`.
- #337 Lingua-Go language detection adapter: selected. `type: task`,
  `priority: p1`, `area: text`, milestone `0.8.0`.
- Korean tokenizer: no implementation issue. Record defer decision in #45 and
  this research note.

## Validation

- `gh repo view` metadata for each dependency candidate.
- `go list -m -versions` for Kagome, Lingua-Go, Whatlanggo, gocld3, MeCab
  wrapper.
- `go get` + `du -sh` module cache sample for model/dictionary size.
- Local source inventory from `bluetape4k-text/tokenizer-korean`,
  `tokenizer-japanese`, and `lingua` READMEs.
