package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

// Signer JWT 문자열을 생성한다.
type Signer interface {
	Compose(options ...ComposeOption) (string, error)
}

// Parser JWT 문자열을 검증하고 읽는다.
type Parser interface {
	Parse(token string, options ...ParseOption) (*Reader, error)
	TryParse(token string, options ...ParseOption) (*Reader, bool)
}

// Rotator in-memory KeyChain 회전을 제어한다.
type Rotator interface {
	CurrentKeyChain() (*KeyChain, error)
	Rotate() (*KeyChain, error)
	ForcedRotate() (*KeyChain, error)
	FindKeyChain(kid string) (*KeyChain, error)
}

// Provider JWT 생성, 검증, in-memory KeyChain 회전을 제공한다.
type Provider struct {
	algorithm Algorithm
	cfg       providerConfig
	repo      *keyChainRepository
	fixedKey  *KeyChain
}

var _ Signer = (*Provider)(nil)
var _ Parser = (*Provider)(nil)
var _ Rotator = (*Provider)(nil)

// NewFixedHMACProvider 하나의 HMAC secret으로 고정 provider를 만든다.
func NewFixedHMACProvider(algorithm Algorithm, secret []byte, options ...ProviderOption) (*Provider, error) {
	cfg, err := normalizeProviderConfig(options)
	if err != nil {
		return nil, err
	}
	kid, err := nextKID(cfg)
	if err != nil {
		return nil, err
	}
	key, err := newHMACKeyChain(kid, algorithm, secret, cfg.now(), cfg.keyTTL)
	if err != nil {
		return nil, err
	}
	return &Provider{algorithm: algorithm, cfg: cfg, fixedKey: key}, nil
}

// NewFixedRSAProvider 하나의 RSA private key로 고정 provider를 만든다.
func NewFixedRSAProvider(algorithm Algorithm, privateKey *rsa.PrivateKey, options ...ProviderOption) (*Provider, error) {
	cfg, err := normalizeProviderConfig(options)
	if err != nil {
		return nil, err
	}
	kid, err := nextKID(cfg)
	if err != nil {
		return nil, err
	}
	key, err := newRSAKeyChain(kid, algorithm, privateKey, cfg.now(), cfg.keyTTL)
	if err != nil {
		return nil, err
	}
	return &Provider{algorithm: algorithm, cfg: cfg, fixedKey: key}, nil
}

// NewHMACProvider in-memory 회전 HMAC provider를 만든다.
func NewHMACProvider(algorithm Algorithm, options ...ProviderOption) (*Provider, error) {
	if _, ok := algorithm.hmacSecretLength(); !ok {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be hmac")}
	}
	return newRotatingProvider(algorithm, options...)
}

// NewRSAProvider in-memory 회전 RSA/PS provider를 만든다.
func NewRSAProvider(algorithm Algorithm, options ...ProviderOption) (*Provider, error) {
	if !algorithm.isRSA() {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be rsa")}
	}
	return newRotatingProvider(algorithm, options...)
}

