package jwks

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

// Provider 는 원격 JWKS snapshot과 bounded refresh 상태를 소유한다.
// Provider는 반드시 New로 생성해야 하며 zero value는 사용할 수 없다.
type Provider struct {
	endpoint string
	client   *http.Client
	cfg      config

	mu             sync.Mutex
	publication    *publication
	flight         *refreshFlight
	generation     uint64
	forcedAt       time.Time
	refreshError   error
	refreshErrorAt time.Time
}

type publication struct {
	keys       map[string]keyRecord
	fetchedAt  time.Time
	generation uint64
}

type refreshFlight struct {
	done           chan struct{}
	err            error
	callerCanceled bool
}

// New 는 직접 JWKS JSON URL을 검증하고 network-free Provider를 만든다.
func New(endpoint string, options ...Option) (*Provider, error) {
	parsed, err := validateEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	cfg := defaultConfig(parsed.Scheme == "http")
	for _, option := range options {
		if option == nil {
			return nil, optionError("option", errors.New("must not be nil"))
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	return &Provider{endpoint: parsed.String(), client: cfg.client, cfg: cfg}, nil
}

// Lookup 은 kid와 algorithm에 맞는 public key를 반환한다.
func (p *Provider) Lookup(ctx context.Context, kid string, algorithm Algorithm) (any, error) {
	if p == nil {
		return nil, optionError("provider", errors.New("must not be nil"))
	}
	if err := p.validateInitialized(); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, optionError("context", errors.New("must not be nil"))
	}
	if !isSupportedAlgorithm(algorithm) {
		return nil, ErrUnsupportedAlgorithm
	}
	if _, allowed := p.cfg.allowedAlgorithms[algorithm]; !allowed {
		return nil, ErrUnsupportedAlgorithm
	}
	if err := validateKID(kid); err != nil {
		return nil, err
	}
	for attempts := 0; attempts < 2; attempts++ {
		now := p.cfg.now()
		p.mu.Lock()
		publication := p.publication
		if publication != nil && now.Before(publication.fetchedAt.Add(p.cfg.cacheTTL)) {
			record, ok := publication.keys[kid]
			p.mu.Unlock()
			if ok {
				if !recordMatchesAlgorithm(record, algorithm) {
					return nil, ErrUnsupportedAlgorithm
				}
				return cloneKey(record.key), nil
			}
			err := p.refresh(ctx, true, false)
			if err != nil {
				return nil, err
			}
			continue
		}
		p.mu.Unlock()
		if err := p.refresh(ctx, false, false); err != nil {
			return nil, err
		}
		p.mu.Lock()
		publication = p.publication
		record, ok := publication.keys[kid]
		p.mu.Unlock()
		if ok {
			if !recordMatchesAlgorithm(record, algorithm) {
				return nil, ErrUnsupportedAlgorithm
			}
			return cloneKey(record.key), nil
		}
		return nil, ErrKeyNotFound
	}
	return nil, ErrKeyNotFound
}

// Refresh 는 원격 snapshot을 명시적으로 갱신한다.
func (p *Provider) Refresh(ctx context.Context) error {
	if p == nil {
		return optionError("provider", errors.New("must not be nil"))
	}
	if err := p.validateInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		return optionError("context", errors.New("must not be nil"))
	}
	return p.refresh(ctx, true, true)
}

// KeyFunc 는 request context를 캡처한 golang-jwt/v5 Keyfunc를 만든다.
func (p *Provider) KeyFunc(ctx context.Context) (golangjwt.Keyfunc, error) {
	if p == nil {
		return nil, optionError("provider", errors.New("must not be nil"))
	}
	if err := p.validateInitialized(); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, optionError("context", errors.New("must not be nil"))
	}
	return func(token *golangjwt.Token) (any, error) {
		if token == nil {
			return nil, rootTokenError(errors.New("token must not be nil"))
		}
		alg, ok := token.Header["alg"].(string)
		if !ok || alg == "" {
			return nil, rootTokenError(errors.New("algorithm header is required"))
		}
		kid, ok := token.Header["kid"].(string)
		if !ok || !validKID(kid) {
			return nil, rootTokenError(ErrKeyNotFound)
		}
		if err := rejectUnsupportedInboundHeaders(token.Header); err != nil {
			return nil, err
		}
		key, err := p.Lookup(ctx, kid, Algorithm(alg))
		if err != nil {
			return nil, rootTokenError(err)
		}
		return key, nil
	}, nil
}

