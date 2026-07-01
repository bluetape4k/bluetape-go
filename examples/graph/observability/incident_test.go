package observability

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"testing"
)

func TestSeedIncidentGraphAnswersIncidentQuestions(t *testing.T) {
	incident, err := SeedIncidentGraph()
	if err != nil {
		t.Fatalf("SeedIncidentGraph() error = %v", err)
	}

	if got, want := len(incident.Vertices()), 10; got != want {
		t.Fatalf("len(Vertices()) = %d, want %d", got, want)
	}
	if got, want := len(incident.Edges()), 10; got != want {
		t.Fatalf("len(Edges()) = %d, want %d", got, want)
	}

	assertIDs(t, "DownstreamDependencies", incident.DownstreamDependencies("checkout-service", 2), []string{"payment-service", "postgres-primary"})
	assertIDs(t, "UpstreamImpactedServices", incident.UpstreamImpactedServices("payment-service", 3), []string{"checkout-service", "edge-api"})
	assertIDs(t, "AffectedAPIs", incident.AffectedAPIs("payment-service", 5), []string{"checkout-api", "mobile-checkout-api"})
	assertIDs(t, "AlertBoundary", incident.AlertBoundary([]string{"payment-latency", "checkout-errors"}, 2), []string{"checkout-service", "payment-service"})
	assertIDs(t, "OwningTeams", incident.OwningTeams("payment-service"), []string{"payments-team"})
}

func TestIncidentGraphRoundTripsThroughNDJSON(t *testing.T) {
	ctx := context.Background()
	incident, err := SeedIncidentGraph()
	if err != nil {
		t.Fatalf("SeedIncidentGraph() error = %v", err)
	}

	var output bytes.Buffer
	writeReport, err := incident.WriteNDJSON(ctx, &output)
	if err != nil {
		t.Fatalf("WriteNDJSON() error = %v", err)
	}
	if writeReport.VerticesWritten != 10 || writeReport.EdgesWritten != 10 {
		t.Fatalf("write report = %+v, want 10 vertices and 10 edges", writeReport)
	}

	read, readReport, err := ReadIncidentGraphNDJSON(ctx, bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("ReadIncidentGraphNDJSON() error = %v", err)
	}
	if readReport.VerticesRead != 10 || readReport.EdgesRead != 10 {
		t.Fatalf("read report = %+v, want 10 vertices and 10 edges", readReport)
	}

	assertIDs(t, "round-trip AffectedAPIs", read.AffectedAPIs("payment-service", 5), []string{"checkout-api", "mobile-checkout-api"})
	assertIDs(t, "round-trip OwningTeams", read.OwningTeams("payment-service"), []string{"payments-team"})
}

func ExampleSeedIncidentGraph() {
	incident, err := SeedIncidentGraph()
	if err != nil {
		panic(err)
	}

	fmt.Println(incident.AffectedAPIs("payment-service", 5))
	fmt.Println(incident.OwningTeams("payment-service"))

	// Output:
	// [checkout-api mobile-checkout-api]
	// [payments-team]
}

func assertIDs(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
