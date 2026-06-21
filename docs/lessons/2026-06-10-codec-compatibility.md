# Codec Compatibility

Issue #187 showed that codec parity needs a matrix, not a single "compatible"
claim. Base58 can match the Kotlin alphabet and leading-zero behavior directly,
but Base62 and URL62 cross a boundary: Kotlin helpers are numeric
`BigInteger`/UUID-oriented while bluetape-go exposes a byte API.

For future codec work:

- Lock upstream vectors in tests before changing docs.
- Document Go-specific byte behavior when Kotlin normalizes numeric values.
- Treat empty input, blank whitespace, high-order zero bytes, and bit-limit
  checks as separate compatibility rows.
- Add bounded goroutine stress when package-level immutable encoders are shared
  across public helpers, then run the same target under `-race`.
- Keep URL62 as a Base62 alias unless a real UUID API is intentionally added.
