package neo4j_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphtest"
	neo4jadapter "github.com/bluetape4k/bluetape-go/graph/neo4j"
	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/testcontainers/testcontainers-go"
	tcneo4j "github.com/testcontainers/testcontainers-go/modules/neo4j"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	neo4jConformanceImage      = "neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5"
	memgraphConformanceImage   = "memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1"
	neo4jConformanceVersion    = "5.26.0"
	memgraphConformanceVersion = "3.5.0"
	memgraphConformancePort    = "7687/tcp"
	vertexColumn               = "n"
	edgeColumn                 = "r"
	traversalColumn            = "keys"
	cancellationColumn         = "total"
	cancellationIterations     = 1_000_000
	createFixtureQuery         = `UNWIND $vertices AS v CREATE (n:BTGraphConformance) SET n = v WITH count(n) AS ignored UNWIND $edges AS e MATCH (a:BTGraphConformance {namespace: e.namespace, btgc_key: e.start}), (b:BTGraphConformance {namespace: e.namespace, btgc_key: e.end}) CREATE (a)-[r:BTGC_LINKS]->(b) SET r = e.props`
	readVerticesQuery          = `MATCH (n:BTGraphConformance {namespace: $namespace}) RETURN n LIMIT $limit`
	readEdgesQuery             = `MATCH (:BTGraphConformance {namespace: $namespace})-[r:BTGC_LINKS]->(:BTGraphConformance {namespace: $namespace}) RETURN r LIMIT $limit`
	traverseQuery              = `MATCH p=(a:BTGraphConformance {namespace: $namespace, btgc_key: $start})-[:BTGC_LINKS]->(b:BTGraphConformance {namespace: $namespace}) RETURN [n IN nodes(p) | n.btgc_key] AS keys LIMIT $limit`
	cancellationQuery          = `UNWIND range(1, $iterations) AS i WITH sum(i) AS total RETURN total`
	cleanupQuery               = `MATCH (n:BTGraphConformance {namespace: $namespace}) DETACH DELETE n`
	invalidQuery               = `RETURN $missing +`
)

var errProviderCallbackPanic = errors.New("graph/neo4j: provider callback panic")

type sanitizedProviderError struct {
	phase string
	cause error
}

func (e *sanitizedProviderError) Error() string {
	return "graph/neo4j: " + e.phase + " failed"
}

func (e *sanitizedProviderError) Unwrap() error {
	return e.cause
}

func sanitizeProviderError(phase string, err error) error {
	if err == nil {
		return nil
	}
	return &sanitizedProviderError{phase: phase, cause: err}
}

type redactingTerminator struct {
	container testcleanup.Terminator
}

func (r redactingTerminator) Terminate(
	ctx context.Context,
	options ...testcontainers.TerminateOption,
) (returnErr error) {
	defer func() {
		if recover() != nil {
			returnErr = errProviderCallbackPanic
		}
	}()
	return sanitizeProviderError("container termination", r.container.Terminate(ctx, options...))
}

type conformanceRequest struct {
	query              string
	params             map[string]any
	column             string
	logicalSubmissions int
}

func createRequest(fixture graphtest.Fixture) conformanceRequest {
	vertices := make([]map[string]any, 0, len(fixture.Vertices()))
	for _, vertex := range fixture.Vertices() {
		vertices = append(vertices, map[string]any(vertex.Properties()))
	}
	edges := make([]map[string]any, 0, len(fixture.Edges()))
	for _, edge := range fixture.Edges() {
		edges = append(edges, map[string]any{
			"namespace": fixture.Namespace(),
			"start":     edge.StartID().String(),
			"end":       edge.EndID().String(),
			"props":     map[string]any(edge.Properties()),
		})
	}
	return conformanceRequest{
		query: createFixtureQuery,
		params: map[string]any{
			"vertices": vertices,
			"edges":    edges,
		},
		logicalSubmissions: 1,
	}
}

