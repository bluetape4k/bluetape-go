package etcdleader_test

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadmeParity(t *testing.T) {
	wantHeadings := []string{
		"## Preflight",
		"## Import And Client Ownership",
		"## Usage",
		"## Encoded Election Range",
		"## Lease And Ownership Signals",
		"## Failure Recovery",
		"## Shutdown And Reconciliation",
		"## RBAC And TLS",
		"## Quorum, Compaction, And Fencing",
		"## Migration And Rollback",
		"## Observability",
		"## Tested Scope",
		"## Test",
	}
	anchors := []string{
		"New", "EffectiveTTL", "ErrCommitUnknown", "ErrCleanupPending", "Session", "Proclaim",
		"InsecureSkipVerify", "ServerName", "username/password", "100", "compaction",
		"fencing", "joined raw", "aggregate Proclaim QPS", "live contenders",
		"leases/sessions/candidate keys", "exact-key watches", "KeepAliveOnce",
		"hostile-tenant isolation", "SessionOption", "restart-resume", "<MemberID>:<random>",
		"failed cleanup attempt", "coordinated hard-stop exception", "synchronous and non-blocking",
	}
	for _, file := range []string{"README.md", "README.ko.md"} {
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		if headings := markdownHeadings(text); !reflect.DeepEqual(headings, wantHeadings) {
			t.Fatalf("%s headings = %v, want %v", file, headings, wantHeadings)
		}
		for _, anchor := range anchors {
			if !strings.Contains(text, anchor) {
				t.Fatalf("%s is missing %q", file, anchor)
			}
		}
	}
}

func TestRollbackDocsVerifyZeroContendersBeforeRestore(t *testing.T) {
	readmes := map[string]string{
		"README.md":    "previous provider",
		"README.ko.md": "이전 provider",
	}
	for file, restoreAnchor := range readmes {
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		rollback := sectionBetween(
			t,
			file,
			string(contents),
			"## Migration And Rollback",
			"## Observability",
		)
		assertOrdered(t, file, rollback,
			"exact candidate range",
			"zero etcd contenders",
			restoreAnchor,
		)
	}

	contents, err := os.ReadFile("../../docs/release/v0.19.0-provider-conformance-runbook.md")
	if err != nil {
		t.Fatal(err)
	}
	for index, section := range strings.SplitN(string(contents), "## 한국어", 2) {
		rollbackStart := strings.Index(section, "symmetric rollback")
		if rollbackStart < 0 {
			t.Fatalf("runbook section %d is missing symmetric rollback", index)
		}
		restoreAnchor := "previous provider"
		if index == 1 {
			restoreAnchor = "이전 provider"
		}
		assertOrdered(t, "runbook", section[rollbackStart:],
			"exact range proof",
			"zero etcd contenders",
			restoreAnchor,
		)
	}
}

func sectionBetween(t *testing.T, file, text, start, end string) string {
	t.Helper()
	startIndex := strings.Index(text, start)
	if startIndex < 0 {
		t.Fatalf("%s is missing %q", file, start)
	}
	section := text[startIndex+len(start):]
	endIndex := strings.Index(section, end)
	if endIndex < 0 {
		t.Fatalf("%s is missing %q after %q", file, end, start)
	}
	return section[:endIndex]
}

func assertOrdered(t *testing.T, file, text string, anchors ...string) {
	t.Helper()
	text = strings.Join(strings.Fields(text), " ")
	for index, anchor := range anchors {
		next := strings.Index(text, anchor)
		if next < 0 {
			if index == 0 {
				t.Fatalf("%s is missing %q", file, anchor)
			}
			t.Fatalf("%s does not contain %q after %q", file, anchor, anchors[index-1])
		}
		text = text[next+len(anchor):]
	}
}

func TestExampleRequiresPerUnitLeadershipGuard(t *testing.T) {
	contents, err := os.ReadFile("example_test.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, anchor := range []string{
		"startProtectedWork func(context.Context, func() bool)",
		"startProtectedWork(protectedCtx, elector.IsLeader)",
	} {
		if !strings.Contains(text, anchor) {
			t.Fatalf("example_test.go is missing %q", anchor)
		}
	}
}

