# I/O Research Boundary 교훈

I/O 작업에서는 bluetape-go wrapper를 만들기 전에 Go standard-library와 canonical Go
package contract를 우선한다. 현재 `codec`, `compression`, `serialization` package는
명확한 가치가 있는 foundation을 이미 덮는다.

encryption과 key material decision은 #71에 속한다. Avro, gRPC setup, Protobuf setup,
archive extraction, SigV4 signing 같은 protocol helper는 implementation issue가 되기
전에 concrete package 또는 example consumer가 필요하다.

Netty, Vert.x, Feign, Retrofit, Jackson, Fastjson2, Okio, JDK serialization, Kryo,
Fory 같은 JVM client/framework surface를 generic Go package로 port하지 않는다.

향후 I/O issue는 protocol 자체보다 caller가 반복해서 틀리기 쉬운 경계를 먼저 찾아야 한다.
buffer ownership, cancellation, streaming cleanup, redaction, compatibility fixture처럼 Go에서
검증 가능한 실패면 package가 될 수 있다. JVM framework parity만 있는 항목은 research note로
남기고 implementation issue로 승격하지 않는다.
