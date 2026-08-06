package jwt

import (
	"context"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

// DistributedProvider JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
type DistributedProvider struct {
	provider *Provider
	repo     DistributedKeyChainRepository
}

// NewDistributedHMACProvider JWT key provider repository에서 생성과 초기화 계약을 설명한다.
func NewDistributedHMACProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error) {
	if _, ok := algorithm.hmacSecretLength(); !ok {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be hmac")}
	}
	return newDistributedProvider(ctx, repo, algorithm, options...)
}

// NewDistributedRSAProvider JWT key provider repository에서 생성과 초기화 계약을 설명한다.
func NewDistributedRSAProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error) {
	if !algorithm.isRSA() {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be rsa")}
	}
	return newDistributedProvider(ctx, repo, algorithm, options...)
}

func newDistributedProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	if err := requireDistributedRepository(repo); err != nil {
		return nil, err
	}
	cfg, err := normalizeProviderConfig(options)
	if err != nil {
		return nil, err
	}
	p := &Provider{algorithm: algorithm, cfg: cfg}
	key, err := repo.Rotate(ctx, createWithContext(ctx, p.createKeyChain), p.now())
	if err != nil {
		return nil, err
	}
	if err := p.requireKeyAlgorithm(key); err != nil {
		return nil, err
	}
	return &DistributedProvider{provider: p, repo: repo}, nil
}

// ComposeContext JWT key provider repository에서 생성과 초기화 계약을 설명한다.
func (p *DistributedProvider) ComposeContext(ctx context.Context, options ...ComposeOption) (string, error) {
	if err := p.validateReady(ctx); err != nil {
		return "", err
	}
	key, err := p.repo.Rotate(ctx, createWithContext(ctx, p.provider.createKeyChain), p.provider.now())
	if err != nil {
		return "", err
	}
	if err := p.provider.requireKeyAlgorithm(key); err != nil {
		return "", err
	}
	return p.provider.composeWithKey(key, options...)
}

// ParseContext JWT key provider repository에서 동작과 caller-visible 계약을 설명한다.
func (p *DistributedProvider) ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	return p.provider.parseWithKeyFunc(token, p.distributedKeyFunc(ctx), options...)
}

// CurrentKeyChainContext JWT key provider repository에서 반환값과 오류 의미를 설명한다.
func (p *DistributedProvider) CurrentKeyChainContext(ctx context.Context) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	key, err := p.repo.Current(ctx, p.provider.now())
	if err != nil {
		return nil, err
	}
	if err := p.provider.requireKeyAlgorithm(key); err != nil {
		return nil, err
	}
	return key, nil
}

// RotateContext JWT key provider repository에서 반환값과 오류 의미를 설명한다.
func (p *DistributedProvider) RotateContext(ctx context.Context) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	key, err := p.repo.Rotate(ctx, createWithContext(ctx, p.provider.createKeyChain), p.provider.now())
	if err != nil {
		return nil, err
	}
	if err := p.provider.requireKeyAlgorithm(key); err != nil {
		return nil, err
	}
	return key, nil
}

// ForcedRotateContext JWT key provider repository에서 생성과 초기화 계약을 설명한다.
func (p *DistributedProvider) ForcedRotateContext(ctx context.Context) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	key, err := p.repo.ForcedRotate(ctx, createWithContext(ctx, p.provider.createKeyChain), p.provider.now())
	if err != nil {
		return nil, err
	}
	if err := p.provider.requireKeyAlgorithm(key); err != nil {
		return nil, err
	}
	return key, nil
}

// FindKeyChainContext JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
func (p *DistributedProvider) FindKeyChainContext(ctx context.Context, kid string) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	if err := validateLookupKID(kid); err != nil {
		return nil, err
	}
	key, err := p.repo.Find(ctx, kid, p.provider.now())
	if err != nil {
		return nil, err
	}
	if err := p.provider.requireKeyAlgorithm(key); err != nil {
		return nil, err
	}
	return key, nil
}

// DeleteKeyChainsContext JWT key provider repository에서 실행, cancellation, cleanup 계약을 설명한다.
func (p *DistributedProvider) DeleteKeyChainsContext(ctx context.Context) error {
	if err := p.validateReady(ctx); err != nil {
		return err
	}
	return p.repo.DeleteAll(ctx)
}

func (p *DistributedProvider) validateReady(ctx context.Context) error {
	if p == nil || p.provider == nil {
		return OptionError{Option: "provider", Err: errorsNew("must be constructed by a constructor")}
	}
	if err := requireDistributedRepository(p.repo); err != nil {
		return err
	}
	return requireContext(ctx)
}

func (p *DistributedProvider) distributedKeyFunc(ctx context.Context) func(func() time.Time) golangjwt.Keyfunc {
	return func(now func() time.Time) golangjwt.Keyfunc {
		return func(token *golangjwt.Token) (any, error) {
			if err := requireContext(ctx); err != nil {
				return nil, err
			}
			if err := rejectUnsupportedInboundHeaders(token.Header); err != nil {
				return nil, err
			}
			alg, _ := token.Header["alg"].(string)
			if Algorithm(alg) != p.provider.algorithm {
				return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("algorithm mismatch")}
			}
			kid, _ := token.Header["kid"].(string)
			if err := validateLookupKID(kid); err != nil {
				return nil, TokenError{Kind: ErrInvalidToken, Err: err}
			}
			key, err := p.repo.Find(ctx, kid, now())
			if err != nil {
				return nil, err
			}
			if err := p.provider.requireKeyAlgorithm(key); err != nil {
				return nil, err
			}
			return key.verificationMaterial(), nil
		}
	}
}
