# Issue #456 JSON Repeated Collection Profile

Issue: #456
Milestone: 0.15.0
Date: 2026-07-07

## Goal

Profile allocation sources in JSON repeated collection decode and round-trip
benchmarks, then decide whether a narrow optimization is justified.

## Baseline

The retained #401 serialization snapshot reported:

- `BenchmarkSerializationDecode/JSON/serde-repeated-collection-v1`:
  `2,062,641 B/op`, `19,234 allocs/op`.
- `BenchmarkSerializationRoundTrip/JSON/serde-repeated-collection-v1`:
  `3,510,457 B/op`, `27,658 allocs/op`.

Fresh #456 five-run baseline on the same fixture reported:

- Decode: `2,062,640-2,062,641 B/op`, `19,234 allocs/op`.
- RoundTrip: `3,460,902-3,513,236 B/op`, `27,657-27,658 allocs/op`.

## Profile Finding

Baseline decode allocation profile:

- `encoding/json.(*Decoder).refill`: `734.32MB`, `50.29%` of alloc space.
- `reflect.mapassign_faststr0`: `244.07MB`, `16.71%`.
- `reflect.growslice`: `219.81MB`, `15.05%`.
- `encoding/json.(*decodeState).literalStore`: `136.01MB`, `9.31%`.

The wrapper used `json.NewDecoder(bytes.NewReader(data))` for every default
decode so it could reject trailing values. For the default path,
`json.Unmarshal` already rejects trailing non-whitespace JSON values and does
not need the extra decoder refill buffer. `WithDisallowUnknownFields` still
requires `json.Decoder`.

## Change

`JSONSerializer.Unmarshal` now uses:

- `json.Unmarshal` when `DisallowUnknownFields` is false.
- `json.Decoder` with `DisallowUnknownFields` when strict unknown-field
  rejection is requested.

The public serializer behavior remains:

- Empty input is rejected with the package-owned empty-input message.
- Corrupt JSON is wrapped as `unmarshal json: ...`.
- Trailing JSON values are rejected.
- `WithDisallowUnknownFields` rejects unknown object fields.

## After Evidence

Five-run benchmark after the change:

- Decode: `1,015,803-1,015,805 B/op`, `19,219 allocs/op`.
- RoundTrip: `2,140,871-2,169,519 B/op`, `27,635-27,636 allocs/op`.

After-change decode allocation profile:

- `encoding/json.(*Decoder).refill` is no longer present.
- Remaining allocation is value materialization through `encoding/json` and
  reflection: map assignment, slice growth, literal string storage, map
  creation, and struct allocation.

## Decision

Accept the narrow default-path optimization. It removes avoidable wrapper
allocation while preserving public behavior. Do not add pooling or unsafe reuse:
the remaining allocation is the expected cost of materializing a 1,200-event
slice with per-event metadata maps and strings through `encoding/json`.

## Retained Outputs

Raw benchmark and profile artifacts are stored under
[`docs/research/outputs/issue-456/`](outputs/issue-456/README.md).