func newRotatingProvider(algorithm Algorithm, options ...ProviderOption) (*Provider, error) {
	cfg, err := normalizeProviderConfig(options)
	if err != nil {
		return nil, err
	}
	if cfg.entropy == nil {
		cfg.entropy = rand.Reader
	}
	repo, err := newKeyChainRepository(cfg.capacity)
	if err != nil {
		return nil, err
	}
	p := &Provider{algorithm: algorithm, cfg: cfg, repo: repo}
	if _, err := p.ForcedRotate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Compose JWT를 생성하고 서명한다.
func (p *Provider) Compose(options ...ComposeOption) (string, error) {
	if err := p.validateReady(); err != nil {
		return "", err
	}
	key, err := p.CurrentKeyChain()
	if err != nil {
		return "", err
	}
	return p.composeWithKey(key, options...)
}

func (p *Provider) composeWithKey(key *KeyChain, options ...ComposeOption) (string, error) {
	if err := p.requireKeyAlgorithm(key); err != nil {
		return "", err
	}
	method, err := p.algorithm.signingMethod()
	if err != nil {
		return "", err
	}
	cfg := newComposeConfig()
	for _, option := range options {
		if option == nil {
			return "", OptionError{Option: "compose_option", Err: errorsNew("must not be nil")}
		}
		if err := option(&cfg); err != nil {
			return "", err
		}
	}
	headers, claims := cfg.build(p.now())
	headers["kid"] = key.KID()
	token := golangjwt.NewWithClaims(method, claims)
	for header, value := range headers {
		token.Header[header] = value
	}
	signed, err := token.SignedString(key.signingMaterial())
	if err != nil {
		return "", TokenError{Kind: ErrInvalidToken, Err: err}
	}
	return signed, nil
}

// Parse JWT를 검증하고 Reader를 반환한다.
func (p *Provider) Parse(tokenValue string, options ...ParseOption) (*Reader, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	return p.parseWithKeyFunc(tokenValue, p.keyFunc, options...)
}

func (p *Provider) parseWithKeyFunc(tokenValue string, keyFunc func(func() time.Time) golangjwt.Keyfunc, options ...ParseOption) (*Reader, error) {
	if tokenValue == "" {
		return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("empty token")}
	}
	cfg, err := normalizeParseConfig(p.now, options)
	if err != nil {
		return nil, err
	}
	claims := golangjwt.MapClaims{}
	parserOptions := []golangjwt.ParserOption{
		golangjwt.WithValidMethods([]string{string(p.algorithm)}),
		golangjwt.WithTimeFunc(cfg.now),
	}
	if cfg.leeway > 0 {
		parserOptions = append(parserOptions, golangjwt.WithLeeway(cfg.leeway))
	}
	if cfg.expectedIssuer != "" {
		parserOptions = append(parserOptions, golangjwt.WithIssuer(cfg.expectedIssuer))
	}
	if len(cfg.expectedAudience) > 0 {
		parserOptions = append(parserOptions, golangjwt.WithAudience(cfg.expectedAudience...))
	}
	if cfg.expectedSubject != "" {
		parserOptions = append(parserOptions, golangjwt.WithSubject(cfg.expectedSubject))
	}
	if cfg.expirationRequired {
		parserOptions = append(parserOptions, golangjwt.WithExpirationRequired())
	}

	parsed, err := golangjwt.ParseWithClaims(tokenValue, claims, keyFunc(cfg.now), parserOptions...)
	if err != nil {
		return nil, mapTokenError(err)
	}
	if parsed == nil || !parsed.Valid {
		return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("token is not valid")}
	}
	reader, err := newReader(parsed, claims)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// TryParse Parse 성공 여부를 bool로 반환한다.
func (p *Provider) TryParse(token string, options ...ParseOption) (*Reader, bool) {
	reader, err := p.Parse(token, options...)
	if err != nil {
		return nil, false
	}
	return reader, true
}

// CurrentKeyChain 은 현재 서명 KeyChain을 반환한다.
func (p *Provider) CurrentKeyChain() (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	if p.fixedKey != nil {
		return p.fixedKey, nil
	}
	return p.repo.rotate(p.createKeyChain, p.now())
}

// Rotate 현재 key가 만료된 경우에만 새 KeyChain을 만든다.
func (p *Provider) Rotate() (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	if p.fixedKey != nil {
		return p.fixedKey, nil
	}
	return p.repo.rotate(p.createKeyChain, p.now())
}

// ForcedRotate 항상 새 KeyChain을 만든다.
func (p *Provider) ForcedRotate() (*KeyChain, error) {
	if err := p.validateReadyForRotation(); err != nil {
		return nil, err
	}
	if p.fixedKey != nil {
		return nil, OptionError{Option: "provider", Err: errorsNew("fixed provider cannot rotate")}
	}
	return p.repo.forceRotate(p.createKeyChain)
}

// FindKeyChain 은 kid에 해당하는 KeyChain을 찾는다.
func (p *Provider) FindKeyChain(kid string) (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	if p.fixedKey != nil {
		if kid == "" || kid == p.fixedKey.KID() {
			return p.fixedKey, nil
		}
		return nil, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("key not found")}
	}
	return p.repo.find(kid, p.now())
}

