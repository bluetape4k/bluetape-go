# Issue #491 GraphML Graph I/O 교훈

첫 GraphML slice는 core `graphio` 밖에 둔다. subpackage는 XML-specific parser
limit, compatibility claim, unsupported construct test가 NDJSON/CSV record boundary를
넓히지 않게 해 준다.

Lesson: GraphML compatibility는 format-wide claim이 아니라 subset으로 표현해야
한다. `graph`, `key`, `data`, `node`, `edge` 지원은 유용하지만 yEd/yFiles visual
payload, nested graph, hyperedge, port, mixed directed/undirected graph
compatibility를 뜻하지 않는다.

Lesson: XML safety는 graph contract의 일부다. `graph.Properties`로 변환하기 전에
directive, extension payload, unknown key, unsupported element를 거절한다. 임의 XML이
caller metadata가 되게 두지 않는다.

Prevention: 향후 producer-specific compatibility 작업은 accepted subset을 넓히기
전에 named fixture를 추가하고 producer/version을 문서화해야 한다.
