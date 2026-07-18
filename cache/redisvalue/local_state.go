package redisvalue

import (
	"context"
	"errors"
	"sync"
)

var errTicketConsumed = errors.New("redisvalue: side-effect ticket already consumed")

type localPhase uint8

const (
	phaseHealthy localPhase = iota
	phaseBlocking
	phaseBlocked
	phaseRepairing
)

type repairOrigin uint8

const (
	repairFromHealthy repairOrigin = iota
	repairFromBlocked
)

type repairKind uint8

const (
	repairMandatory repairKind = iota
	repairExplicit
)

type localDisposition uint8

const (
	localCurrent localDisposition = iota
	localNewerGeneration
	localBlocked
)

type localState struct {
	mu         sync.Mutex
	phase      localPhase
	origin     repairOrigin
	generation uint64
	active     int64
	epoch      uint64
	changed    chan struct{}
}

type localLease struct {
	state      *localState
	generation uint64
	released   bool
}

type maintenanceLease struct {
	state    *localState
	released bool
}

type sideEffectTicket struct {
	generation uint64
	used       bool
}

type repairLease struct {
	epoch  uint64
	origin repairOrigin
	kind   repairKind
}

func newLocalState() *localState {
	return &localState{changed: make(chan struct{})}
}

func (s *localState) acquireHealthy(ctx context.Context) (localLease, error) {
	ctx = normalizeContext(ctx)
	for {
		if err := ctx.Err(); err != nil {
			return localLease{}, err
		}
		lease, changed, err := s.tryAcquireHealthy()
		if err != nil {
			return localLease{}, err
		}
		if changed == nil {
			return lease, nil
		}
		select {
		case <-ctx.Done():
			return localLease{}, ctx.Err()
		case <-changed:
		}
	}
}

func (s *localState) tryAcquireHealthy() (localLease, <-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.phase == phaseHealthy:
		s.active++
		return localLease{state: s, generation: s.generation}, nil, nil
	case s.phase == phaseRepairing && s.origin == repairFromHealthy:
		return localLease{}, s.changed, nil
	default:
		return localLease{}, nil, localBlockedError(nil)
	}
}

func (s *localState) acquireMaintenance(ctx context.Context) (maintenanceLease, error) {
	ctx = normalizeContext(ctx)
	for {
		if err := ctx.Err(); err != nil {
			return maintenanceLease{}, err
		}
		s.mu.Lock()
		if s.phase == phaseHealthy || s.phase == phaseBlocked {
			s.active++
			lease := maintenanceLease{state: s}
			s.mu.Unlock()
			return lease, nil
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return maintenanceLease{}, ctx.Err()
		case <-changed:
		}
	}
}

func (l *localLease) issueTicket() (sideEffectTicket, bool) {
	if l == nil || l.state == nil || l.released {
		return sideEffectTicket{}, false
	}
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	if l.released || l.state.phase != phaseHealthy || l.state.generation != l.generation {
		return sideEffectTicket{}, false
	}
	return sideEffectTicket{generation: l.generation}, true
}

func (t *sideEffectTicket) consume(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if t.used {
		return errTicketConsumed
	}
	t.used = true
	return nil
}

func (l *localLease) release() {
	if l == nil || l.state == nil || l.released {
		return
	}
	l.state.mu.Lock()
	if !l.released {
		l.released = true
		l.state.releaseActiveLocked()
	}
	l.state.mu.Unlock()
}

func (l *maintenanceLease) release() {
	if l == nil || l.state == nil || l.released {
		return
	}
	l.state.mu.Lock()
	if !l.released {
		l.released = true
		l.state.releaseActiveLocked()
	}
	l.state.mu.Unlock()
}

func (s *localState) releaseActiveLocked() {
	if s.active <= 0 {
		panic("redisvalue: local-state active lease underflow")
	}
	s.active--
	if s.active == 0 && s.phase == phaseRepairing {
		s.broadcastLocked()
	}
}

func (s *localState) beginRepair(ctx context.Context, kind repairKind) (repairLease, error) {
	ctx = normalizeContext(ctx)
	for {
		if err := ctx.Err(); err != nil {
			return repairLease{}, err
		}
		s.mu.Lock()
		if s.phase == phaseRepairing {
			changed := s.changed
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return repairLease{}, ctx.Err()
			case <-changed:
				continue
			}
		}

		origin := repairFromHealthy
		if s.phase == phaseBlocked || s.phase == phaseBlocking {
			origin = repairFromBlocked
		}
		s.phase = phaseRepairing
		s.origin = origin
		s.generation++
		s.epoch++
		lease := repairLease{epoch: s.epoch, origin: origin, kind: kind}
		s.broadcastLocked()

		for s.active > 0 && s.phase == phaseRepairing && s.epoch == lease.epoch {
			changed := s.changed
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				s.mu.Lock()
				if s.phase == phaseRepairing && s.epoch == lease.epoch {
					s.phase = phaseBlocked
					s.origin = repairFromBlocked
					s.broadcastLocked()
				}
				s.mu.Unlock()
				return repairLease{}, ctx.Err()
			case <-changed:
				s.mu.Lock()
			}
		}
		if s.phase != phaseRepairing || s.epoch != lease.epoch {
			s.mu.Unlock()
			return repairLease{}, localBlockedError(nil)
		}
		s.mu.Unlock()
		return lease, nil
	}
}

func (s *localState) finishRepair(lease repairLease, repairErr error) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != phaseRepairing || s.epoch != lease.epoch {
		return false
	}
	healed := repairErr == nil && (lease.origin == repairFromHealthy || lease.kind == repairExplicit)
	if healed {
		s.phase = phaseHealthy
		s.origin = repairFromHealthy
	} else {
		s.phase = phaseBlocked
		s.origin = repairFromBlocked
	}
	s.broadcastLocked()
	return healed
}

func (s *localState) block() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == phaseBlocking || s.phase == phaseBlocked {
		return
	}
	s.phase = phaseBlocking
	s.origin = repairFromBlocked
	s.generation++
	s.epoch++
	s.broadcastLocked()
	s.phase = phaseBlocked
	s.broadcastLocked()
}

func (s *localState) classify(generation uint64) localDisposition {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.phase == phaseHealthy && s.generation == generation:
		return localCurrent
	case s.phase == phaseHealthy:
		return localNewerGeneration
	case s.phase == phaseRepairing && s.origin == repairFromHealthy:
		return localNewerGeneration
	default:
		return localBlocked
	}
}

func (s *localState) phaseValue() localPhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

func (s *localState) activeValue() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *localState) repairEpochValue() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

func (s *localState) broadcastLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func localBlockedError(cause error) error {
	return newCacheError("local-state", ReasonLocalBlocked, "", cause)
}
