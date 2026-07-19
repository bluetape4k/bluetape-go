package etcdleader_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	etcdleader "github.com/bluetape4k/bluetape-go/leader/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func ExampleNew() {
	run := func(
		ctx context.Context,
		client *clientv3.Client,
		startProtectedWork func(context.Context, func() bool) <-chan struct{},
		reconcile func(context.Context) error,
		scheduleRecheck func(time.Duration) error,
		recordCleanupFailure func(error),
	) (resultErr error) {
		opts := leader.Options{
			Group:         "billing-workers",
			MemberID:      "worker-1",
			Lease:         30 * time.Second,
			RenewInterval: 10 * time.Second,
		}
		elector, err := etcdleader.New(client, opts)
		if err != nil {
			return err
		}
		campaignCtx, cancelCampaign := context.WithTimeout(ctx, 15*time.Second)
		defer cancelCampaign()
		if campaignErr := elector.Campaign(campaignCtx); campaignErr != nil {
			return errors.Join(campaignErr, retryResignAndReconcile(
				elector, reconcile, scheduleRecheck, recordCleanupFailure,
			))
		}
		// A nil Campaign result does not outrank a concurrently completed caller deadline.
		if err := campaignCtx.Err(); err != nil {
			return errors.Join(err, retryResignAndReconcile(
				elector, reconcile, scheduleRecheck, recordCleanupFailure,
			))
		}
		if !elector.IsLeader() {
			return errors.Join(leader.ErrNotLeader, retryResignAndReconcile(
				elector, reconcile, scheduleRecheck, recordCleanupFailure,
			))
		}

		protectedCtx, stopProtectedWork := context.WithCancel(ctx)
		// startProtectedWork must call the supplied guard immediately before
		// every protected work unit; the loop below also guards long units.
		protectedDone := startProtectedWork(protectedCtx, elector.IsLeader)
		defer func() {
			stopProtectedWork()
			joinErr := joinProtectedWork(protectedDone)
			resultErr = errors.Join(resultErr, joinErr)
			if joinErr != nil {
				return // Preserve leadership inventory; the caller must terminate this process lane.
			}
			resultErr = errors.Join(
				resultErr,
				retryResignAndReconcile(
					elector, reconcile, scheduleRecheck, recordCleanupFailure,
				),
			)
		}()
		poll := time.NewTicker(min(opts.RenewInterval, time.Second))
		defer poll.Stop()
		for {
			select {
			case <-ctx.Done():
				stopProtectedWork()
				return ctx.Err()
			case <-poll.C:
				if !elector.IsLeader() {
					stopProtectedWork()
					return leader.ErrNotLeader
				}
			}
		}
	}

	_ = run
}

func Example_shutdownSupervisor() {
	shutdown := func(
		initiatingErr error,
		cancelCampaigns func(),
		campaignsDone <-chan struct{},
		electors []*etcdleader.Elector,
		stopAndJoinProtectedWork func() error,
		coordinateSharedClientUsers func() error,
		closeCallerClient func() error,
		persistUnresolved func(error) error,
		proveExactRangeAbsent func(context.Context) error,
		scheduleRecheck func(time.Duration) error,
		recordCleanupFailure func(error),
		restorePreviousProvider func() error,
		verifyZeroEtcdContenders func() error,
	) error {
		cancelCampaigns()
		joinErr := stopAndJoinProtectedWork()
		if joinErr != nil {
			return errors.Join(initiatingErr, joinErr, persistUnresolved(joinErr))
		}
		var cleanupErr error
		grace := time.NewTimer(2 * time.Second)
		joined := false
		select {
		case <-campaignsDone:
			joined = true
			grace.Stop()
		case <-grace.C:
		}
		if joined {
			for _, elector := range electors {
				cleanupErr = errors.Join(
					cleanupErr,
					retryResignAndReconcile(
						elector, proveExactRangeAbsent, scheduleRecheck, recordCleanupFailure,
					),
				)
			}
		} else {
			cleanupErr = errors.Join(cleanupErr, hardStopCampaigns(
				campaignsDone,
				coordinateSharedClientUsers,
				closeCallerClient,
				5*time.Second,
			))
			if cleanupErr != nil {
				return errors.Join(initiatingErr, cleanupErr, persistUnresolved(cleanupErr))
			}
		}
		if cleanupErr != nil {
			cleanupErr = errors.Join(cleanupErr, persistUnresolved(cleanupErr))
		}

		// A separate healthy client must prove exact absence; elapsed TTL is not proof.
		proofCtx, cancelProof := context.WithTimeout(context.Background(), 5*time.Second)
		proofErr := proveExactRangeAbsent(proofCtx)
		cancelProof()
		if proofErr != nil {
			return errors.Join(initiatingErr, cleanupErr, proofErr)
		}
		// Rollback is symmetric and starts only after protected work and etcd contenders stop.
		zeroErr := verifyZeroEtcdContenders()
		if zeroErr != nil {
			return errors.Join(initiatingErr, cleanupErr, zeroErr)
		}
		return errors.Join(initiatingErr, cleanupErr, restorePreviousProvider())
	}

	_ = shutdown
}