func (p *Provider) validateReady() error {
	if p == nil {
		return OptionError{Option: "provider", Err: errorsNew("must not be nil")}
	}
	if p.fixedKey == nil && p.repo == nil {
		return OptionError{Option: "provider", Err: errorsNew("must be constructed by a constructor")}
	}
	return nil
}

func (p *Provider) validateReadyForRotation() error {
	if p == nil {
		return OptionError{Option: "provider", Err: errorsNew("must not be nil")}
	}
	if p.fixedKey == nil && p.repo == nil {
		return OptionError{Option: "provider", Err: errorsNew("must be constructed by a constructor")}
	}
	return nil
}

func (p *Provider) requireKeyAlgorithm(key *KeyChain) error {
	if key == nil {
		return KeyError{Kind: ErrInvalidKey, Err: errorsNew("key must not be nil")}
	}
	if key.Algorithm() != p.algorithm {
		return KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("key algorithm mismatch")}
	}
	return nil
}

func (p *Provider) keyFunc(now func() time.Time) golangjwt.Keyfunc {
	return func(token *golangjwt.Token) (any, error) {
		if err := rejectUnsupportedInboundHeaders(token.Header); err != nil {
			return nil, err
		}
		alg, _ := token.Header["alg"].(string)
		if Algorithm(alg) != p.algorithm {
			return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("algorithm mismatch")}
		}
		kid, _ := token.Header["kid"].(string)
		var key *KeyChain
		var err error
		if p.fixedKey != nil && kid == "" {
			key = p.fixedKey
		} else if p.fixedKey != nil {
			key, err = p.FindKeyChain(kid)
		} else {
			key, err = p.repo.find(kid, now())
		}
		if err != nil {
			return nil, TokenError{Kind: ErrInvalidToken, Err: err}
		}
		if key.Algorithm() != p.algorithm {
			return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("key algorithm mismatch")}
		}
		return key.verificationMaterial(), nil
	}
}

func rejectUnsupportedInboundHeaders(headers map[string]any) error {
	for header := range headers {
		if _, reserved := reservedHeaders[header]; reserved && header != "alg" && header != "kid" {
			return TokenError{Kind: ErrInvalidToken, Err: errorsNew("unsupported header")}
		}
	}
	return nil
}

func (p *Provider) createKeyChain() (*KeyChain, error) {
	kid, err := nextKID(p.cfg)
	if err != nil {
		return nil, err
	}
	now := p.now()
	if _, ok := p.algorithm.hmacSecretLength(); ok {
		secret, err := generateHMACSecret(p.algorithm, p.entropy())
		if err != nil {
			return nil, err
		}
		return newHMACKeyChain(kid, p.algorithm, secret, now, p.cfg.keyTTL)
	}
	privateKey, err := rsa.GenerateKey(p.entropy(), p.cfg.rsaKeyBits)
	if err != nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: err}
	}
	return newRSAKeyChain(kid, p.algorithm, privateKey, now, p.cfg.keyTTL)
}

func (p *Provider) now() time.Time {
	if p.cfg.now == nil {
		return time.Now()
	}
	return p.cfg.now()
}

func (p *Provider) entropy() io.Reader {
	if p.cfg.entropy == nil {
		return rand.Reader
	}
	return p.cfg.entropy
}

func nextKID(cfg providerConfig) (string, error) {
	if cfg.keyID != nil {
		return cfg.keyID()
	}
	return generateKID(cfg.entropy)
}

func mapTokenError(err error) error {
	switch {
	case errors.Is(err, golangjwt.ErrTokenExpired):
		return TokenError{Kind: ErrExpiredToken, Err: err}
	case errors.Is(err, golangjwt.ErrTokenNotValidYet), errors.Is(err, golangjwt.ErrTokenUsedBeforeIssued):
		return TokenError{Kind: ErrNotYetValid, Err: err}
	case errors.Is(err, ErrInvalidKey), errors.Is(err, ErrKeyNotFound):
		return TokenError{Kind: ErrInvalidToken, Err: err}
	default:
		return TokenError{Kind: ErrInvalidToken, Err: err}
	}
}
