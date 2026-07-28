package money

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/govalues/decimal"
)

const (
	// IMFSource is the provider source name for IMF Exchange Rates data.
	IMFSource = "IMF ER"

	defaultIMFEndpoint     = "https://api.imf.org/external/sdmx/2.1"
	defaultIMFTimeout      = 5 * time.Second
	defaultIMFCacheTTL     = 24 * time.Hour
	defaultIMFMaxStale     = 72 * time.Hour
	defaultIMFRetryBackoff = 100 * time.Millisecond
	defaultIMFLookback     = 18
	defaultIMFMaxBodyBytes = 4 << 20
)

// IMFFrequency string 공개 타입이다.
type IMFFrequency string

const (
	// IMFFrequencyDaily selects daily observations.
	IMFFrequencyDaily IMFFrequency = "D"
	// IMFFrequencyMonthly selects monthly observations.
	IMFFrequencyMonthly IMFFrequency = "M"
	// IMFFrequencyQuarterly selects quarterly observations.
	IMFFrequencyQuarterly IMFFrequency = "Q"
	// IMFFrequencyAnnual selects annual observations.
	IMFFrequencyAnnual IMFFrequency = "A"
)

// IMFRateFamily string 공개 타입이다.
type IMFRateFamily string

const (
	// IMFRateEndOfPeriod selects end-of-period rates.
	IMFRateEndOfPeriod IMFRateFamily = "EOP_RT"
	// IMFRatePeriodAverage selects period-average rates.
	IMFRatePeriodAverage IMFRateFamily = "PA_RT"
)

// IMFProviderOptions 패키지에서 공개하는 구조체다.
type IMFProviderOptions struct {
	// Client is the HTTP client used for requests. nil uses http.DefaultClient.
	Client *http.Client
	// Endpoint is the IMF SDMX 2.1 endpoint root.
	Endpoint string
	// Timeout is the provider-owned timeout for one fetch.
	Timeout time.Duration
	// CacheTTL is how long a fetched rate remains fresh.
	CacheTTL time.Duration
	// MaxStale is how long a stale rate can be returned after refresh failure.
	MaxStale time.Duration
	// RetryCount is the number of retry attempts after the first fetch.
	RetryCount int
	// RetryBackoff is the delay between retry attempts.
	RetryBackoff time.Duration
	// AllowStaleOnError allows stale cache fallback after refresh failure.
	AllowStaleOnError bool
	// Now is the time provider used for freshness and default period windows.
	Now func() time.Time
	// Frequency selects the IMF ER frequency. Empty defaults to monthly.
	Frequency IMFFrequency
	// RateFamily selects period-average or end-of-period rates. Empty defaults to end-of-period.
	RateFamily IMFRateFamily
	// StartPeriod overrides the IMF startPeriod query parameter.
	StartPeriod string
	// EndPeriod overrides the IMF endPeriod query parameter.
	EndPeriod string
	// CountryCodes maps ISO 4217 domestic currency codes to IMF country codes.
	CountryCodes map[string]string
}

// IMFProvider 패키지에서 공개하는 구조체다.
type IMFProvider struct {
	client   *http.Client
	endpoint string

	timeout           time.Duration
	cacheTTL          time.Duration
	maxStale          time.Duration
	retryCount        int
	retryBackoff      time.Duration
	allowStaleOnError bool
	now               func() time.Time
	frequency         IMFFrequency
	rateFamily        IMFRateFamily
	startPeriod       string
	endPeriod         string
	countryCodes      map[string]string

	mu        sync.RWMutex
	snapshots map[imfRateKey]*imfSnapshot
}

type imfRateKey struct {
	country    string
	indicator  string
	family     IMFRateFamily
	frequency  IMFFrequency
	base       Currency
	target     Currency
	sourceRate string
}

type imfSnapshot struct {
	observedAt time.Time
	fetchedAt  time.Time
	expiresAt  time.Time
	rate       decimal.Decimal
	source     string
}

