package neo4j_test

import (
	"errors"
	"testing"

	neo4jadapter "github.com/bluetape4k/bluetape-go/graph/neo4j"
)

func TestClientRejectsInvalidInputs(t *testing.T) {
	if _, err := neo4jadapter.NewClient(nil); !errors.Is(err, neo4jadapter.ErrInvalidOptions) {
		t.Fatalf("NewClient(nil) error = %v, want ErrInvalidOptions", err)
	}
}