func (p *Provider) validateInitialized() error {
	if p.endpoint == "" || p.client == nil || p.cfg.cacheTTL <= 0 ||
		p.cfg.refreshCooldown <= 0 || p.cfg.fetchTimeout <= 0 ||
		p.cfg.maxBodySize <= 0 || p.cfg.now == nil || len(p.cfg.allowedAlgorithms) == 0 {
		return optionError("provider", errors.New("must be initialized with New"))
	}
	return nil
}

func (p *Provider) refresh(ctx context.Context, forced, bypassCooldown bool) error {
	for {
		p.mu.Lock()
		if p.flight != nil {
			flight := p.flight
			p.mu.Unlock()
			select {
			case <-flight.done:
				if err := ctx.Err(); err != nil {
					return err
				}
				if flight.callerCanceled {
					continue
				}
				return flight.err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		now := p.cfg.now()
		if forced && !bypassCooldown && !p.forcedAt.IsZero() && now.Sub(p.forcedAt) < p.cfg.refreshCooldown {
			err := p.refreshError
			p.mu.Unlock()
			return err
		}
		if !forced && p.refreshError != nil && now.Sub(p.refreshErrorAt) < p.cfg.refreshCooldown {
			err := p.refreshError
			p.mu.Unlock()
			return err
		}
		flight := &refreshFlight{done: make(chan struct{})}
		p.flight = flight
		p.mu.Unlock()
		return p.executeRefresh(ctx, forced, flight)
	}
}

func (p *Provider) executeRefresh(ctx context.Context, forced bool, flight *refreshFlight) error {
	type result struct {
		keys map[string]keyRecord
		err  error
	}
	results := make(chan result, 1)
	go func() {
		body, err := p.fetch(ctx)
		if err == nil {
			var keys map[string]keyRecord
			keys, err = parseKeySet(body)
			results <- result{keys: keys, err: err}
			return
		}
		results <- result{err: err}
	}()
	select {
	case completed := <-results:
		if callerErr := ctx.Err(); callerErr != nil {
			return p.finishRefresh(flight, forced, nil, callerErr, true)
		}
		return p.finishRefresh(flight, forced, completed.keys, completed.err, false)
	case <-ctx.Done():
		return p.finishRefresh(flight, forced, nil, ctx.Err(), true)
	}
}

func (p *Provider) finishRefresh(flight *refreshFlight, forced bool, keys map[string]keyRecord, err error, callerCanceled bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.flight != flight {
		return err
	}
	flight.err = err
	flight.callerCanceled = callerCanceled
	if err == nil {
		p.generation++
		p.publication = &publication{keys: keys, fetchedAt: p.cfg.now(), generation: p.generation}
		p.refreshError = nil
		p.refreshErrorAt = time.Time{}
		// A successful snapshot also establishes the cooldown anchor for
		// unknown-kid refreshes. Without this, every repeated miss against a
		// warm snapshot would immediately trigger another network request.
		p.forcedAt = p.cfg.now()
	} else if !callerCanceled {
		p.refreshError = err
		p.refreshErrorAt = p.cfg.now()
		if forced {
			p.forcedAt = p.cfg.now()
		}
	}
	p.flight = nil
	close(flight.done)
	return err
}

func recordMatchesAlgorithm(record keyRecord, algorithm Algorithm) bool {
	if record.algorithm != "" && record.algorithm != algorithm {
		return false
	}
	switch key := record.key.(type) {
	case *rsa.PublicKey:
		return isRSAAlgorithm(algorithm)
	case *ecdsa.PublicKey:
		return curveAlgorithm(key.Curve) == algorithm
	case ed25519.PublicKey:
		return algorithm == EdDSA
	default:
		return false
	}
}

func rootTokenError(err error) error {
	return rootjwt.TokenError{Kind: rootjwt.ErrInvalidToken, Err: err}
}

func rejectUnsupportedInboundHeaders(headers map[string]any) error {
	for _, header := range []string{"zip", "crit", "jku", "jwk", "x5u", "x5c"} {
		if _, exists := headers[header]; exists {
			return rootTokenError(errors.New("unsupported header"))
		}
	}
	return nil
}

func (p *Provider) fetch(ctx context.Context) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(ctx, p.cfg.fetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return nil, FetchError{Class: FetchClassTransport, Err: sanitizeFetchCause(err)}
	}
	response, err := p.client.Do(request)
	if err != nil {
		cause := requestContext.Err()
		if cause == nil {
			cause = ctx.Err()
		}
		if cause == nil {
			cause = sanitizeFetchCause(err)
		}
		return nil, FetchError{Class: FetchClassTransport, Err: sanitizeFetchCause(cause)}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, FetchError{Class: FetchClassStatus, Status: response.StatusCode}
	}
	if response.ContentLength > p.cfg.maxBodySize {
		return nil, FetchError{Class: FetchClassBody, Status: response.StatusCode}
	}
	if p.cfg.maxBodySize >= int64(^uint64(0)>>1) {
		return nil, FetchError{Class: FetchClassBody, Status: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, p.cfg.maxBodySize+1))
	if err != nil {
		return nil, FetchError{Class: FetchClassTransport, Status: response.StatusCode, Err: sanitizeFetchCause(err)}
	}
	if int64(len(body)) > p.cfg.maxBodySize {
		return nil, FetchError{Class: FetchClassBody, Status: response.StatusCode}
	}
	return body, nil
}

func validateEndpoint(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, optionError("endpoint", errors.New("must not be empty"))
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, optionError("endpoint", errors.New("must be an absolute URL"))
	}
	if u.User != nil || u.Fragment != "" {
		return nil, optionError("endpoint", errors.New("userinfo and fragment are not allowed"))
	}
	host := u.Hostname()
	if host == "" {
		return nil, optionError("endpoint", errors.New("host must not be empty"))
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, optionError("endpoint", errors.New("scheme must be https"))
	}
	addr := net.ParseIP(host)
	if addr != nil {
		if isBlockedAddress(addr) && (u.Scheme != "http" || !addr.IsLoopback()) {
			return nil, optionError("endpoint", errors.New("private or link-local address is not allowed"))
		}
	}
	if u.Scheme == "http" && (addr == nil || !addr.IsLoopback()) {
		return nil, optionError("endpoint", errors.New("http is only allowed for loopback"))
	}
	return u, nil
}

