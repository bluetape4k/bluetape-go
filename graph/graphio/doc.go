// Package graphio provides stream-oriented graph import and export helpers.
//
// The package supports NDJSON and paired CSV streams for graph.Vertex and
// graph.Edge values. The optional graph/graphio/graphml subpackage owns the
// bounded GraphML subset. Compression, encryption, path ownership, atomic file
// replacement, and backend integration are intentionally deferred.
package graphio
