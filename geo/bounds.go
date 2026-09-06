package geo

// Bounds는 [west, south, east, north] 순서의 inclusive degree 경계다.
type Bounds struct {
	west  float64
	south float64
	east  float64
	north float64
}

// NewBounds는 일반 경계 또는 antimeridian을 가로지르는 경계를 생성한다.
func NewBounds(west, south, east, north float64) (Bounds, error) {
	bounds := Bounds{west: west, south: south, east: east, north: north}
	if err := bounds.Validate(); err != nil {
		return Bounds{}, err
	}
	return bounds, nil
}

// West는 degree 단위 서쪽 경계를 반환한다.
func (b Bounds) West() float64 { return b.west }

// South는 degree 단위 남쪽 경계를 반환한다.
func (b Bounds) South() float64 { return b.south }

// East는 degree 단위 동쪽 경계를 반환한다.
func (b Bounds) East() float64 { return b.east }

// North는 degree 단위 북쪽 경계를 반환한다.
func (b Bounds) North() float64 { return b.north }

// Validate는 좌표 범위와 south/north 순서를 확인한다.
func (b Bounds) Validate() error {
	if !finiteInRange(b.west, -180, 180) {
		return fieldError(ErrInvalidBounds, "west")
	}
	if !finiteInRange(b.south, -90, 90) {
		return fieldError(ErrInvalidBounds, "south")
	}
	if !finiteInRange(b.east, -180, 180) {
		return fieldError(ErrInvalidBounds, "east")
	}
	if !finiteInRange(b.north, -90, 90) {
		return fieldError(ErrInvalidBounds, "north")
	}
	if b.south > b.north {
		return fieldError(ErrInvalidBounds, "south/north ordering")
	}
	return nil
}

// Contains는 point가 inclusive 경계 안에 있는지 반환한다.
func (b Bounds) Contains(point Point) bool {
	if b.Validate() != nil || point.Validate() != nil {
		return false
	}
	return b.containsValidPoint(point)
}

// containsValidPoint는 이미 검증된 Bounds와 Point에만 사용한다.
func (b Bounds) containsValidPoint(point Point) bool {
	if point.latitude < b.south || point.latitude > b.north {
		return false
	}
	if b.containsLongitude(point.longitude) {
		return true
	}
	if point.longitude == -180 {
		return b.containsLongitude(180)
	}
	if point.longitude == 180 {
		return b.containsLongitude(-180)
	}
	return false
}

// CrossesAntimeridian는 east가 west보다 작은 crossing 경계인지 반환한다.
func (b Bounds) CrossesAntimeridian() bool {
	return b.Validate() == nil && b.east < b.west
}

func (b Bounds) containsLongitude(longitude float64) bool {
	if b.east < b.west {
		return longitude >= b.west || longitude <= b.east
	}
	return longitude >= b.west && longitude <= b.east
}
