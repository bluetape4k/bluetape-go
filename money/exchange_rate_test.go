package money

import (
	"errors"
	"testing"
)

func TestExchangeRateConvert(t *testing.T) {
	rate, err := NewExchangeRate(USD, KRW, "1300")
	if err != nil {
		t.Fatalf("NewExchangeRate failed: %v", err)
	}
	usd, _ := New("2.00", USD)
	krw, err := Convert(usd, rate)
	if err != nil {
		t.Fatalf("Convert direct failed: %v", err)
	}
	if krw.String() != "KRW 2600" {
		t.Fatalf("direct conversion = %q", krw.String())
	}

	back, err := Convert(krw, rate)
	if err != nil {
		t.Fatalf("Convert reverse failed: %v", err)
	}
	if back.String() != "USD 2.00" {
		t.Fatalf("reverse conversion = %q", back.String())
	}
}

func TestExchangeRateValidation(t *testing.T) {
	same, err := NewExchangeRate(USD, USD, "1")
	if err != nil {
		t.Fatalf("same-currency rate 1 should be valid: %v", err)
	}
	if !same.Valid() || same.Base() != USD || same.Quote() != USD || same.Rate() != "1.00" {
		t.Fatalf("unexpected same-currency rate: valid=%v base=%v quote=%v rate=%q", same.Valid(), same.Base(), same.Quote(), same.Rate())
	}
	if _, err := NewExchangeRate(USD, KRW, "0"); !errors.Is(err, ErrInvalidExchangeRate) {
		t.Fatalf("expected zero invalid rate, got %v", err)
	}
	if _, err := NewExchangeRate(USD, KRW, "-1"); !errors.Is(err, ErrInvalidExchangeRate) {
		t.Fatalf("expected negative invalid rate, got %v", err)
	}
	if _, err := NewExchangeRate(USD, USD, "2"); !errors.Is(err, ErrInvalidExchangeRate) {
		t.Fatalf("expected same-currency non-1 invalid rate, got %v", err)
	}
	if _, err := NewExchangeRate(USD, KRW, "not-a-rate"); !errors.Is(err, ErrInvalidExchangeRate) {
		t.Fatalf("expected malformed invalid rate, got %v", err)
	}
	if _, err := NewExchangeRate(Currency{}, USD, "1"); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid currency, got %v", err)
	}
}

func TestExchangeRateZeroValueAndMismatch(t *testing.T) {
	usd, _ := New("1.00", USD)
	if _, err := Convert(usd, ExchangeRate{}); !errors.Is(err, ErrInvalidExchangeRate) {
		t.Fatalf("expected invalid exchange rate, got %v", err)
	}

	rate, _ := NewExchangeRate(USD, KRW, "1300")
	eur, _ := New("1.00", EUR)
	if _, err := Convert(eur, rate); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
}

func TestExchangeRateMethods(t *testing.T) {
	rate, err := NewExchangeRate(USD, KRW, "1300")
	if err != nil {
		t.Fatalf("NewExchangeRate failed: %v", err)
	}
	if !rate.Valid() || rate.IsZero() {
		t.Fatalf("rate should be valid")
	}
	if rate.Base() != USD || rate.Quote() != KRW || rate.Rate() != "1300" {
		t.Fatalf("unexpected rate metadata: %v %v %q", rate.Base(), rate.Quote(), rate.Rate())
	}
}

func TestExchangeRateConvertOverflow(t *testing.T) {
	rate, err := NewExchangeRate(USD, JPY, "9999999999999999999")
	if err != nil {
		t.Fatalf("NewExchangeRate failed: %v", err)
	}
	usd, _ := New("2.00", USD)
	_, err = Convert(usd, rate)
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected ErrOverflow, got %v", err)
	}
}