type imfDataMessage struct {
	DataSet imfDataSet `xml:"DataSet"`
}

type imfDataSet struct {
	Series []imfSeries `xml:"Series"`
}

type imfSeries struct {
	Country   string           `xml:"COUNTRY,attr"`
	Indicator string           `xml:"INDICATOR,attr"`
	Family    string           `xml:"TYPE_OF_TRANSFORMATION,attr"`
	Frequency string           `xml:"FREQUENCY,attr"`
	Obs       []imfObservation `xml:"Obs"`
}

type imfObservation struct {
	TimePeriod string `xml:"TIME_PERIOD,attr"`
	Value      string `xml:"OBS_VALUE,attr"`
}

type imfHTTPStatusError struct {
	statusCode int
	body       string
}

func (e imfHTTPStatusError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("%s: IMF HTTP status %d", ErrExchangeRateProvider, e.statusCode)
	}
	return fmt.Sprintf("%s: IMF HTTP status %d: %s", ErrExchangeRateProvider, e.statusCode, e.body)
}

func (e imfHTTPStatusError) Unwrap() error {
	return ErrExchangeRateProvider
}

// NewIMFProvider IMFProvider 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewIMFProvider(options IMFProviderOptions) (*IMFProvider, error) {
	options = normalizeIMFOptions(options)
	if err := validateIMFOptions(options); err != nil {
		return nil, err
	}
	client := options.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &IMFProvider{
		client:            client,
		endpoint:          strings.TrimRight(strings.TrimSpace(options.Endpoint), "/"),
		timeout:           options.Timeout,
		cacheTTL:          options.CacheTTL,
		maxStale:          options.MaxStale,
		retryCount:        options.RetryCount,
		retryBackoff:      options.RetryBackoff,
		allowStaleOnError: options.AllowStaleOnError,
		now:               options.Now,
		frequency:         options.Frequency,
		rateFamily:        options.RateFamily,
		startPeriod:       strings.TrimSpace(options.StartPeriod),
		endPeriod:         strings.TrimSpace(options.EndPeriod),
		countryCodes:      cloneStringMap(options.CountryCodes),
		snapshots:         make(map[imfRateKey]*imfSnapshot),
	}, nil
}

// Rate 기준 통화와 대상 통화 사이의 환율을 반환한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - base: Rate에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - target: Rate에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (p *IMFProvider) Rate(ctx context.Context, base Currency, target Currency) (ExchangeRateQuote, error) {
	if p == nil {
		return ExchangeRateQuote{}, ErrExchangeRateProvider
	}
	if err := base.validate(); err != nil {
		return ExchangeRateQuote{}, err
	}
	if err := target.validate(); err != nil {
		return ExchangeRateQuote{}, err
	}

	ctx = normalizeProviderContext(ctx)
	now := p.currentTime()
	if sameCurrency(base, target) {
		rate, err := NewExchangeRate(base, target, "1")
		if err != nil {
			return ExchangeRateQuote{}, err
		}
		return ExchangeRateQuote{
			Rate:       rate,
			Source:     IMFSource,
			ObservedAt: now,
			FetchedAt:  now,
			ExpiresAt:  now.Add(p.cacheTTL),
		}, nil
	}

	key, err := p.rateKey(base, target)
	if err != nil {
		return ExchangeRateQuote{}, err
	}
	if snapshot := p.cachedSnapshot(key, now); snapshot != nil {
		return quoteFromIMFSnapshot(snapshot, base, target, false, nil)
	}

	stale := p.staleSnapshot(key)
	refreshed, err := p.refresh(ctx, key)
	if err == nil {
		return quoteFromIMFSnapshot(refreshed, base, target, false, nil)
	}
	if isContextError(err) {
		return ExchangeRateQuote{}, err
	}
	failedAt := p.currentTime()
	if stale != nil {
		if !p.allowStaleOnError || p.staleTooOld(stale, failedAt) {
			return ExchangeRateQuote{}, fmt.Errorf("%w: %w", ErrExchangeRateStale, err)
		}
		return quoteFromIMFSnapshot(stale, base, target, true, err)
	}
	return ExchangeRateQuote{}, fmt.Errorf("%w: %w", ErrExchangeRateUnavailable, err)
}

