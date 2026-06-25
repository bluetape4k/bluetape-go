# Issue #222 Lessons

- Focused testing examples are a better Go fit than a public assertion DSL when
  standard `testing`, `cmp.Diff`, package-local builders, and existing
  bluetape-go helpers already cover the workflow.
- Golden fixtures should stay under package-local `testdata`; `TempOutputPath`
  is for generated output, not canonical expected files.
- Seeded `math/rand/v2` examples should assert the exact generated values so CI
  proves determinism instead of only demonstrating random-looking data.
- Importing `cmp.Diff` directly means `github.com/google/go-cmp` must be a direct
  module dependency even if it was already present transitively.
