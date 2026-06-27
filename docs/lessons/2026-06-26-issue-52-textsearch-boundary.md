# Issue #52 Textsearch Boundary

`textsearch` should stay a deterministic compiled search package, not a full
tokenizer or moderation system.

- Immutable compiled matchers keep concurrent reads simple and race-testable.
- Replacement and masking are integration hooks for #53, but masking remains
  helper behavior and not a security boundary.
- Unicode normalization is useful for caller ergonomics, but offset reporting
  must continue to point at original input byte spans.
- Boundary modes are explicit and limited to ASCII or Unicode word runes.
- External Aho-Corasick engines remain deferred until benchmarks prove the
  first-party implementation is insufficient.