func TestHardStopCampaignsRequiresSuccessfulCoordination(t *testing.T) {
	want := errors.New("shared users are active")
	closed := false
	err := hardStopCampaigns(
		make(chan struct{}),
		func() error { return want },
		func() error { closed = true; return nil },
		10*time.Millisecond,
	)
	if !errors.Is(err, want) {
		t.Fatalf("hardStopCampaigns() error = %v, want %v", err, want)
	}
	if closed {
		t.Fatal("hardStopCampaigns() closed the client before shared-user coordination")
	}
}

func TestHardStopCampaignsBoundsPostCloseJoin(t *testing.T) {
	closed := false
	err := hardStopCampaigns(
		make(chan struct{}),
		func() error { return nil },
		func() error { closed = true; return nil },
		10*time.Millisecond,
	)
	if !closed {
		t.Fatal("hardStopCampaigns() did not close the coordinated client")
	}
	if err == nil || !strings.Contains(err.Error(), "campaigns did not join") {
		t.Fatalf("hardStopCampaigns() error = %v, want bounded join failure", err)
	}
}

func TestHardStopCampaignsRejectsNilCompletionBeforeClose(t *testing.T) {
	coordinated := false
	closed := false
	err := hardStopCampaigns(
		nil,
		func() error { coordinated = true; return nil },
		func() error { closed = true; return nil },
		10*time.Millisecond,
	)
	if err == nil || !strings.Contains(err.Error(), "completion channel is nil") {
		t.Fatalf("hardStopCampaigns() error = %v, want nil completion failure", err)
	}
	if coordinated || closed {
		t.Fatal("hardStopCampaigns() performed side effects for an invalid completion channel")
	}
}

func TestSuccessfulCleanupProofClearsUnresolvedState(t *testing.T) {
	priorFailure := errors.New("resign outcome unknown")
	scheduled := false
	err := finishCleanupAfterProof(
		priorFailure,
		nil,
		time.Second,
		func(time.Duration) error { scheduled = true; return nil },
		nil,
	)
	if err != nil {
		t.Fatalf("finishCleanupAfterProof() error = %v, want resolved cleanup", err)
	}
	if scheduled {
		t.Fatal("finishCleanupAfterProof() scheduled a recheck after exact cleanup proof")
	}
}

func TestShutdownFinalProofClearsTransientCleanupFailure(t *testing.T) {
	transient := errors.New("initial cleanup proof failed")
	persisted := false
	restored := false
	err := finishShutdownAfterProof(
		nil,
		transient,
		nil,
		func(error) error { persisted = true; return nil },
		func() error { restored = true; return nil },
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("finishShutdownAfterProof() error = %v, want successful rollback", err)
	}
	if persisted {
		t.Fatal("finishShutdownAfterProof() persisted cleanup after exact absence proof")
	}
	if !restored {
		t.Fatal("finishShutdownAfterProof() did not restore the previous provider")
	}
}

func TestShutdownFinalProofPersistsUnresolvedCleanup(t *testing.T) {
	cleanupErr := errors.New("cleanup outcome unknown")
	proofErr := errors.New("exact absence proof failed")
	var persisted error
	restored := false
	err := finishShutdownAfterProof(
		nil,
		cleanupErr,
		proofErr,
		func(err error) error { persisted = err; return nil },
		func() error { restored = true; return nil },
		func() error { return nil },
	)
	if !errors.Is(err, cleanupErr) || !errors.Is(err, proofErr) {
		t.Fatalf("finishShutdownAfterProof() error = %v, want cleanup and proof failures", err)
	}
	if !errors.Is(persisted, cleanupErr) || !errors.Is(persisted, proofErr) {
		t.Fatalf("persisted error = %v, want cleanup and proof failures", persisted)
	}
	if restored {
		t.Fatal("finishShutdownAfterProof() restored before exact absence proof")
	}
}

func TestShutdownZeroContenderFailureBlocksRestore(t *testing.T) {
	zeroErr := errors.New("etcd contenders remain")
	var persisted error
	restored := false
	err := finishShutdownAfterProof(
		nil,
		nil,
		nil,
		func(err error) error { persisted = err; return nil },
		func() error { restored = true; return nil },
		func() error { return zeroErr },
	)
	if !errors.Is(err, zeroErr) {
		t.Fatalf("finishShutdownAfterProof() error = %v, want %v", err, zeroErr)
	}
	if !errors.Is(persisted, zeroErr) {
		t.Fatalf("persisted error = %v, want %v", persisted, zeroErr)
	}
	if restored {
		t.Fatal("finishShutdownAfterProof() restored while etcd contenders remained")
	}
}

