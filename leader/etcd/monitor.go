package etcdleader

import (
	"context"
	"errors"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type generationMonitor struct {
	generation *generation
	created    chan error
	terminal   chan error
	done       chan struct{}
	published  chan struct{}
}

func startGenerationMonitor(
	elector *Elector,
	generation *generation,
	watch clientv3.WatchChan,
	token string,
) *generationMonitor {
	monitor := &generationMonitor{
		generation: generation,
		created:    make(chan error, 1),
		terminal:   make(chan error, 1),
		done:       make(chan struct{}),
		published:  make(chan struct{}),
	}
	generation.monitorDone = monitor.done
	go monitor.run(elector, generation, watch, token)
	return monitor
}

func (monitor *generationMonitor) publish() {
	monitor.generation.publishMonitor()
	close(monitor.published)
}

func (monitor *generationMonitor) waitCreated(ctx context.Context) error {
	select {
	case err := <-monitor.created:
		return err
	case err := <-monitor.terminal:
		if err == nil {
			return errors.New("etcd leader monitor terminated before watch creation")
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (monitor *generationMonitor) run(
	elector *Elector,
	generation *generation,
	watch clientv3.WatchChan,
	token string,
) {
	defer close(monitor.done)
	defer generation.finishMonitor()
	created := false
	publish := monitor.published
	var ticker electorTicker
	var ticks <-chan time.Time
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()
	fail := func(err error) {
		if !created {
			monitor.created <- err
		}
		monitor.terminal <- err
		elector.loseGeneration(generation)
	}
	sessionDone := generation.ops.sessionDone(generation.session)

	for {
		select {
		case <-generation.ctx.Done():
			fail(generation.ctx.Err())
			return
		case <-sessionDone:
			fail(errors.New("etcd leader Session ended"))
			return
		case <-publish:
			publish = nil
			ticker = generation.ops.newTicker(elector.opts.RenewInterval)
			if ticker == nil {
				fail(errors.New("etcd leader renewal ticker is unavailable"))
				return
			}
			ticks = ticker.C()
			if ticks == nil {
				fail(errors.New("etcd leader renewal ticker is unavailable"))
				return
			}
		case <-ticks:
			renewCtx, cancel := context.WithTimeout(generation.ctx, elector.operationBudget(generation.ttl))
			err := func() error {
				inFlightEtcdProclaims.Add(1)
				defer inFlightEtcdProclaims.Add(-1)
				return generation.ops.proclaim(renewCtx, generation.election, token)
			}()
			cancel()
			if err == nil {
				err = generation.runTestHook("renew", "after")
			}
			if err != nil {
				fail(err)
				return
			}
		case response, ok := <-watch:
			if !ok {
				fail(errors.New("etcd leader watch closed"))
				return
			}
			watchCreated, err := validateWatchResponse(response, generation, token)
			if err != nil {
				fail(err)
				return
			}
			if watchCreated && !created {
				if err := monitor.drainReadyWatch(watch, generation, token); err != nil {
					fail(err)
					return
				}
				created = true
				monitor.created <- nil
			}
		}
	}
}

func (e *Elector) loseGeneration(generation *generation) {
	e.mu.Lock()
	if e.current == generation {
		generation.published = false
	}
	e.mu.Unlock()
	_ = generation.shutdown(context.Background())
}

func (monitor *generationMonitor) drainReadyWatch(
	watch clientv3.WatchChan,
	generation *generation,
	token string,
) error {
	for {
		select {
		case response, ok := <-watch:
			if !ok {
				return errors.New("etcd leader watch closed")
			}
			if _, err := validateWatchResponse(response, generation, token); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func validateWatchResponse(response clientv3.WatchResponse, generation *generation, token string) (bool, error) {
	if err := response.Err(); err != nil {
		return false, err
	}
	if response.Canceled || response.CompactRevision != 0 {
		return false, errors.New("etcd leader watch canceled")
	}
	for _, event := range response.Events {
		if event == nil || event.Kv == nil {
			return false, errors.New("etcd leader watch event is invalid")
		}
		if event.Type == clientv3.EventTypeDelete {
			return false, errors.New("etcd leader candidate was deleted")
		}
		if event.Type != clientv3.EventTypePut || string(event.Kv.Key) != generation.key ||
			string(event.Kv.Value) != token || event.Kv.CreateRevision != generation.createRev ||
			clientv3.LeaseID(event.Kv.Lease) != generation.leaseID {
			return false, errors.New("etcd leader candidate changed")
		}
	}
	return response.Created, nil
}
