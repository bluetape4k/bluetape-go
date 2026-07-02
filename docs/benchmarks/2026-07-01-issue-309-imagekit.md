# Issue #309 ImageKit Pure-Go Benchmark

## Environment

- Date: 2026-07-01
- Command: `go test -run '^$' -bench '^BenchmarkTransform' -benchmem ./imagekit`
- Go: `go version go1.26.4 darwin/arm64`
- OS/Arch: `darwin/arm64`
- CPU: `Apple M5`

## Notes

These rows are the pure-Go baseline for issue #310 libvips comparison. They do
not claim libvips-level throughput.

## Raw Output

```text
goos: darwin
goarch: arm64
pkg: github.com/bluetape4k/bluetape-go/imagekit
cpu: Apple M5
BenchmarkTransformSmallJPEGToJPEG-10      	    1033	   1076786 ns/op	   2.63 MB/s	 1183676 B/op	      38 allocs/op
BenchmarkTransformSmallPNGToPNG-10        	    1390	    849245 ns/op	   0.74 MB/s	 2184004 B/op	      67 allocs/op
BenchmarkTransformMediumJPEGToPNG-10      	      62	  18017075 ns/op	   1.65 MB/s	12781509 B/op	      70 allocs/op
BenchmarkTransformToMediumJPEGToPNG-10    	      66	  17917148 ns/op	   1.65 MB/s	12780629 B/op	      67 allocs/op
PASS
ok  	github.com/bluetape4k/bluetape-go/imagekit	5.154s
```
