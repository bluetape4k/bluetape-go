# Issue #310 Libvips Evaluation

## Environment

- Date: 2026-07-02
- Go: `go version go1.26.4 darwin/arm64`
- OS/Arch: `darwin/arm64`
- CPU: `Apple M5`
- Native libvips: `vips-8.18.3`
- `pkg-config --modversion vips`: `8.18.3`
- `CGO_ENABLED=1`

## Candidate Summary

| Candidate | Module status | Maintenance signal | License | Native burden | Decision |
|---|---|---|---|---|---|
| `github.com/davidbyttow/govips/v2` | `v2.18.0`, Go module, Go 1.25 metadata | Not archived, pushed 2026-06-07, latest release 2026-04-01 | MIT | Requires libvips 8.14+, C compiler, `pkg-config`, cgo | Selected for optional example |
| `github.com/h2non/bimg/v2` | No matching `v2` module version | Not usable as requested | MIT | Requires libvips and cgo | Rejected |
| `github.com/h2non/bimg` | `v1.1.9`, no source `go.mod` in checked module tree | Latest GitHub release is old, though repository has later activity | MIT | Requires libvips and cgo | Benchmarked only, not selected |

## Native Codec Evidence

The local `vips -l foreign` output lists loaders/savers for JPEG, PNG, GIF,
WebP, TIFF, HEIF/AVIF, JXL, SVG, PDF, JP2K, OpenEXR, and ImageMagick-backed
formats. This is host-specific native capability evidence only. The optional
example intentionally documents and tests JPEG, PNG, and GIF input with JPEG/PNG
output and does not claim AVIF, HEIC, TIFF, or other advanced codec support.

## Lifecycle Notes

`govips` exposes process-wide `Startup` and `Shutdown` calls. Its source
rejects restart after shutdown, so the example uses `sync.Once` startup and
does not call `Shutdown` in normal request paths. Every transform closes its
`ImageRef` with `defer img.Close()` after export. The example configures quiet
logging and sets low default libvips concurrency because it is an optional
adapter, not the default image path.

## Benchmark Comparison

Command:

```bash
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -run '^$' -bench . -benchmem ./...
```

Temporary comparison harness:

```text
goos: darwin
goarch: arm64
pkg: issue310-vips-bench
cpu: Apple M5
BenchmarkImageKitSmallJPEGToJPEG-10        1203   1004494 ns/op    2.82 MB/s   1183673 B/op   38 allocs/op
BenchmarkGovipsSmallJPEGToJPEG-10          1692    686853 ns/op    4.13 MB/s      2769 B/op   37 allocs/op
BenchmarkBimgSmallJPEGToJPEG-10            1827    664884 ns/op    4.26 MB/s      1432 B/op   19 allocs/op
BenchmarkImageKitSmallPNGToPNG-10          1365    873964 ns/op    0.72 MB/s   2184008 B/op   67 allocs/op
BenchmarkGovipsSmallPNGToPNG-10            1586    749294 ns/op    0.83 MB/s      1556 B/op   35 allocs/op
BenchmarkBimgSmallPNGToPNG-10              1480    811852 ns/op    0.77 MB/s       920 B/op   19 allocs/op
BenchmarkImageKitMediumJPEGToPNG-10          64  17972874 ns/op    1.65 MB/s  12781512 B/op   70 allocs/op
BenchmarkGovipsMediumJPEGToPNG-10           705   1692108 ns/op   17.52 MB/s      2488 B/op   37 allocs/op
BenchmarkBimgMediumJPEGToPNG-10             535   2216631 ns/op   13.38 MB/s      1393 B/op   25 allocs/op
```

Nested example benchmark after implementation:

```text
BenchmarkGovipsSmallJPEGToJPEG-10          1701    695747 ns/op    4.07 MB/s     29808 B/op   54 allocs/op
BenchmarkGovipsMediumJPEGToPNG-10           678   1717585 ns/op   17.26 MB/s     86416 B/op   60 allocs/op
```

## Decision

Add `examples/imagekit-govips` as an isolated optional module. This keeps the
root module and default `imagekit` package pure Go while proving that govips is
worth using for larger transforms where native setup is acceptable. Do not add a
root build-tagged adapter package yet; that would pull a native dependency into
the main module graph and broaden CI/release burden beyond the research issue.

## Verification Commands

```bash
cd examples/imagekit-govips
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -race ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -run '^$' -bench . -benchmem ./...

cd ../..
go test ./...
```
