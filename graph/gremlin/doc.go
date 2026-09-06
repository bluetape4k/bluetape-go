// Package gremlin provides a narrow remote Gremlin adapter for the existing
// graph model.
//
// The adapter keeps the Apache TinkerPop connection and its TLS/auth
// settings at the caller-owned boundary. It adds bounded result collection,
// local context checkpoints, and deterministic conversion to graph.Vertex and
// graph.Edge without claiming server-side cancellation.
package gremlin
