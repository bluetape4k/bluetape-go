# Benchmark Artifact Template

Issue: #NNN
Milestone: X.Y.Z
Date: YYYY-MM-DD
Scope: package 또는 cross-repo benchmark scope

## Snapshot Boundary

이 report는 local benchmark snapshot이다. production ranking이 아니며 그 자체로 default를 바꾸지 않는다.

## Environment

| Field | Value |
|---|---|
| OS/Arch | TODO |
| CPU | TODO |
| Logical CPUs | TODO |
| Go version | TODO |
| Git SHA | TODO |
| Dirty tree | TODO |
| Fixture version | TODO |

## Commands And Outputs

| Command | Raw output file | Notes |
|---|---|---|
| `TODO` | `docs/research/outputs/issue-NNN/TODO.txt` | TODO |

## Metric Direction

| Metric | Direction |
|---|---|
| `ns/op` | 낮을수록 좋다 |
| `B/op` | 낮을수록 좋다 |
| `allocs/op` | 낮을수록 좋다 |
| `MB/s` | 높을수록 좋다 |
| `encoded_bytes` | 낮을수록 조밀하다 |
| `compressed_bytes` | 낮을수록 조밀하다 |
| `compressed/original` | 낮을수록 조밀하다 |

## Interpretation

- measured evidence:
  - TODO
- hypothesis 또는 follow-up candidate:
  - TODO
- 이 snapshot만으로 비교할 수 없는 항목:
  - TODO

## Traceability Checklist

- [ ] 모든 measured statement가 command를 cite한다.
- [ ] 모든 measured statement가 raw output file을 cite한다.
- [ ] value를 비교하기 전에 metric direction을 적는다.
- [ ] local-snapshot language를 사용한다.
- [ ] production-ranking language가 없다.
