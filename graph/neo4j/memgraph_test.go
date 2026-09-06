package neo4j_test

import (
	"context"
	"errors"
	"time"

	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

const memgraphBoltPort = "7687/tcp"

func waitForMemgraphConnectivity(ctx context.Context, driver neo4jdriver.Driver) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := driver.VerifyConnectivity(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}
