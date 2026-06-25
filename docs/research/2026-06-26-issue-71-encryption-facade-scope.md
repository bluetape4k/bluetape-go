# Issue 71 Encryption Facade Research Scope

Issue #71 decides whether bluetape-go should expose a Go service encryption
facade after #42 routed KMS here and #43 routed AEAD/DAEAD/keyset/MAC/digest
boundaries here. The outcome is narrow: implement the default byte/string AEAD
facade with the Go standard library, and defer keyset/KMS/streaming variants
until concrete callers need them.

## Source Inventory

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-projects/io/tink`

- `TinkAead` wraps AEAD byte and UTF-8 string encryption with associated data.
- `TinkDeterministicAead` wraps deterministic AEAD for searchable fields and
  documents equality leakage.
- `VersionedKeysetStore` provides current/find/rotate/rotate-if-due operations.
- `VersionedCiphertextSupport` prefixes ciphertext with an 8-byte version.
- Redis-backed stores provide a protected shared keyset boundary, not a generic
  cache or config wrapper.
- MAC and digest helpers are explicitly documented as non-encryption.

Relevant bluetape-go evidence:

- Go module target is `go 1.26.3`; local toolchain is `go1.26.4`.
- `jwt` already has sentinel errors, safe key material copying, repository
  rotation, Redis/Mongo storage boundaries, and stress/race expectations.
- `docs/research/2026-06-25-issue-42-aws-scope.md` routes KMS envelope
  compatibility here and rejects generic AWS KMS wrappers.
- `docs/research/2026-06-25-issue-43-io-codec-protocol-scope.md` routes
  AEAD/DAEAD/keysets/MAC/digest here and rejects broad I/O wrappers.
- `bluetape4k-exposed` PR #300 removed unsafe generated-keyset defaults for
  persisted encrypted columns, proving durable ciphertext needs explicit
  persisted/versioned key material.

## External Evidence

- Go `crypto/cipher` in Go 1.26 exposes `NewGCMWithRandomNonce`, which prepends
  a random 96-bit nonce to ciphertext and gives callers an AEAD with nonce size
  zero. This directly supports the #71 requirement that the common API must not
  expose caller-managed nonces.
- Tink deterministic encryption documentation recommends AES256-SIV but warns
  that deterministic ciphertext can reveal repeated plaintext equality. It also
  warns against cleartext keysets in real applications.
- Tink key-management documentation recommends encrypted keysets protected by a
  KMS/KEK and documents AWS KMS URI support, but that is an operational keyset
  story rather than a small default encryption facade.
- `filippo.io/age` is credible for recipient/identity-based file or stream
  encryption, including password-derived identities, but it is file/recipient
  shaped rather than a general service byte/string AEAD facade.
- AWS KMS `GenerateDataKey` returns plaintext data keys plus encrypted data-key
  blobs for local envelope encryption, requires encryption-context matching for
  decrypt, and says plaintext data keys should be erased after local use.
- AWS Encryption SDK for Go exists and requires Go 1.23+, but adopting it would
  make AWS/KMS policy central to the default package instead of optional.

Sources:

- https://pkg.go.dev/crypto/cipher
- https://developers.google.com/tink/deterministic-encryption
- https://developers.google.com/tink/key-management-overview
- https://pkg.go.dev/filippo.io/age
- https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html
- https://docs.aws.amazon.com/encryption-sdk/latest/developer-guide/go.html

## Candidate Ranking

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Stdlib AES-GCM byte/string facade | High | Medium | Implement in #315. |
| Versioned ciphertext envelope | High | Medium | Include in #315 so future keys/KMS can evolve safely. |
| Explicit associated data | High | Medium | Include in #315 as a first-class input. |
| Deterministic AEAD/searchable encryption | Medium | High | Defer; needs explicit searchable-field consumer and leakage docs. |
| Tink Go keyset integration | Medium/high | High | Defer until versioned keyset store/KMS owner is needed. |
| Redis keyset store | Medium | High | Defer; do not store encryption keys in Redis without a protected secret-store contract. |
| AWS KMS envelope support | Medium/high | High | Defer to a future adapter; keep caller-owned AWS SDK clients and encryption context. |
| AWS Encryption SDK | Medium | High | Defer; too broad for the default facade. |
| age stream/file encryption | Medium | Medium/high | Defer until a file/archive or recipient workflow needs it. |
| MAC/digest helpers | Medium | Medium | Document boundaries; no package now. |
| JWT signing/password hashing | Low | High | Out of scope; existing packages and specialized libraries own these. |

## Implement

Create #315 for a focused stdlib-backed package:

- Encrypt/decrypt `[]byte` and UTF-8 `string`.
- Use authenticated encryption only.
- Hide nonce management by using Go 1.26 `cipher.NewGCMWithRandomNonce`.
- Require explicit caller-provided key material or a narrow key provider.
- Accept associated data for tenant/entity/column/message context binding.
- Return typed/sentinel errors for invalid key, malformed envelope,
  authentication failure, wrong key, tamper, and invalid options.
- Use a versioned envelope so later key IDs, deterministic variants, or KMS
  envelope data can be added without changing the initial API.
- Document safe logging: never include plaintext, ciphertext, key bytes, or
  associated data in errors.

## Defer

- Deterministic AEAD until a concrete searchable-field package or database
  example needs it.
- Tink Go until keyset rotation, encrypted keyset storage, or KMS-backed keyset
  protection becomes a first-class feature.
- Redis/Mongo keyset repositories until the key material owner is defined and
  protected storage requirements are explicit.
- AWS KMS envelope encryption until a cloud adapter issue owns data-key
  generation, encryption context, redaction, caching, and retry behavior.
- age until a file/stream/recipient workflow exists.
- MAC/digest helpers unless a later integrity-only API proves repeated caller
  value.

## Follow-up Issue

- #315: implement the stdlib AES-GCM encryption facade.

No additional follow-up issue is created for Tink, age, AWS KMS, deterministic
AEAD, Redis keysets, MAC, or digest in this pass.

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify #71 and #315 issue bodies contain the #71 research outcome.
- Preserve external evidence in `bluetape4k-wiki` and validate with
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`.
- No Go tests are required for this PR because no Go code changes.

## Follow-up Recommendation

Close #71 through this research PR, then implement #315 in a later 0.13.0
delivery slice. Keep the default package dependency-free unless #315 discovers
that the standard-library envelope cannot satisfy the documented tests.
