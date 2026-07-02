package iamaccess

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"testing"
)

func TestSeedIAMAccessGraphAnswersAccessQuestions(t *testing.T) {
	access, err := SeedIAMAccessGraph()
	if err != nil {
		t.Fatalf("SeedIAMAccessGraph() error = %v", err)
	}

	if got, want := len(access.Vertices()), 22; got != want {
		t.Fatalf("len(Vertices()) = %d, want %d", got, want)
	}
	if got, want := len(access.Edges()), 20; got != want {
		t.Fatalf("len(Edges()) = %d, want %d", got, want)
	}

	direct := access.ExplainAccess("bob", "audit-dashboard", "read")
	if !direct.Allowed {
		t.Fatalf("bob audit-dashboard read should be allowed: %+v", direct)
	}
	assertContains(t, "direct path", direct.Path, "user:bob", "role:readonly-role", "resource:audit-dashboard")

	inherited := access.ExplainAccess("alice", "staging-service", "deploy")
	if !inherited.Allowed {
		t.Fatalf("alice staging-service deploy should be allowed: %+v", inherited)
	}
	assertContains(t, "inherited path", inherited.Path, "group:engineering", "role:deployer-role", "resource:staging-service")
}

func TestIAMAccessGraphExplainsDeniedAndAbsentAccess(t *testing.T) {
	access, err := SeedIAMAccessGraph()
	if err != nil {
		t.Fatalf("SeedIAMAccessGraph() error = %v", err)
	}

	denied := access.ExplainAccess("eve", "prod-db", "delete")
	if denied.Allowed {
		t.Fatalf("eve prod-db delete should be denied: %+v", denied)
	}
	if got, want := denied.Reason, "Denied by explicit policy path"; got != want {
		t.Fatalf("denied reason = %q, want %q", got, want)
	}
	assertContains(t, "denied path", denied.Path, "policy:deny-prod-delete-policy")

	absent := access.ExplainAccess("bob", "prod-db", "delete")
	if absent.Allowed {
		t.Fatalf("bob prod-db delete should not be allowed: %+v", absent)
	}
	if got, want := absent.Reason, "No matching grant path"; got != want {
		t.Fatalf("absent reason = %q, want %q", got, want)
	}
	if len(absent.Path) != 0 {
		t.Fatalf("absent path = %v, want empty", absent.Path)
	}
}

func TestIAMAccessGraphDetectsRiskAndLeastPrivilegeDrift(t *testing.T) {
	access, err := SeedIAMAccessGraph()
	if err != nil {
		t.Fatalf("SeedIAMAccessGraph() error = %v", err)
	}

	chains := access.RiskyPrivilegeChains("alice")
	if got, want := len(chains), 1; got != want {
		t.Fatalf("len(RiskyPrivilegeChains) = %d, want %d: %+v", got, want, chains)
	}
	if got, want := chains[0].RoleID, "prod-admin-role"; got != want {
		t.Fatalf("chain role ID = %q, want %q", got, want)
	}
	assertContains(t, "risky path", chains[0].Path, "group:engineering", "group:platform-admins")

	findings := access.ExcessivePermissions("alice", map[string][]string{
		"staging-service": {"deploy"},
		"prod-db":         {"read"},
	})
	if got, want := len(findings), 1; got != want {
		t.Fatalf("len(ExcessivePermissions) = %d, want %d: %+v", got, want, findings)
	}
	if got, want := findings[0].ResourceID, "prod-db"; got != want {
		t.Fatalf("finding resource = %q, want %q", got, want)
	}
	if got, want := findings[0].Action, "delete"; got != want {
		t.Fatalf("finding action = %q, want %q", got, want)
	}
}

func TestIAMAccessGraphExplainsTemporaryBreakGlassGrant(t *testing.T) {
	access, err := SeedIAMAccessGraph()
	if err != nil {
		t.Fatalf("SeedIAMAccessGraph() error = %v", err)
	}

	explanation := access.ExplainAccess("carol", "prod-db", "read")
	if !explanation.Allowed {
		t.Fatalf("carol prod-db read should be allowed: %+v", explanation)
	}
	assertContains(t, "temporary path", explanation.Path, "grant:break-glass-1001", "permission:read")
}

func TestIAMAccessGraphRoundTripsThroughNDJSON(t *testing.T) {
	ctx := context.Background()
	access, err := SeedIAMAccessGraph()
	if err != nil {
		t.Fatalf("SeedIAMAccessGraph() error = %v", err)
	}

	var output bytes.Buffer
	writeReport, err := access.WriteNDJSON(ctx, &output)
	if err != nil {
		t.Fatalf("WriteNDJSON() error = %v", err)
	}
	if writeReport.VerticesWritten != 22 || writeReport.EdgesWritten != 20 {
		t.Fatalf("write report = %+v, want 22 vertices and 20 edges", writeReport)
	}

	read, readReport, err := ReadIAMAccessGraphNDJSON(ctx, bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("ReadIAMAccessGraphNDJSON() error = %v", err)
	}
	if readReport.VerticesRead != 22 || readReport.EdgesRead != 20 {
		t.Fatalf("read report = %+v, want 22 vertices and 20 edges", readReport)
	}

	assertContains(t, "round-trip access", read.ExplainAccess("alice", "staging-service", "deploy").Path, "group:engineering", "role:deployer-role")
}

func ExampleSeedIAMAccessGraph() {
	access, err := SeedIAMAccessGraph()
	if err != nil {
		panic(err)
	}

	fmt.Println(access.ExplainAccess("alice", "staging-service", "deploy").Allowed)
	fmt.Println(access.RiskyPrivilegeChains("alice")[0].RoleID)
	fmt.Println(access.ExplainAccess("eve", "prod-db", "delete").Reason)

	// Output:
	// true
	// prod-admin-role
	// Denied by explicit policy path
}

func assertContains(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	for _, value := range want {
		if !slices.Contains(got, value) {
			t.Fatalf("%s = %v, want to contain %q", name, got, value)
		}
	}
}
