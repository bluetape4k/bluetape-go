package money

import (
	"encoding/json"
	"testing"

	gmoney "github.com/govalues/money"
)

func BenchmarkMoneyNewMinorUSD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := NewMinor(12345, USD)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid money")
		}
	}
}

func BenchmarkMoneyNewMinorJPY(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := NewMinor(12345, JPY)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid money")
		}
	}
}

func BenchmarkMoneyMinorUnitsUSD(b *testing.B) {
	value, err := New("123.45", USD)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		units, err := value.MinorUnits()
		if err != nil {
			b.Fatal(err)
		}
		if units != 12345 {
			b.Fatalf("units = %d", units)
		}
	}
}

func BenchmarkMoneyAddUSD(b *testing.B) {
	left, err := New("123.45", USD)
	if err != nil {
		b.Fatal(err)
	}
	right, err := New("67.89", USD)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := left.Add(right)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid sum")
		}
	}
}

func BenchmarkMoneySumUSD10(b *testing.B) {
	values := make([]Money, 10)
	for i := range values {
		value, err := NewMinor(int64(1000+i), USD)
		if err != nil {
			b.Fatal(err)
		}
		values[i] = value
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := Sum(USD, values...)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid total")
		}
	}
}

func BenchmarkMoneyParseUSD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := Parse("USD 123.45")
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid money")
		}
	}
}

func BenchmarkMoneyMarshalJSON(b *testing.B) {
	value, err := New("123.45", USD)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload, err := json.Marshal(value)
		if err != nil {
			b.Fatal(err)
		}
		if len(payload) == 0 {
			b.Fatal("expected payload")
		}
	}
}

func BenchmarkMoneyDirectGovaluesNewAmountFromMinorUnits(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := gmoney.NewAmountFromMinorUnits("USD", 12345)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid amount")
		}
	}
}
