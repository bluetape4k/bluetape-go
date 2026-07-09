# Issue #436 NLP Memory Benchmark Lesson

Optional NLP adapter benchmarks need both warm steady-state rows and one-shot
startup evidence.

- Kagome IPA dictionary startup can be hidden by package-level warm caches in
  normal Go benchmark calibration; keep isolated one-subcase `-benchtime=1x`
  process snapshots when first-load cost matters.
- A single-process `-bench . -benchtime=1x` run is useful as a smoke artifact,
  but benchmark order and caches can contaminate later startup rows.
- Lingua high-accuracy first use is the meaningful cost center. Low-accuracy
  subsets and detector reuse should remain the default recommendation for
  predictable service behavior.
- Module cache size and process RSS snapshots are useful deployment signals,
  but they are local evidence, not production memory limits.
- Keep Kagome and Lingua behind `textsearch/japanese` and
  `textsearch/language`; verify the core `textsearch` package stays free of
  those optional dependencies.
