package geocoding_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/geo"
	"github.com/bluetape4k/bluetape-go/geocoding"
)

func TestNewNominatimRequiresCallerOwnedEndpointAndClient(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, agent string
		client                *http.Client
	}{
		{"relative endpoint", "/reverse", "agent", http.DefaultClient},
		{"nil client", "https://example.test", "agent", nil},
		{"blank agent", "https://example.test", "", http.DefaultClient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := geocoding.NewNominatim(tc.endpoint, tc.client, tc.agent); !errors.Is(err, geocoding.ErrInvalidOptions) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestNominatimRejectsOversizedBodyAndMalformedCoordinates(t *testing.T) {
	tests := []struct {
		name, body string
		limit      int64
		want       error
	}{
		{"oversized", strings.Repeat("x", 20), 10, geocoding.ErrResponseTooLarge},
		{"bad coordinates", `{"display_name":"x","lat":"not-a-number","lon":"127"}`, 1 << 20, geocoding.ErrParse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			provider, err := geocoding.NewNominatim(server.URL, server.Client(), "agent", geocoding.WithMaxResponseBytes(tt.limit))
			if err != nil {
				t.Fatal(err)
			}
			point, _ := geo.NewPoint(1, 2)
			_, err = provider.Reverse(t.Context(), point, geocoding.Options{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v want=%v", err, tt.want)
			}
		})
	}
}