func normalizeIMFOptions(options IMFProviderOptions) IMFProviderOptions {
	if options.Endpoint == "" {
		options.Endpoint = defaultIMFEndpoint
	}
	if options.Timeout == 0 {
		options.Timeout = defaultIMFTimeout
	}
	if options.CacheTTL == 0 {
		options.CacheTTL = defaultIMFCacheTTL
	}
	if options.MaxStale == 0 {
		options.MaxStale = defaultIMFMaxStale
	}
	if options.RetryBackoff == 0 {
		options.RetryBackoff = defaultIMFRetryBackoff
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Frequency == "" {
		options.Frequency = IMFFrequencyMonthly
	}
	if options.RateFamily == "" {
		options.RateFamily = IMFRateEndOfPeriod
	}
	if options.CountryCodes == nil {
		options.CountryCodes = defaultIMFCountryCodes()
	}
	return options
}

func validateIMFOptions(options IMFProviderOptions) error {
	if options.Timeout < 0 {
		return fmt.Errorf("%w: negative timeout %s", ErrExchangeRateProvider, options.Timeout)
	}
	if options.CacheTTL < 0 {
		return fmt.Errorf("%w: negative cache ttl %s", ErrExchangeRateProvider, options.CacheTTL)
	}
	if options.MaxStale < 0 {
		return fmt.Errorf("%w: negative max stale %s", ErrExchangeRateProvider, options.MaxStale)
	}
	if options.RetryCount < 0 {
		return fmt.Errorf("%w: negative retry count %d", ErrExchangeRateProvider, options.RetryCount)
	}
	if options.RetryBackoff < 0 {
		return fmt.Errorf("%w: negative retry backoff %s", ErrExchangeRateProvider, options.RetryBackoff)
	}
	parsed, err := url.Parse(strings.TrimSpace(options.Endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: invalid endpoint %q", ErrExchangeRateProvider, options.Endpoint)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: invalid endpoint scheme %q", ErrExchangeRateProvider, parsed.Scheme)
	}
	if !validIMFFrequency(options.Frequency) {
		return fmt.Errorf("%w: invalid IMF frequency %q", ErrExchangeRateProvider, options.Frequency)
	}
	if !validIMFRateFamily(options.RateFamily) {
		return fmt.Errorf("%w: invalid IMF rate family %q", ErrExchangeRateProvider, options.RateFamily)
	}
	if (options.StartPeriod == "") != (options.EndPeriod == "") {
		return fmt.Errorf("%w: startPeriod and endPeriod must be set together", ErrExchangeRateProvider)
	}
	for currencyCode, countryCode := range options.CountryCodes {
		if _, err := ParseCurrency(currencyCode); err != nil {
			return fmt.Errorf("%w: invalid IMF currency code %q: %w", ErrExchangeRateProvider, currencyCode, err)
		}
		if !validIMFCountryCode(countryCode) {
			return fmt.Errorf("%w: invalid IMF country code %q", ErrExchangeRateProvider, countryCode)
		}
	}
	return nil
}

func validIMFFrequency(frequency IMFFrequency) bool {
	switch frequency {
	case IMFFrequencyDaily, IMFFrequencyMonthly, IMFFrequencyQuarterly, IMFFrequencyAnnual:
		return true
	default:
		return false
	}
}

func validIMFRateFamily(family IMFRateFamily) bool {
	switch family {
	case IMFRateEndOfPeriod, IMFRatePeriodAverage:
		return true
	default:
		return false
	}
}

func (p *IMFProvider) currentTime() time.Time {
	return p.now().UTC()
}

func (p *IMFProvider) rateKey(base Currency, target Currency) (imfRateKey, error) {
	key := imfRateKey{
		family:    p.rateFamily,
		frequency: p.frequency,
		base:      base,
		target:    target,
	}
	switch {
	case p.isPivot(base) && !p.isPivot(target):
		country, err := p.countryCode(target)
		if err != nil {
			return imfRateKey{}, err
		}
		key.country = country
		key.indicator = "XDC_" + pivotCode(base)
		key.sourceRate = key.indicator
		return key, nil
	case !p.isPivot(base) && p.isPivot(target):
		country, err := p.countryCode(base)
		if err != nil {
			return imfRateKey{}, err
		}
		key.country = country
		key.indicator = pivotCode(target) + "_XDC"
		key.sourceRate = key.indicator
		return key, nil
	default:
		return imfRateKey{}, fmt.Errorf("%w: IMF ER supports one domestic currency and one USD/EUR pivot", ErrUnsupportedExchangeRate)
	}
}

func (p *IMFProvider) isPivot(currency Currency) bool {
	return sameCurrency(currency, USD) || sameCurrency(currency, EUR)
}

func (p *IMFProvider) countryCode(currency Currency) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(currency.Code()))
	country := strings.ToUpper(strings.TrimSpace(p.countryCodes[code]))
	if country == "" {
		return "", fmt.Errorf("%w: IMF country code for %s", ErrUnsupportedExchangeRate, currency)
	}
	return country, nil
}

