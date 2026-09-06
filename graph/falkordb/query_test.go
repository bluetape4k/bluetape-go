package falkordb_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph/falkordb"
)

func TestQueryRejectsUnsafeGraphNamesQueriesAndParameters(t *testing.T) {
	var fake fakeRedis
	for _, graphName := range []string{"", "graph name", "graph;drop"} {
		if _, err := falkordb.NewClient(&fake, graphName); !errors.Is(err, falkordb.ErrInvalidOptions) {
			t.Fatalf("graph=%q err=%v", graphName, err)
		}
	}
	client, _ := falkordb.NewClient(&fake, "graph")
	for _, params := range []map[string]any{{"bad-key": 1}, {"unsupported": struct{}{}}} {
		if _, err := client.Query(t.Context(), "RETURN 1", params); !errors.Is(err, falkordb.ErrInvalidQuery) {
			t.Fatalf("params=%#v err=%v", params, err)
		}
	}
}
