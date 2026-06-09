package id

// StringGenerator produces string identifiers.
type StringGenerator interface {
	NextString() (string, error)
}

// Int64Generator produces int64 identifiers.
type Int64Generator interface {
	NextInt64() (int64, error)
}
