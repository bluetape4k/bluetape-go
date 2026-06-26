# Encryption Facade Boundaries

For Go encryption work, the default facade should hide nonce management and keep
the common path dependency-free when Go's standard library can satisfy the
contract. With Go 1.26, `cipher.NewGCMWithRandomNonce` is the right starting
point for a byte/string AEAD facade.

Do not generate ephemeral singleton keys for durable ciphertext. Key material
must be caller-owned, persisted, or loaded from an explicit provider. This
matches the exposed Tink lesson that persisted encrypted columns cannot rely on
new generated keysets.

Keep deterministic AEAD, Tink keysets, Redis keyset stores, AWS KMS envelope
support, age file encryption, MAC, and digest helpers out of the default package
until a concrete caller owns their operational risks.
