# I/O Research Boundaries

For I/O work, prefer Go standard-library and canonical Go package contracts
before creating bluetape-go wrappers. The current `codec`, `compression`, and
`serialization` packages already cover the foundation that has clear value.

Encryption and key material decisions belong in #71. Protocol helpers such as
Avro, gRPC setup, Protobuf setup, archive extraction, and SigV4 signing need a
concrete package or example consumer before they become implementation issues.

Do not port JVM client/framework surfaces such as Netty, Vert.x, Feign,
Retrofit, Jackson, Fastjson2, Okio, JDK serialization, Kryo, or Fory as generic
Go packages.
