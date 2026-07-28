package jwt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"golang.org/x/sync/singleflight"
)

// CachedDistributedProvider DistributedProvider에 신뢰된 Reader cache를 추가한다.
type CachedDistributedProvider struct {
	provider *DistributedProvider
	cache    cache.Cache[string, *Reader]
	cfg      cacheConfig
	flights  singleflight.Group
	mu       sync.Mutex
	epoch    atomic.Uint64
}

// NewCachedDistributedProvider distributed JWT provider용 cache adapter를 만든다.
func NewCachedDistributedProvider(provider *DistributedProvider, c cache.Cache[string, *Reader], options ...CacheOption) (*CachedDistributedProvider, error) {
	if provider == nil {
		return nil, OptionError{Option: "provider", Err: errorsNew("must not be nil")}
	}
	if err := provider.validateReady(context.Background()); err != nil {
		return nil, err
	}
	if err := requireReaderCache(c); err != nil {
		return nil, err
	}
	cfg, err := normalizeCacheConfig(options)
	if err != nil {
		return nil, err
	}
	if !cfg.customNow {
		cfg.now = provider.provider.cfg.now
	}
	return &CachedDistributedProvider{provider: provider, cache: c, cfg: cfg}, nil
}

// ComposeContext token 생성을 wrapped DistributedProvider에 위임한다.
func (p *CachedDistributedProvider) ComposeContext(ctx context.Context, options ...ComposeOption) (string, error) {
	if err := p.validateReady(ctx); err != nil {
		return "", err
	}
	return p.provider.ComposeContext(ctx, options...)
}

// ParseContext token을 검증하고 성공한 distributed parse 결과를 cache한다.
func (p *CachedDistributedProvider) ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	if token == "" {
		return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("empty token")}
	}
	parse, err := normalizeParseConfig(p.provider.provider.now, options)
	if err != nil {
		return nil, err
	}
	profile := buildCacheProfile(p.provider.provider.algorithm, p.cfg, parse, token)
	if !profile.cacheable {
		return p.provider.ParseContext(ctx, token, options...)
	}
	return p.parseWithCache(ctx, profile.key, token, options...)
}

// CurrentKeyChainContext 현재 key 조회를 wrapped provider에 위임한다.
func (p *CachedDistributedProvider) CurrentKeyChainContext(ctx context.Context) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	return p.provider.CurrentKeyChainContext(ctx)
}

// RotateContext 강제하지 않는 key 회전을 wrapped provider에 위임한다.
func (p *CachedDistributedProvider) RotateContext(ctx context.Context) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	return p.provider.RotateContext(ctx)
}

// ForcedRotateContext wrapped provider를 강제 회전하고 cached Reader를 지운다.
func (p *CachedDistributedProvider) ForcedRotateContext(ctx context.Context) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	key, err := p.provider.ForcedRotateContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.ClearCache(ctx); err != nil {
		return nil, err
	}
	return key, nil
}

// FindKeyChainContext kid 조회를 wrapped provider에 위임한다.
func (p *CachedDistributedProvider) FindKeyChainContext(ctx context.Context, kid string) (*KeyChain, error) {
	if err := p.validateReady(ctx); err != nil {
		return nil, err
	}
	return p.provider.FindKeyChainContext(ctx, kid)
}

// DeleteKeyChainsContext repository key를 삭제하고 cached Reader를 지운다.
func (p *CachedDistributedProvider) DeleteKeyChainsContext(ctx context.Context) error {
	if err := p.validateReady(ctx); err != nil {
		return err
	}
	if err := p.provider.DeleteKeyChainsContext(ctx); err != nil {
		return err
	}
	return p.ClearCache(ctx)
}

// ClearCache 설정된 cache backend의 모든 cached Reader를 지운다.
func (p *CachedDistributedProvider) ClearCache(ctx context.Context) error {
	if err := p.validateReady(ctx); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.epoch.Add(1)
	if err := p.cache.Clear(ctx); err != nil {
		return fmt.Errorf("jwt cache clear failed: %w", err)
	}
	return nil
}

