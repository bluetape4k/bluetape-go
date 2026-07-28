package id

// StringGenerator 패키지에서 공개하는 인터페이스다.
type StringGenerator interface {
	NextString() (string, error)
}

// Int64Generator 패키지에서 공개하는 인터페이스다.
type Int64Generator interface {
	NextInt64() (int64, error)
}
