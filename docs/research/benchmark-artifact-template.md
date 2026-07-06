# Benchmark Artifact Template

Issue: #NNN
Milestone: X.Y.Z
Date: YYYY-MM-DD
Scope: package or cross-repo benchmark scope

## Snapshot Boundary

This report is a local benchmark snapshot. It is not a production ranking and
does not change defaults by itself.

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
| `ns/op` | Lower is better |
| `B/op` | Lower is better |
| `allocs/op` | Lower is better |
| `MB/s` | Higher is better |
| `encoded_bytes` | Lower is denser |
| `compressed_bytes` | Lower is denser |
| `compressed/original` | Lower is denser |

## Interpretation

- Measured evidence:
  - TODO
- Hypotheses or follow-up candidates:
  - TODO
- Not comparable from this snapshot:
  - TODO

## Traceability Checklist

- [ ] Every measured statement cites a command.
- [ ] Every measured statement cites a raw output file.
- [ ] Metric direction is stated before comparing values.
- [ ] Local-snapshot language is used.
- [ ] Production-ranking language is absent.