func pivotCode(currency Currency) string {
	return currency.Code()
}

func (p *IMFProvider) cachedSnapshot(key imfRateKey, now time.Time) *imfSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snapshot := p.snapshots[key]
	if snapshot == nil || now.After(snapshot.expiresAt) {
		return nil
	}
	return snapshot.clone()
}

func (p *IMFProvider) staleSnapshot(key imfRateKey) *imfSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snapshot := p.snapshots[key]
	if snapshot == nil {
		return nil
	}
	return snapshot.clone()
}

func (p *IMFProvider) staleTooOld(snapshot *imfSnapshot, now time.Time) bool {
	if snapshot == nil || p.maxStale <= 0 {
		return true
	}
	return now.After(snapshot.expiresAt.Add(p.maxStale))
}

func (p *IMFProvider) refresh(ctx context.Context, key imfRateKey) (*imfSnapshot, error) {
	var lastErr error
	attempts := p.retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := p.waitBeforeRetry(ctx); err != nil {
				return nil, err
			}
		}
		snapshot, err := p.fetch(ctx, key)
		if err == nil {
			p.mu.Lock()
			p.snapshots[key] = snapshot.clone()
			p.mu.Unlock()
			return snapshot, nil
		}
		if isContextError(err) {
			return nil, err
		}
		if !isRetryableIMFError(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (p *IMFProvider) waitBeforeRetry(ctx context.Context) error {
	if p.retryBackoff <= 0 {
		return nil
	}
	timer := time.NewTimer(p.retryBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p *IMFProvider) fetch(ctx context.Context, key imfRateKey) (*imfSnapshot, error) {
	fetchCtx, cancel := p.contextWithTimeout(ctx)
	defer cancel()

	requestURL, err := p.requestURL(key)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
	request.Header.Set("Accept", "application/vnd.sdmx.structurespecificdata+xml;version=2.1")
	response, err := p.client.Do(request)
	if err != nil {
		if fetchCtx.Err() != nil {
			return nil, fetchCtx.Err()
		}
		return nil, fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, imfHTTPStatusError{
			statusCode: response.StatusCode,
			body:       readIMFErrorBody(response.Body),
		}
	}
	body, err := readLimitedIMFBody(response.Body)
	if err != nil {
		return nil, err
	}
	return parseIMFSnapshot(strings.NewReader(body), key, p.currentTime(), p.cacheTTL)
}

func (p *IMFProvider) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = normalizeProviderContext(ctx)
	if p.timeout <= 0 {
		return ctx, func() {}
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= p.timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, p.timeout)
}

func (p *IMFProvider) requestURL(key imfRateKey) (string, error) {
	periods, err := p.periodWindow()
	if err != nil {
		return "", err
	}
	requestURL := fmt.Sprintf("%s/data/ER/%s.%s.%s.%s",
		p.endpoint,
		key.country,
		key.indicator,
		key.family,
		key.frequency,
	)
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
	query := parsed.Query()
	query.Set("startPeriod", periods.start)
	query.Set("endPeriod", periods.end)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type imfPeriodWindow struct {
	start string
	end   string
}

func (p *IMFProvider) periodWindow() (imfPeriodWindow, error) {
	if p.startPeriod != "" && p.endPeriod != "" {
		return imfPeriodWindow{start: p.startPeriod, end: p.endPeriod}, nil
	}
	now := p.currentTime()
	switch p.frequency {
	case IMFFrequencyMonthly:
		end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		start := end.AddDate(0, -defaultIMFLookback, 0)
		return imfPeriodWindow{start: formatIMFMonth(start), end: formatIMFMonth(end)}, nil
	case IMFFrequencyDaily:
		end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		start := end.AddDate(0, 0, -defaultIMFLookback*31)
		return imfPeriodWindow{start: start.Format(time.DateOnly), end: end.Format(time.DateOnly)}, nil
	case IMFFrequencyQuarterly:
		quarter := (int(now.Month())-1)/3 + 1
		end := fmt.Sprintf("%04d-Q%d", now.Year(), quarter)
		startYear := now.Year() - 5
		return imfPeriodWindow{start: fmt.Sprintf("%04d-Q%d", startYear, quarter), end: end}, nil
	case IMFFrequencyAnnual:
		return imfPeriodWindow{start: strconv.Itoa(now.Year() - 5), end: strconv.Itoa(now.Year())}, nil
	default:
		return imfPeriodWindow{}, fmt.Errorf("%w: invalid IMF frequency %q", ErrExchangeRateProvider, p.frequency)
	}
}

func parseIMFSnapshot(reader io.Reader, key imfRateKey, fetchedAt time.Time, cacheTTL time.Duration) (*imfSnapshot, error) {
	var message imfDataMessage
	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(&message); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
	var latest *imfSnapshot
	for _, series := range message.DataSet.Series {
		if !series.matches(key) {
			continue
		}
		for _, obs := range series.Obs {
			observedAt, err := parseIMFPeriod(strings.TrimSpace(obs.TimePeriod), key.frequency)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid IMF period %q: %w", ErrExchangeRateProvider, obs.TimePeriod, err)
			}
			rate, err := decimal.Parse(strings.TrimSpace(obs.Value))
			if err != nil || rate.IsZero() || !rate.IsPos() {
				if err == nil {
					err = ErrInvalidExchangeRate
				}
				return nil, fmt.Errorf("%w: invalid IMF rate %q: %w", ErrExchangeRateProvider, obs.Value, err)
			}
			candidate := &imfSnapshot{
				observedAt: observedAt,
				fetchedAt:  fetchedAt,
				expiresAt:  fetchedAt.Add(cacheTTL),
				rate:       rate,
				source:     key.source(),
			}
			if latest == nil || candidate.observedAt.After(latest.observedAt) {
				latest = candidate
			}
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("%w: missing IMF observation", ErrExchangeRateProvider)
	}
	return latest, nil
}

func (s imfSeries) matches(key imfRateKey) bool {
	return strings.EqualFold(strings.TrimSpace(s.Country), key.country) &&
		strings.EqualFold(strings.TrimSpace(s.Indicator), key.indicator) &&
		strings.EqualFold(strings.TrimSpace(s.Family), string(key.family)) &&
		strings.EqualFold(strings.TrimSpace(s.Frequency), string(key.frequency))
}

func parseIMFPeriod(period string, frequency IMFFrequency) (time.Time, error) {
	switch frequency {
	case IMFFrequencyDaily:
		return time.ParseInLocation(time.DateOnly, period, time.UTC)
	case IMFFrequencyMonthly:
		parsed, err := time.ParseInLocation("2006-M01", period, time.UTC)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(parsed.Year(), parsed.Month()+1, 0, 0, 0, 0, 0, time.UTC), nil
	case IMFFrequencyQuarterly:
		parts := strings.Split(period, "-Q")
		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("invalid quarter")
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, err
		}
		quarter, err := strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, err
		}
		if quarter < 1 || quarter > 4 {
			return time.Time{}, fmt.Errorf("invalid quarter %d", quarter)
		}
		month := time.Month(quarter * 3)
		return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC), nil
	case IMFFrequencyAnnual:
		year, err := strconv.Atoi(period)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Time{}, fmt.Errorf("invalid frequency %q", frequency)
	}
}