func isBlockedAddress(addr net.IP) bool {
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() || !addr.IsGlobalUnicast()
}

func validateKID(kid string) error {
	if kid == "" || len(kid) > 128 {
		return ErrKeyNotFound
	}
	for i := 0; i < len(kid); i++ {
		if kid[i] < 0x21 || kid[i] > 0x7e {
			return ErrKeyNotFound
		}
	}
	return nil
}

func defaultHTTPClient(allowLoopback bool) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = (&http.Transport{}).Clone()
	}
	transport.Proxy = nil
	transport.DialTLSContext = nil
	transport.DialTLS = nil //nolint:staticcheck // clear a legacy hook inherited from a mutated global transport
	transport.TLSClientConfig = nil
	transport.TLSNextProto = nil
	transport.MaxResponseHeaderBytes = defaultMaxHeaderSize
	transport.DialContext = restrictedDialContext
	if allowLoopback {
		transport.DialContext = restrictedDialContextAllowLoopback
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func sanitizeFetchCause(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func restrictedDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return restrictedDialContextWithPolicy(ctx, network, address, false, net.DefaultResolver.LookupIPAddr)
}

func restrictedDialContextAllowLoopback(ctx context.Context, network, address string) (net.Conn, error) {
	return restrictedDialContextWithPolicy(ctx, network, address, true, net.DefaultResolver.LookupIPAddr)
}

func restrictedDialContextWithPolicy(
	ctx context.Context,
	network string,
	address string,
	allowLoopback bool,
	lookupIPAddr func(context.Context, string) ([]net.IPAddr, error),
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if literal := net.ParseIP(host); literal != nil {
		if isBlockedAddress(literal) && (!allowLoopback || !literal.IsLoopback()) {
			return nil, errors.New("private or link-local address is not allowed")
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	addresses, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolved := range addresses {
		if isBlockedAddress(resolved.IP) && (!allowLoopback || !resolved.IP.IsLoopback()) {
			lastErr = errors.New("private or link-local address is not allowed")
			continue
		}
		conn, dialErr := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("no usable address")
	}
	return nil, lastErr
}
