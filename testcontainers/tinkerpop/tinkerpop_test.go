package tinkerpoptestcontainer

import (
	"context"
	"strings"
	"testing"
)

func TestStartTinkerPop(t *testing.T) {
	endpoint := Start(context.Background(), t)
	if !strings.HasPrefix(endpoint, "ws://") || !strings.HasSuffix(endpoint, "/gremlin") {
		t.Fatalf("endpoint=%q", endpoint)
	}
}
