package money

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMarshalJSON(t *testing.T) {
	value, _ := New("12.34", USD)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(data) != `{"amount":"12.34","currency":"USD"}` {
		t.Fatalf("MarshalJSON = %s", data)
	}
}

func TestUnmarshalJSONPopulatesZeroValueDestination(t *testing.T) {
	var value Money
	if err := json.Unmarshal([]byte(`{"amount":"12.34","currency":"USD"}`), &value); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}
	if value.String() != "USD 12.34" {
		t.Fatalf("UnmarshalJSON = %q", value.String())
	}
}

func TestUnmarshalJSONRejectsInvalidInput(t *testing.T) {
	tests := []string{
		`{"amount":"","currency":"USD"}`,
		`{"amount":"12.34","currency":"XXX"}`,
		`{"amount":"bad","currency":"USD"}`,
		`[]`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			var value Money
			if err := json.Unmarshal([]byte(input), &value); err == nil ||
				(!errors.Is(err, ErrInvalidMoney) &&
					!errors.Is(err, ErrInvalidCurrency) &&
					!errors.Is(err, ErrInvalidAmount)) {
				t.Fatalf("expected typed error for %s, got %v", input, err)
			}
		})
	}
}

func TestMarshalRejectsInvalidMoney(t *testing.T) {
	_, err := json.Marshal(Money{})
	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected ErrInvalidMoney, got %v", err)
	}
}

func TestNilUnmarshalReceivers(t *testing.T) {
	var value *Money
	if err := value.UnmarshalJSON([]byte(`{"amount":"1.00","currency":"USD"}`)); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected ErrInvalidMoney for nil JSON receiver, got %v", err)
	}
	if err := value.UnmarshalText([]byte("USD 1.00")); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected ErrInvalidMoney for nil text receiver, got %v", err)
	}
}

func TestTextMarshalRoundTrip(t *testing.T) {
	value, _ := New("12.34", USD)
	text, err := value.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}
	if string(text) != "USD 12.34" {
		t.Fatalf("MarshalText = %q", text)
	}
	var parsed Money
	if err := parsed.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText failed: %v", err)
	}
	if !parsed.Equal(value) {
		t.Fatalf("round trip mismatch: %v vs %v", parsed, value)
	}
}

func TestParseRejectsAmbiguousOrOversizedInput(t *testing.T) {
	if _, err := Parse(""); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected invalid empty parse, got %v", err)
	}
	oversized := "USD " + strings.Repeat("9", 128)
	if _, err := Parse(oversized); err == nil {
		t.Fatalf("expected oversized parse to fail")
	}
	var value Money
	input := `{"amount":"` + strings.Repeat("9", 128) + `","currency":"USD"}`
	if err := json.Unmarshal([]byte(input), &value); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected ErrOverflow for oversized unmarshal, got %v", err)
	}
}
