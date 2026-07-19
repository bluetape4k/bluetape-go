package etcdleader_test

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"reflect"
	"strings"
	"testing"
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
		"fencing", "errors.Unwrap",
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
		"campaign drain",
		"exact range",
		"symmetric rollback",
		"quorum",
		"sampling cadence",
		"git diff -- go.mod go.sum",
		"git rm -r leader/etcd",
		"README.md README.ko.md leader/README.md leader/README.ko.md leader/elector.go CHANGELOG.md",
		`go list -m -f '{{.Version}}' golang.org/x/crypto`,
		`go list -m -f '{{.Version}}' golang.org/x/net`,
		`go list -m -f '{{.Version}}' golang.org/x/sys`,
		`go list -m -f '{{.Version}}' google.golang.org/protobuf`,
		`go list -m -f '{{.Version}}' go.opentelemetry.io/otel/sdk`,
		`go list -m -f '{{.Version}}' go.opentelemetry.io/otel/sdk/metric`,
		"TTL only schedules another proof attempt",
		"go mod tidy",
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
