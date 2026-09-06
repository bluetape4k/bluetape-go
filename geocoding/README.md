# geocoding

`geocoding` defines a small reverse-geocoding `Provider` for `geo.Point` and a
Nominatim-compatible HTTP adapter. The adapter requires a caller-owned absolute
base URL, `*http.Client`, and identifying User-Agent; it never installs a
public endpoint or global policy.

```go
provider, _ := geocoding.NewNominatim(
    "https://geocoder.example.test/nominatim",
    http.DefaultClient,
    "my-service/1.0",
)
result, err := provider.Reverse(ctx, point, geocoding.Options{
    Language: "ko,en", Zoom: 14, AddressDetails: true,
})
```

Requests use `/reverse`, WGS84 `lat`/`lon`, `format=jsonv2`, and optional
language/detail parameters. Response bodies are bounded and always closed.
`RateLimiter` and `Cache` are caller-owned hooks; retry, timeout, service
selection, attribution and legal policy remain caller/operator decisions.

Errors are classified with `errors.Is` as `ErrNoResult`, `ErrProvider`,
`ErrRateLimited`, `ErrTimeout`, `ErrParse`, or `ErrResponseTooLarge` without
including provider URLs, payloads, or credentials.

The default CI uses `httptest` only. Do not use the public OSM Nominatim service
for bulk enrichment; production callers must follow the service policy,
identify their application, rate-limit requests, cache responsibly, and retain
attribution.

```bash
go test -race -count=1 ./geocoding
```
