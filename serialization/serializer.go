package serialization

// Serializer 패키지에서 공개하는 인터페이스다.
type Serializer[T any] interface {
	Marshal(value T) ([]byte, error)
	Unmarshal(data []byte) (T, error)
}

// NamedSerializer 패키지에서 공개하는 인터페이스다.
type NamedSerializer[T any] interface {
	Serializer[T]
	Format() string
}
