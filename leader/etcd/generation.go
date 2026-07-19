package etcdleader

import (
	"errors"
	"time"
)

type generation struct {
	ttl       time.Duration
	published bool
}

func (e *Elector) publishGenerationTTL(generation *generation, seconds int64) error {
	ttl, err := ttlDuration(seconds)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if generation == nil || e.current != generation {
		return errors.New("etcd leader generation is no longer active")
	}
	generation.ttl = ttl
	generation.published = true
	e.lastTTL = ttl
	return nil
}
