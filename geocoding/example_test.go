package geocoding_test

import (
	"context"
	"net/http"

	"github.com/bluetape4k/bluetape-go/geo"
	"github.com/bluetape4k/bluetape-go/geocoding"
)

func ExampleNewNominatim() {
	provider, err := geocoding.NewNominatim("https://geocoder.example.test", http.DefaultClient, "my-service/1.0")
	if err != nil {
		panic(err)
	}
	point, _ := geo.NewPoint(37.4979, 127.0276)
	_, _ = provider.Reverse(context.Background(), point, geocoding.Options{Language: "ko", Zoom: 14})
	// Output:
}
