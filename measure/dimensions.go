package measure

// Length 패키지에서 공개하는 구조체다.
type Length struct{}

// Time 패키지에서 공개하는 구조체다.
type Time struct{}

// Mass 패키지에서 공개하는 구조체다.
type Mass struct{}

// Area 패키지에서 공개하는 구조체다.
type Area struct{}

// Volume 패키지에서 공개하는 구조체다.
type Volume struct{}

// Storage 패키지에서 공개하는 구조체다.
type Storage struct{}

// BinarySize 패키지에서 공개하는 구조체다.
type BinarySize struct{}

// Frequency 패키지에서 공개하는 구조체다.
type Frequency struct{}

// Energy 패키지에서 공개하는 구조체다.
type Energy struct{}

// Power 패키지에서 공개하는 구조체다.
type Power struct{}

// Pressure 패키지에서 공개하는 구조체다.
type Pressure struct{}

// Angle 패키지에서 공개하는 구조체다.
type Angle struct{}

// GraphicsLength 패키지에서 공개하는 구조체다.
type GraphicsLength struct{}

// Product 패키지에서 공개하는 구조체다.
type Product[A, B any] struct{}

// Ratio 패키지에서 공개하는 구조체다.
type Ratio[A, B any] struct{}

// Inverse 패키지에서 공개하는 구조체다.
type Inverse[D any] struct{}

// Velocity  길이/시간 차원입니다.
type Velocity = Ratio[Length, Time]

// Acceleration  길이/시간 제곱 차원입니다.
type Acceleration = Ratio[Length, Product[Time, Time]]
