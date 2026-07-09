# Issue #436 NLP Adapter Memory Benchmark Evidence

Issue #436 measures optional NLP packages so Kagome and Lingua-Go costs stay
explicit. The benchmark suite covers package-local startup, construction plus
first use, steady-state tokenization/detection, POS filtering, allocations,
dependency module sizes, and isolated process RSS snapshots.

## Artifacts

- Raw acceptance benchmark:
  `docs/research/outputs/issue-436/nlp-bench.txt`
- Isolated one-case-per-process startup/RSS snapshots:
  `docs/research/outputs/issue-436/nlp-cold-start-isolated.txt`
- Single-process startup smoke output:
  `docs/research/outputs/issue-436/nlp-startup-benchtime-1x.txt`
- Environment, dependency versions, Go versions, and module cache sizes:
  `docs/research/outputs/issue-436/environment.md`
- Benchmark sources:
  `textsearch/japanese/tokenizer_benchmark_test.go`
  `textsearch/language/detector_benchmark_test.go`

## Run Conditions

- Date: 2026-07-08
- OS/arch: `darwin/arm64`
- CPU: Apple M5
- Go: `go1.26.5`
- Command:
  `go test -run '^$' -bench . -benchmem ./textsearch/japanese ./textsearch/language`
- Isolated startup/RSS command shape:
  `/usr/bin/time -l go test -run '^$' -bench '^BenchmarkName$/^case$' -benchtime=1x -benchmem ./package`
- Single-process smoke command:
  `/usr/bin/time -l go test -run '^$' -bench . -benchtime=1x -benchmem ./textsearch/japanese ./textsearch/language`

## Dependency Footprint

| Module | Version | Go version | Module cache size |
|---|---|---:|---:|
| `github.com/ikawaha/kagome/v2` | `v2.11.0` | `1.24.0` | 31M |
| `github.com/ikawaha/kagome-dict` | `v1.1.7` | `1.24.0` | 4.5M |
| `github.com/ikawaha/kagome-dict/ipa` | `v1.2.6` | `1.24.0` | 22M |
| `github.com/pemistahl/lingua-go` | `v1.4.0` | `1.18` | 123M |

Isolated startup snapshots reported `154714112` to `156368896` bytes maximum
resident set size for Japanese/Kagome cases and `386465792` to `387448832`
bytes for Lingua-Go cases. Treat these as local process snapshots, not
production memory limits.

## Japanese Kagome Results

Warm steady-state rows from `nlp-bench.txt`:

| Case | Result |
|---|---:|
| `BenchmarkTokenizerConstruction/normal` | `42.79 ns/op`, `128 B/op`, `4 allocs/op` |
| `BenchmarkTokenizerConstructionAndFirstUse/short` | `5169 ns/op`, `8536 B/op`, `102 allocs/op` |
| `BenchmarkTokenizerTokenize/normal/medium` | `18647 ns/op`, `25303 B/op`, `295 allocs/op` |
| `BenchmarkTokenizerTokenize/normal/large` | `438346 ns/op`, `615754 B/op`, `7050 allocs/op` |
| `BenchmarkTokenizerTokenize/search/large` | `616160 ns/op`, `709276 B/op`, `8139 allocs/op` |
| `BenchmarkTokenizerFilterPOS/nouns` | `454.8 ns/op`, `1536 B/op`, `1 alloc/op` |
| `BenchmarkTokenizerFilterPOS/verbs` | `419.1 ns/op`, `1536 B/op`, `1 alloc/op` |

Isolated startup snapshots show the cold package/dictionary shape more clearly:

| Case | Result |
|---|---:|
| `BenchmarkTokenizerConstruction/normal` | `281207666 ns/op`, `221170304 B/op`, `5594008 allocs/op`, `154714112 RSS` |
| `BenchmarkTokenizerConstructionAndFirstUse/large` | `284994834 ns/op`, `222528032 B/op`, `5609775 allocs/op`, `156368896 RSS` |

