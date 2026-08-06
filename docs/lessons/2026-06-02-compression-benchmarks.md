# Compression Benchmarks

Issue #76은 benchmark를 CI 일부가 아니라 명시적인 opt-in target으로 둔다.
Compression benchmark는 compressible payload와 pseudo-random payload 모두에서
throughput, allocation, compressed bytes, compressed/original ratio를 보고해야
한다. 결과가 local run과 PR review 사이에서 비교 가능하도록 benchmark payload는
deterministic하게 유지한다.