func quoteFromIMFSnapshot(snapshot *imfSnapshot, base Currency, target Currency, stale bool, refreshErr error) (ExchangeRateQuote, error) {
	rate, err := NewExchangeRate(base, target, snapshot.rate.String())
	if err != nil {
		return ExchangeRateQuote{}, err
	}
	return ExchangeRateQuote{
		Rate:         rate,
		Source:       snapshot.source,
		ObservedAt:   snapshot.observedAt,
		FetchedAt:    snapshot.fetchedAt,
		ExpiresAt:    snapshot.expiresAt,
		Stale:        stale,
		RefreshError: refreshErr,
	}, nil
}

func (k imfRateKey) source() string {
	return fmt.Sprintf("%s:%s:%s:%s", IMFSource, k.sourceRate, k.family, k.frequency)
}

func (s *imfSnapshot) clone() *imfSnapshot {
	if s == nil {
		return nil
	}
	return &imfSnapshot{
		observedAt: s.observedAt,
		fetchedAt:  s.fetchedAt,
		expiresAt:  s.expiresAt,
		rate:       s.rate,
		source:     s.source,
	}
}

func formatIMFMonth(t time.Time) string {
	return fmt.Sprintf("%04d-M%02d", t.Year(), t.Month())
}

func cloneStringMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[strings.ToUpper(strings.TrimSpace(key))] = strings.ToUpper(strings.TrimSpace(value))
	}
	return copied
}

