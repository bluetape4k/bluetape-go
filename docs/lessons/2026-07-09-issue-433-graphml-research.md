# Issue #433 GraphML Research 교훈

GraphML은 단순 XML parser 작업이 아니라 compatibility project다.

- 실제 Go caller가 NDJSON이나 paired CSV보다 GraphML이 필요함을 증명할 때까지
  `graph/graphio`는 bounded record stream 중심으로 유지한다.
- GraphML을 되살릴 때는 XML-specific behavior를 optional package boundary 뒤에
  두고, 코드를 쓰기 전에 subset을 정의한다.
- compatibility claim에는 producer fixture가 필요하다. 손으로 만든 minimal
  GraphML만으로 NetworkX, Gephi, Neo4j APOC, yEd compatibility를 주장할 수 없다.
- typed value, default, unknown key, duplicate ID, missing endpoint, nested
  graph, hyperedge, port, yFiles visual payload를 명시적 contract decision으로
  취급한다.
- caller가 trusted file을 명시적으로 opt-in하지 않는 한 모든 GraphML reader는
  untrusted XML input으로 설계한다. bounded decoding과 caller-owned
  deadline/close behavior도 acceptance criteria의 일부다.
