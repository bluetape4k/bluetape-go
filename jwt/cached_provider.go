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

// CachedProvider 는 Provider에 신뢰된 Reader cache를 추가한다.
type CachedProvider struct {
	provider *Provider
	cache    cache.Cache[string, *Reader]
	cfg      cacheConfig
	flights  singleflight.Group
	mu       sync.Mutex
	epoch    atomic.Uint64
}

var _ Signer = (*CachedProvider)(nil)
var _ Parser = (*CachedProvider)(nil)
var _ Rotator = (*CachedProvider)(nil)

// NewCachedProvider 는 in-memory JWT Provider용 cache adapter를 만든다.
func NewCachedProvider(provider *Provider, c cache.Cache[string, *Reader], options ...CacheOption) (*CachedProvider, error) {
	if err := provider.validateReady(); err != nil {
		return nil, err
	}
	if err := requireReaderCache(c); err != nil {
		return nil, err
	}
	cfg, err := normalizeCacheConfig(options)
	if err != nil {
		return nil, err
	}
	return &CachedProvider{provider: provider, cache: c, cfg: cfg}, nil
}

// Compose 는 token 생성을 wrapped Provider에 위임한다.
func (p *CachedProvider) Compose(options ...ComposeOption) (string, error) {
	if err := p.validateReady(); err != nil {
		return "", err
	}
	return p.provider.Compose(options...)
}

// Parse 는 cache 작업에 context.Background를 사용해 token을 검증한다.
func (p *CachedProvider) Parse(token string, options ...ParseOption) (*Reader, error) {
	return p.ParseContext(context.Background(), token, options...)
}

// TryParse 는 Parse 성공 여부를 bool로 반환한다.
func (p *CachedProvider) TryParse(token string, options ...ParseOption) (*Reader, bool) {
	reader, err := p.Parse(token, options...)
	if err != nil {
		return nil, false
	}
	return reader, true
}

// ParseContext 는 token을 검증하고 성공한 Reader 결과를 cache한다.
func (p *CachedProvider) ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	if token == "" {
		return nil, TokenError{Kind: ErrInvalidToken, Err: errorsNew("empty token")}
	}
	parse, err := normalizeParseConfig(p.provider.now, options)
	if err != nil {
		return nil, err
	}
	profile := buildCacheProfile(p.provider.algorithm, p.cfg, parse, token)
	if !profile.cacheable {
		return p.provider.Parse(token, options...)
	}
	return p.parseWithCache(ctx, profile.key, token, options...)
}

// CurrentKeyChain 은 현재 key 조회를 wrapped Provider에 위임한다.
func (p *CachedProvider) CurrentKeyChain() (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	return p.provider.CurrentKeyChain()
}

// Rotate 는 강제하지 않는 key 회전을 wrapped Provider에 위임한다.
func (p *CachedProvider) Rotate() (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	return p.provider.Rotate()
}

// ForcedRotate 는 wrapped Provider를 강제 회전하고 cached Reader를 지운다.
func (p *CachedProvider) ForcedRotate() (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	key, err := p.provider.ForcedRotate()
	if err != nil {
		return nil, err
	}
	if err := p.ClearCache(context.Background()); err != nil {
		return nil, err
	}
	return key, nil
}

// FindKeyChain 은 kid 조회를 wrapped Provider에 위임한다.
func (p *CachedProvider) FindKeyChain(kid string) (*KeyChain, error) {
	if err := p.validateReady(); err != nil {
		return nil, err
	}
	return p.provider.FindKeyChain(kid)
}

// ClearCache 는 설정된 cache backend의 모든 cached Reader를 지운다.
func (p *CachedProvider) ClearCache(ctx context.Context) error {
	if err := p.validateReady(); err != nil {
		return err
	}
	if err := requireContext(ctx); err != nil {
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

func (p *CachedProvider) parseWithCache(ctx context.Context, key string, token string, options ...ParseOption) (*Reader, error) {
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
			reader, err = p.provider.Parse(token, options...)
			if err != nil {
				return nil, err
			}
			ttl, err := p.readerTTL(reader)
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

func (p *CachedProvider) revalidateCachedReader(ctx context.Context, cacheKey string, reader *Reader) (bool, error) {
	if err := requireContext(ctx); err != nil {
		return false, err
	}
	if err := p.validateReaderFresh(reader); err != nil {
		if deleteErr := p.cache.Delete(ctx, cacheKey); deleteErr != nil {
			return false, fmt.Errorf("jwt cache delete failed: %w", deleteErr)
		}
		return false, nil
	}
	return true, nil
}

func (p *CachedProvider) validateReaderFresh(reader *Reader) error {
	if reader == nil {
		return errorsNew("cached reader must not be nil")
	}
	now := p.cfg.now()
	if reader.Algorithm() != p.provider.algorithm || reader.IsExpired(now) {
		return errorsNew("cached reader is stale")
	}
	key, err := p.provider.FindKeyChain(reader.Kid())
	if err != nil {
		return err
	}
	if key.Algorithm() != p.provider.algorithm || key.Algorithm() != reader.Algorithm() || key.Expired(now) {
		return errorsNew("cached key is stale")
	}
	return nil
}

func (p *CachedProvider) readerTTL(reader *Reader) (time.Duration, error) {
	now := p.cfg.now()
	key, err := p.provider.FindKeyChain(reader.Kid())
	if err != nil {
		return 0, err
	}
	if err := p.provider.requireKeyAlgorithm(key); err != nil {
		return 0, err
	}
	return cacheTTL(p.cfg.maxTTL, now, reader, key), nil
}

func (p *CachedProvider) validateReady() error {
	if p == nil || p.provider == nil {
		return OptionError{Option: "provider", Err: errorsNew("must be constructed by a constructor")}
	}
	if err := p.provider.validateReady(); err != nil {
		return err
	}
	return requireReaderCache(p.cache)
}
