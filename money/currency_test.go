package money

import (
	"errors"
	"testing"
)

func TestParseCurrency(t *testing.T) {
	tests := []struct {
		name string
		code string
		want Currency
	}{
		{name: "upper", code: "USD", want: USD},
		{name: "lower", code: "krw", want: KRW},
		{name: "numeric", code: "392", want: JPY},
		{name: "trimmed", code: " EUR ", want: EUR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCurrency(tt.code)
			if err != nil {
				t.Fatalf("ParseCurrency(%q) failed: %v", tt.code, err)
			}
			if got != tt.want {
				t.Fatalf("ParseCurrency(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestParseCurrencyRejectsNoCurrency(t *testing.T) {
	for _, code := range []string{"", "XXX", "xxx", "999", "not-a-currency"} {
		t.Run(code, func(t *testing.T) {
			_, err := ParseCurrency(code)
			if !errors.Is(err, ErrInvalidCurrency) {
				t.Fatalf("expected ErrInvalidCurrency for %q, got %v", code, err)
			}
		})
	}
}

func TestMustParseCurrency(t *testing.T) {
	if MustParseCurrency("USD") != USD {
		t.Fatalf("MustParseCurrency did not return USD")
	}

	defer func() {
		if recover() == nil {
			t.Fatalf("MustParseCurrency did not panic for invalid code")
		}
	}()
	_ = MustParseCurrency("XXX")
}

func TestIsCurrency(t *testing.T) {
	if !IsCurrency("USD") {
		t.Fatalf("USD should be valid")
	}
	if IsCurrency("XXX") {
		t.Fatalf("XXX should be rejected as no-currency")
	}
	if IsCurrency("bogus") {
		t.Fatalf("bogus should be invalid")
	}
}

func TestCurrencyByLocale(t *testing.T) {
	tests := []struct {
		tag  string
		want Currency
	}{
		{tag: "ko-KR", want: KRW},
		{tag: "en_US", want: USD},
		{tag: "ko_KR", want: KRW},
		{tag: "en-us", want: USD},
		{tag: "ja-JP", want: JPY},
		{tag: "zh-CN", want: CNY},
		{tag: "de-DE", want: EUR},
		{tag: "fr_FR", want: EUR},
		{tag: "it-IT", want: EUR},
		{tag: "es-ES", want: EUR},
		{tag: "nl-NL", want: EUR},
		{tag: "pt-PT", want: EUR},
		{tag: "fi-FI", want: EUR},
		{tag: "ie-IE", want: EUR},
		{tag: "at-AT", want: EUR},
		{tag: "be-BE", want: EUR},
		{tag: "en-GB", want: MustParseCurrency("GBP")},
		{tag: "fr-CA", want: MustParseCurrency("CAD")},
		{tag: "en-AU", want: MustParseCurrency("AUD")},
		{tag: "pt-BR", want: MustParseCurrency("BRL")},
		{tag: "hi-IN", want: MustParseCurrency("INR")},
		{tag: "es-MX", want: MustParseCurrency("MXN")},
		{tag: "zh-Hant-TW", want: MustParseCurrency("TWD")},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, err := CurrencyByLocale(tt.tag)
			if err != nil {
				t.Fatalf("CurrencyByLocale(%q) failed: %v", tt.tag, err)
			}
			if got != tt.want {
				t.Fatalf("CurrencyByLocale(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestCurrencyByLocaleRejectsUnsupportedTags(t *testing.T) {
	for _, tag := range []string{
		"ko",
		"",
		"und",
		"en-001",
		"en-QM",
		"en-AQ",
		"es-PA",
		"en-u-cu-usd",
	} {
		t.Run(tag, func(t *testing.T) {
			_, err := CurrencyByLocale(tag)
			if !errors.Is(err, ErrInvalidCurrency) {
				t.Fatalf("expected ErrInvalidCurrency for %q, got %v", tag, err)
			}
		})
	}
}

func TestCurrencyMethods(t *testing.T) {
	if USD.Code() != "USD" {
		t.Fatalf("USD.Code() = %q", USD.Code())
	}
	if USD.Num() != "840" {
		t.Fatalf("USD.Num() = %q", USD.Num())
	}
	if USD.Scale() != 2 {
		t.Fatalf("USD.Scale() = %d", USD.Scale())
	}
	if (Currency{}).String() != "" || !(Currency{}).IsZero() {
		t.Fatalf("zero currency should be empty and zero")
	}
}
