package money

import (
	"encoding/json"
	"fmt"
)

type moneyJSON struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// MarshalText는 MarshalText 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Money) MarshalText() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return []byte(m.String()), nil
}

// UnmarshalText는 UnmarshalText 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: UnmarshalText가 읽거나 복사하는 text 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m *Money) UnmarshalText(text []byte) error {
	if m == nil {
		return ErrInvalidMoney
	}
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// MarshalJSON는 MarshalJSON 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Money) MarshalJSON() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(moneyJSON{
		Amount:   m.Amount(),
		Currency: m.Currency().Code(),
	})
}

// UnmarshalJSON는 UnmarshalJSON 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - data: UnmarshalJSON가 읽거나 복사하는 data 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m *Money) UnmarshalJSON(data []byte) error {
	if m == nil {
		return ErrInvalidMoney
	}
	var payload moneyJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidMoney, err)
	}
	if payload.Amount == "" || payload.Currency == "" {
		return fmt.Errorf("%w: missing amount or currency", ErrInvalidMoney)
	}
	curr, err := ParseCurrency(payload.Currency)
	if err != nil {
		return err
	}
	parsed, err := New(payload.Amount, curr)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
