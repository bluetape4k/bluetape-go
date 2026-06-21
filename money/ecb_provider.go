package money

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/govalues/decimal"
)

const (
	// ECBSource 는 ECB daily reference-rate provider source 이름입니다.
	ECBSource = "ECB"

	defaultECBEndpoint     = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"
	defaultECBTimeout      = 5 * time.Second
	defaultECBCacheTTL     = 24 * time.Hour
	defaultECBMaxStale     = 72 * time.Hour
	defaultECBRetryBackoff = 100 * time.Millisecond
)

// ECBProviderOptions 는 ECBProvider 동작을 설정합니다.
type ECBProviderOptions struct {
	// Client 는 HTTP 요청에 사용할 client입니다. nil이면 http.DefaultClient를 사용합니다.
	Client *http.Client
	// Endpoint 는 ECB daily XML endpoint입니다.
	Endpoint string
	// Timeout 은 fetch 한 번에 적용할 provider-owned timeout입니다.
	Timeout time.Duration
	// CacheTTL 은 fetch된 snapshot을 fresh로 간주하는 기간입니다.
	CacheTTL time.Duration
	// MaxStale 은 refresh 실패 시 stale snapshot을 반환할 수 있는 최대 기간입니다.
	MaxStale time.Duration
	// RetryCount 는 첫 시도 이후 추가 재시도 횟수입니다.
	RetryCount int
	// RetryBackoff 는 재시도 사이의 대기 시간입니다.
	RetryBackoff time.Duration
	// AllowStaleOnError 는 refresh 실패 시 stale snapshot 반환을 허용합니다.
	AllowStaleOnError bool
	// Now 는 freshness test를 위한 시간 provider입니다.
	Now func() time.Time
}

// ECBProvider 는 ECB euro reference-rate XML snapshot을 사용하는 provider입니다.
type ECBProvider struct {
	client   *http.Client
	endpoint string

	timeout           time.Duration
	cacheTTL          time.Duration
	maxStale          time.Duration
	retryCount        int
	retryBackoff      time.Duration
	allowStaleOnError bool
	now               func() time.Time

	mu       sync.RWMutex
	snapshot *ecbSnapshot
}

type ecbSnapshot struct {
	observedAt time.Time
	fetchedAt  time.Time
	expiresAt  time.Time
	rates      map[Currency]decimal.Decimal
}

type ecbEnvelope struct {
	Cube ecbOuterCube `xml:"Cube"`
}

type ecbOuterCube struct {
	Dates []ecbDateCube `xml:"Cube"`
}

type ecbDateCube struct {
	Time  string        `xml:"time,attr"`
	Rates []ecbRateCube `xml:"Cube"`
}

type ecbRateCube struct {
	Currency string `xml:"currency,attr"`
	Rate     string `xml:"rate,attr"`
}

// NewECBProvider 는 ECB daily XML 기반 ExchangeRateProvider를 생성합니다.
func NewECBProvider(options ECBProviderOptions) (*ECBProvider, error) {
	options = normalizeECBOptions(options)
	if err := validateECBOptions(options); err != nil {
		return nil, err
	}
	client := options.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &ECBProvider{
		client:            client,
		endpoint:          strings.TrimSpace(options.Endpoint),
		timeout:           options.Timeout,
		cacheTTL:          options.CacheTTL,
		maxStale:          options.MaxStale,
		retryCount:        options.RetryCount,
		retryBackoff:      options.RetryBackoff,
		allowStaleOnError: options.AllowStaleOnError,
		now:               options.Now,
	}, nil
}

// Rate 는 ECB snapshot에서 base/target 통화쌍에 사용할 환율을 반환합니다.
func (p *ECBProvider) Rate(ctx context.Context, base Currency, target Currency) (ExchangeRateQuote, error) {
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
			Source:     ECBSource,
			ObservedAt: now,
			FetchedAt:  now,
			ExpiresAt:  now.Add(p.cacheTTL),
		}, nil
	}

	if snapshot := p.cachedSnapshot(now); snapshot != nil {
		return p.quoteFromSnapshot(snapshot, base, target, false, nil)
	}

	stale := p.staleSnapshot()
	refreshed, err := p.refresh(ctx)
	if err == nil {
		return p.quoteFromSnapshot(refreshed, base, target, false, nil)
	}
	if stale != nil {
		if !p.allowStaleOnError || p.staleTooOld(stale, now) {
			return ExchangeRateQuote{}, fmt.Errorf("%w: %w", ErrExchangeRateStale, err)
		}
		return p.quoteFromSnapshot(stale, base, target, true, err)
	}
	return ExchangeRateQuote{}, fmt.Errorf("%w: %w", ErrExchangeRateUnavailable, err)
}

func normalizeECBOptions(options ECBProviderOptions) ECBProviderOptions {
	if options.Endpoint == "" {
		options.Endpoint = defaultECBEndpoint
	}
	if options.Timeout == 0 {
		options.Timeout = defaultECBTimeout
	}
	if options.CacheTTL == 0 {
		options.CacheTTL = defaultECBCacheTTL
	}
	if options.MaxStale == 0 {
		options.MaxStale = defaultECBMaxStale
	}
	if options.RetryBackoff == 0 {
		options.RetryBackoff = defaultECBRetryBackoff
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return options
}

func validateECBOptions(options ECBProviderOptions) error {
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
	return nil
}

func (p *ECBProvider) currentTime() time.Time {
	return p.now().UTC()
}

func (p *ECBProvider) cachedSnapshot(now time.Time) *ecbSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.snapshot == nil || now.After(p.snapshot.expiresAt) {
		return nil
	}
	return p.snapshot.clone()
}

