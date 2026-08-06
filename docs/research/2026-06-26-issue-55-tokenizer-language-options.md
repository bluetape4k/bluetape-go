# Issue #55 Tokenizer And Language Detection Options

Issue #55는 first-party `textsearch` core 뒤에 어떤 dependency-backed text
feature가 와야 하는지 결정한다. Go ecosystem을 `bluetape4k-text` source
parity와 비교하고, 큰 NLP dependency는 core 밖에 둔다.

## 소스 Parity 대상

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-text`

| Source module | Source capability | Go parity implication |
|---|---|---|
| `tokenizer-korean` | Korean normalization, 26-class POS tokenization, noun tokenization, phrase extraction, stemming, sentence splitting, detokenization, runtime noun/blockword dictionary updates, and thread-safe facade methods. | Full parity는 큰 NLP project이며 `textsearch.Tokenizer` 주변 adapter가 아니다. |
| `tokenizer-japanese` | Kuromoji IPAdic tokenization, POS helpers, noun/verb filtering, compound blockword detection, masking, and dynamic blockword dictionary management. | Japanese는 Kagome을 통한 credible Go-native adapter path가 있지만 dictionary/model cost는 optional이어야 한다. |
| `lingua` | Lingua-backed language detection, mixed-language detection, detector builders, Unicode script helpers, and detector reuse guidance. | Lingua-Go가 가장 가까운 parity path이다. Lightweight detector는 fallback-only로 둔다. |

## 외부 근거

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

이 size evidence 때문에 dependency isolation은 필수다. 이 중 어떤 것도 base
`textsearch` package로 끌어오면 안 된다.

## 비교

| Area | Best candidate | Quality | Dictionary/model story | Deployment cost | Decision |
|---|---|---:|---|---|---|
| Japanese tokenizer | Kagome v2 + IPA dictionary | High | Pure Go, embedded IPA/UniDic dictionaries, user dictionary support, POS features. | Medium/high: 31M + 22M IPA or 87M UniDic. | Optional `textsearch/japanese` package에서 채택한다. |
| Korean tokenizer | None for full parity | Low/medium | Kagome ko dict exists but has tiny adoption signal, 90M cache size, and does not cover source normalization/stemming/sentence/dictionary facade parity. MeCab wrappers add cgo/deployment cost. | High. | Full Korean tokenizer는 보류한다. Core dependency는 추가하지 않는다. |
| Language detection parity | Lingua-Go | High | Short-text and mixed-language oriented, detector reuse, high/low accuracy modes, shared model memory. | High: 123M cache size and large runtime memory risk. | Optional `textsearch/language` package에서 subset builder와 함께 채택한다. |
| Lightweight language/script detection | Whatlanggo | Medium | 84 languages, script recognition, pure Go/no dependencies. | Low: 388K cache size. | Fallback으로만 비교한다. Parity replacement가 아니다. |
| CLD3-style detection | gocld3 | Low/medium | Small module but low adoption; may need external native-style model assumptions depending on implementation. | Low/medium. | Lingua-Go가 benchmark/memory proof에서 기각될 때까지 보류한다. |

## 결정

### 구현 / 채택

1. **Japanese tokenizer adapter**: Kagome v2와 `textsearch.Tokenizer` 주변의
   optional package를 만든다. IPA dictionary, POS mapping, byte-span
   preservation, noun/verb filters, blockword examples로 시작한다. 필요해질
   때까지 UniDic은 documented opt-in으로 둔다.
2. **Language detection adapter**: Lingua-Go 주변의 optional package를 만든다.
   subset construction, detector reuse, mixed-language helper, Unicode script
   helper, memory guidance를 포함한다. Memory budget이 lighter detector path를
   강제할 때만 Whatlanggo comparison test를 추가한다.

### 보류

- Full Korean tokenizer parity: normalization, POS, stemming, phrase
  extraction, sentence splitting, detokenization, runtime noun dictionary
  updates는 보류한다. Mature Go-native Korean NLP path가 나타나기 전까지는
  기존 `textsearch` blockword matching과 examples로 충분하다.
- Kagome Korean dictionary adoption: 지금 선택하지 않는다. 90M cache size,
  tiny adoption signal, narrower dictionary-only surface는 public Korean
  tokenizer package를 정당화하지 못한다.
- MeCab cgo wrappers: system-level runtime dependency와 platform deployment
  cost를 추가하므로 bluetape-go package scope 밖에 둔다.

### Go Core에서 시도하지 않을 것

- Direct Kotlin/JVM facade port는 하지 않는다.
- `textsearch`에 large NLP model/dictionary dependency를 넣지 않는다.
- 향후 package가 `GoroutineStressTester`와 `go test -race`로 lock-free swap
  또는 synchronized mutation을 증명하기 전까지 runtime mutable dictionary
  contract를 만들지 않는다.
- Blockword masking이나 language detection을 security policy라고 주장하지 않는다.

## 후속 구현 이슈

- #336 Japanese Kagome adapter: selected. `type: task`, `priority: p1`,
  `area: text`, milestone `0.8.0`.
- #337 Lingua-Go language detection adapter: selected. `type: task`,
  `priority: p1`, `area: text`, milestone `0.8.0`.
- Korean tokenizer: no implementation issue. Defer decision은 #45와 이
  research note에 기록한다.

## 검증

- 각 dependency candidate에 대한 `gh repo view` metadata.
- Kagome, Lingua-Go, Whatlanggo, gocld3, MeCab wrapper에 대한
  `go list -m -versions`.
- Model/dictionary size에 대한 `go get` + `du -sh` module cache sample.
- `bluetape4k-text/tokenizer-korean`, `tokenizer-japanese`, `lingua` README의
  local source inventory.