func verticesRequest(namespace string, config graphtest.Config) conformanceRequest {
	return conformanceRequest{
		query: readVerticesQuery,
		params: map[string]any{
			"namespace": namespace,
			"limit":     config.MaxVertices + 1,
		},
		column:             vertexColumn,
		logicalSubmissions: 1,
	}
}

func edgesRequest(namespace string, config graphtest.Config) conformanceRequest {
	return conformanceRequest{
		query: readEdgesQuery,
		params: map[string]any{
			"namespace": namespace,
			"limit":     config.MaxEdges + 1,
		},
		column:             edgeColumn,
		logicalSubmissions: 1,
	}
}

func traversalRequest(namespace string, config graphtest.Config) conformanceRequest {
	return conformanceRequest{
		query: traverseQuery,
		params: map[string]any{
			"namespace": namespace,
			"start":     "left",
			"limit":     config.MaxTraversalResults + 1,
		},
		column:             traversalColumn,
		logicalSubmissions: 1,
	}
}

func cleanupRequest(namespace string) conformanceRequest {
	return conformanceRequest{
		query:              cleanupQuery,
		params:             map[string]any{"namespace": namespace},
		logicalSubmissions: 1,
	}
}

func executeRequest[T any](
	ctx context.Context,
	request conformanceRequest,
	submit func(conformanceRequest) (T, error),
) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return submit(request)
}

type providerStarter func(
	context.Context,
	testing.TB,
	string,
	string,
) (neo4jdriver.Driver, error)

func neo4jConformanceHarness() graphtest.Harness {
	return conformanceHarness(
		"neo4j",
		neo4jConformanceVersion,
		neo4jConformanceImage,
		startNeo4jConformanceDriver,
	)
}

func memgraphConformanceHarness() graphtest.Harness {
	return conformanceHarness(
		"memgraph",
		memgraphConformanceVersion,
		memgraphConformanceImage,
		startMemgraphConformanceDriver,
	)
}

func conformanceHarness(
	name string,
	version string,
	image string,
	start providerStarter,
) graphtest.Harness {
	return graphtest.Harness{
		Provider: graphtest.ProviderMetadata{
			Name:           name,
			Version:        version,
			ImageReference: image,
		},
		New: newNeo4jAdapterFactory(name, image, version, start),
		Capabilities: graphtest.Capabilities{
			graphtest.CapabilityTraversal: {Enabled: true},
		},
	}
}

func newNeo4jAdapterFactory(
	provider string,
	image string,
	version string,
	start providerStarter,
) graphtest.Factory {
	return func(
		startupCtx context.Context,
		tb testing.TB,
		config graphtest.Config,
	) (adapter graphtest.Adapter, returnErr error) {
		expectedImage := expectedConformanceImage(provider)
		if image != expectedImage {
			return graphtest.Adapter{}, sanitizeProviderError("image validation", errors.New("unexpected image reference"))
		}
		var driver neo4jdriver.Driver
		completed := false
		defer func() {
			panicValue := recover()
			if !completed && driver != nil {
				closeCtx, cancel := context.WithTimeout(context.WithoutCancel(startupCtx), config.CloseTimeout)
				closeErr := driver.Close(closeCtx)
				cancel()
				returnErr = errors.Join(returnErr, sanitizeProviderError("partial driver close", closeErr))
			}
			if panicValue != nil {
				panic(errProviderCallbackPanic)
			}
		}()
		driver, returnErr = start(startupCtx, tb, image, version)
		if returnErr != nil {
			returnErr = closePartial(startupCtx, config.CloseTimeout, driver, returnErr)
			driver = nil
			return graphtest.Adapter{}, returnErr
		}
		client, err := neo4jadapter.NewClient(driver)
		if err != nil {
			returnErr = closePartial(startupCtx, config.CloseTimeout, driver, err)
			driver = nil
			return graphtest.Adapter{}, returnErr
		}
		adapter = newConformanceAdapter(driver, client, config)
		completed = true
		return adapter, nil
	}
}

