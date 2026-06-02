# Compression Benchmarks

Issue #76 adds benchmarks as an explicit opt-in target instead of part of CI.
Compression benchmarks should report throughput, allocations, compressed bytes,
and compressed/original ratio across both compressible and pseudo-random
payloads. Keep benchmark payloads deterministic so results are comparable
across local runs and PR review.
