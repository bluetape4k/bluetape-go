package barcode_test

import (
	"bytes"
	"context"
	"fmt"
	"image/png"

	"github.com/bluetape4k/bluetape-go/imagekit/barcode"
)

func ExampleRender_qr() {
	img, err := barcode.Render(context.Background(), barcode.Request{
		Kind:    barcode.QR,
		Content: "블루테이프 QR",
		Width:   256,
		Height:  256,
	})
	if err != nil {
		return
	}

	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy())

	// Output:
	// 256 256
}

func ExampleEncodePNG_code128() {
	payload, err := barcode.EncodePNG(context.Background(), barcode.Request{
		Kind:    barcode.Code128,
		Content: "BLUETAPE-546",
		Width:   512,
		Height:  128,
	})
	if err != nil {
		return
	}
	img, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		return
	}

	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy())

	// Output:
	// 512 128
}