func expectedConformanceImage(provider string) string {
	switch provider {
	case "neo4j":
		return neo4jConformanceImage
	case "memgraph":
		return memgraphConformanceImage
	default:
		return ""
	}
}

func closePartial(
	startupCtx context.Context,
	timeout time.Duration,
	driver neo4jdriver.Driver,
	original error,
) error {
	startupErr := sanitizeProviderError("startup", original)
	if driver == nil {
		return startupErr
	}
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(startupCtx), timeout)
	defer cancel()
	return errors.Join(
		startupErr,
		sanitizeProviderError("partial driver close", driver.Close(closeCtx)),
	)
}

func startNeo4jConformanceDriver(
	ctx context.Context,
	tb testing.TB,
	image string,
	version string,
) (neo4jdriver.Driver, error) {
	if image != neo4jConformanceImage {
		return nil, errors.New("graph/neo4j: unexpected neo4j image")
	}
	container, err := tcneo4j.Run(ctx, image)
	if container != nil {
		testcleanup.Register(ctx, tb, "neo4j", redactingTerminator{container: container})
	}
	if err != nil {
		return nil, err
	}
	endpoint, err := container.BoltUrl(ctx)
	if err != nil {
		return nil, err
	}
	driver, err := neo4jdriver.NewDriver(endpoint, neo4jdriver.NoAuth())
	if err != nil {
		return nil, err
	}
	if err := verifyProvider(ctx, driver, version, false); err != nil {
		return driver, err
	}
	return driver, nil
}

func startMemgraphConformanceDriver(
	ctx context.Context,
	tb testing.TB,
	image string,
	version string,
) (neo4jdriver.Driver, error) {
	if image != memgraphConformanceImage {
		return nil, errors.New("graph/neo4j: unexpected memgraph image")
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{memgraphConformancePort},
			WaitingFor: wait.ForListeningPort(memgraphConformancePort).
				WithStartupTimeout(graphtest.DefaultStartupTimeout),
		},
		Started: true,
	})
	if container != nil {
		testcleanup.Register(ctx, tb, "memgraph", redactingTerminator{container: container})
	}
	if err != nil {
		return nil, err
	}
	endpoint, err := container.PortEndpoint(ctx, memgraphConformancePort, "bolt")
	if err != nil {
		return nil, err
	}
	driver, err := neo4jdriver.NewDriver(endpoint, neo4jdriver.NoAuth())
	if err != nil {
		return nil, err
	}
	if err := verifyProvider(ctx, driver, version, true); err != nil {
		return driver, err
	}
	return driver, nil
}

func verifyProvider(
	ctx context.Context,
	driver neo4jdriver.Driver,
	expectedVersion string,
	memgraph bool,
) error {
	if err := waitForProviderConnectivity(ctx, driver); err != nil {
		return err
	}
	reported, err := providerVersion(ctx, driver, memgraph)
	if err != nil {
		return err
	}
	if !providerVersionMatches(reported, expectedVersion) {
		return errors.New("graph/neo4j: provider version mismatch")
	}
	return nil
}

func waitForProviderConnectivity(ctx context.Context, driver neo4jdriver.Driver) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		lastErr = driver.VerifyConnectivity(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func providerVersion(
	ctx context.Context,
	driver neo4jdriver.Driver,
	memgraph bool,
) (string, error) {
	if !memgraph {
		info, err := driver.GetServerInfo(ctx)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(info.Agent()), nil
	}
	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead})
	result, err := session.Run(ctx, "SHOW VERSION", nil)
	if err != nil {
		return "", errors.Join(err, closeProviderSession(ctx, session))
	}
	records, collectErr := result.Collect(ctx)
	closeErr := closeProviderSession(ctx, session)
	if err := errors.Join(collectErr, closeErr); err != nil {
		return "", err
	}
	if len(records) != 1 || len(records[0].Values) != 1 {
		return "", errors.New("graph/neo4j: unexpected version result shape")
	}
	version, ok := records[0].Values[0].(string)
	if !ok {
		return "", errors.New("graph/neo4j: unexpected version result type")
	}
	return strings.TrimSpace(version), nil
}

