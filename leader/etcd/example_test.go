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
		startProtectedWork func(context.Context) <-chan struct{},
		reconcile func(context.Context) error,
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
			return errors.Join(campaignErr, retryResignAndReconcile(elector, reconcile))
		}
		defer func() { resultErr = errors.Join(resultErr, retryResignAndReconcile(elector, reconcile)) }()
		// A nil Campaign result does not outrank a concurrently completed caller deadline.
		if err := campaignCtx.Err(); err != nil {
			return err
		}

		protectedCtx, stopProtectedWork := context.WithCancel(ctx)
		protectedDone := startProtectedWork(protectedCtx)
		defer func() {
			stopProtectedWork()
			resultErr = errors.Join(resultErr, joinProtectedWork(protectedDone))
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
		restorePreviousProvider func() error,
		verifyZeroEtcdContenders func() error,
	) error {
		cancelCampaigns()
		cleanupErr := stopAndJoinProtectedWork()
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
				cleanupErr = errors.Join(cleanupErr, retryResignAndReconcile(elector, proveExactRangeAbsent))
			}
		} else {
			cleanupErr = errors.Join(cleanupErr, coordinateSharedClientUsers(), closeCallerClient())
			<-campaignsDone // client close is the hard stop for official cleanup on client.Ctx().
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
		return errors.Join(
			initiatingErr,
			cleanupErr,
			restorePreviousProvider(),
			verifyZeroEtcdContenders(),
		)
	}

	_ = shutdown
}

func ExampleNew_productionTLS() {
	newClient := func(endpoints []string, roots *x509.CertPool, serverName string) (*clientv3.Client, error) {
		tlsConfig := &tls.Config{
			MinVersion:         tls.VersionTLS13,
			RootCAs:            roots,
			ServerName:         serverName,
			InsecureSkipVerify: false,
		}
		if err := validateProductionTLS(tlsConfig); err != nil {
			return nil, err
		}
		return clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 5 * time.Second,
			TLS:         tlsConfig,
		})
	}

	_ = newClient
}

func retryResignAndReconcile(elector *etcdleader.Elector, reconcile func(context.Context) error) error {
	var lastErr error
	for range 3 {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), min(5*time.Second, elector.EffectiveTTL()/4))
		lastErr = elector.Resign(cleanupCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
	}
	timer := time.NewTimer(elector.EffectiveTTL())
	<-timer.C // Schedule another proof attempt; do not infer deletion from this wait.
	proofCtx, cancelProof := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelProof()
	return errors.Join(lastErr, reconcile(proofCtx))
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
