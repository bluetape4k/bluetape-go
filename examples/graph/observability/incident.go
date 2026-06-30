package observability

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

const (
	labelService  = "Service"
	labelAPI      = "Api"
	labelTeam     = "Team"
	labelAlert    = "Alert"
	labelIncident = "Incident"

	edgeDependsOn = "DEPENDS_ON"
	edgeOwnedBy   = "OWNED_BY"
	edgeAlertsOn  = "ALERTS_ON"
	edgeRootCause = "ROOT_CAUSE"
)

// IncidentGraph is a small immutable incident-response graph fixture.
type IncidentGraph struct {
	vertices []graph.Vertex
	edges    []graph.Edge

	verticesByID map[string]graph.Vertex
	serviceIDs   map[string]string
	apiIDs       map[string]string
	teamIDs      map[string]string
	alertIDs     map[string]string

	outgoing map[string][]graph.Edge
	incoming map[string][]graph.Edge
}

type rawVertex struct {
	id         string
	label      string
	properties graph.Properties
}

type rawEdge struct {
	id         string
	label      string
	start      string
	end        string
	properties graph.Properties
}

// SeedIncidentGraph returns the bundled checkout payment incident graph.
func SeedIncidentGraph() (IncidentGraph, error) {
	return newIncidentGraph(seedVertices(), seedEdges())
}

// ReadIncidentGraphNDJSON imports an incident graph from graphio NDJSON records.
func ReadIncidentGraphNDJSON(ctx context.Context, reader io.Reader) (IncidentGraph, graphio.Report, error) {
	records, report, err := graphio.ReadNDJSON(ctx, reader, graphio.ReadOptions{})
	if err != nil {
		return IncidentGraph{}, report, err
	}

	vertices := make([]graph.Vertex, 0, len(records))
	edges := make([]graph.Edge, 0, len(records))
	for _, record := range records {
		switch record.Kind {
		case graphio.RecordVertex:
			vertices = append(vertices, record.Vertex)
		case graphio.RecordEdge:
			edges = append(edges, record.Edge)
		default:
			return IncidentGraph{}, report, fmt.Errorf("unsupported graph record kind %q", record.Kind)
		}
	}

	incident, err := assembleIncidentGraph(vertices, edges)
	if err != nil {
		return IncidentGraph{}, report, err
	}
	return incident, report, nil
}

// Vertices returns the graph vertices in fixture order.
func (g IncidentGraph) Vertices() []graph.Vertex {
	return append([]graph.Vertex(nil), g.vertices...)
}

// Edges returns the graph edges in fixture order.
func (g IncidentGraph) Edges() []graph.Edge {
	return append([]graph.Edge(nil), g.edges...)
}