func (p *CachedDistributedProvider) parseWithCache(ctx context.Context, key string, token string, options ...ParseOption) (*Reader, error) {
	reader, err := p.cache.Get(ctx, key)
	if err == nil {
		valid, err := p.revalidateCachedReader(ctx, key, reader)
		if err != nil {
			return nil, err
		}
		if valid {
			return reader, nil
		}
	} else if !errors.Is(err, cache.ErrCacheMiss) {
		return nil, fmt.Errorf("jwt cache get failed: %w", err)
	}

	for {
		loadEpoch := p.epoch.Load()
		call := p.flights.DoChan(key, func() (any, error) {
			if err := requireContext(ctx); err != nil {
				return nil, err
			}
			reader, err := p.cache.Get(ctx, key)
			if err == nil {
				valid, err := p.revalidateCachedReader(ctx, key, reader)
				if err != nil {
					return nil, err
				}
				if valid {
					return reader, nil
				}
			} else if !errors.Is(err, cache.ErrCacheMiss) {
				return nil, fmt.Errorf("jwt cache get failed: %w", err)
			}
			reader, err = p.provider.ParseContext(ctx, token, options...)
			if err != nil {
				return nil, err
			}
			ttl, err := p.readerTTL(ctx, reader)
			if err != nil {
				return nil, err
			}
			if ttl > 0 {
				p.mu.Lock()
				if p.epoch.Load() == loadEpoch {
					err = p.cache.Set(ctx, key, reader, ttl)
				}
				p.mu.Unlock()
				if err != nil {
					return nil, fmt.Errorf("jwt cache set failed: %w", err)
				}
			}
			return reader, nil
		})

		select {
		case result := <-call:
			if result.Err != nil {
				if errors.Is(result.Err, context.Canceled) || errors.Is(result.Err, context.DeadlineExceeded) {
					if err := ctx.Err(); err == nil {
						p.flights.Forget(key)
						continue
					}
				}
				return nil, result.Err
			}
			reader, ok := result.Val.(*Reader)
			if !ok || reader == nil {
				return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("cached reader is invalid")}
			}
			return reader, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (p *CachedDistributedProvider) revalidateCachedReader(ctx context.Context, cacheKey string, reader *Reader) (bool, error) {
	if err := requireContext(ctx); err != nil {
		return false, err
	}
	if err := p.validateReaderFresh(ctx, reader); err != nil {
		if deleteErr := p.cache.Delete(ctx, cacheKey); deleteErr != nil {
			return false, fmt.Errorf("jwt cache delete failed: %w", deleteErr)
		}
		return false, nil
	}
	return true, nil
}

func (p *CachedDistributedProvider) validateReaderFresh(ctx context.Context, reader *Reader) error {
	if reader == nil {
		return errorsNew("cached reader must not be nil")
	}
	now := p.cfg.now()
	if reader.Algorithm() != p.provider.provider.algorithm || reader.IsExpired(now) {
		return errorsNew("cached reader is stale")
	}
	key, err := p.provider.FindKeyChainContext(ctx, reader.Kid())
	if err != nil {
		return err
	}
	if key.Algorithm() != p.provider.provider.algorithm || key.Algorithm() != reader.Algorithm() || key.Expired(now) {
		return errorsNew("cached key is stale")
	}
	return nil
}

func (p *CachedDistributedProvider) readerTTL(ctx context.Context, reader *Reader) (time.Duration, error) {
	now := p.cfg.now()
	key, err := p.provider.FindKeyChainContext(ctx, reader.Kid())
	if err != nil {
		return 0, err
	}
	if err := p.provider.provider.requireKeyAlgorithm(key); err != nil {
		return 0, err
	}
	return cacheTTL(p.cfg.maxTTL, now, reader, key), nil
}

func (p *CachedDistributedProvider) validateReady(ctx context.Context) error {
	if p == nil || p.provider == nil {
		return OptionError{Option: "provider", Err: errorsNew("must be constructed by a constructor")}
	}
	if err := p.provider.validateReady(ctx); err != nil {
		return err
	}
	return requireReaderCache(p.cache)
}
