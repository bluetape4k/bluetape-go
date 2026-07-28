# Issue #434 UUID v7 parallel hot-path evaluation

Issue: #434
Date: 2026-07-08
Work type: Type B performance evaluation

## Question

`id.NewUUIDV7Generator`가 generator-wide UUID v7 mutex를 atomic logical tick reservation 같은 더 좁은 hot-path strategy로
대체해야 하는가?

## Evidence

- Environment: `outputs/issue-434/environment.md`
- Baseline benchmark: `outputs/issue-434/uuid-v7-baseline-count10.txt`
- Baseline profile bench: `outputs/issue-434/uuid-v7-reuse-parallel-profile-bench.txt`
- Baseline CPU top: `outputs/issue-434/uuid-v7-reuse-parallel-cpu-top.txt`
- Baseline mutex top: `outputs/issue-434/uuid-v7-reuse-parallel-mutex-top.txt`
- Atomic candidate benchmark: `outputs/issue-434/uuid-v7-atomic-count10.txt`
- Baseline vs atomic candidate: `outputs/issue-434/benchstat-atomic-candidate.txt`

## Baseline

현재 implementation은 issue #192 뒤 기록된 shape를 그대로 보인다. single-thread UUID v7 generation은 안정적이지만,
`b.RunParallel`에서 shared generator를 쓰면 UUID v7 ordering mutex 주변 시간이 눈에 띈다.

Apple M5와 Go `1.26.5` fresh baseline:

| Benchmark | Baseline result |
|---|---:|
| `UUIDV7NewString` | `82.02 ns/op +/- 4%`, `112 B/op`, `3 allocs/op` |
| `UUIDV7NewStringParallel` | `200.1 ns/op +/- 1%`, `112 B/op`, `3 allocs/op` |
| `UUIDV7ReuseGenerator` | `68.00 ns/op +/- 0%`, `64 B/op`, `2 allocs/op` |
| `UUIDV7ReuseGeneratorParallel` | `192.6 ns/op +/- 1%`, `64 B/op`, `2 allocs/op` |

CPU 및 mutex profile은 shared-generator parallel UUID v7이 여전히 contention-shaped임을 확인한다. sampled CPU profile에서
`io.ReadFull`은 작은 cumulative slice만 차지했으므로 issue #192 entropy-buffering fix는 계속 유효하다.

## Candidate

local candidate는 UUID v7 generator mutex를 atomic logical tick reservation으로 교체했다. 같은 benchmark/test surface를
보존했지만 target row를 실질적으로 개선하지 못해 거부했다.

`benchstat` 결과:

| Benchmark | Candidate result | Decision |
|---|---:|---|
| `UUIDV7NewString` | `79.52 ns/op +/- 6%` | noise, not material |
| `UUIDV7NewStringParallel` | `200.5 ns/op +/- 1%` | no improvement |
| `UUIDV7ReuseGenerator` | `68.04 ns/op +/- 0%` | no improvement |
| `UUIDV7ReuseGeneratorParallel` | `193.2 ns/op +/- 1%` | no improvement |

candidate geomean은 `-0.62%`만 변했고 allocation은 바뀌지 않았다.

## 결정

이 issue에서는 production UUID v7 generation을 바꾸지 않는다.

measured target row가 개선되지 않았으므로 mutex를 atomic tick reservation으로 바꾸면 증명된 performance benefit 없이 concurrency
complexity만 늘어난다. 기존 mutex-based implementation은 더 단순하고 deterministic reader/clock behavior를 보존하며 ordering,
rollback, overflow, stress, race check를 이미 통과하므로 유지한다.

## Follow-up

추가 UUID v7 work는 다른 hypothesis에서 시작해야 한다.

- string allocation/encoding cost 감소.
- 단일 shared generator 대신 caller-owned generator pool benchmark.
- per-shard ordering tradeoff에 대한 명확한 API contract가 있을 때만 sharded generator 평가.

새 Go runtime 또는 benchmark profile이 실질적으로 다른 결과를 보이기 전에는 atomic logical tick reservation 접근을 다시 열지 않는다.
