# Issue #456 JSON Repeated Collection Profile

Issue: #456
Milestone: 0.15.0
Date: 2026-07-07

## Goal

JSON repeated collection decode 및 round-trip benchmark의 allocation source를 profile한 뒤 좁은 optimization이 정당한지 결정한다.

## Baseline

보존된 #401 serialization snapshot은 다음을 보고했다.

- `BenchmarkSerializationDecode/JSON/serde-repeated-collection-v1`: `2,062,641 B/op`, `19,234 allocs/op`.
- `BenchmarkSerializationRoundTrip/JSON/serde-repeated-collection-v1`: `3,510,457 B/op`, `27,658 allocs/op`.

같은 fixture에서 새로 실행한 #456 five-run baseline은 다음을 보고했다.

- Decode: `2,062,640-2,062,641 B/op`, `19,234 allocs/op`.
- RoundTrip: `3,460,902-3,513,236 B/op`, `27,657-27,658 allocs/op`.

## Profile Finding

baseline decode allocation profile:

- `encoding/json.(*Decoder).refill`: `734.32MB`, alloc space의 `50.29%`.
- `reflect.mapassign_faststr0`: `244.07MB`, `16.71%`.
- `reflect.growslice`: `219.81MB`, `15.05%`.
- `encoding/json.(*decodeState).literalStore`: `136.01MB`, `9.31%`.

wrapper는 trailing value를 거부하기 위해 default decode마다 `json.NewDecoder(bytes.NewReader(data))`를 사용했다. default path에서는
`json.Unmarshal`이 이미 trailing non-whitespace JSON value를 거부하므로 extra decoder refill buffer가 필요하지 않다.
`WithDisallowUnknownFields`는 여전히 `json.Decoder`가 필요하다.

## Change

`JSONSerializer.Unmarshal`은 이제 다음을 사용한다.

- `DisallowUnknownFields`가 false이면 `json.Unmarshal`.
- strict unknown-field rejection이 요청되면 `DisallowUnknownFields`를 가진 `json.Decoder`.

public serializer behavior는 유지된다.

- empty input은 package-owned empty-input message로 거부된다.
- corrupt JSON은 `unmarshal json: ...`으로 wrap된다.
- trailing JSON value는 거부된다.
- `WithDisallowUnknownFields`는 unknown object field를 거부한다.

## After Evidence

변경 후 five-run benchmark:

- Decode: `1,015,803-1,015,805 B/op`, `19,219 allocs/op`.
- RoundTrip: `2,140,871-2,169,519 B/op`, `27,635-27,636 allocs/op`.

after-change decode allocation profile:

- `encoding/json.(*Decoder).refill`은 더 이상 나타나지 않는다.
- 남은 allocation은 `encoding/json`과 reflection을 통한 value materialization이다: map assignment, slice growth,
  literal string storage, map creation, struct allocation.

## 결정

좁은 default-path optimization을 수용한다. public behavior를 보존하면서 피할 수 있는 wrapper allocation을 제거한다. pooling 또는
unsafe reuse는 추가하지 않는다. 남은 allocation은 1,200-event slice와 per-event metadata map/string을 `encoding/json`으로
materialize하는 예상 비용이다.

## Retained Outputs

raw benchmark 및 profile artifact는
[`docs/research/outputs/issue-456/`](outputs/issue-456/README.md)에 저장한다.