func TestRunbookContract(t *testing.T) {
	contents, err := os.ReadFile("../../docs/release/v0.19.0-provider-conformance-runbook.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(contents), "## 한국어", 2)
	if len(parts) != 2 {
		t.Fatal("runbook is missing the Korean section")
	}
	required := []string{
		"### etcd Leader Deployment Gates",
		`etcdctl --endpoints="$ETCD_ENDPOINTS" endpoint status --write-out=table`,
		`etcdctl --endpoints="$ETCD_ENDPOINTS" put "$ETCD_PREFLIGHT_KEY" ready`,
		`etcdctl --endpoints="$ETCD_ENDPOINTS" get "$ETCD_PREFLIGHT_KEY"`,
		`etcdctl --endpoints="$ETCD_ENDPOINTS" del "$ETCD_PREFLIGHT_KEY"`,
		`ETCD_EXACT_RANGE_COUNT="$(etcdctl --endpoints="$ETCD_DIAGNOSTIC_ENDPOINTS" get "$ETCD_CANDIDATE_ROOT" --prefix --count-only)"`,
		`test "$ETCD_EXACT_RANGE_COUNT" -eq 0`,
		"ETCD_MAX_PUBLISHED_GROUPS",
		"ETCD_MAX_LIVE_CONTENDERS",
		"ETCD_APPROVED_PROCLAIM_QPS",
		"etcd_server_proposals_pending",
		"etcd_server_proposals_failed_total",
		"etcd_disk_wal_fsync_duration_seconds",
		"etcd_disk_backend_commit_duration_seconds",
		"campaign drain",
		"exact range",
		"symmetric rollback",
		"quorum",
		"cross-principal keepalive denial",
		"sampling cadence",
		"git diff -- go.mod go.sum",
		"git rm -r leader/etcd",
		"README.md README.ko.md leader/README.md leader/README.ko.md leader/elector.go CHANGELOG.md",
		`git grep -n 'etcd provider' -- leader/elector.go`,
		`go list -m -f '{{.Version}}' golang.org/x/crypto`,
		`go list -m -f '{{.Version}}' golang.org/x/net`,
		`go list -m -f '{{.Version}}' golang.org/x/sys`,
		`go list -m -f '{{.Version}}' google.golang.org/protobuf`,
		`go list -m -f '{{.Version}}' go.opentelemetry.io/otel/sdk`,
		`go list -m -f '{{.Version}}' go.opentelemetry.io/otel/sdk/metric`,
		"TTL only schedules another proof attempt",
		"go mod tidy",
		"go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...",
		"security floor",
		"make ci",
	}
	for index, section := range parts {
		for _, item := range required {
			if !strings.Contains(section, item) {
				t.Fatalf("runbook section %d is missing %q", index, item)
			}
		}
		if strings.Contains(section, "get \"$ETCD_CANDIDATE_ROOT\" --prefix\n") {
			t.Fatalf("runbook section %d prints raw candidate keys and values", index)
		}
	}
}

func TestTLSExample(t *testing.T) {
	pool := x509.NewCertPool()
	valid := &tls.Config{RootCAs: pool, ServerName: "etcd.internal"}
	if err := validateProductionTLS(valid); err == nil {
		t.Fatal("empty root pool accepted")
	}
	pool.AddCert(&x509.Certificate{RawSubject: []byte("test-root")})
	if err := validateProductionTLS(&tls.Config{RootCAs: pool}); err == nil {
		t.Fatal("empty ServerName accepted")
	}
	if err := validateProductionTLS(&tls.Config{RootCAs: pool, ServerName: "etcd.internal", InsecureSkipVerify: true}); err == nil { //nolint:gosec // insecure input is the contract under test.
		t.Fatal("InsecureSkipVerify accepted")
	}
	if err := validateProductionTLS(&tls.Config{RootCAs: pool, ServerName: "etcd.internal"}); err != nil {
		t.Fatalf("valid TLS config rejected: %v", err)
	}
	if err := validateProductionCredentials("", "secret"); err == nil {
		t.Fatal("empty etcd username accepted")
	}
	if err := validateProductionCredentials("election", ""); err == nil {
		t.Fatal("empty etcd password accepted")
	}
	if err := validateProductionCredentials("election", "secret"); err != nil {
		t.Fatalf("valid etcd credentials rejected: %v", err)
	}
}

func markdownHeadings(contents string) []string {
	var headings []string
	for line := range strings.Lines(contents) {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(line))
		}
	}
	return headings
}