func readLimitedIMFBody(reader io.Reader) (string, error) {
	limited := io.LimitReader(reader, defaultIMFMaxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
	if len(body) > defaultIMFMaxBodyBytes {
		return "", fmt.Errorf("%w: IMF response body exceeds %d bytes", ErrExchangeRateProvider, defaultIMFMaxBodyBytes)
	}
	return string(body), nil
}

func readIMFErrorBody(reader io.Reader) string {
	const maxErrorBodyBytes = 512
	body, err := io.ReadAll(io.LimitReader(reader, maxErrorBodyBytes+1))
	if err != nil {
		return ""
	}
	body = []byte(strings.Join(strings.Fields(string(body)), " "))
	if len(body) > maxErrorBodyBytes {
		body = body[:maxErrorBodyBytes]
	}
	return string(body)
}

func isRetryableIMFError(err error) bool {
	var statusErr imfHTTPStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.statusCode == http.StatusTooManyRequests || statusErr.statusCode >= http.StatusInternalServerError
}

func validIMFCountryCode(code string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if len(normalized) != 3 {
		return false
	}
	for _, r := range normalized {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func defaultIMFCountryCodes() map[string]string {
	return map[string]string{
		"AUD": "AUS",
		"CAD": "CAN",
		"CHF": "CHE",
		"CNY": "CHN",
		"GBP": "GBR",
		"JPY": "JPN",
		"KRW": "KOR",
	}
}