func providerVersionMatches(reported string, expected string) bool {
	reported = strings.TrimSpace(reported)
	expected = strings.TrimSpace(expected)
	if reported == "" || expected == "" {
		return false
	}
	pattern := regexp.MustCompile(`(?:^|[^0-9])` + regexp.QuoteMeta(expected) + `(?:$|[^0-9])`)
	return pattern.MatchString(reported)
}

func newConformanceAdapter(
	driver neo4jdriver.Driver,
	client *neo4jadapter.Client,
	config graphtest.Config,
) graphtest.Adapter {
	return graphtest.Adapter{
		VerifyConnectivity: func(ctx context.Context) error {
			return sanitizeProviderError("verify connectivity", client.VerifyConnectivity(ctx))
		},
		CreateFixture: func(ctx context.Context, fixture graphtest.Fixture) error {
			request := createRequest(fixture)
			_, err := executeRequest(ctx, request, func(request conformanceRequest) (struct{}, error) {
				return struct{}{}, client.ExecuteWrite(ctx, request.query, request.params)
			})
			return sanitizeProviderError("create fixture", err)
		},
		ReadVertices: func(ctx context.Context, fixture graphtest.Fixture) ([]graph.Vertex, error) {
			request := verticesRequest(fixture.Namespace(), config)
			vertices, err := executeRequest(ctx, request, func(request conformanceRequest) ([]graph.Vertex, error) {
				return client.ReadVertices(ctx, request.query, request.params, request.column)
			})
			return vertices, sanitizeProviderError("read vertices", err)
		},
		ReadEdges: func(ctx context.Context, fixture graphtest.Fixture) ([]graph.Edge, error) {
			request := edgesRequest(fixture.Namespace(), config)
			edges, err := executeRequest(ctx, request, func(request conformanceRequest) ([]graph.Edge, error) {
				return client.ReadEdges(ctx, request.query, request.params, request.column)
			})
			return edges, sanitizeProviderError("read edges", err)
		},
		InvalidOperation: func(ctx context.Context, _ graphtest.Fixture) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return sanitizeProviderError("invalid operation", client.ExecuteWrite(ctx, invalidQuery, nil))
		},
		BlockUntilCanceled: func(
			ctx context.Context,
			_ graphtest.Fixture,
			started graphtest.Started,
		) (returnErr error) {
			session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead})
			defer func() {
				returnErr = errors.Join(returnErr, sanitizeProviderError("session close", closeProviderSession(ctx, session)))
			}()
			_, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
				result, err := tx.Run(ctx, cancellationQuery, map[string]any{
					"iterations": cancellationIterations,
				})
				if err != nil {
					return nil, err
				}
				started()
				_, err = result.Consume(ctx)
				return nil, err
			})
			if err != nil {
				returnErr = sanitizeProviderError("blocking operation", err)
			}
			return returnErr
		},
		CleanupFixture: func(ctx context.Context, fixture graphtest.Fixture) error {
			request := cleanupRequest(fixture.Namespace())
			_, err := executeRequest(ctx, request, func(request conformanceRequest) (struct{}, error) {
				return struct{}{}, client.ExecuteWrite(ctx, request.query, request.params)
			})
			return sanitizeProviderError("cleanup fixture", err)
		},
		Close: func(ctx context.Context) error {
			return sanitizeProviderError("close", client.Close(ctx))
		},
		Traverse: func(ctx context.Context, fixture graphtest.Fixture) ([]string, error) {
			request := traversalRequest(fixture.Namespace(), config)
			keys, err := executeRequest(ctx, request, func(request conformanceRequest) ([]string, error) {
				return collectTraversalKeys(ctx, driver, request, config.CloseTimeout)
			})
			return keys, sanitizeProviderError("traversal", err)
		},
		IsProviderError: func(err error) bool {
			return errors.Is(err, neo4jadapter.ErrDriver)
		},
	}
}

