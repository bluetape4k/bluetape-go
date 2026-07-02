package iamaccess

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

const (
	labelUser         = "IamUser"
	labelGroup        = "IamGroup"
	labelRole         = "IamRole"
	labelPolicy       = "IamPolicy"
	labelPermission   = "IamPermission"
	labelResource     = "IamResource"
	labelSessionGrant = "IamSessionGrant"

	edgeMemberOf            = "MEMBER_OF"
	edgeHasRole             = "HAS_ROLE"
	edgeAttachedPolicy      = "ATTACHED_POLICY"
	edgeGrantsPermission    = "GRANTS_PERMISSION"
	edgeAppliesTo           = "APPLIES_TO"
	edgeHasTempGrant        = "HAS_TEMP_GRANT"
	edgeTemporaryPermission = "TEMPORARY_PERMISSION"

	effectAllow   = "allow"
	effectDeny    = "deny"
	maxGroupDepth = 4
)

// AccessExplanation explains whether a user can perform an action on a resource.
type AccessExplanation struct {
	UserID     string
	ResourceID string
	Action     string
	Allowed    bool
	Path       []string
	Reason     string
}

// PrivilegeChain describes inherited privileged access that should be reviewed.
type PrivilegeChain struct {
	UserID string
	RoleID string
	Path   []string
	Reason string
}

// Graph is a small immutable IAM access graph fixture.
type Graph struct {
	vertices []graph.Vertex
	edges    []graph.Edge

	verticesByID map[string]graph.Vertex
	userIDs      map[string]string

	outgoing map[string][]graph.Edge
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

type vertexPath struct {
	vertex   graph.Vertex
	vertices []graph.Vertex
}

// SeedIAMAccessGraph returns the bundled IAM access graph.
func SeedIAMAccessGraph() (Graph, error) {
	return newIAMAccessGraph(seedVertices(), seedEdges())
}

// ReadIAMAccessGraphNDJSON imports an IAM access graph from graphio NDJSON records.
func ReadIAMAccessGraphNDJSON(ctx context.Context, reader io.Reader) (Graph, graphio.Report, error) {
	records, report, err := graphio.ReadNDJSON(ctx, reader, graphio.ReadOptions{})
	if err != nil {
		return Graph{}, report, err
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
			return Graph{}, report, fmt.Errorf("unsupported graph record kind %q", record.Kind)
		}
	}

	access, err := assembleIAMAccessGraph(vertices, edges)
	if err != nil {
		return Graph{}, report, err
	}
	return access, report, nil
}

// Vertices returns the graph vertices in fixture order.
func (g Graph) Vertices() []graph.Vertex {
	return append([]graph.Vertex(nil), g.vertices...)
}

// Edges returns the graph edges in fixture order.
func (g Graph) Edges() []graph.Edge {
	return append([]graph.Edge(nil), g.edges...)
}

