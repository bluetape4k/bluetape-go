# Issue #310 Libvips Evaluation

## 환경

- Date: 2026-07-02
- Go: `go version go1.26.4 darwin/arm64`
- OS/Arch: `darwin/arm64`
- CPU: `Apple M5`
- Native libvips: `vips-8.18.3`
- `pkg-config --modversion vips`: `8.18.3`
- `CGO_ENABLED=1`

## 후보 요약

| Candidate | Module status | Maintenance signal | License | Native burden | Decision |
|---|---|---|---|---|---|
| `github.com/davidbyttow/govips/v2` | `v2.18.0`, Go module, Go 1.25 metadata | Not archived, pushed 2026-06-07, latest release 2026-04-01 | MIT | Requires libvips 8.14+, C compiler, `pkg-config`, cgo | Optional example로 선택 |
| `github.com/h2non/bimg/v2` | No matching `v2` module version | Not usable as requested | MIT | Requires libvips and cgo | 기각 |
| `github.com/h2non/bimg` | `v1.1.9`, no source `go.mod` in checked module tree | Latest GitHub release is old, though repository has later activity | MIT | Requires libvips and cgo | Benchmark만 수행, 선택하지 않음 |

## Native Codec 근거

Local `vips -l foreign` output은 JPEG, PNG, GIF, WebP, TIFF, HEIF/AVIF,
JXL, SVG, PDF, JP2K, OpenEXR, ImageMagick-backed format의 loader/saver를
나열한다. 이는 host-specific native capability evidence일 뿐이다. Optional
example은 의도적으로 JPEG, PNG, GIF input과 JPEG/PNG output만 문서화하고
테스트하며, AVIF, HEIC, TIFF 또는 다른 advanced codec support를 주장하지
않는다.

## Lifecycle 메모

`govips`는 process-wide `Startup`과 `Shutdown` call을 노출한다. Source는
shutdown 후 restart를 거부하므로, example은 `sync.Once` startup을 사용하고
normal request path에서 `Shutdown`을 호출하지 않는다. 모든 transform은 export
이후 `defer img.Close()`로 `ImageRef`를 닫는다. Optional adapter이고 default
image path가 아니므로, example은 quiet logging을 설정하고 낮은 default
libvips concurrency를 둔다.

## Benchmark 비교

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

## 결정

`examples/imagekit-govips`를 isolated optional module로 추가한다. 이렇게 하면
root module과 default `imagekit` package는 pure Go로 유지하면서, native setup을
받아들일 수 있는 larger transform에서 govips가 쓸 만하다는 점을 증명할 수
있다. 아직 root build-tagged adapter package는 추가하지 않는다. 그러면 native
dependency가 main module graph로 들어오고 CI/release burden이 research issue
범위를 넘어 넓어진다.

## 검증 명령

```bash
cd examples/imagekit-govips
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -race ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -run '^$' -bench . -benchmem ./...

cd ../..
go test ./...
```
