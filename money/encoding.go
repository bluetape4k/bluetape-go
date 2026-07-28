package money

import (
	"encoding/json"
	"fmt"
)

type moneyJSON struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// MarshalText 값을 text 표현으로 직렬화한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Money) MarshalText() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return []byte(m.String()), nil
}

// UnmarshalText 직렬화된 표현을 현재 값으로 복원한다.
//
// 매개변수:
//   - text: UnmarshalText가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MarshalJSON 값을 JSON 표현으로 직렬화한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Money) MarshalJSON() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(moneyJSON{
		Amount:   m.Amount(),
		Currency: m.Currency().Code(),
	})
}

// UnmarshalJSON 직렬화된 표현을 현재 값으로 복원한다.
//
// 매개변수:
//   - data: UnmarshalJSON가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
