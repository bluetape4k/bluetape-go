# Compression

Issue #13 keeps compression as explicit algorithm adapters with a shared stream
contract. Use stdlib for gzip, zlib, and raw DEFLATE; add direct dependencies
only for algorithms absent from stdlib and valuable for service payloads:
zstd, lz4, and snappy. Defer bzip2 compression, zip archives, brotli, and s2
until a concrete compatibility or performance issue needs them.
