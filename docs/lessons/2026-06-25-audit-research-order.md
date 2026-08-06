# Audit Research Ordering 교훈

audit 작업은 JaVers를 port하거나 Kafka, Redis, SQL adapter를 먼저 고르는 방식으로
시작하지 않는다. 첫 Go value는 explicit aggregate ID, revision, metadata,
serialization rule, history query semantic을 가진 작은 audit/event model이다.

storage는 durable adapter 전에 in-memory conformance implementation으로 증명한다. SQL은
가능성이 높은 durable history source지만 relational SQL boundary와 맞아야 한다. Kafka와
NATS는 publisher/outbox adapter이지 history query store가 아니다. Redis는 replay와 head
semantic이 정의된 경우에만 projection 또는 explicit audit store가 된다.
