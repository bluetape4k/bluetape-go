# geocoding

`geocoding`은 `geo.Point`를 위한 작은 reverse-geocoding `Provider`와
Nominatim 호환 HTTP adapter를 제공합니다. adapter는 호출자가 소유한 절대
base URL, `*http.Client`, 식별 가능한 User-Agent를 요구하며 public endpoint나
전역 policy를 설치하지 않습니다.

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

request는 `/reverse`, WGS84 `lat`/`lon`, `format=jsonv2`와 선택적인 언어/상세
parameter를 사용합니다. response body는 크기가 제한되고 항상 닫힙니다.
`RateLimiter`와 `Cache`는 caller-owned hook이며 retry, timeout, service 선택,
attribution 및 법적 policy는 caller/operator가 결정합니다.

`errors.Is`로 `ErrNoResult`, `ErrProvider`, `ErrRateLimited`, `ErrTimeout`,
`ErrParse`, `ErrResponseTooLarge`를 분류할 수 있습니다. 오류 문자열에는
provider URL, payload, credential을 넣지 않습니다.

기본 CI는 `httptest`만 사용합니다. 공개 OSM Nominatim을 대량 enrichment에
사용하지 말고, production caller는 service policy를 준수하고 application을
식별하며 요청을 rate-limit하고 적절히 cache하고 attribution을 보존해야 합니다.

```bash
go test -race -count=1 ./geocoding
```
