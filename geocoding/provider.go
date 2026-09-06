package geocoding

import (
	"context"

	"github.com/bluetape4k/bluetape-go/geo"
)

// Provider 는 좌표 하나를 주소 결과로 변환하는 최소 reverse geocoding 계약이다.
type Provider interface {
	Reverse(context.Context, geo.Point, Options) (Result, error)
}

// Result 는 provider 응답에서 caller에게 유용한 제한된 주소 정보를 보관한다.
type Result struct {
	// PlaceID는 provider의 안정적인 장소 식별자다.
	PlaceID int64
	// DisplayName은 provider가 반환한 표시용 주소다.
	DisplayName string
	// Latitude는 provider가 정규화해 반환한 위도다.
	Latitude float64
	// Longitude는 provider가 정규화해 반환한 경도다.
	Longitude float64
	// Address는 주소 구성 요소의 defensive copy다.
	Address map[string]string
	// Attribution은 IncludeAttribution을 선택했을 때만 채워진 licence 문자열이다.
	Attribution string
}

func (r Result) clone() Result {
	r.Address = cloneAddress(r.Address)
	return r
}

func cloneAddress(address map[string]string) map[string]string {
	if address == nil {
		return nil
	}
	clone := make(map[string]string, len(address))
	for key, value := range address {
		clone[key] = value
	}
	return clone
}