Interpretation: Kagome IPA dictionary initialization is the meaningful startup
cost. Once the dictionary is warm in-process, `NewTokenizer` is cheap and the
steady tokenization cost is proportional to input length and tokenize mode.

## Lingua-Go Results

Warm steady-state rows from `nlp-bench.txt`:

| Case | Result |
|---|---:|
| `BenchmarkDetectorConstruction/subset_lazy_low_accuracy` | `10874 ns/op`, `13280 B/op`, `1385 allocs/op` |
| `BenchmarkDetectorConstruction/all_lazy_low_accuracy` | `15683 ns/op`, `19720 B/op`, `1399 allocs/op` |
| `BenchmarkDetectorConstructionAndFirstUse/subset_lazy_high_accuracy` | `124650 ns/op`, `76965 B/op`, `3214 allocs/op` |
| `BenchmarkDetectorConstructionAndFirstUse/subset_lazy_low_accuracy` | `92915 ns/op`, `34939 B/op`, `2053 allocs/op` |
| `BenchmarkDetectorConstructionAndFirstUse/all_lazy_low_accuracy` | `531890 ns/op`, `158172 B/op`, `5484 allocs/op` |
| `BenchmarkDetectorDetect/subset_low_accuracy_english_short` | `81474 ns/op`, `21641 B/op`, `668 allocs/op` |
| `BenchmarkDetectorDetect/subset_low_accuracy_japanese_short` | `5711 ns/op`, `1817 B/op`, `70 allocs/op` |
| `BenchmarkDetectorDetect/subset_low_accuracy_english_medium` | `1164313 ns/op`, `77132 B/op`, `5140 allocs/op` |
| `BenchmarkDetectorDetect/latin_script_low_accuracy_english` | `470824 ns/op`, `136353 B/op`, `3981 allocs/op` |
| `BenchmarkDetectorDetect/all_low_accuracy_english` | `512562 ns/op`, `138364 B/op`, `4085 allocs/op` |
| `BenchmarkDetectorConfidences` | `40654 ns/op`, `11010 B/op`, `335 allocs/op` |
| `BenchmarkDetectorDetectMultiple` | `853115 ns/op`, `318926 B/op`, `10261 allocs/op` |

Isolated startup snapshots show the first model load cost:

| Case | Result |
|---|---:|
| `BenchmarkDetectorConstructionAndFirstUse/subset_lazy_high_accuracy` | `90440000 ns/op`, `132146368 B/op`, `1490291 allocs/op`, `386465792 RSS` |
| `BenchmarkDetectorConstructionAndFirstUse/subset_lazy_low_accuracy` | `15611667 ns/op`, `7321296 B/op`, `96071 allocs/op`, `386875392 RSS` |
| `BenchmarkDetectorConstruction/subset_preloaded_low_accuracy` | `7975500 ns/op`, `7229128 B/op`, `95419 allocs/op`, `387448832 RSS` |
| `BenchmarkDetectorConstructionAndFirstUse/all_lazy_low_accuracy` | `130951875 ns/op`, `146485976 B/op`, `1967005 allocs/op`, `386514944 RSS` |

Interpretation: high-accuracy first use and all-language first use are the
meaningful Lingua cold-start cost centers. Low-accuracy subsets remain much
cheaper in allocation and first-use time, while broad Latin/all-language
steady-state detection is still materially slower than a domain subset for
English text.

## Decision

No production API change is needed in this issue.

Existing README guidance remains directionally correct:

- keep `textsearch/japanese` and `textsearch/language` optional,
- build tokenizers/detectors once and reuse them,
- prefer selected language subsets when the domain is known,
- use `WithLowAccuracyMode` when memory/startup cost matters more than
  short-text accuracy,
- avoid treating local benchmark output as a production memory limit.

No follow-up issue is created. The measurements show meaningful startup and
first-use costs, but no missing reuse/subset API or avoidable per-call
allocation problem that requires implementation work.
