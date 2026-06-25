# Issue 43 I/O, Codec, Crypto, And Protocol Research Scope

Issue #43 is the 0.7.0 research gate for deciding which portable I/O,
compression, serialization, crypto, and protocol concepts should become
additional Go work. The current repo already has `codec`, `compression`, and
`serialization`, so the main decision is to avoid duplicating the Go standard
library or the existing packages.

## Source Inventory

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-projects/io`

- `io/io` provides Kotlin compressors, binary serializers, serializer +
  compression combinations, file utilities, safe path helpers, and zip-bomb
  protection.
- `io/tink` provides AEAD/DAEAD wrappers, associated-data-first APIs, string
  and byte encryption helpers, versioned keysets, Redis keyset stores, MAC,
  digest helpers, and rotation guidance.
- `io/http`, `io/grpc`, `io/csv`, `io/protobuf`, `io/avro`, `io/json`,
  `io/jackson*`, `io/fastjson2`, `io/okio`, `io/netty`, `io/vertx`,
  `io/feign`, and `io/retrofit2` are mostly JVM/backend integration surfaces.
- `bluetape4k-aws` contributes SigV4 and service-protocol examples, but #42
  already rejected a Ktor-shaped or broad AWS wrapper port.

## Current bluetape-go Evidence

- `codec` already covers Base58, Base62, Base64, hex, URL-safe helpers,
  alphabet validation, and UTF-8 distinction between text and byte APIs.
- `compression` already covers gzip, zlib, deflate, zstd, lz4, snappy,
  registry-backed selection, byte-slice APIs, stream APIs, and bounded
  decompression helpers.
- `serialization` already covers JSON, raw bytes, UTF-8 strings, strict JSON,
  and versioned `BTGS` envelopes.
- #71 is already open for encryption facade decisions and is the correct owner
  for AEAD/DAEAD/keyset/KMS/Redis key material work.
- #42 routed SigV4 protocol work to #43 but rejected a standalone AWS wrapper.

## External Go Evidence

- Go standard library packages already cover DEFLATE, gzip, zlib, JSON, CSV,
  HTTP, TLS, URL handling, hashes, and core cryptographic primitives.
- `google.golang.org/protobuf` is the current Go Protobuf implementation and
  code-generation/runtime path; the older `github.com/golang/protobuf` API is
  superseded.
- `google.golang.org/grpc` is the canonical Go gRPC stack and should remain
  direct unless a concrete service example needs shared setup.
- Avro has credible Go options such as `github.com/linkedin/goavro/v2` and
  `github.com/hamba/avro/v2`, but schema evolution, registry integration, and
  Kafka payload policy need a concrete consumer before a bluetape-go package.
- Tink Go and `filippo.io/age` are credible encryption candidates, but the API
  shape belongs to #71 because key material boundaries dominate the design.
- AWS SDK for Go v2 already has generic SigV4/SigV4A signer support; a
  bluetape-go wrapper needs a non-AWS HTTP client consumer before it is
  justified.

## Ranking

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Existing `codec` | High | Low | Keep; no new issue. |
| Existing `compression` | High | Medium | Keep; no new issue until bzip2/zip or benchmark gaps are proven. |
| Existing `serialization` | High | Medium | Keep; no new issue until a non-JSON binary format has a consumer. |
| AEAD/DAEAD/keysets/KMS | High | High | Route to #71. |
| MAC/digest helpers | Medium | Medium/high | Route to #71 when security-related; otherwise use stdlib directly. |
| Protobuf | Medium/high | Medium | Use canonical generated-code/runtime stack directly; examples only if a service needs it. |
| Avro | Medium | High | Defer until Kafka/schema-registry or data pipeline consumer exists. |
| CSV/JSON | High | Low/medium | Use stdlib/current serialization package; no wrapper. |
| HTTP/gRPC | High | Medium | Use stdlib/gRPC directly; package wrappers need concrete repeated setup evidence. |
| Okio-style buffered pipeline | Low/medium | High | Defer; Go `io.Reader`/`io.Writer` already owns this shape. |
| Netty/Vert.x/Feign/Retrofit | Low | High | Reject as JVM framework/client ports. |
| SigV4 generic HTTP signing | Medium | Medium/high | Defer; AWS SDK signer exists and #42 rejected AWS wrapper ports. |

## Keep / Implemented

- Keep the existing `codec`, `compression`, and `serialization` packages as the
  Go-native I/O foundation.
- Keep `compression.DecompressLimit` as the default guidance for untrusted
  compressed bytes.
- Keep `serialization.VersionedSerializer` as the repo-owned envelope boundary.

## Route To #71

- AEAD, deterministic AEAD, associated data, keysets, Redis keyset stores,
  KMS envelope compatibility, MAC, digest, and encryption error semantics.
- #43 should not define crypto APIs beyond saying where they belong.

## Example-only / Direct Use

- Protobuf and gRPC should use `google.golang.org/protobuf` and
  `google.golang.org/grpc` directly. Add examples only when a service package
  needs shared setup or fixtures.
- CSV and JSON should use the standard library and existing `serialization`
  package.
- HTTP should use `net/http` and existing `resilience` HTTP adapters unless a
  repeated caller-owned setup emerges.

## Defer

- Avro, schema registry integration, and Kafka payload codecs until a messaging
  or data-pipeline issue requires them.
- Zip archive helpers and bzip2 until there is a file/archive workflow and
  bounded extraction contract.
- SigV4 generic signing until a non-AWS SDK HTTP client package needs it.
- Okio-style pipeline helpers until a concrete streaming codec package proves
  `io.Reader`/`io.Writer` composition is insufficient.

## Rejected Wrappers

- Generic JSON, CSV, HTTP, gRPC, Protobuf, AWS SigV4, Feign, Retrofit, Netty,
  Vert.x, Jackson, Fastjson2, or Okio ports.
- JVM binary serializers such as JDK/Kryo/Fory as general Go package surfaces.
- Unsafe deserialization or dynamic type loading boundaries.

## Issue Updates Required

- #43 should record that no new implementation issues are created from this
  research pass.
- #71 should record that encryption/keyset/MAC/digest/KMS decisions are owned
  there and should use #43 only as routing evidence.

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify #43 and #71 issue bodies contain the #43 research update.
- Preserve external evidence in `bluetape4k-wiki` and validate with
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`.
- No Go tests are required for this PR because no Go code changes.

## Follow-up Recommendation

Work #71 next for encryption. Do not add a new I/O implementation issue until
a real caller needs Avro/schema-registry, archive extraction, SigV4 generic
HTTP signing, or shared Protobuf/gRPC setup.
