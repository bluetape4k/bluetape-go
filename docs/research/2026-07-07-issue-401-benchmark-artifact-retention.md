# Issue #401 Benchmark Artifact Retention

Issue: #401
Parent: #398
Milestone: 0.14.0
Date: 2026-07-07
Work type: Benchmark evidence retention

## Goal

local run 하나를 production ranking으로 바꾸지 않으면서 0.14.0 SerDe baseline에 필요한 raw benchmark output과 environment
metadata를 보존한다.

이 note는 #402가 cross-repo recommendation matrix를 publish할 때 사용해야 하는 retention path와 report template을 정의한다.

## Retention Path

accepted 0.14.0 Go SerDe baseline artifact는 다음 경로에 둔다.

```text
docs/research/outputs/issue-401/
```

| File | Purpose |
|---|---|
| `environment.md` | host, OS, CPU, Go version, git revision, dirty-tree state, metric direction, fixture version, command inventory. |
| `go-serialization-bench.txt` | full `go test -run '^$' -bench '^BenchmarkSerialization' -benchmem ./serialization` output. |
| `go-codec-bench.txt` | full `go test -run '^$' -bench '^BenchmarkCodec' -benchmem ./codec` output. |
| `go-compression-bench.txt` | full `go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression` output. |
| `README.md` | artifact directory용 human-readable inventory. |

future SerDe benchmark refresh는 새 issue-specific output directory를 append하거나 dated subdirectory를 추가해야 한다. downstream
report가 cite한 accepted raw output file은 overwrite하지 않는다.

## Traceability Rule

#402의 모든 benchmark-derived statement는 세 가지를 cite해야 한다.

| Required citation | Example |
|---|---|
| Command | `environment.md` command inventory row |
| Raw output file | `docs/research/outputs/issue-401/go-codec-bench.txt` |
| Row or metric boundary | benchmark name plus metric direction from `environment.md` |

report가 여러 row를 aggregate하면 summary를 쓰기 전에 aggregation에 사용한 모든 raw output file을 나열해야 한다. result가 measured
evidence가 아니라 hypothesis라면 hypothesis로 label하고 local snapshot result처럼 cite하지 않는다.

## Metric Direction

| Metric | Direction | Notes |
|---|---|---|
| `ns/op` | 낮을수록 좋다 | local elapsed benchmark time. environment 기록 없이 machine 간 비교하지 않는다. |
| `B/op` | 낮을수록 좋다 | Go가 보고하는 operation당 allocation volume. |
| `allocs/op` | 낮을수록 좋다 | operation당 allocation count. |
| `MB/s` | 높을수록 좋다 | Go가 `SetBytes`에서 계산한 throughput. 같은 fixture class에서만 비교한다. |
| `encoded_bytes` | 낮을수록 조밀하다 | codec/serializer output size이며 standalone performance winner가 아니다. |
| `serialized_bytes` | 낮을수록 조밀하다 | compression 전 serialization output size. |
| `compressed_bytes` | 낮을수록 조밀하다 | compression output size. |
| `compressed/original` | 낮을수록 조밀하다 | original byte 대비 compression ratio. |
| `compressed/serialized` | 낮을수록 조밀하다 | serialized byte 대비 compression ratio. |

## Language Boundary

local-snapshot language를 사용한다.

- "In this local Go snapshot..."
- "The retained output reports..."
- "This row is evidence for the fixture and command above..."

absolute winner, default choice, 또는 이 local run만으로 operational selection을 선언하는 표현은 피한다.

production recommendation에는 raw Go, Rust, JVM output과 caller constraint, security boundary를 결합한 별도 decision record가 필요하다.

## Template

future benchmark update에는 [benchmark-artifact-template.md](benchmark-artifact-template.md)를 사용한다. 이 template은 command,
environment, output file, metric direction, interpretation boundary를 하나의 compact report로 묶는다.

## Follow-up Ownership

| Issue | Responsibility |
|---|---|
| #402 | retained artifact와 sibling-repo evidence를 사용해 cross-repo SerDe recommendation matrix를 publish한다. |
| #403 | retained evidence가 concrete, scoped gap을 보일 때만 optimization follow-up을 만든다. |