// Records returns graphio records for every vertex and edge in fixture order.
func (g Graph) Records() ([]graphio.Record, error) {
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

// WriteNDJSON exports the IAM access graph as graphio NDJSON records.
func (g Graph) WriteNDJSON(ctx context.Context, writer io.Writer) (graphio.Report, error) {
	records, err := g.Records()
	if err != nil {
		return graphio.Report{}, err
	}
	return graphio.WriteNDJSON(ctx, writer, records, graphio.WriteOptions{})
}

// ExplainAccess explains whether a user can perform action on resourceID.
func (g Graph) ExplainAccess(userID string, resourceID string, action string) AccessExplanation {
	user, ok := g.userByID(userID)
	if !ok {
		return denied(userID, resourceID, action, "User not found")
	}

	if paths := g.policyPaths(user, effectDeny, &resourceID, &action); len(paths) > 0 {
		return AccessExplanation{
			UserID:     userID,
			ResourceID: resourceID,
			Action:     action,
			Allowed:    false,
			Path:       displayPath(paths[0]),
			Reason:     "Denied by explicit policy path",
		}
	}

	if paths := g.allowPaths(user, &resourceID, &action); len(paths) > 0 {
		return AccessExplanation{
			UserID:     userID,
			ResourceID: resourceID,
			Action:     action,
			Allowed:    true,
			Path:       displayPath(paths[0]),
			Reason:     "Granted by reachable IAM path",
		}
	}

	return denied(userID, resourceID, action, "No matching grant path")
}

// RiskyPrivilegeChains returns nested group paths that grant admin roles.
func (g Graph) RiskyPrivilegeChains(userID string) []PrivilegeChain {
	user, ok := g.userByID(userID)
	if !ok {
		return nil
	}

	var chains []PrivilegeChain
	for _, principalPath := range g.principalPaths(user) {
		if countLabel(principalPath.vertices, labelGroup) < 2 {
			continue
		}
		for _, role := range g.outgoingVertices(principalPath.vertex, edgeHasRole, labelRole) {
			if propertyString(role, "privilege") != "admin" {
				continue
			}
			chains = append(chains, PrivilegeChain{
				UserID: userID,
				RoleID: propertyString(role, "roleId"),
				Path:   displayPath(append(append([]graph.Vertex(nil), principalPath.vertices...), role)),
				Reason: "Admin role inherited through nested groups",
			})
		}
	}
	sort.Slice(chains, func(i, j int) bool {
		return chains[i].RoleID < chains[j].RoleID
	})
	return chains
}

// ExcessivePermissions finds allowed actions outside the approved resource set.
func (g Graph) ExcessivePermissions(userID string, approvedActionsByResource map[string][]string) []AccessExplanation {
	user, ok := g.userByID(userID)
	if !ok {
		return nil
	}

	var findings []AccessExplanation
	for _, path := range g.allowPaths(user, nil, nil) {
		permission, ok := firstByLabel(path, labelPermission)
		if !ok {
			continue
		}
		resource := path[len(path)-1]
		resourceID := propertyString(resource, "resourceId")
		action := propertyString(permission, "action")
		if slices.Contains(approvedActionsByResource[resourceID], action) {
			continue
		}
		findings = append(findings, AccessExplanation{
			UserID:     userID,
			ResourceID: resourceID,
			Action:     action,
			Allowed:    true,
			Path:       displayPath(path),
			Reason:     "Granted action is outside the approved least-privilege set",
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].ResourceID == findings[j].ResourceID {
			return findings[i].Action < findings[j].Action
		}
		return findings[i].ResourceID < findings[j].ResourceID
	})
	return findings
}

func newIAMAccessGraph(rawVertices []rawVertex, rawEdges []rawEdge) (Graph, error) {
	vertices := make([]graph.Vertex, 0, len(rawVertices))
	for _, raw := range rawVertices {
		vertex, err := graph.ParseVertex(raw.id, raw.label, raw.properties)
		if err != nil {
			return Graph{}, fmt.Errorf("parse vertex %q: %w", raw.id, err)
		}
		vertices = append(vertices, vertex)
	}

	edges := make([]graph.Edge, 0, len(rawEdges))
	for _, raw := range rawEdges {
		edge, err := graph.ParseEdge(raw.id, raw.label, graph.RawEdgeEndpoints{Start: raw.start, End: raw.end}, raw.properties)
		if err != nil {
			return Graph{}, fmt.Errorf("parse edge %q: %w", raw.id, err)
		}
		edges = append(edges, edge)
	}

	return assembleIAMAccessGraph(vertices, edges)
}

func assembleIAMAccessGraph(vertices []graph.Vertex, edges []graph.Edge) (Graph, error) {
	g := Graph{
		vertices:     append([]graph.Vertex(nil), vertices...),
		edges:        append([]graph.Edge(nil), edges...),
		verticesByID: make(map[string]graph.Vertex, len(vertices)),
		userIDs:      make(map[string]string),
		outgoing:     make(map[string][]graph.Edge),
	}

	for _, vertex := range g.vertices {
		id := vertex.ID().String()
		if _, ok := g.verticesByID[id]; ok {
			return Graph{}, fmt.Errorf("duplicate vertex %q", id)
		}
		g.verticesByID[id] = vertex
		if vertex.Label().String() == labelUser {
			indexExternalID(g.userIDs, vertex.Properties(), "userId", id)
		}
	}

	for _, edge := range g.edges {
		start := edge.StartID().String()
		end := edge.EndID().String()
		if _, ok := g.verticesByID[start]; !ok {
			return Graph{}, fmt.Errorf("edge %q missing start vertex %q", edge.ID(), start)
		}
		if _, ok := g.verticesByID[end]; !ok {
			return Graph{}, fmt.Errorf("edge %q missing end vertex %q", edge.ID(), end)
		}
		g.outgoing[start] = append(g.outgoing[start], edge)
	}

	return g, nil
}

func (g Graph) userByID(userID string) (graph.Vertex, bool) {
	vertexID, ok := g.userIDs[userID]
	if !ok {
		return graph.Vertex{}, false
	}
	vertex, ok := g.verticesByID[vertexID]
	return vertex, ok
}

func (g Graph) allowPaths(user graph.Vertex, resourceID *string, action *string) [][]graph.Vertex {
	return append(g.policyPaths(user, effectAllow, resourceID, action), g.temporaryPaths(user, resourceID, action)...)
}

func (g Graph) policyPaths(user graph.Vertex, effect string, resourceID *string, action *string) [][]graph.Vertex {
	var paths [][]graph.Vertex
	for _, principalPath := range g.principalPaths(user) {
		for _, role := range g.outgoingVertices(principalPath.vertex, edgeHasRole, labelRole) {
			for _, policy := range g.outgoingVertices(role, edgeAttachedPolicy, labelPolicy) {
				if propertyString(policy, "effect") != effect {
					continue
				}
				for _, permission := range g.outgoingVertices(policy, edgeGrantsPermission, labelPermission) {
					if action != nil && propertyString(permission, "action") != *action {
						continue
					}
					for _, resource := range g.matchingResources(permission, resourceID) {
						path := append(append([]graph.Vertex(nil), principalPath.vertices...), role, policy, permission, resource)
						paths = append(paths, path)
					}
				}
			}
		}
	}
	return paths
}

func (g Graph) temporaryPaths(user graph.Vertex, resourceID *string, action *string) [][]graph.Vertex {
	var paths [][]graph.Vertex
	for _, grant := range g.outgoingVertices(user, edgeHasTempGrant, labelSessionGrant) {
		for _, permission := range g.outgoingVertices(grant, edgeTemporaryPermission, labelPermission) {
			if action != nil && propertyString(permission, "action") != *action {
				continue
			}
			for _, resource := range g.matchingResources(permission, resourceID) {
				paths = append(paths, []graph.Vertex{user, grant, permission, resource})
			}
		}
	}
	return paths
}

func (g Graph) principalPaths(user graph.Vertex) []vertexPath {
	paths := []vertexPath{{vertex: user, vertices: []graph.Vertex{user}}}
	queue := []vertexPath{{vertex: user, vertices: []graph.Vertex{user}}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if len(current.vertices) > maxGroupDepth+1 {
			continue
		}
		for _, group := range g.outgoingVertices(current.vertex, edgeMemberOf, labelGroup) {
			if containsVertex(current.vertices, group.ID().String()) {
				continue
			}
			nextPath := append(append([]graph.Vertex(nil), current.vertices...), group)
			next := vertexPath{vertex: group, vertices: nextPath}
			paths = append(paths, next)
			queue = append(queue, next)
		}
	}

	return paths
}

func (g Graph) matchingResources(permission graph.Vertex, resourceID *string) []graph.Vertex {
	resources := g.outgoingVertices(permission, edgeAppliesTo, labelResource)
	if resourceID == nil {
		return resources
	}
	filtered := resources[:0]
	for _, resource := range resources {
		if propertyString(resource, "resourceId") == *resourceID {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func (g Graph) outgoingVertices(vertex graph.Vertex, edgeLabel string, vertexLabel string) []graph.Vertex {
	var result []graph.Vertex
	for _, edge := range g.outgoing[vertex.ID().String()] {
		if edge.Label().String() != edgeLabel {
			continue
		}
		next, ok := g.verticesByID[edge.EndID().String()]
		if !ok || next.Label().String() != vertexLabel {
			continue
		}
		result = append(result, next)
	}
	return result
}

func indexExternalID(index map[string]string, props graph.Properties, key string, vertexID string) {
	if externalID, ok := props[key].(string); ok && externalID != "" {
		index[externalID] = vertexID
	}
}

func displayPath(vertices []graph.Vertex) []string {
	path := make([]string, 0, len(vertices))
	for _, vertex := range vertices {
		path = append(path, displayID(vertex))
	}
	return path
}

func displayID(vertex graph.Vertex) string {
	props := vertex.Properties()
	switch vertex.Label().String() {
	case labelUser:
		return "user:" + stringProperty(props, "userId")
	case labelGroup:
		return "group:" + stringProperty(props, "groupId")
	case labelRole:
		return "role:" + stringProperty(props, "roleId")
	case labelPolicy:
		return "policy:" + stringProperty(props, "policyId")
	case labelPermission:
		return "permission:" + stringProperty(props, "action")
	case labelResource:
		return "resource:" + stringProperty(props, "resourceId")
	case labelSessionGrant:
		return "grant:" + stringProperty(props, "grantId")
	default:
		return vertex.Label().String() + ":" + vertex.ID().String()
	}
}

func propertyString(vertex graph.Vertex, key string) string {
	return stringProperty(vertex.Properties(), key)
}

func stringProperty(props graph.Properties, key string) string {
	value, _ := props[key].(string)
	return value
}

func countLabel(vertices []graph.Vertex, label string) int {
	count := 0
	for _, vertex := range vertices {
		if vertex.Label().String() == label {
			count++
		}
	}
	return count
}

func containsVertex(vertices []graph.Vertex, id string) bool {
	for _, vertex := range vertices {
		if vertex.ID().String() == id {
			return true
		}
	}
	return false
}

func firstByLabel(vertices []graph.Vertex, label string) (graph.Vertex, bool) {
	for _, vertex := range vertices {
		if vertex.Label().String() == label {
			return vertex, true
		}
	}
	return graph.Vertex{}, false
}

func denied(userID string, resourceID string, action string, reason string) AccessExplanation {
	return AccessExplanation{
		UserID:     userID,
		ResourceID: resourceID,
		Action:     action,
		Allowed:    false,
		Reason:     reason,
	}
}

func seedVertices() []rawVertex {
	return []rawVertex{
		{id: "user-alice", label: labelUser, properties: graph.Properties{"userId": "alice", "displayName": "Alice Kim", "department": "engineering"}},
		{id: "user-bob", label: labelUser, properties: graph.Properties{"userId": "bob", "displayName": "Bob Lee", "department": "audit"}},
		{id: "user-carol", label: labelUser, properties: graph.Properties{"userId": "carol", "displayName": "Carol Park", "department": "operations"}},
		{id: "user-eve", label: labelUser, properties: graph.Properties{"userId": "eve", "displayName": "Eve Contractor", "department": "contractor"}},
		{id: "group-engineering", label: labelGroup, properties: graph.Properties{"groupId": "engineering", "name": "Engineering", "riskTier": "standard"}},
		{id: "group-platform-admins", label: labelGroup, properties: graph.Properties{"groupId": "platform-admins", "name": "Platform Admins", "riskTier": "privileged"}},
		{id: "role-readonly", label: labelRole, properties: graph.Properties{"roleId": "readonly-role", "name": "Read-only Analyst", "privilege": "read"}},
		{id: "role-deployer", label: labelRole, properties: graph.Properties{"roleId": "deployer-role", "name": "Staging Deployer", "privilege": "write"}},
		{id: "role-prod-admin", label: labelRole, properties: graph.Properties{"roleId": "prod-admin-role", "name": "Production Admin", "privilege": "admin"}},
		{id: "role-contractor", label: labelRole, properties: graph.Properties{"roleId": "contractor-role", "name": "Contractor Guardrail", "privilege": "restricted"}},
		{id: "policy-read-audit", label: labelPolicy, properties: graph.Properties{"policyId": "read-audit-policy", "name": "Read audit dashboard", "effect": effectAllow}},
		{id: "policy-deploy-staging", label: labelPolicy, properties: graph.Properties{"policyId": "deploy-staging-policy", "name": "Deploy staging service", "effect": effectAllow}},
		{id: "policy-prod-admin", label: labelPolicy, properties: graph.Properties{"policyId": "prod-admin-policy", "name": "Production administration", "effect": effectAllow}},
		{id: "policy-deny-prod-delete", label: labelPolicy, properties: graph.Properties{"policyId": "deny-prod-delete-policy", "name": "Deny production delete", "effect": effectDeny}},
		{id: "perm-read-audit-dashboard", label: labelPermission, properties: graph.Properties{"permissionId": "read-audit-dashboard", "action": "read"}},
		{id: "perm-deploy-staging-service", label: labelPermission, properties: graph.Properties{"permissionId": "deploy-staging-service", "action": "deploy"}},
		{id: "perm-delete-prod-db", label: labelPermission, properties: graph.Properties{"permissionId": "delete-prod-db", "action": "delete"}},
		{id: "perm-read-prod-db", label: labelPermission, properties: graph.Properties{"permissionId": "read-prod-db", "action": "read"}},
		{id: "res-audit-dashboard", label: labelResource, properties: graph.Properties{"resourceId": "audit-dashboard", "name": "Audit Dashboard", "resourceType": "dashboard", "classification": "internal"}},
		{id: "res-staging-service", label: labelResource, properties: graph.Properties{"resourceId": "staging-service", "name": "Staging Service", "resourceType": "service", "classification": "internal"}},
		{id: "res-prod-db", label: labelResource, properties: graph.Properties{"resourceId": "prod-db", "name": "Production Database", "resourceType": "database", "classification": "restricted"}},
		{id: "grant-break-glass-1001", label: labelSessionGrant, properties: graph.Properties{"grantId": "break-glass-1001", "reason": "temporary production read during incident", "expiresAt": "2026-06-02T00:00:00Z"}},
	}
}

func seedEdges() []rawEdge {
	return []rawEdge{
		{id: "member-alice-engineering", label: edgeMemberOf, start: "user-alice", end: "group-engineering", properties: graph.Properties{"kind": "standing"}},
		{id: "member-engineering-platform-admins", label: edgeMemberOf, start: "group-engineering", end: "group-platform-admins", properties: graph.Properties{"kind": "nested"}},
		{id: "role-bob-readonly", label: edgeHasRole, start: "user-bob", end: "role-readonly", properties: graph.Properties{"source": "direct"}},
		{id: "role-engineering-deployer", label: edgeHasRole, start: "group-engineering", end: "role-deployer", properties: graph.Properties{"source": "group"}},
		{id: "role-platform-admins-prod-admin", label: edgeHasRole, start: "group-platform-admins", end: "role-prod-admin", properties: graph.Properties{"source": "nested-group"}},
		{id: "role-eve-contractor", label: edgeHasRole, start: "user-eve", end: "role-contractor", properties: graph.Properties{"source": "contract"}},
		{id: "policy-readonly-read-audit", label: edgeAttachedPolicy, start: "role-readonly", end: "policy-read-audit", properties: graph.Properties{"scope": "account"}},
		{id: "policy-deployer-deploy-staging", label: edgeAttachedPolicy, start: "role-deployer", end: "policy-deploy-staging", properties: graph.Properties{"scope": "account"}},
		{id: "policy-prod-admin-admin", label: edgeAttachedPolicy, start: "role-prod-admin", end: "policy-prod-admin", properties: graph.Properties{"scope": "account"}},
		{id: "policy-contractor-deny-delete", label: edgeAttachedPolicy, start: "role-contractor", end: "policy-deny-prod-delete", properties: graph.Properties{"scope": "account"}},
		{id: "grant-read-audit", label: edgeGrantsPermission, start: "policy-read-audit", end: "perm-read-audit-dashboard", properties: graph.Properties{"condition": "none"}},
		{id: "grant-deploy-staging", label: edgeGrantsPermission, start: "policy-deploy-staging", end: "perm-deploy-staging-service", properties: graph.Properties{"condition": "none"}},
		{id: "grant-prod-admin-delete", label: edgeGrantsPermission, start: "policy-prod-admin", end: "perm-delete-prod-db", properties: graph.Properties{"condition": "none"}},
		{id: "grant-deny-delete", label: edgeGrantsPermission, start: "policy-deny-prod-delete", end: "perm-delete-prod-db", properties: graph.Properties{"condition": "none"}},
		{id: "applies-read-audit", label: edgeAppliesTo, start: "perm-read-audit-dashboard", end: "res-audit-dashboard", properties: graph.Properties{"scope": "resource"}},
		{id: "applies-deploy-staging", label: edgeAppliesTo, start: "perm-deploy-staging-service", end: "res-staging-service", properties: graph.Properties{"scope": "resource"}},
		{id: "applies-delete-prod", label: edgeAppliesTo, start: "perm-delete-prod-db", end: "res-prod-db", properties: graph.Properties{"scope": "resource"}},
		{id: "applies-read-prod", label: edgeAppliesTo, start: "perm-read-prod-db", end: "res-prod-db", properties: graph.Properties{"scope": "resource"}},
		{id: "temp-carol-break-glass", label: edgeHasTempGrant, start: "user-carol", end: "grant-break-glass-1001", properties: graph.Properties{"state": "active"}},
		{id: "temp-break-glass-read-prod", label: edgeTemporaryPermission, start: "grant-break-glass-1001", end: "perm-read-prod-db", properties: graph.Properties{"source": "break-glass"}},
	}
}
