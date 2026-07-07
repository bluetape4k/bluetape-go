# Issue #456 JSON Repeated Collection Profile Artifacts

This directory stores raw benchmark and allocation-profile artifacts for issue
#456.

## Files

| File | Purpose |
|---|---|
| `environment.md` | Host, revision, command inventory, and metric direction. |
| `json-repeated-baseline-bench.txt` | Five-run baseline `-benchmem` output before the decode change. |
| `json-repeated-after-unmarshal-bench.txt` | Five-run `-benchmem` output after the default decode path used `json.Unmarshal`. |
| `json-repeated-decode-profile-bench.txt` | Baseline decode profile benchmark command output. |
| `json-repeated-decode-mem-top.txt` | Baseline decode `go tool pprof -top -alloc_space` output. |
| `json-repeated-decode.mem.pprof` | Baseline decode allocation profile. |
| `json-repeated-decode-after-profile-bench.txt` | After-change decode profile benchmark command output. |
| `json-repeated-decode-after-mem-top.txt` | After-change decode `go tool pprof -top -alloc_space` output. |
| `json-repeated-decode-after.mem.pprof` | After-change decode allocation profile. |
| `json-repeated-roundtrip-profile-bench.txt` | Baseline round-trip profile benchmark command output. |
| `json-repeated-roundtrip-mem-top.txt` | Baseline round-trip `go tool pprof -top -alloc_space` output. |
| `json-repeated-roundtrip.mem.pprof` | Baseline round-trip allocation profile. |
| `json-repeated-roundtrip-after-profile-bench.txt` | After-change round-trip profile benchmark command output. |
| `json-repeated-roundtrip-after-mem-top.txt` | After-change round-trip `go tool pprof -top -alloc_space` output. |
| `json-repeated-roundtrip-after.mem.pprof` | After-change round-trip allocation profile. |

These artifacts are local performance evidence for one fixture and host. They
are not production regression thresholds by themselves.
