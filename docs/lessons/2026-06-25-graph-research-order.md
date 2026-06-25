# Graph Research Ordering

When a bluetape-go research issue maps to a large Kotlin source module, do not
start by porting the broadest source abstraction. First rank the Go caller value,
driver maturity, and local-test story.

For the graph track, the evidence points to NDJSON/CSV I/O and one concrete
domain example before a backend-independent repository/session contract. Neo4j is
the first backend candidate because it has official Go driver and Testcontainers
support. Memgraph should begin as Neo4j-driver compatibility coverage. AGE,
FalkorDB, TinkerGraph, GraphML, and Kotlin web integration parity stay deferred
until smaller Go proofs justify them.
