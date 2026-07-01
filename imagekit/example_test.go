package imagekit

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
)

func ExampleTransform_fit() {
	result, err := Transform(context.Background(), bytes.NewReader(examplePNG(400, 200)), Request{
		Width:        100,
		Height:       100,
		Mode:         ModeFit,
		OutputFormat: OutputPNG,
	})
	if err != nil {
		return
	}

	fmt.Println(result.OutputFormat, result.OutputWidth, result.OutputHeight)

	// Output:
	// png 100 50
}

func ExampleTransform_fill() {
	result, err := Transform(context.Background(), bytes.NewReader(examplePNG(400, 200)), Request{
		Width:        100,
		Height:       100,
		Mode:         ModeFill,
		OutputFormat: OutputPNG,
	})
	if err != nil {
		return
	}

	fmt.Println(result.OutputFormat, result.OutputWidth, result.OutputHeight)

	// Output:
	// png 100 100
}

func ExampleTransform_exact() {
	result, err := Transform(context.Background(), bytes.NewReader(examplePNG(400, 200)), Request{
		Width:        100,
		Height:       100,
		Mode:         ModeExact,
		OutputFormat: OutputPNG,
	})
	if err != nil {
		return
	}

	fmt.Println(result.OutputFormat, result.OutputWidth, result.OutputHeight)

	// Output:
	// png 100 100
}

func ExampleTransform_outputFormat() {
	result, err := Transform(context.Background(), bytes.NewReader(examplePNG(32, 32)), Request{
		Width:        16,
		Height:       16,
		OutputFormat: OutputJPEG,
		JPEGQuality:  90,
	})
	if err != nil {
		return
	}

	fmt.Println(result.OutputFormat, len(result.Bytes) > 0)

	// Output:
	// jpeg true
}

func ExampleTransform_zeroValueDefaults() {
	result, err := Transform(context.Background(), bytes.NewReader(examplePNG(32, 16)), Request{
		Width:  16,
		Height: 16,
	})
	if err != nil {
		return
	}

	fmt.Println(result.OutputFormat, result.OutputWidth, result.OutputHeight)

	// Output:
	// jpeg 16 8
}

func ExampleTransformTo() {
	var staged bytes.Buffer
	result, err := TransformTo(context.Background(), &staged, bytes.NewReader(examplePNG(32, 32)), Request{
		Width:        16,
		Height:       16,
		OutputFormat: OutputPNG,
	})
	if err != nil {
		return
	}

	var final bytes.Buffer
	if _, err := final.Write(staged.Bytes()); err != nil {
		return
	}

	fmt.Println(result.OutputFormat, result.OutputWidth, result.OutputHeight, len(result.Bytes), final.Len() > 0)

	// Output:
	// png 16 16 0 true
}

func examplePNG(width int, height int) []byte {
	var payload bytes.Buffer
	if err := png.Encode(&payload, quadrantImage(width, height)); err != nil {
		panic(err)
	}
	return payload.Bytes()
}
