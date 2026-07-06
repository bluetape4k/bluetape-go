# Cross-Repo SerDe Recommendation Matrix

Issue #402 publishes recommendations only after raw evidence exists.

## Lessons

- Keep recommendation language scoped to the benchmark evidence. A matrix can
  name first candidates, but it must not silently change defaults.
- Separate wire-format, trust-boundary, and speed concerns. JVM Fory/Kryo speed
  does not make those formats safe for untrusted or cross-language boundaries.
- Treat missing adapter evidence as a real result. Rust serialization is
  contract-first today, so Go/JVM rows should not create Rust adapter claims.
- Keep user READMEs short. Detailed evidence belongs in `docs/research`; package
  READMEs should only expose stable caller guidance.

## Evidence

- `docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md`
- `docs/research/outputs/issue-401/`
- `serialization/README.md` and `serialization/README.ko.md`
- `codec/README.md` and `codec/README.ko.md`
- `compression/README.md` and `compression/README.ko.md`
