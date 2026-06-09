package jwt

import (
	"sync"
	"time"
)

type keyChainRepository struct {
	mu       sync.RWMutex
	capacity int
	keys     []*KeyChain
}

func newKeyChainRepository(capacity int) (*keyChainRepository, error) {
	if capacity == 0 {
		capacity = defaultRepositorySize
	}
	if capacity < minRepositorySize || capacity > maxRepositorySize {
		return nil, OptionError{Option: "capacity", Err: errorsNew("outside repository capacity bounds")}
	}
	return &keyChainRepository{capacity: capacity}, nil
}

func (r *keyChainRepository) current(now time.Time) (*KeyChain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, key := range r.keys {
		if !key.Expired(now) {
			return key, nil
		}
	}
	return nil, KeyError{Kind: ErrKeyNotFound, Err: errorsNew("current key not found")}
}

func (r *keyChainRepository) find(kid string, now time.Time) (*KeyChain, error) {
	if kid == "" {
		return nil, KeyError{Kind: ErrKeyNotFound, Err: errorsNew("kid is required")}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, key := range r.keys {
		if key.KID() == kid {
			if key.Expired(now) {
				return nil, KeyError{Kind: ErrInvalidKey, KID: kid, Err: errorsNew("key expired")}
			}
			return key, nil
		}
	}
	return nil, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("key not found")}
}

func (r *keyChainRepository) rotate(create func() (*KeyChain, error), now time.Time) (*KeyChain, error) {
	if current, err := r.current(now); err == nil {
		return current, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, key := range r.keys {
		if !key.Expired(now) {
			return key, nil
		}
	}
	key, err := create()
	if err != nil {
		return nil, err
	}
	r.prependLocked(key)
	return key, nil
}

func (r *keyChainRepository) forceRotate(create func() (*KeyChain, error)) (*KeyChain, error) {
	key, err := create()
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prependLocked(key)
	return key, nil
}

func (r *keyChainRepository) prependLocked(key *KeyChain) {
	r.keys = append([]*KeyChain{key}, r.keys...)
	if len(r.keys) > r.capacity {
		r.keys = r.keys[:r.capacity]
	}
}
