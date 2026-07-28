# Issue #435 Textsearch Benchmark Evidence

Issue #435는 first-party `textsearch` matcher와 benchmark-only Aho-Corasick candidate 두 개에 대한 benchmark evidence를
추가한다. 목표는 adoption discipline이다. production dependency를 검토하기 전에 compile cost, steady-state matching, overlap,
Unicode normalization, no-match-heavy input, replacement, masking을 측정한다.

## Artifacts

- raw benchmark output: `docs/research/outputs/issue-435/textsearch-bench.txt`
- environment 및 candidate metadata: `docs/research/outputs/issue-435/environment.md`
- benchmark source: `textsearch/matcher_benchmark_test.go`

## Benchmark Scope

first-party benchmark case:

- `small_success_contains`: small dictionary, repeated success match, case folding.
- `medium_no_match_heavy`: 128-pattern dictionary, repeated no-match input.
- `large_success_tail`: 2048-pattern dictionary, input tail 근처의 success match.
- `overlap_leftmost_longest`: `he`, `she`, `hers`, `hero` 같은 overlapping pattern.
- `unicode_nfkc_case`: Korean, Japanese, Latin accent composition, compatibility kana input을 가진 NFKC/case-folding path.
- `Matcher.Replace`, `Matcher.Mask`, `BlockwordDictionary.Process`를 통한 replacement 및 masking.

candidate benchmark case는 의도적으로 더 좁다. Cloudflare와 RRethy는 API가 비교 가능한 raw string matching에서만 측정한다.
candidate `Contains`는 비교하지 않는다. Cloudflare는 early-exit `Contains` API를 노출하지만 RRethy는 `FindAllString`만 노출하므로
`len(FindAllString(...)) > 0`은 equivalent early exit이 아니라 match materialization을 측정한다. candidate는 `textsearch`의
offset mapping, boundary filtering, replacement, masking, normalized Unicode equivalence도 덮지 않는다.

## Candidate Metadata

| Candidate | Module version | Go version | License | Repository signal | API fit |
|---|---|---:|---|---|---|
| `github.com/cloudflare/ahocorasick` | `v0.0.0-20240916140611-054963ec9396` | no `GoVersion` in `go list -m -json` | BSD-3-Clause | not archived; 723 stars; pushed 2026-04-24; no semantic tag in `go list -m -versions` | raw `Contains`/`Match`는 빠르지만 byte-oriented ID뿐이며 first-party offset remap, normalization, boundary, replacement, masking surface가 없다. |
| `github.com/rrethy/ahocorasick` | `v1.0.0` | `1.19` | MIT | not archived; 28 stars; pushed 2024-11-29 | tagged and simple이지만 `FindAllString`이 match object를 allocate하고 raw matching에 머문다. |

## Result Highlights

Apple M5, Go `go1.26.5 darwin/arm64`에서 측정했다.

| Case | First-party | Cloudflare | RRethy | Interpretation |
|---|---:|---:|---:|---|
| Compile, small dictionary | `2051 ns/op`, `7192 B/op` | `11998 ns/op`, `91504 B/op` | `7414 ns/op`, `98776 B/op` | small dictionary compile cost는 first-party가 낮다. |
| Compile, 2048 patterns | `531083 ns/op`, `1225960 B/op` | `2771014 ns/op`, `68615036 B/op` | `100642 ns/op`, `523720 B/op` | RRethy가 large raw dictionary를 가장 빨리 compile한다. Cloudflare는 build allocation이 높다. |
| Contains, no-match-heavy | `38511 ns/op`, `94456 B/op` | not comparable | not comparable | candidate contains는 equivalent early-exit behavior가 없어 제외한다. |
| Contains, large success tail | `8191 ns/op`, `18352 B/op` | not comparable | not comparable | first-party contains는 계속 측정한다. candidate ranking은 compile 및 find-all만 사용한다. |
| Overlap find-all | `64510 ns/op`, `451346 B/op` | `2620 ns/op`, `64 B/op` | `15491 ns/op`, `49984 B/op` | raw engine은 overlap-heavy input에서 큰 matching-speed gap을 보인다. |
| Unicode NFKC + case | `56999 ns/op`, `171513 B/op` | not comparable | not comparable | external candidate는 normalized Unicode span behavior parity를 증명하지 않는다. |
| Replacement/masking | `74204-76405 ns/op`, `242193-261777 B/op` | not comparable | not comparable | external candidate는 integrated caller behavior를 덮지 않는다. |

## 결정

이 issue에서는 production matcher를 교체하지 않는다.

raw matching gap은 실제이며, 특히 Cloudflare steady-state matching과 RRethy large-dictionary compile cost에서 두드러진다.
하지만 #435는 production bottleneck, latency target, API-compatible dependency win을 제시하지 않는다. first-party package는
candidate가 제공하지 않는 behavior를 계속 소유한다: original-byte-span reporting을 가진 normalization, boundary mode, overlap
policy, replacement, masking, blockword integration.

candidate는 현재 benchmark-only dependency로 유지한다. 나중에 production profile이 `textsearch` matching cost가 user-visible함을
보이면 existing `Matcher` behavior 뒤 internal adapter prototype을 만들고 semantic parity를 증명하는 좁은 follow-up을 연다.

## Follow-up Issue

이 run에서는 follow-up issue를 열지 않는다. issue acceptance는 measured bottleneck 또는 dependency win이 증명될 때만 follow-up을
요구한다. 이 run은 raw benchmark gap은 증명하지만 end-to-end bottleneck 또는 semantic parity win은 증명하지 않는다.
