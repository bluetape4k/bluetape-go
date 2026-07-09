# Issue #435 Textsearch Benchmark Evidence

Issue #435 adds benchmark evidence for the first-party `textsearch` matcher and
two benchmark-only Aho-Corasick candidates. The goal is adoption discipline:
measure compile cost, steady-state matching, overlap, Unicode normalization,
no-match-heavy input, replacement, and masking before considering any
production dependency.

## Artifacts

- Raw benchmark output:
  `docs/research/outputs/issue-435/textsearch-bench.txt`
- Environment and candidate metadata:
  `docs/research/outputs/issue-435/environment.md`
- Benchmark source:
  `textsearch/matcher_benchmark_test.go`

## Benchmark Scope

First-party benchmark cases:

- `small_success_contains`: small dictionary, repeated success matches, case
  folding.
- `medium_no_match_heavy`: 128-pattern dictionary, repeated no-match input.
- `large_success_tail`: 2048-pattern dictionary, successful match near input
  tail.
- `overlap_leftmost_longest`: overlapping patterns such as `he`, `she`,
  `hers`, and `hero`.
- `unicode_nfkc_case`: NFKC and case-folding path with Korean, Japanese,
  Latin accent composition, and compatibility kana input.
- replacement and masking via `Matcher.Replace`, `Matcher.Mask`, and
  `BlockwordDictionary.Process`.

Candidate benchmark cases are intentionally narrower. Cloudflare and RRethy are
measured only on raw string matching where their APIs are comparable. Candidate
`Contains` is not compared because Cloudflare exposes an early-exit `Contains`
API while RRethy exposes `FindAllString`; treating `len(FindAllString(...)) > 0`
as contains would measure match materialization instead of equivalent early
exit behavior. The candidates also do not cover `textsearch` offset mapping,
boundary filtering, replacement, masking, or normalized Unicode equivalence.

## Candidate Metadata

| Candidate | Module version | Go version | License | Repository signal | API fit |
|---|---|---|---|---|---|
| `github.com/cloudflare/ahocorasick` | `v0.0.0-20240916140611-054963ec9396` | No `GoVersion` field observed in `go list -m -json`. | BSD-3-Clause | Not archived; 723 stars; pushed 2026-04-24; no semantic tag observed by `go list -m -versions`. | Very fast raw `Contains`/`Match`, but byte-oriented IDs only; no first-party offset remap, normalization, boundary, replacement, or masking surface. |
| `github.com/rrethy/ahocorasick` | `v1.0.0` | `1.19` | MIT | Not archived; 28 stars; pushed 2024-11-29. | Tagged and simple, but `FindAllString` allocates match objects and remains raw matching only. |

## Result Highlights

Measured on Apple M5, Go `go1.26.5 darwin/arm64`.

| Case | First-party | Cloudflare | RRethy | Interpretation |
|---|---:|---:|---:|---|
| Compile, small dictionary | `2051 ns/op`, `7192 B/op` | `11998 ns/op`, `91504 B/op` | `7414 ns/op`, `98776 B/op` | First-party compile cost is lower for small dictionaries. |
| Compile, 2048 patterns | `531083 ns/op`, `1225960 B/op` | `2771014 ns/op`, `68615036 B/op` | `100642 ns/op`, `523720 B/op` | RRethy compiles large raw dictionaries fastest; Cloudflare has high build allocation. |
| Contains, no-match-heavy | `38511 ns/op`, `94456 B/op` | Not comparable | Not comparable | Candidate contains is omitted because their APIs do not expose equivalent early-exit behavior. |
| Contains, large success tail | `8191 ns/op`, `18352 B/op` | Not comparable | Not comparable | First-party contains remains covered; candidate ranking uses compile and find-all only. |
| Overlap find-all | `64510 ns/op`, `451346 B/op` | `2620 ns/op`, `64 B/op` | `15491 ns/op`, `49984 B/op` | Raw engines show a large matching-speed gap on overlap-heavy input. |
| Unicode NFKC + case | `56999 ns/op`, `171513 B/op` | Not comparable | Not comparable | External candidates do not prove parity for normalized Unicode span behavior. |
| Replacement/masking | `74204-76405 ns/op`, `242193-261777 B/op` | Not comparable | Not comparable | External candidates do not cover the integrated caller behavior. |

## Decision

Do not replace the production matcher in this issue.

The raw matching gap is real, especially Cloudflare steady-state matching and
RRethy large-dictionary compile cost. However, #435 does not provide a
production bottleneck, latency target, or API-compatible dependency win. The
first-party package still owns behavior that the candidates do not provide:
normalization with original-byte-span reporting, boundary modes, overlap policy,
replacement, masking, and blockword integration.

Keep the candidates as benchmark-only dependencies for now. If production
profiling later shows `textsearch` matching cost is user-visible, create a
narrow follow-up that prototypes an internal adapter behind the existing
`Matcher` behavior and proves semantic parity before any public API change.

## Follow-up Issue

No follow-up issue is opened from this run. The issue acceptance asks for a
follow-up only if a measured bottleneck or dependency win is proven. This run
proves a raw benchmark gap, but not an end-to-end bottleneck or semantic parity
win.
