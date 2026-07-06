# Issue #401 Benchmark Artifact Retention

Issue: #401
Parent: #398
Milestone: 0.14.0
Date: 2026-07-07
Work type: Benchmark evidence retention

## Goal

Preserve the raw benchmark outputs and environment metadata needed for the
0.14.0 SerDe baseline without turning one local run into a production ranking.

This note defines the retention path and report template that #402 must use
when it publishes the cross-repo recommendation matrix.

## Retention Path

Accepted 0.14.0 Go SerDe baseline artifacts live under:

```text
docs/research/outputs/issue-401/
```

| File | Purpose |
|---|---|
| `environment.md` | Host, OS, CPU, Go version, git revision, dirty-tree state, metric direction, fixture versions, and command inventory. |
| `go-serialization-bench.txt` | Full `go test -run '^$' -bench '^BenchmarkSerialization' -benchmem ./serialization` output. |
| `go-codec-bench.txt` | Full `go test -run '^$' -bench '^BenchmarkCodec' -benchmem ./codec` output. |
| `go-compression-bench.txt` | Full `go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression` output. |
| `README.md` | Human-readable inventory for the artifact directory. |

Future SerDe benchmark refreshes should either append a new issue-specific
output directory or add a dated subdirectory. Do not overwrite an accepted raw
output file after downstream reports cite it.

## Traceability Rule

Every benchmark-derived statement in #402 must cite three things:

| Required citation | Example |
|---|---|
| Command | `environment.md` command inventory row |
| Raw output file | `docs/research/outputs/issue-401/go-codec-bench.txt` |
| Row or metric boundary | Benchmark name plus metric direction from `environment.md` |

If a report aggregates several rows, it must list every raw output file used by
the aggregation before giving the summary. If a result is a hypothesis rather
than measured evidence, label it as a hypothesis and do not cite it as a local
snapshot result.

## Metric Direction

| Metric | Direction | Notes |
|---|---|---|
| `ns/op` | Lower is better | Local elapsed benchmark time. Do not compare across machines without recording environment. |
| `B/op` | Lower is better | Allocation volume per operation where Go reports it. |
| `allocs/op` | Lower is better | Allocation count per operation. |
| `MB/s` | Higher is better | Throughput derived by Go from `SetBytes`; compare only for same fixture class. |
| `encoded_bytes` | Lower is denser | Codec/serializer output size, not by itself a performance winner. |
| `serialized_bytes` | Lower is denser | Serialization output size before compression. |
| `compressed_bytes` | Lower is denser | Compression output size. |
| `compressed/original` | Lower is denser | Compression ratio against original bytes. |
| `compressed/serialized` | Lower is denser | Compression ratio against serialized bytes. |

## Language Boundary

Use local-snapshot language:

- "In this local Go snapshot..."
- "The retained output reports..."
- "This row is evidence for the fixture and command above..."

Avoid language that declares an absolute winner, a default choice, or an
operational selection from this local run alone.

Any production recommendation needs a separate decision record that combines raw
Go, Rust, and JVM outputs with caller constraints and security boundaries.

## Template

Use [benchmark-artifact-template.md](benchmark-artifact-template.md) for future
benchmark updates. The template keeps command, environment, output file, metric
direction, and interpretation boundary in one compact report.

## Follow-up Ownership

| Issue | Responsibility |
|---|---|
| #402 | Publish the cross-repo SerDe recommendation matrix using these retained artifacts and sibling-repo evidence. |
| #403 | Create optimization follow-ups only where retained evidence shows a concrete, scoped gap. |
