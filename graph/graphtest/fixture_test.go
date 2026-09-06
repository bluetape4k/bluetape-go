package graphtest

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestNewFixtureIsValidAndDefensivelyCopied(t *testing.T) {
	f, err := newFixture()
	if err != nil {
		t.Fatalf("newFixture() error = %v", err)
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !strings.HasPrefix(f.Namespace(), "btgc_") || len(f.Namespace()) != len("btgc_")+32 {
		t.Fatalf("namespace = %q", f.Namespace())
	}
	for _, vertex := range f.Vertices() {
		if got := vertex.Label().String(); got != "BTGraphConformance" {
			t.Fatalf("vertex label = %q", got)
		}
	}
	vertices := f.Vertices()
	vertices[0], _ = graph.ParseVertex("changed", "Changed", nil)
	if f.Vertices()[0].ID().String() == "changed" {
		t.Fatal("fixture leaked vertex mutation")
	}
}

func TestCanonicalVerticesRejectsLimitBeforeSorting(t *testing.T) {
	f, _ := newFixture()
	got := append(f.Vertices(), f.Vertices()[0])
	if _, err := canonicalVertices(got, 2); err == nil {
		t.Fatal("canonicalVertices() error = nil")
	}
}

func TestCanonicalEdgesRejectsLimitBeforeSorting(t *testing.T) {
	f, err := newFixture()
	if err != nil {
		t.Fatalf("newFixture() error = %v", err)
	}
	values := append(f.Edges(), f.Edges()[0])
	if _, err := canonicalEdges(values, 1); err == nil {
		t.Fatal("canonicalEdges() error = nil")
	}
}