func (p *ECBProvider) staleSnapshot() *ecbSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.snapshot == nil {
		return nil
	}
	return p.snapshot.clone()
}

func (p *ECBProvider) staleTooOld(snapshot *ecbSnapshot, now time.Time) bool {
	if snapshot == nil || p.maxStale <= 0 {
		return true
	}
	return now.After(snapshot.expiresAt.Add(p.maxStale))
}

func (p *ECBProvider) refresh(ctx context.Context) (*ecbSnapshot, error) {
	var lastErr error
	attempts := p.retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := p.waitBeforeRetry(ctx); err != nil {
				return nil, err
			}
		}
		snapshot, err := p.fetch(ctx)
		if err == nil {
			p.mu.Lock()
			p.snapshot = snapshot.clone()
			p.mu.Unlock()
			return snapshot, nil
		}
		if isContextError(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (p *ECBProvider) waitBeforeRetry(ctx context.Context) error {
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

func (p *ECBProvider) fetch(ctx context.Context) (*ecbSnapshot, error) {
	fetchCtx, cancel := p.contextWithTimeout(ctx)
	defer cancel()

	request, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
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
		_, _ = io.Copy(io.Discard, response.Body)
		return nil, fmt.Errorf("%w: ECB HTTP status %d", ErrExchangeRateProvider, response.StatusCode)
	}
	snapshot, err := parseECBSnapshot(response.Body, p.currentTime(), p.cacheTTL)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (p *ECBProvider) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = normalizeProviderContext(ctx)
	if p.timeout <= 0 {
		return ctx, func() {}
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= p.timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, p.timeout)
}

func parseECBSnapshot(reader io.Reader, fetchedAt time.Time, cacheTTL time.Duration) (*ecbSnapshot, error) {
	var envelope ecbEnvelope
	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
	}
	if len(envelope.Cube.Dates) == 0 {
		return nil, fmt.Errorf("%w: missing ECB observation", ErrExchangeRateProvider)
	}
	day := envelope.Cube.Dates[0]
	observedAt, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(day.Time), time.UTC)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid ECB observation date %q", ErrExchangeRateProvider, day.Time)
	}
	if len(day.Rates) == 0 {
		return nil, fmt.Errorf("%w: missing ECB rates", ErrExchangeRateProvider)
	}

	rates := make(map[Currency]decimal.Decimal, len(day.Rates)+1)
	one, _ := decimal.Parse("1")
	rates[EUR] = one
	for _, entry := range day.Rates {
		curr, err := ParseCurrency(entry.Currency)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrExchangeRateProvider, err)
		}
		if _, ok := rates[curr]; ok {
			return nil, fmt.Errorf("%w: duplicate ECB currency %s", ErrExchangeRateProvider, curr)
		}
		rate, err := decimal.Parse(strings.TrimSpace(entry.Rate))
		if err != nil || rate.IsZero() || !rate.IsPos() {
			if err == nil {
				err = ErrInvalidExchangeRate
			}
			return nil, fmt.Errorf("%w: invalid ECB rate for %s: %w", ErrExchangeRateProvider, curr, err)
		}
		rates[curr] = rate
	}
	return &ecbSnapshot{
		observedAt: observedAt,
		fetchedAt:  fetchedAt,
		expiresAt:  fetchedAt.Add(cacheTTL),
		rates:      rates,
	}, nil
}

func (p *ECBProvider) quoteFromSnapshot(snapshot *ecbSnapshot, base Currency, target Currency, stale bool, refreshErr error) (ExchangeRateQuote, error) {
	rate, err := snapshot.exchangeRate(base, target)
	if err != nil {
		return ExchangeRateQuote{}, err
	}
	return ExchangeRateQuote{
		Rate:         rate,
		Source:       ECBSource,
		ObservedAt:   snapshot.observedAt,
		FetchedAt:    snapshot.fetchedAt,
		ExpiresAt:    snapshot.expiresAt,
		Stale:        stale,
		RefreshError: refreshErr,
	}, nil
}

func (s *ecbSnapshot) exchangeRate(base Currency, target Currency) (ExchangeRate, error) {
	if sameCurrency(base, target) {
		return NewExchangeRate(base, target, "1")
	}
	baseRate, ok := s.rates[base]
	if !ok {
		return ExchangeRate{}, fmt.Errorf("%w: %s", ErrUnsupportedExchangeRate, base)
	}
	targetRate, ok := s.rates[target]
	if !ok {
		return ExchangeRate{}, fmt.Errorf("%w: %s", ErrUnsupportedExchangeRate, target)
	}
	if sameCurrency(base, EUR) {
		return NewExchangeRate(base, target, targetRate.String())
	}
	if sameCurrency(target, EUR) {
		return NewExchangeRate(target, base, baseRate.String())
	}
	cross, err := targetRate.Quo(baseRate)
	if err != nil {
		return ExchangeRate{}, fmt.Errorf("%w: %w", ErrInvalidExchangeRate, err)
	}
	return NewExchangeRate(base, target, cross.String())
}

func (s *ecbSnapshot) clone() *ecbSnapshot {
	if s == nil {
		return nil
	}
	copied := make(map[Currency]decimal.Decimal, len(s.rates))
	for curr, rate := range s.rates {
		copied[curr] = rate
	}
	return &ecbSnapshot{
		observedAt: s.observedAt,
		fetchedAt:  s.fetchedAt,
		expiresAt:  s.expiresAt,
		rates:      copied,
	}
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
