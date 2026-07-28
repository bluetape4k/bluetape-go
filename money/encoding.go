package money

import (
	"encoding/json"
	"fmt"
)

type moneyJSON struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// MarshalText Money 를 `USD 12.34` 형식으로 직렬화합니다.
func (m Money) MarshalText() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return []byte(m.String()), nil
}

// UnmarshalText `USD 12.34` 형식의 텍스트를 Money 로 역직렬화합니다.
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

// MarshalJSON 은 Money 를 명시적 amount/currency object로 직렬화합니다.
func (m Money) MarshalJSON() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(moneyJSON{
		Amount:   m.Amount(),
		Currency: m.Currency().Code(),
	})
}

// UnmarshalJSON 은 명시적 amount/currency object를 Money 로 역직렬화합니다.
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