func collectTraversalKeys(
	ctx context.Context,
	driver neo4jdriver.Driver,
	request conformanceRequest,
	closeTimeout time.Duration,
) (_ []string, returnErr error) {
	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead})
	defer func() {
		returnErr = errors.Join(returnErr, closeProviderSessionWithTimeout(ctx, session, closeTimeout))
	}()
	value, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, request.query, request.params)
		if err != nil {
			return nil, err
		}
		records, err := result.Collect(ctx)
		if err != nil {
			return nil, err
		}
		if len(records) != 1 {
			return nil, errors.New("graph/neo4j: unexpected traversal row count")
		}
		raw, ok := records[0].Get(request.column)
		if !ok {
			return nil, errors.New("graph/neo4j: traversal column missing")
		}
		values, ok := raw.([]any)
		if !ok {
			if stringsValue, stringsOK := raw.([]string); stringsOK {
				return stringsValue, nil
			}
			return nil, errors.New("graph/neo4j: unexpected traversal result type")
		}
		keys := make([]string, 0, len(values))
		for _, value := range values {
			key, ok := value.(string)
			if !ok {
				return nil, errors.New("graph/neo4j: unexpected traversal key type")
			}
			keys = append(keys, key)
		}
		return keys, nil
	})
	if err != nil {
		return nil, err
	}
	keys, ok := value.([]string)
	if !ok {
		return nil, errors.New("graph/neo4j: unexpected traversal transaction result")
	}
	return keys, nil
}

func closeProviderSession(parent context.Context, session neo4jdriver.Session) error {
	return closeProviderSessionWithTimeout(parent, session, graphtest.DefaultCloseTimeout)
}

func closeProviderSessionWithTimeout(
	parent context.Context,
	session neo4jdriver.Session,
	timeout time.Duration,
) error {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()
	return session.Close(closeCtx)
}

func TestConformanceReadRequestBuilders(t *testing.T) {
	for _, limit := range []int{1, graphtest.DefaultConfig().MaxVertices, graphtest.MaxResultLimit} {
		config := graphtest.DefaultConfig()
		config.MaxVertices = limit
		config.MaxEdges = limit
		vertexRequest := verticesRequest("btgc_test", config)
		if vertexRequest.query != readVerticesQuery || vertexRequest.column != vertexColumn ||
			vertexRequest.params["namespace"] != "btgc_test" ||
			vertexRequest.params["limit"] != limit+1 || vertexRequest.logicalSubmissions != 1 {
			t.Fatalf("invalid vertex request for limit %d", limit)
		}
		edgeRequest := edgesRequest("btgc_test", config)
		if edgeRequest.query != readEdgesQuery || edgeRequest.column != edgeColumn ||
			edgeRequest.params["namespace"] != "btgc_test" ||
			edgeRequest.params["limit"] != limit+1 || edgeRequest.logicalSubmissions != 1 {
			t.Fatalf("invalid edge request for limit %d", limit)
		}
	}
}

func TestConformanceTraversalRequestBuilder(t *testing.T) {
	for _, limit := range []int{1, graphtest.DefaultConfig().MaxTraversalResults, graphtest.MaxResultLimit} {
		config := graphtest.DefaultConfig()
		config.MaxTraversalResults = limit
		request := traversalRequest("btgc_test", config)
		if request.query != traverseQuery || request.column != traversalColumn ||
			request.params["namespace"] != "btgc_test" || request.params["start"] != "left" ||
			request.params["limit"] != limit+1 || request.logicalSubmissions != 1 {
			t.Fatalf("invalid traversal request for limit %d", limit)
		}
	}
}

func TestConformancePreCanceledRequestSubmitsNoQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	submissions := 0
	_, err := executeRequest(ctx, verticesRequest("btgc_test", graphtest.DefaultConfig()), func(conformanceRequest) ([]graph.Vertex, error) {
		submissions++
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) || submissions != 0 {
		t.Fatalf("pre-canceled request submissions = %d", submissions)
	}
}

