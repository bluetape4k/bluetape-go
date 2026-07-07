# Issue #456 JSON Repeated Collection Profile Lesson

Date: 2026-07-07
Scope: `serialization.JSONSerializer`

## Lesson

Default JSON decode does not need `json.Decoder` just to reject trailing JSON
values. `json.Unmarshal` already rejects trailing non-whitespace data and avoids
the decoder refill buffer allocation. Keep `json.Decoder` only for options that
need it, such as `DisallowUnknownFields`.

## Pattern

- Profile before optimizing benchmark rows with high allocation counts.
- Preserve raw `-benchmem` output and `pprof -alloc_space` top output before
  and after the change.
- Keep strict behavior tests around corrupt input, empty input, trailing JSON,
  and unknown-field rejection.
- Do not add pooling for decoded JSON object graphs unless tests prove no
  caller-visible aliasing or race risk.

## Follow-Up

#455 should profile compression allocation separately. The #456 result does not
change `compression.Default()` or zstd policy.
