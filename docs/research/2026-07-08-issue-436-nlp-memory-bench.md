# Issue #436 NLP Adapter Memory Benchmark Evidence

Issue #436은 optional NLP package를 측정해 Kagome과 Lingua-Go 비용을 명시적으로 유지한다. benchmark suite는
package-local startup, construction plus first use, steady-state tokenization/detection, POS filtering, allocation,
dependency module size, isolated process RSS snapshot을 덮는다.

## Artifacts

- raw acceptance benchmark: `docs/research/outputs/issue-436/nlp-bench.txt`
- isolated one-case-per-process startup/RSS snapshot: `docs/research/outputs/issue-436/nlp-cold-start-isolated.txt`
- single-process startup smoke output: `docs/research/outputs/issue-436/nlp-startup-benchtime-1x.txt`
- environment, dependency version, Go version, module cache size: `docs/research/outputs/issue-436/environment.md`
- benchmark source: `textsearch/japanese/tokenizer_benchmark_test.go`, `textsearch/language/detector_benchmark_test.go`

## Run Conditions

- Date: 2026-07-08
- OS/arch: `darwin/arm64`
- CPU: Apple M5
- Go: `go1.26.5`
- Command:
  `go test -run '^$' -bench . -benchmem ./textsearch/japanese ./textsearch/language`
- isolated startup/RSS command shape:
  `/usr/bin/time -l go test -run '^$' -bench '^BenchmarkName$/^case$' -benchtime=1x -benchmem ./package`
- single-process smoke command:
  `/usr/bin/time -l go test -run '^$' -bench . -benchtime=1x -benchmem ./textsearch/japanese ./textsearch/language`

## Dependency Footprint

| Module | Version | Go version | Module cache size |
|---|---|---:|---:|
| `github.com/ikawaha/kagome/v2` | `v2.11.0` | `1.24.0` | 31M |
| `github.com/ikawaha/kagome-dict` | `v1.1.7` | `1.24.0` | 4.5M |
| `github.com/ikawaha/kagome-dict/ipa` | `v1.2.6` | `1.24.0` | 22M |
| `github.com/pemistahl/lingua-go` | `v1.4.0` | `1.18` | 123M |

isolated startup snapshot은 Japanese/Kagome case에서 `154714112`부터 `156368896` bytes maximum resident set size를,
Lingua-Go case에서 `386465792`부터 `387448832` bytes를 보고했다. 이는 local process snapshot이지 production memory limit이 아니다.

## Japanese Kagome Results

`nlp-bench.txt`의 warm steady-state row:

| Case | Result |
|---|---:|
| `BenchmarkTokenizerConstruction/normal` | `42.79 ns/op`, `128 B/op`, `4 allocs/op` |
| `BenchmarkTokenizerConstructionAndFirstUse/short` | `5169 ns/op`, `8536 B/op`, `102 allocs/op` |
| `BenchmarkTokenizerTokenize/normal/medium` | `18647 ns/op`, `25303 B/op`, `295 allocs/op` |
| `BenchmarkTokenizerTokenize/normal/large` | `438346 ns/op`, `615754 B/op`, `7050 allocs/op` |
| `BenchmarkTokenizerTokenize/search/large` | `616160 ns/op`, `709276 B/op`, `8139 allocs/op` |
| `BenchmarkTokenizerFilterPOS/nouns` | `454.8 ns/op`, `1536 B/op`, `1 alloc/op` |
| `BenchmarkTokenizerFilterPOS/verbs` | `419.1 ns/op`, `1536 B/op`, `1 alloc/op` |

isolated startup snapshot은 cold package/dictionary shape를 더 분명하게 보여 준다.

| Case | Result |
|---|---:|
| `BenchmarkTokenizerConstruction/normal` | `281207666 ns/op`, `221170304 B/op`, `5594008 allocs/op`, `154714112 RSS` |
| `BenchmarkTokenizerConstructionAndFirstUse/large` | `284994834 ns/op`, `222528032 B/op`, `5609775 allocs/op`, `156368896 RSS` |

해석: Kagome IPA dictionary initialization이 의미 있는 startup cost다. dictionary가 process 안에서 warm 상태가 되면
`NewTokenizer`는 저렴하고 steady tokenization cost는 input length와 tokenize mode에 비례한다.

## Lingua-Go Results

`nlp-bench.txt`의 warm steady-state row:

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

isolated startup snapshot은 first model load cost를 보여 준다.

| Case | Result |
|---|---:|
| `BenchmarkDetectorConstructionAndFirstUse/subset_lazy_high_accuracy` | `90440000 ns/op`, `132146368 B/op`, `1490291 allocs/op`, `386465792 RSS` |
| `BenchmarkDetectorConstructionAndFirstUse/subset_lazy_low_accuracy` | `15611667 ns/op`, `7321296 B/op`, `96071 allocs/op`, `386875392 RSS` |
| `BenchmarkDetectorConstruction/subset_preloaded_low_accuracy` | `7975500 ns/op`, `7229128 B/op`, `95419 allocs/op`, `387448832 RSS` |
| `BenchmarkDetectorConstructionAndFirstUse/all_lazy_low_accuracy` | `130951875 ns/op`, `146485976 B/op`, `1967005 allocs/op`, `386514944 RSS` |

해석: high-accuracy first use와 all-language first use가 의미 있는 Lingua cold-start cost center다. low-accuracy subset은
allocation과 first-use time이 훨씬 저렴하고, broad Latin/all-language steady-state detection은 English text에서 domain subset보다
여전히 느리다.

## 결정

이 issue에서는 production API change가 필요하지 않다.

기존 README guidance는 계속 맞다.

- `textsearch/japanese`와 `textsearch/language`는 optional로 유지한다.
- tokenizer/detector는 한 번 만들고 재사용한다.
- domain을 알고 있으면 selected language subset을 우선한다.
- memory/startup cost가 short-text accuracy보다 중요하면 `WithLowAccuracyMode`를 사용한다.
- local benchmark output을 production memory limit으로 취급하지 않는다.

새 follow-up issue는 만들지 않는다. 측정값은 meaningful startup 및 first-use cost를 보여 주지만, implementation work가 필요한
missing reuse/subset API 또는 avoidable per-call allocation 문제는 보이지 않는다.
