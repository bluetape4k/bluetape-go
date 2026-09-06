package geocoding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/geo"
)

// Nominatim 은 호출자가 소유한 HTTP client로 Nominatim 호환 /reverse endpoint를 호출한다.
type Nominatim struct {
	baseURL          *url.URL
	httpClient       *http.Client
	userAgent        string
	maxResponseBytes int64
	timeout          time.Duration
	rateLimiter      RateLimiter
	cache            Cache
}

var _ Provider = (*Nominatim)(nil)

// NewNominatim 은 명시적인 base URL, HTTP client와 식별 가능한 User-Agent로 adapter를 만든다.
//
// baseURL은 public endpoint 기본값으로 대체되지 않으며, caller가 service
// policy와 attribution을 직접 준수해야 한다.
func NewNominatim(baseURL string, httpClient *http.Client, userAgent string, options ...Option) (*Nominatim, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("%w: http client is nil", ErrInvalidOptions)
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%w: base URL must be an absolute HTTP URL", ErrInvalidOptions)
	}
	if strings.TrimSpace(userAgent) == "" || len(userAgent) > 256 {
		return nil, fmt.Errorf("%w: user-agent must be non-empty and bounded", ErrInvalidOptions)
	}
	client := &Nominatim{
		baseURL:          parsed,
		httpClient:       httpClient,
		userAgent:        userAgent,
		maxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: nil option", ErrInvalidOptions)
		}
		if err := option(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// New 는 NewNominatim의 짧은 별칭이다.
func New(baseURL string, httpClient *http.Client, userAgent string, options ...Option) (*Nominatim, error) {
	return NewNominatim(baseURL, httpClient, userAgent, options...)
}

// Reverse 는 좌표를 /reverse JSON endpoint로 조회하고 bounded Result를 반환한다.
func (n *Nominatim) Reverse(ctx context.Context, point geo.Point, options Options) (Result, error) {
	if n == nil || n.baseURL == nil || n.httpClient == nil {
		return Result{}, classified(ErrInvalidOptions, 0, nil)
	}
	if ctx == nil {
		return Result{}, classified(ErrInvalidOptions, 0, nil)
	}
	if err := point.Validate(); err != nil {
		return Result{}, classified(ErrInvalidCoordinate, 0, err)
	}
	if err := options.Validate(); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cacheKey := options.CacheKey(point)
	if n.cache != nil {
		cached, found, err := n.cache.Get(ctx, cacheKey)
		if err != nil {
			return Result{}, classified(ErrCache, 0, err)
		}
		if found {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			return cached.clone(), nil
		}
	}
	if n.rateLimiter != nil {
		if err := n.rateLimiter.Wait(ctx); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Result{}, ctxErr
			}
			return Result{}, classified(ErrRateLimited, 0, err)
		}
	}
	requestCtx := ctx
	var cancel context.CancelFunc
	if n.timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, n.timeout)
		defer cancel()
	}
	endpoint := *n.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/reverse"
	query := endpoint.Query()
	query.Set("lat", strconv.FormatFloat(point.Latitude(), 'g', -1, 64))
	query.Set("lon", strconv.FormatFloat(point.Longitude(), 'g', -1, 64))
	query.Set("format", "jsonv2")
	if options.Language != "" {
		query.Set("accept-language", options.Language)
	}
	if options.Zoom > 0 {
		query.Set("zoom", strconv.Itoa(options.Zoom))
	}
	if options.AddressDetails {
		query.Set("addressdetails", "1")
	}
	if options.ExtraTags {
		query.Set("extratags", "1")
	}
	if options.NameDetails {
		query.Set("namedetails", "1")
	}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Result{}, classified(ErrInvalidOptions, 0, err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := n.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return Result{}, classified(ErrTimeout, 0, ctxErr)
			}
			return Result{}, ctxErr
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return Result{}, classified(ErrTimeout, 0, err)
		}
		return Result{}, classified(ErrProvider, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, n.maxResponseBytes+1))
	if err != nil {
		return Result{}, classified(ErrProvider, resp.StatusCode, err)
	}
	if int64(len(body)) > n.maxResponseBytes {
		return Result{}, classified(ErrResponseTooLarge, resp.StatusCode, nil)
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return Result{}, classified(ErrTimeout, resp.StatusCode, err)
		}
		return Result{}, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return Result{}, classified(ErrRateLimited, resp.StatusCode, nil)
	}
	if resp.StatusCode == http.StatusNotFound {
		return Result{}, classified(ErrNoResult, resp.StatusCode, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, classified(ErrProvider, resp.StatusCode, nil)
	}
	var payload nominatimResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return Result{}, classified(ErrParse, resp.StatusCode, err)
	}
	if payload.Error != "" {
		return Result{}, classified(ErrNoResult, resp.StatusCode, nil)
	}
	result, err := payload.result(options)
	if err != nil {
		return Result{}, classified(ErrParse, resp.StatusCode, err)
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return Result{}, classified(ErrTimeout, resp.StatusCode, err)
		}
		return Result{}, err
	}
	if n.cache != nil {
		if err := n.cache.Set(ctx, cacheKey, result.clone()); err != nil {
			return Result{}, classified(ErrCache, 0, err)
		}
	}
	return result.clone(), nil
}

type nominatimResponse struct {
	PlaceID     int64             `json:"place_id"`
	DisplayName string            `json:"display_name"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Address     map[string]string `json:"address"`
	Licence     string            `json:"licence"`
	Error       string            `json:"error"`
}

func (p nominatimResponse) result(options Options) (Result, error) {
	if p.DisplayName == "" || p.Lat == "" || p.Lon == "" {
		return Result{}, errors.New("missing required result fields")
	}
	latitude, err := strconv.ParseFloat(p.Lat, 64)
	if err != nil {
		return Result{}, errors.New("invalid latitude")
	}
	longitude, err := strconv.ParseFloat(p.Lon, 64)
	if err != nil {
		return Result{}, errors.New("invalid longitude")
	}
	point, err := geo.NewPoint(latitude, longitude)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		PlaceID:     p.PlaceID,
		DisplayName: p.DisplayName,
		Latitude:    point.Latitude(),
		Longitude:   point.Longitude(),
		Address:     cloneAddress(p.Address),
	}
	if options.IncludeAttribution {
		result.Attribution = p.Licence
	}
	return result, nil
}
