package geocoding_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/geo"
	"github.com/bluetape4k/bluetape-go/geocoding"
)

func TestNominatimReverseSuccessAndRequestContract(t *testing.T) {
	var request struct {
		path, userAgent, language, zoom string
		addressDetails                  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request.path = r.URL.Path
		request.userAgent = r.Header.Get("User-Agent")
		request.language = r.URL.Query().Get("accept-language")
		request.zoom = r.URL.Query().Get("zoom")
		request.addressDetails = r.URL.Query().Get("addressdetails")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"place_id":7,"display_name":"Seoul","lat":"37.4979","lon":"127.0276","licence":"OSM","address":{"city":"Seoul"}}`))
	}))
	defer server.Close()
	provider, err := geocoding.NewNominatim(server.URL+"/nominatim", server.Client(), "bluetape-go-test/1.0", geocoding.WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	point, _ := geo.NewPoint(37.4979, 127.0276)
	result, err := provider.Reverse(context.Background(), point, geocoding.Options{
		Language: "ko,en", Zoom: 14, AddressDetails: true, IncludeAttribution: true,
	})
	if err != nil {
		t.Fatalf("Reverse failed: %v", err)
	}
	if result.PlaceID != 7 || result.DisplayName != "Seoul" || result.Latitude != 37.4979 || result.Longitude != 127.0276 || result.Attribution != "OSM" || result.Address["city"] != "Seoul" {
		t.Fatalf("result=%#v", result)
	}
	if request.path != "/nominatim/reverse" || request.userAgent != "bluetape-go-test/1.0" || request.language != "ko,en" || request.zoom != "14" || request.addressDetails != "1" {
		t.Fatalf("request=%#v", request)
	}
}

func TestNominatimReverseErrorTaxonomyAndRedaction(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		want       error
		wantStatus int
	}{
		{"no result body", http.StatusOK, `{"error":"Unable to geocode"}`, geocoding.ErrNoResult, 200},
		{"not found", http.StatusNotFound, `not found secret payload`, geocoding.ErrNoResult, 404},
		{"rate limit", http.StatusTooManyRequests, `retry later secret`, geocoding.ErrRateLimited, 429},
		{"provider", http.StatusBadGateway, `backend secret`, geocoding.ErrProvider, 502},
		{"parse", http.StatusOK, `{not-json`, geocoding.ErrParse, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			provider, err := geocoding.NewNominatim(server.URL, server.Client(), "test-agent")
			if err != nil {
				t.Fatal(err)
			}
			point, _ := geo.NewPoint(1, 2)
			_, err = provider.Reverse(context.Background(), point, geocoding.Options{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v want=%v", err, tt.want)
			}
			var classified *geocoding.Error
			if !errors.As(err, &classified) || classified.StatusCode != tt.wantStatus {
				t.Fatalf("classified=%#v", classified)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("error leaked provider detail: %v", err)
			}
		})
	}
}

func TestNominatimReverseCancellationAndBounds(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-r.Context().Done()
	}))
	defer server.Close()
	provider, err := geocoding.NewNominatim(server.URL, server.Client(), "test-agent", geocoding.WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	point, _ := geo.NewPoint(1, 2)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = provider.Reverse(canceled, point, geocoding.Options{})
	if !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("pre-cancel err=%v calls=%d", err, calls.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = provider.Reverse(ctx, point, geocoding.Options{})
	if !errors.Is(err, geocoding.ErrTimeout) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout err=%v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d want=1", calls.Load())
	}
	if _, err := provider.Reverse(context.Background(), point, geocoding.Options{Zoom: 19}); !errors.Is(err, geocoding.ErrInvalidOptions) {
		t.Fatalf("invalid zoom err=%v", err)
	}
}

func TestOptionsCacheKeyAndCacheOwnership(t *testing.T) {
	point, _ := geo.NewPoint(37.5, 127.0)
	first := (geocoding.Options{Language: "ko", Zoom: 12}).CacheKey(point)
	second := (geocoding.Options{Language: "ko", Zoom: 12}).CacheKey(point)
	if first != second || first == "" {
		t.Fatalf("unstable cache key: %q/%q", first, second)
	}
	cache := &memoryCache{}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"place_id":1,"display_name":"cached","lat":"37.5","lon":"127","address":{}}`))
	}))
	defer server.Close()
	provider, err := geocoding.NewNominatim(server.URL, server.Client(), "agent", geocoding.WithCache(cache))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Reverse(context.Background(), point, geocoding.Options{}); err != nil {
		t.Fatal(err)
	}
	result, err := provider.Reverse(context.Background(), point, geocoding.Options{})
	if err != nil || result.DisplayName != "cached" || calls != 1 {
		t.Fatalf("cached result=%#v err=%v calls=%d", result, err, calls)
	}
	result.Address["mutated"] = "caller"
	again, _ := provider.Reverse(context.Background(), point, geocoding.Options{})
	if _, ok := again.Address["mutated"]; ok {
		t.Fatalf("cache exposed mutable address")
	}
}

type memoryCache struct {
	mu   sync.Mutex
	data map[string]geocoding.Result
}

func (c *memoryCache) Get(_ context.Context, key string) (geocoding.Result, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		return geocoding.Result{}, false, nil
	}
	result, ok := c.data[key]
	return result, ok, nil
}

func (c *memoryCache) Set(_ context.Context, key string, result geocoding.Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = map[string]geocoding.Result{}
	}
	c.data[key] = result
	return nil
}
