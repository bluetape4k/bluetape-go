package etcdleader

import (
	"math"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestNewRejectsInvalidInputs(t *testing.T) {
	valid := testOptions()
	if _, err := New(nil, valid); err == nil {
		t.Fatal("New accepted a nil client")
	}

	client := &clientv3.Client{}
	invalid := []leader.Options{
		{},
		{Group: "group", MemberID: ""},
		{Group: "group", MemberID: "member", Lease: time.Second, RenewInterval: minimumProclaimInterval - time.Nanosecond},
		{Group: "group", MemberID: "member", Lease: time.Second, RenewInterval: time.Second},
		{Group: "group", MemberID: "member", Lease: time.Second, RenewInterval: 2 * time.Second},
	}
	for _, opts := range invalid {
		if _, err := New(client, opts); err == nil {
			t.Fatalf("New accepted invalid options: %+v", opts)
		}
	}
}

func TestNewAcceptsMinimumProclaimInterval(t *testing.T) {
	_, err := New(&clientv3.Client{}, leader.Options{
		Group:         "group",
		MemberID:      "member",
		Lease:         time.Second,
		RenewInterval: minimumProclaimInterval,
	})
	if err != nil {
		t.Fatalf("New rejected minimum Proclaim interval: %v", err)
	}
}

func TestNewCreatesUniqueOwnerTokensWithoutBackendIO(t *testing.T) {
	client := &clientv3.Client{}
	first, err := New(client, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(client, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	if first.client != client || second.client != client {
		t.Fatal("New did not retain the caller-owned client")
	}
	if first.token == second.token {
		t.Fatal("New reused an owner token")
	}
	if ok, _ := regexp.MatchString(`^member:[0-9a-f]{32}$`, first.token); !ok {
		t.Fatalf("token = %q", first.token)
	}
}

func TestEncodeElectionRange(t *testing.T) {
	got := electionPaths(leader.Options{KeyPrefix: "tenant/a", Group: "billing/b"})
	wantBase := "/bluetape4k/leader/dGVuYW50L2E/YmlsbGluZy9i"
	if got.base != wantBase || got.root != wantBase+"/" ||
		got.end != clientv3.GetPrefixRangeEnd(wantBase+"/") {
		t.Fatalf("paths = %+v", got)
	}
}

func TestEncodedElectionRangesIsolateSiblingGroups(t *testing.T) {
	left := electionPaths(leader.Options{KeyPrefix: "tenant/a", Group: "billing/b"})
	right := electionPaths(leader.Options{KeyPrefix: "tenant/a", Group: "billing/b/c"})
	leftKey := left.root + "candidate"
	rightKey := right.root + "candidate"
	contains := func(paths electionPath, key string) bool {
		return key >= paths.root && key < paths.end
	}
	if !contains(left, leftKey) || contains(left, rightKey) ||
		!contains(right, rightKey) || contains(right, leftKey) {
		t.Fatalf("ranges overlap: left=%+v right=%+v", left, right)
	}
}

func TestRequestedTTL(t *testing.T) {
	cases := []struct {
		lease time.Duration
		want  int64
	}{
		{time.Second, 1},
		{time.Second + time.Nanosecond, 2},
		{500 * time.Millisecond, 1},
	}
	for _, tc := range cases {
		got, err := requestedTTL(tc.lease)
		if err != nil || got != tc.want {
			t.Fatalf("ttl(%s) = %d, %v", tc.lease, got, err)
		}
	}
	for _, lease := range []time.Duration{0, -time.Second, time.Duration(math.MaxInt64)} {
		if _, err := requestedTTL(lease); err == nil {
			t.Fatalf("requestedTTL accepted %s", lease)
		}
	}
}

func TestEffectiveTTLTransitionsAndRejectsInvalidGrant(t *testing.T) {
	elector, err := New(&clientv3.Client{}, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got := elector.EffectiveTTL(); got != 2*time.Second {
		t.Fatalf("requested EffectiveTTL = %s", got)
	}

	first := &generation{}
	elector.mu.Lock()
	elector.current = first
	elector.mu.Unlock()
	if got := elector.EffectiveTTL(); got != 2*time.Second {
		t.Fatalf("unpublished EffectiveTTL = %s", got)
	}
	if err := elector.publishGenerationTTL(first, 3); err != nil {
		t.Fatal(err)
	}
	if got := elector.EffectiveTTL(); got != 3*time.Second {
		t.Fatalf("published EffectiveTTL = %s", got)
	}

	second := &generation{}
	elector.mu.Lock()
	elector.current = second
	elector.mu.Unlock()
	for _, ttl := range []int64{0, -1, math.MaxInt64} {
		if err := elector.publishGenerationTTL(second, ttl); err == nil {
			t.Fatalf("published invalid grant TTL %d", ttl)
		}
		if got := elector.EffectiveTTL(); got != 3*time.Second {
			t.Fatalf("invalid grant replaced last TTL: %s", got)
		}
	}
	if err := elector.publishGenerationTTL(second, 4); err != nil {
		t.Fatal(err)
	}
	elector.mu.Lock()
	elector.current = nil
	elector.mu.Unlock()
	if got := elector.EffectiveTTL(); got != 4*time.Second {
		t.Fatalf("last published EffectiveTTL = %s", got)
	}
}

func TestEffectiveTTLConcurrentReaders(t *testing.T) {
	elector, err := New(&clientv3.Client{}, testOptions())
	if err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 16 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					ttl := elector.EffectiveTTL()
					if ttl != 2*time.Second && ttl != 3*time.Second && ttl != 4*time.Second {
						t.Errorf("unexpected EffectiveTTL %s", ttl)
						return
					}
				}
			}
		}()
	}
	for _, seconds := range []int64{3, 4} {
		generation := &generation{}
		elector.mu.Lock()
		elector.current = generation
		elector.mu.Unlock()
		if err := elector.publishGenerationTTL(generation, seconds); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	readers.Wait()
}

func testOptions() leader.Options {
	return leader.Options{
		Group:         "group",
		MemberID:      "member",
		Lease:         time.Second + time.Nanosecond,
		RenewInterval: 500 * time.Millisecond,
		KeyPrefix:     "tenant/a",
	}
}
