package serialization

// Serializer marshals values into bytes and unmarshals bytes back into values.
type Serializer[T any] interface {
	Marshal(value T) ([]byte, error)
	Unmarshal(data []byte) (T, error)
}

// NamedSerializer exposes stable format metadata for persisted payloads.
type NamedSerializer[T any] interface {
	Serializer[T]
	Format() string
}
