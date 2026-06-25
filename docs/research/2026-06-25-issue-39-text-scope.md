# Issue 39 Text Research Scope

Issue #39 is the 0.7.0 research gate for deciding how much of
`bluetape4k-text` should become Go packages in the 0.10.0 text milestone.
This note supersedes the broad June 1 placeholder for concrete scope decisions
on #45 and #52-#55.

## Source Inventory

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

## Current Go Ecosystem Evidence

- Aho-Corasick has several Go libraries, but their tradeoffs vary. Cloudflare's
  package is established and BSD-licensed, but module metadata has no tagged
  release. `orcasecurity/aho-corasick` is fresh and inspired by Rust's
  implementation, but has little adoption signal. `rrethy/ahocorasick` has a
  tagged MIT release but a small maintenance surface. `coregx/ahocorasick` is
  new and performance-focused, but requires a newer Go version than this repo.
- A small first-party Aho-Corasick implementation remains realistic for
  bluetape-go because #52 needs Unicode normalization, offset mapping,
  boundary behavior, replacement, and masking integration more than raw
  throughput claims. Adopting a package can be deferred until benchmark
  evidence proves it is needed.
- `ikawaha/kagome/v2` is the strongest Japanese tokenizer candidate: pure Go,
  active, tagged, and Go-native. It still brings dictionary/model size and
  deployment cost that should stay outside the tokenizer core.
- `pemistahl/lingua-go` is the closest parity candidate for language detection,
  including mixed-language support. Its own documentation notes high memory use
  when all high-accuracy language models are loaded, so it should be isolated
  behind an optional package and subset-building API.
- `abadojack/whatlanggo` is simpler, pure Go, and dependency-light, but its
  latest module release is old. It can be a fallback or comparison target, not
  the first parity choice.
- Korean tokenization is the weakest Go adoption path. Evidence found no
  mature Go-native Korean tokenizer that matches the source module's
  normalization, POS, stemming, phrase extraction, sentence splitting,
  dictionary update, and blockword behavior. A direct port would be a large NLP
  project, not a helper package.

## Ranking

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

## Implement

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

## Adopt

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

## Issue Updates Required

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

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify #45/#52-#55 issue bodies contain the #39 research update.
- Preserve external evidence in `bluetape4k-wiki` and validate with
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`.

## Follow-up Recommendation

Do not begin #54 by porting all `tokenizer-core` and Korean/Japanese concepts.
Start #52 with the smallest compiled search API, then let #53 masking expose
the dictionary/core model surface that is actually needed.
