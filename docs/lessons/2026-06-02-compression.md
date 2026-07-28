# Compression

Issue #13은 compression을 shared stream contract를 가진 explicit algorithm
adapter로 유지한다. gzip, zlib, raw DEFLATE는 stdlib를 사용하고, stdlib에 없지만
service payload에 가치가 있는 zstd, lz4, snappy만 direct dependency로 추가한다.
bzip2 compression, zip archive, brotli, s2는 구체적인 compatibility 또는
performance issue가 생길 때까지 defer한다.