func ExampleNew_productionTLS() {
	newClient := func(
		endpoints []string,
		roots *x509.CertPool,
		serverName, username, password string,
	) (*clientv3.Client, error) {
		tlsConfig := &tls.Config{
			MinVersion:         tls.VersionTLS13,
			RootCAs:            roots,
			ServerName:         serverName,
			InsecureSkipVerify: false,
		}
		if err := validateProductionTLS(tlsConfig); err != nil {
			return nil, err
		}
		if err := validateProductionCredentials(username, password); err != nil {
			return nil, err
		}
		return clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 5 * time.Second,
			TLS:         tlsConfig,
			Username:    username,
			Password:    password,
		})
	}

	_ = newClient
}

func retryResignAndReconcile(
	elector *etcdleader.Elector,
	reconcile func(context.Context) error,
	scheduleRecheck func(time.Duration) error,
	recordFailure func(error),
) error {
	var lastErr error
	for range 3 {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), min(5*time.Second, elector.EffectiveTTL()/4))
		lastErr = elector.Resign(cleanupCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		recordCleanupAttempt(recordFailure, lastErr)
	}
	proofCtx, cancelProof := context.WithTimeout(context.Background(), 5*time.Second)
	proofErr := reconcile(proofCtx)
	cancelProof()
	return finishCleanupAfterProof(
		lastErr, proofErr, elector.EffectiveTTL(), scheduleRecheck, recordFailure,
	)
}

func finishCleanupAfterProof(
	lastErr error,
	proofErr error,
	retryAfter time.Duration,
	scheduleRecheck func(time.Duration) error,
	recordFailure func(error),
) error {
	if proofErr == nil {
		return nil // Prior failed attempts remain in the recorder, not unresolved inventory.
	}
	recordCleanupAttempt(recordFailure, proofErr)
	if scheduleRecheck == nil {
		schedulerErr := errors.New("cleanup recheck scheduler is nil")
		recordCleanupAttempt(recordFailure, schedulerErr)
		return errors.Join(lastErr, proofErr, schedulerErr)
	}
	// EffectiveTTL schedules a future proof attempt; it is not deletion proof.
	scheduleErr := scheduleRecheck(retryAfter)
	recordCleanupAttempt(recordFailure, scheduleErr)
	return errors.Join(lastErr, proofErr, scheduleErr)
}

func recordCleanupAttempt(record func(error), err error) {
	if record != nil && err != nil {
		record(err) // The caller must sanitize before logging or labeling the error.
	}
}

func hardStopCampaigns(
	campaignsDone <-chan struct{},
	coordinateSharedClientUsers func() error,
	closeCallerClient func() error,
	joinTimeout time.Duration,
) error {
	if campaignsDone == nil {
		return errors.New("campaign completion channel is nil")
	}
	if err := coordinateSharedClientUsers(); err != nil {
		return err
	}
	if err := closeCallerClient(); err != nil {
		return err
	}
	timer := time.NewTimer(joinTimeout)
	defer timer.Stop()
	select {
	case <-campaignsDone:
		return nil
	case <-timer.C:
		return errors.New("campaigns did not join after caller client close")
	}
}

func joinProtectedWork(done <-chan struct{}) error {
	if done == nil {
		return errors.New("protected work completion channel is nil")
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("protected work did not join")
	}
}

func validateProductionTLS(config *tls.Config) error {
	if config == nil || config.RootCAs == nil || config.RootCAs.Equal(x509.NewCertPool()) {
		return errors.New("etcd TLS requires a non-empty root CA pool")
	}
	if strings.TrimSpace(config.ServerName) == "" {
		return errors.New("etcd TLS requires ServerName")
	}
	if config.InsecureSkipVerify {
		return errors.New("etcd TLS forbids InsecureSkipVerify")
	}
	return nil
}

func validateProductionCredentials(username, password string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("etcd authentication requires a username")
	}
	if password == "" {
		return errors.New("etcd authentication requires a password")
	}
	return nil
}
