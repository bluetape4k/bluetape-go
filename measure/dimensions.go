package measure

// Length  길이 차원입니다.
type Length struct{}

// Time  시간 차원입니다.
type Time struct{}

// Mass  질량 차원입니다.
type Mass struct{}

// Area  면적 차원입니다.
type Area struct{}

// Volume  부피 차원입니다.
type Volume struct{}

// Storage  1024 배율 저장 용량 차원입니다.
type Storage struct{}

// BinarySize  1000/1024 배율과 bit 단위를 함께 다루는 binary size 차원입니다.
type BinarySize struct{}

// Frequency  주파수 차원입니다.
type Frequency struct{}

// Energy  에너지 차원입니다.
type Energy struct{}

// Power  전력 차원입니다.
type Power struct{}

// Pressure  압력 차원입니다.
type Pressure struct{}

// Angle  각도 차원입니다.
type Angle struct{}

// GraphicsLength  픽셀 기반 그래픽 길이 차원입니다.
type GraphicsLength struct{}

// Product  두 차원의 곱 차원입니다.
type Product[A, B any] struct{}

// Ratio  두 차원의 비율 차원입니다.
type Ratio[A, B any] struct{}

// Inverse  한 차원의 역수 차원입니다.
type Inverse[D any] struct{}

// Velocity  길이/시간 차원입니다.
type Velocity = Ratio[Length, Time]

// Acceleration  길이/시간 제곱 차원입니다.
type Acceleration = Ratio[Length, Product[Time, Time]]