func TestConformanceCancellationRequestIsBoundedAndParameterized(t *testing.T) {
	if cancellationIterations > 1_000_000 || cancellationColumn != "total" {
		t.Fatal("cancellation request exceeded its fixed bound")
	}
	if !strings.Contains(cancellationQuery, "$iterations") || strings.Contains(cancellationQuery, "1000000") {
		t.Fatal("cancellation query does not use a bound parameter")
	}
}

func TestConformanceImageValidationRunsBeforeStarter(t *testing.T) {
	called := false
	factory := newNeo4jAdapterFactory(
		"neo4j",
		"neo4j:5.26.0",
		neo4jConformanceVersion,
		func(context.Context, testing.TB, string, string) (neo4jdriver.Driver, error) {
			called = true
			return nil, nil
		},
	)
	_, err := factory(context.Background(), t, graphtest.DefaultConfig())
	if err == nil || called {
		t.Fatal("mutable image reached the provider starter")
	}
}

func TestSanitizedProviderErrorPreservesChainWithoutRenderingCause(t *testing.T) {
	cause := errors.New("bolt://user:credential@secret-marker MATCH (n)")
	err := sanitizeProviderError("startup", cause)
	if !errors.Is(err, cause) {
		t.Fatal("sanitized error did not preserve the cause chain")
	}
	if strings.Contains(fmt.Sprint(err), "secret-marker") || strings.Contains(fmt.Sprint(err), "MATCH") {
		t.Fatal("sanitized error rendered its raw cause")
	}
}

type conformanceFakeTerminator struct {
	err        error
	panics     bool
	terminated bool
}

func (f *conformanceFakeTerminator) Terminate(
	context.Context,
	...testcontainers.TerminateOption,
) error {
	f.terminated = true
	if f.panics {
		panic("terminator-secret")
	}
	return f.err
}

func TestRedactingTerminatorPreservesErrorAndRecoversPanic(t *testing.T) {
	cause := errors.New("termination-secret-marker")
	fake := &conformanceFakeTerminator{err: cause}
	err := redactingTerminator{container: fake}.Terminate(context.Background())
	if !fake.terminated || !errors.Is(err, cause) || strings.Contains(fmt.Sprint(err), "secret-marker") {
		t.Fatal("terminator error contract mismatch")
	}
	fake = &conformanceFakeTerminator{panics: true}
	err = redactingTerminator{container: fake}.Terminate(context.Background())
	if !errors.Is(err, errProviderCallbackPanic) || strings.Contains(fmt.Sprint(err), "terminator-secret") {
		t.Fatal("terminator panic was not safely normalized")
	}
}

func TestProviderVersionMatchesAuthority(t *testing.T) {
	for _, testCase := range []struct {
		reported string
		expected string
		want     bool
	}{
		{"Neo4j/5.26.0", "5.26.0", true},
		{"Memgraph 3.5.0", "3.5.0", true},
		{"Neo4j/5.26.1", "5.26.0", false},
		{"", "5.26.0", false},
	} {
		if got := providerVersionMatches(testCase.reported, testCase.expected); got != testCase.want {
			t.Fatalf("providerVersionMatches(%q, %q) = %v", testCase.reported, testCase.expected, got)
		}
	}
}

func TestBackendConformance(t *testing.T) {
	started := time.Now()
	for _, testCase := range []struct {
		name    string
		harness graphtest.Harness
	}{
		{"neo4j", neo4jConformanceHarness()},
		{"memgraph", memgraphConformanceHarness()},
	} {
		if !t.Run(testCase.name, func(t *testing.T) {
			graphtest.Run(t, testCase.harness)
		}) {
			return
		}
	}
	if elapsed := time.Since(started); elapsed > 10*time.Minute {
		t.Fatalf("backend conformance elapsed=%s, want <=10m", elapsed)
	}
}
