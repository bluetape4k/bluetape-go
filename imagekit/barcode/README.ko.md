# imagekit/barcode

[English](README.md) | [한국어](README.ko.md)

`imagekit/barcode`는 제한된 QR·Code128을 표준 `image.Image` 또는 PNG 바이트로
생성하는 helper입니다. provider를 작은 package 경계 뒤에 숨기고 호출마다 새
흑백 `*image.Gray`를 반환합니다.

## 설치

이 패키지는 `bluetape-go` module에 포함되어 있습니다.

```bash
go get github.com/bluetape4k/bluetape-go
```

renderer는 고정된 `github.com/boombuler/barcode v1.1.0` provider를 사용하며,
호출자가 provider client나 credential을 설정하지 않습니다.

## 사용

```go
img, err := barcode.Render(ctx, barcode.Request{
    Kind:    barcode.QR,
    Content: "블루테이프 QR",
    Width:   256,
    Height:  256,
})
```

```go
payload, err := barcode.EncodePNG(ctx, barcode.Request{
    Kind:    barcode.Code128,
    Content: "BLUETAPE-546",
    Width:   512,
    Height:  128,
})
```

`Render`는 caller-owned `*image.Gray`를 반환하고, `EncodePNG`는 표준
`image/png` 결과를 담은 새 byte slice를 반환합니다. 실행 가능한 한글 QR·ASCII
Code128 예제는 `example_test.go`에서 확인할 수 있습니다.

## 입력과 크기 제한

| 요청 | 계약 |
|---|---|
| `Kind` | `QR` 또는 `Code128`만 허용 |
| QR `Content` | valid UTF-8, 1–1,024 byte; trim·normalization을 하지 않음 |
| Code128 `Content` | printable ASCII `0x20`–`0x7e`, 1–80 byte |
| `QRLevel` | QR은 `QRLevelM`, `QRLevelL`, `QRLevelQ`, `QRLevelH`; Code128은 `QRLevelM`만 허용 |
| `Width`, `Height` | 각각 1–4,096 pixel, 전체 4,194,304 pixel 이하 |
| QR shape | `Width == Height` |
| PNG output | 최대 4 MiB, 초과 output은 폐기 |

입력과 이미지 제한은 provider 호출 및 이미지 할당 전에 검사합니다. payload가
허용되어도 모든 canvas가 symbol과 quiet zone을 담을 수 있다는 뜻은 아닙니다.

## Geometry와 quiet zone

QR은 중앙 정렬된 정수 배율과 네 방향 최소 4개 white module을 사용합니다.
Code128은 가로 정수 배율, 좌우 최소 10개 white module을 사용하고 각 bar를 요청한
높이 전체에 복제합니다. symbol이 맞지 않을 때 interpolation, crop 또는 강제
downscale을 하지 않습니다.

quiet-zone 계약은 이미지 조합을 돕지만 physical scanner나 특정 산업 symbology
profile 인증을 의미하지 않습니다. 독립 decoder round trip과 실제 scanner 시험은
이 helper의 범위에 포함하지 않습니다.

## Context와 오류

`ctx`는 nil일 수 없습니다. 취소와 deadline 오류는 dispatch 전 및 rendering/PNG
checkpoint에서 원형 그대로 반환합니다. provider에 context-aware API가 없으므로
이미 시작한 호출을 선점할 수 없으며, 이 제한을 감추기 위해 detached goroutine을
만들지 않습니다.

`ErrInvalidOptions`, `ErrInputTooLarge`, `ErrImageTooLarge`, `ErrEncode`를
`errors.Is`로 분류할 수 있습니다. provider와 PNG 실패는 provider cause나 원문을
담지 않는 고정 `ErrEncode`로 반환합니다. 실패한 호출의 image 또는 byte slice는
nil입니다.

## imagekit과 조합

PNG 바이트는 `bytes.NewReader`로 `imagekit.Transform`에 전달할 수 있습니다. 임의
resize·crop 또는 JPEG 변환은 module edge와 quiet zone을 훼손해 판독성을 낮출 수
있습니다. 이런 변환 뒤의 scanner 호환성은 보장하지 않습니다.

## 비목표

custom color, logo, caption, decoding, OCR, CAPTCHA, file/network I/O, provider
client 설정, global cache는 제공하지 않습니다.

