# imagekit-govips

`imagekit-govips`는 `github.com/davidbyttow/govips/v2`를 bluetape-go
`imagekit` 요청/결과 타입과 함께 사용하는 선택형 예제 모듈이다.

이 모듈은 의도적으로 nested module로 둔다. 루트
`github.com/bluetape4k/bluetape-go` 모듈과 기본 `imagekit` 패키지는 pure Go
상태를 유지하며 native libvips를 요구하지 않는다.

![imagekit-govips 선택형 native 경계](../../docs/images/readme-diagrams/imagekit-govips-optional-boundary.png)

## 요구 사항

- libvips 8.14 이상
- C compiler와 `pkg-config`
- cgo 활성화

macOS:

```bash
brew install vips pkg-config
```

일부 macOS/Homebrew 환경에서는 다음 설정이 필요할 수 있다.

```bash
export CGO_CFLAGS_ALLOW='-Xpreprocessor'
```

## 범위

- 입력: pure-Go `imagekit` allowlist와 맞춘 JPEG, PNG, GIF 메타데이터 경로
- 출력: JPEG, PNG
- 모드: fit, fill, exact
- 수명주기: `Startup`은 프로세스에서 한 번 실행하고, native image handle은
  export 후 닫는다.
- 취소: bounded read 전후, native 작업 전, output write 전에 context를 확인한다.
  이미 시작된 libvips 호출은 중간에 선점할 수 없다.

AVIF, HEIC, TIFF, WebP, JXL 같은 고급 codec은 이 예제의 지원 범위로
문서화하지 않는다. native libvips 빌드는 host와 codec plugin에 따라 다르므로
해당 codec 지원은 별도로 검증해야 한다.

## 사용

```go
result, err := imagekitgovips.Transform(ctx, reader, imagekit.Request{
	Width:        320,
	Height:       180,
	Mode:         imagekit.ModeFill,
	OutputFormat: imagekit.OutputJPEG,
	JPEGQuality:  85,
})
```

`RuntimeInfo`는 시작 시점이나 진단 시점에 감지된 libvips/govips 버전을 기록할
때 사용한다.

## 검증

```bash
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -race ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -run '^$' -bench . -benchmem ./...
```

루트 모듈 검증은 여전히 다음 명령이다.

```bash
cd ../..
go test ./...
```
