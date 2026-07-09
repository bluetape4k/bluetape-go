# Issue #435 Textsearch Benchmark Lesson

Raw Aho-Corasick speed is not enough to justify replacing `textsearch`.

- Benchmark external engines only against comparable raw matching behavior;
  keep normalization, original byte spans, boundaries, replacement, masking, and
  blockword processing measured on the first-party API.
- A production replacement needs both a measured caller bottleneck and semantic
  parity proof. A microbenchmark win alone should stay as evidence, not a
  dependency decision.
- Cloudflare is a strong steady-state raw matcher candidate, but its build
  allocation and untagged module version need adoption caution.
- RRethy is attractive for large-dictionary compile cost, but its smaller
  adoption signal and allocating match API need caller-specific profiling.
- Keep benchmark outputs with environment metadata so later optimization work
  can compare against the same dictionary and input shapes instead of repeating
  ecosystem research.