// Records returns graphio records for every vertex and edge in fixture order.
func (g IncidentGraph) Records() ([]graphio.Record, error) {
	records := make([]graphio.Record, 0, len(g.vertices)+len(g.edges))
	for _, vertex := range g.vertices {
		record, err := graphio.VertexRecord(vertex)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	for _, edge := range g.edges {
		record, err := graphio.EdgeRecord(edge)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// WriteNDJSON exports the incident graph as graphio NDJSON records.
func (g IncidentGraph) WriteNDJSON(ctx context.Context, writer io.Writer) (graphio.Report, error) {
	records, err := g.Records()
	if err != nil {
		return graphio.Report{}, err
	}
	return graphio.WriteNDJSON(ctx, writer, records, graphio.WriteOptions{})
}

// DownstreamDependencies returns service IDs reachable by outgoing DEPENDS_ON edges.
func (g IncidentGraph) DownstreamDependencies(serviceID string, maxDepth int) []string {
	start, ok := g.serviceIDs[serviceID]
	if !ok {
		return nil
	}
	return g.traverseServices(start, maxDepth, g.outgoing, edgeDependsOn)
}

// UpstreamImpactedServices returns service IDs that call the given service.
func (g IncidentGraph) UpstreamImpactedServices(serviceID string, maxDepth int) []string {
	start, ok := g.serviceIDs[serviceID]
	if !ok {
		return nil
	}
	return g.traverseServices(start, maxDepth, g.incoming, edgeDependsOn)
}

// AffectedAPIs returns public API IDs that eventually depend on the service.
func (g IncidentGraph) AffectedAPIs(serviceID string, maxDepth int) []string {
	start, ok := g.serviceIDs[serviceID]
	if !ok {
		return nil
	}
	return g.traverseByLabel(start, maxDepth, g.incoming, edgeDependsOn, labelAPI, "apiId")
}

// AlertBoundary returns service IDs targeted by the supplied alert IDs.
func (g IncidentGraph) AlertBoundary(alertIDs []string, _ int) []string {
	result := make(map[string]struct{})
	for _, alertID := range alertIDs {
		vertexID, ok := g.alertIDs[alertID]
		if !ok {
			continue
		}
		for _, edge := range g.outgoing[vertexID] {
			if edge.Label().String() != edgeAlertsOn {
				continue
			}
			if serviceID := g.propertyString(edge.EndID().String(), "serviceId"); serviceID != "" {
				result[serviceID] = struct{}{}
			}
		}
	}
	return sortedKeys(result)
}

// OwningTeams returns team IDs that own the supplied service.
func (g IncidentGraph) OwningTeams(serviceID string) []string {
	vertexID, ok := g.serviceIDs[serviceID]
	if !ok {
		return nil
	}

	result := make(map[string]struct{})
	for _, edge := range g.outgoing[vertexID] {
		if edge.Label().String() != edgeOwnedBy {
			continue
		}
		if teamID := g.propertyString(edge.EndID().String(), "teamId"); teamID != "" {
			result[teamID] = struct{}{}
		}
	}
	return sortedKeys(result)
}

func newIncidentGraph(rawVertices []rawVertex, rawEdges []rawEdge) (IncidentGraph, error) {
	vertices := make([]graph.Vertex, 0, len(rawVertices))
	for _, raw := range rawVertices {
		vertex, err := graph.ParseVertex(raw.id, raw.label, raw.properties)
		if err != nil {
			return IncidentGraph{}, fmt.Errorf("parse vertex %q: %w", raw.id, err)
		}
		vertices = append(vertices, vertex)
	}

	edges := make([]graph.Edge, 0, len(rawEdges))
	for _, raw := range rawEdges {
		edge, err := graph.ParseEdge(raw.id, raw.label, graph.RawEdgeEndpoints{Start: raw.start, End: raw.end}, raw.properties)
		if err != nil {
			return IncidentGraph{}, fmt.Errorf("parse edge %q: %w", raw.id, err)
		}
		edges = append(edges, edge)
	}

	return assembleIncidentGraph(vertices, edges)
}

func assembleIncidentGraph(vertices []graph.Vertex, edges []graph.Edge) (IncidentGraph, error) {
	g := IncidentGraph{
		vertices:     append([]graph.Vertex(nil), vertices...),
		edges:        append([]graph.Edge(nil), edges...),
		verticesByID: make(map[string]graph.Vertex, len(vertices)),
		serviceIDs:   make(map[string]string),
		apiIDs:       make(map[string]string),
		teamIDs:      make(map[string]string),
		alertIDs:     make(map[string]string),
		outgoing:     make(map[string][]graph.Edge),
		incoming:     make(map[string][]graph.Edge),
	}

	for _, vertex := range g.vertices {
		id := vertex.ID().String()
		if _, ok := g.verticesByID[id]; ok {
			return IncidentGraph{}, fmt.Errorf("duplicate vertex %q", id)
		}
		g.verticesByID[id] = vertex
		g.indexVertex(vertex)
	}

	for _, edge := range g.edges {
		start := edge.StartID().String()
		end := edge.EndID().String()
		if _, ok := g.verticesByID[start]; !ok {
			return IncidentGraph{}, fmt.Errorf("edge %q missing start vertex %q", edge.ID(), start)
		}
		if _, ok := g.verticesByID[end]; !ok {
			return IncidentGraph{}, fmt.Errorf("edge %q missing end vertex %q", edge.ID(), end)
		}
		g.outgoing[start] = append(g.outgoing[start], edge)
		g.incoming[end] = append(g.incoming[end], edge)
	}

	return g, nil
}

func (g IncidentGraph) indexVertex(vertex graph.Vertex) {
	id := vertex.ID().String()
	props := vertex.Properties()
	switch vertex.Label().String() {
	case labelService:
		indexExternalID(g.serviceIDs, props, "serviceId", id)
	case labelAPI:
		indexExternalID(g.apiIDs, props, "apiId", id)
	case labelTeam:
		indexExternalID(g.teamIDs, props, "teamId", id)
	case labelAlert:
		indexExternalID(g.alertIDs, props, "alertId", id)
	}
}

func indexExternalID(index map[string]string, props graph.Properties, key string, vertexID string) {
	if externalID, ok := props[key].(string); ok && externalID != "" {
		index[externalID] = vertexID
	}
}

func (g IncidentGraph) traverseServices(start string, maxDepth int, edges map[string][]graph.Edge, label string) []string {
	return g.traverseByLabel(start, maxDepth, edges, label, labelService, "serviceId")
}

func (g IncidentGraph) traverseByLabel(start string, maxDepth int, edges map[string][]graph.Edge, edgeLabel string, vertexLabel string, propertyKey string) []string {
	if maxDepth <= 0 {
		return nil
	}

	type item struct {
		vertexID string
		depth    int
	}
	queue := []item{{vertexID: start, depth: 0}}
	visited := map[string]struct{}{start: {}}
	result := make(map[string]struct{})

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}

		for _, edge := range edges[current.vertexID] {
			if edge.Label().String() != edgeLabel {
				continue
			}
			nextID := nextVertexID(edge, current.vertexID)
			if _, ok := visited[nextID]; ok {
				continue
			}
			visited[nextID] = struct{}{}
			queue = append(queue, item{vertexID: nextID, depth: current.depth + 1})

			vertex, ok := g.verticesByID[nextID]
			if !ok || vertex.Label().String() != vertexLabel {
				continue
			}
			if value := g.propertyString(nextID, propertyKey); value != "" {
				result[value] = struct{}{}
			}
		}
	}

	return sortedKeys(result)
}

func nextVertexID(edge graph.Edge, currentID string) string {
	if edge.StartID().String() == currentID {
		return edge.EndID().String()
	}
	return edge.StartID().String()
}

func (g IncidentGraph) propertyString(vertexID string, key string) string {
	vertex, ok := g.verticesByID[vertexID]
	if !ok {
		return ""
	}
	value, _ := vertex.Properties()[key].(string)
	return value
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func seedVertices() []rawVertex {
	return []rawVertex{
		{id: "svc-edge", label: labelService, properties: graph.Properties{"serviceId": "edge-api", "name": "Edge API", "tier": "edge", "status": "healthy"}},
		{id: "svc-checkout", label: labelService, properties: graph.Properties{"serviceId": "checkout-service", "name": "Checkout Service", "tier": "application", "status": "degraded"}},
		{id: "svc-payment", label: labelService, properties: graph.Properties{"serviceId": "payment-service", "name": "Payment Service", "tier": "application", "status": "failing"}},
		{id: "svc-postgres", label: labelService, properties: graph.Properties{"serviceId": "postgres-primary", "name": "PostgreSQL Primary", "tier": "database", "status": "degraded"}},
		{id: "api-checkout", label: labelAPI, properties: graph.Properties{"apiId": "checkout-api", "name": "Checkout API", "tier": "public", "status": "degraded"}},
		{id: "api-mobile", label: labelAPI, properties: graph.Properties{"apiId": "mobile-checkout-api", "name": "Mobile Checkout API", "tier": "public", "status": "degraded"}},
		{id: "team-payments", label: labelTeam, properties: graph.Properties{"teamId": "payments-team", "name": "Payments Team", "tier": "oncall", "status": "active"}},
		{id: "alert-payment", label: labelAlert, properties: graph.Properties{"alertId": "payment-latency", "name": "Payment latency high", "severity": "critical", "status": "open"}},
		{id: "alert-checkout", label: labelAlert, properties: graph.Properties{"alertId": "checkout-errors", "name": "Checkout errors", "severity": "warning", "status": "open"}},
		{id: "incident-1001", label: labelIncident, properties: graph.Properties{"incidentId": "incident-1001", "name": "Checkout payment incident", "severity": "critical", "status": "open"}},
	}
}

func seedEdges() []rawEdge {
	return []rawEdge{
		{id: "dep-checkout-payment", label: edgeDependsOn, start: "svc-checkout", end: "svc-payment", properties: graph.Properties{"kind": "sync-call"}},
		{id: "dep-payment-postgres", label: edgeDependsOn, start: "svc-payment", end: "svc-postgres", properties: graph.Properties{"kind": "jdbc"}},
		{id: "dep-edge-checkout", label: edgeDependsOn, start: "svc-edge", end: "svc-checkout", properties: graph.Properties{"kind": "http"}},
		{id: "dep-api-checkout", label: edgeDependsOn, start: "api-checkout", end: "svc-edge", properties: graph.Properties{"kind": "http"}},
		{id: "dep-api-mobile", label: edgeDependsOn, start: "api-mobile", end: "svc-edge", properties: graph.Properties{"kind": "http"}},
		{id: "own-payment", label: edgeOwnedBy, start: "svc-payment", end: "team-payments", properties: graph.Properties{"kind": "oncall"}},
		{id: "own-checkout", label: edgeOwnedBy, start: "svc-checkout", end: "team-payments", properties: graph.Properties{"kind": "oncall"}},
		{id: "alert-payment", label: edgeAlertsOn, start: "alert-payment", end: "svc-payment", properties: graph.Properties{"kind": "metric"}},
		{id: "alert-checkout", label: edgeAlertsOn, start: "alert-checkout", end: "svc-checkout", properties: graph.Properties{"kind": "metric"}},
		{id: "root-incident", label: edgeRootCause, start: "incident-1001", end: "svc-payment", properties: graph.Properties{"kind": "investigation"}},
	}
}
