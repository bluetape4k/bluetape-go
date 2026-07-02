// Package iamaccess demonstrates a backend-neutral IAM access graph.
//
// The example keeps the graph in memory so callers can inspect identity,
// group, role, policy, permission, resource, and temporary-grant reachability
// without adopting a graph database adapter.
package iamaccess
