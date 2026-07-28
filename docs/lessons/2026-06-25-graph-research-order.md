# Graph Research Ordering 교훈

bluetape-go research issue가 큰 Kotlin source module에 mapping되더라도 가장 넓은
source abstraction을 port하는 것으로 시작하지 않는다. 먼저 Go caller value, driver
maturity, local-test story를 순위화한다.

graph track에서 evidence는 backend-independent repository/session contract 전에
NDJSON/CSV I/O와 하나의 concrete domain example을 가리킨다. Neo4j는 official Go
driver와 Testcontainers support가 있으므로 첫 backend candidate다. Memgraph는
Neo4j-driver compatibility coverage로 시작해야 한다. AGE, FalkorDB, TinkerGraph,
GraphML, Kotlin web integration parity는 더 작은 Go proof가 정당화할 때까지 deferred로
둔다.
