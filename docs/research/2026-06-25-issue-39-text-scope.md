# Issue 39 Text 연구 범위

Issue #39는 0.9.0 text milestone에서 `bluetape4k-text` 중 어느 범위를 Go package로
가져올지 결정하는 0.7.0 research gate다. 이 노트는 #45와 #52-#55의 구체적인 scope
decision으로 넓은 June 1 placeholder를 대체한다.

## 소스 인벤토리

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-text`

- `tokenizer-core` provides request/response models, `Severity`,
  `BlockwordRequest`, `BlockwordResponse`, dictionary loading helpers,
  `CharArraySet`, and `CharArrayMap`.
- `text-search` provides an immutable Aho-Corasick automaton with generic
  values, first/all match modes, overlap control, replacement, tokenization,
  Unicode NFC/NFKC normalization with offset mapping, Latin/whitespace boundary
  modes, and Kotlin Flow matching.
- `tokenizer-korean` provides a large first-party Korean NLP stack:
  normalization, 26-class POS tokenization, noun-focused tokenization, phrase
  extraction, stemming, sentence splitting, detokenization, chunking,
  runtime noun/blockword dictionary updates, and thread-safe facade methods.
  Its source resource corpus is about 2.9 MiB across 39 dictionary files.
- `tokenizer-japanese` uses Kuromoji IPAdic for Japanese morphological analysis,
  POS helpers, noun/verb filtering, compound blockword detection, masking, and
  runtime blockword dictionary updates. Its local source resources are small,
  but the tokenizer dictionary comes from the upstream Kuromoji dependency.
- `lingua` wraps upstream Lingua with builder helpers, mixed-language
  detection, Unicode script helpers, and an explicit warning that detectors
  should be reused because model loading is expensive.

## 현재 Go Ecosystem 증거

- Aho-Corasick에는 여러 Go library가 있지만 tradeoff가 다르다. Cloudflare package는
  established이고 BSD-licensed지만 module metadata에 tagged release가 없다.
  `orcasecurity/aho-corasick`은 새롭고 Rust 구현에서 영향을 받았지만 adoption signal이
  약하다. `rrethy/ahocorasick`은 tagged MIT release가 있으나 maintenance surface가 작다.
  `coregx/ahocorasick`은 새롭고 performance-focused지만 이 repo보다 새 Go version을
  요구한다.
- #52는 raw throughput claim보다 Unicode normalization, offset mapping, boundary
  behavior, replacement, masking integration이 더 중요하므로, bluetape-go에서는 작은
  first-party Aho-Corasick implementation이 현실적이다. Package adoption은 benchmark
  evidence가 필요성을 증명할 때까지 defer할 수 있다.
- `ikawaha/kagome/v2`는 가장 강한 Japanese tokenizer 후보이다. pure Go이고 active,
  tagged, Go-native다. 다만 dictionary/model size와 deployment cost가 있으므로
  tokenizer core 밖에 둬야 한다.
- `pemistahl/lingua-go`는 mixed-language support를 포함해 language detection에서 가장
  가까운 parity 후보이다. 자체 documentation이 모든 high-accuracy language model loading
  시 high memory use를 언급하므로 optional package와 subset-building API 뒤에 격리해야 한다.
- `abadojack/whatlanggo`는 더 단순하고 pure Go이며 dependency-light지만 최신 module
  release가 오래됐다. 첫 parity choice가 아니라 fallback 또는 comparison target으로 둔다.
- Korean tokenization은 Go adoption path가 가장 약하다. Source module의 normalization,
  POS, stemming, phrase extraction, sentence splitting, dictionary update,
  blockword behavior에 맞는 mature Go-native Korean tokenizer 증거를 찾지 못했다.
  직접 port는 helper package가 아니라 큰 NLP project다.

## 순위

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Multi-pattern search | High | Medium | Implement first-party #52; keep API small and benchmark before adopting external engines. |
| Blockword detection/masking | High | Medium | Implement #53 on top of compiled search plus static/rebuildable dictionaries. |
| Tokenizer core models | Medium/high | Medium | Implement #54 only as models and interfaces needed by search/masking; avoid NLP dependency coupling. |
| Unicode script helpers | Medium/high | Low/medium | Implement small helpers when needed by #53/#55; prefer `unicode` and `x/text`. |
| Japanese tokenizer | Medium | Medium/high | Adopt Kagome in optional follow-up after #55; do not put it in core. |
| Language detection | Medium | High | Adopt Lingua-Go only behind optional package with subset and memory guidance; compare Whatlanggo. |
| Korean tokenizer | Low/medium | High | Defer full POS/stemming/phrase parity; allow only small normalization or dictionary examples if later justified. |
| Runtime dictionary mutation | Medium | Medium/high | Prefer immutable compiled dictionaries plus explicit rebuild/swap workflow; runtime mutation needs stress/race proof. |
| Streaming/Flow matching | Medium | Medium | Translate to iterator/channel only after synchronous API stabilizes; no goroutine-heavy API first. |

## 구현

- #52 first-party package for deterministic multi-pattern search:
  immutable compiled automata, `Contains`, `First`, `FindAll`, replacement,
  overlap policy, duplicate pattern handling, Unicode normalization, and
  explicit boundary modes.
- #53 blockword masking on top of #52:
  severity metadata, match reporting, masking transform, Korean/Japanese/ASCII
  fixtures, and static dictionary rebuild semantics.
- #54 narrow core models:
  token span, normalized text, dictionary entry/provider, severity,
  blockword request/response, and optional tokenizer interfaces. Keep the
  package usable without Korean/Japanese/language-detection dependencies.

## 채택

- Adopt Kagome only in a separate optional Japanese tokenizer package if #55
  confirms dictionary size, license, POS mapping, and benchmark/test shape.
- Adopt Lingua-Go only in an optional language-detection package if #55 proves
  acceptable memory, subset-building ergonomics, and deployment cost.
- Compare Whatlanggo as a lightweight language/script detector, not as parity
  replacement for Lingua unless the memory budget is the deciding constraint.

## Example-only

- Korean/Japanese blockword examples can demonstrate Unicode normalization and
  dictionary masking without claiming full morphological analysis.
- Language-detection examples should show detector reuse, language subset
  construction, and memory caveats.

## Defer

- Full Korean POS tokenizer, stemming, phrase extraction, sentence splitting,
  detokenization, and runtime dictionary extension.
- Japanese Kuromoji parity. Go should use Kagome if it needs Japanese NLP,
  not port Kuromoji-shaped JVM APIs.
- Runtime mutable dictionaries in public APIs until stress/race tests prove
  safe swaps under concurrent readers.
- Flow-style asynchronous matching until the synchronous API and blockword
  package prove caller value.

## 필요한 Issue 업데이트

- #45: record the implementation order as #52, #53, #54, then optional #55
  dependency-backed packages.
- #52: keep as first-party Aho-Corasick/search implementation and require
  Unicode offset/boundary tests plus benchmark evidence before external engine
  adoption.
- #53: use #52 compiled search for masking; keep dictionary updates rebuildable
  or immutable unless stress/race tests prove runtime mutation.
- #54: narrow tokenizer core to shared models and extension points only.
- #55: split Korean defer, Japanese Kagome adoption research, and Lingua-Go vs
  Whatlanggo language-detection comparison into explicit decisions.

## 검증 계획

- Documentation-only PR에서는 `git diff --check`와 targeted `rg`를 실행한다.
- #45/#52-#55 issue body가 #39 research update를 포함하는지 확인한다.
- External evidence는 `bluetape4k-wiki`에 보존하고
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`로 검증한다.

## 후속 권고

#54를 모든 `tokenizer-core`와 Korean/Japanese concept port로 시작하지 않는다. #52는
가장 작은 compiled search API로 시작하고, #53 masking이 실제로 필요한
dictionary/core model surface를 드러내게 한다.
