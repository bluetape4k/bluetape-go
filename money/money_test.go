package money

import (
	"errors"
	"math"
	"testing"
)

func TestNewConstructors(t *testing.T) {
	usd, err := New("12.34", USD)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if usd.String() != "USD 12.34" || usd.Amount() != "12.34" {
		t.Fatalf("unexpected USD amount: %q %q", usd.String(), usd.Amount())
	}

	major, err := NewFromInt64(12, USD)
	if err != nil {
		t.Fatalf("NewFromInt64 failed: %v", err)
	}
	if major.String() != "USD 12.00" {
		t.Fatalf("NewFromInt64 = %q", major.String())
	}

	minor, err := NewMinor(12, USD)
	if err != nil {
		t.Fatalf("NewMinor failed: %v", err)
	}
	if minor.String() != "USD 0.12" {
		t.Fatalf("NewMinor USD = %q", minor.String())
	}

	jpyMinor, err := NewMinor(12, JPY)
	if err != nil {
		t.Fatalf("NewMinor JPY failed: %v", err)
	}
	if jpyMinor.String() != "JPY 12" {
		t.Fatalf("NewMinor JPY = %q", jpyMinor.String())
	}

	zero, err := Zero(KRW)
	if err != nil {
		t.Fatalf("Zero failed: %v", err)
	}
	if zero.String() != "KRW 0" {
		t.Fatalf("Zero = %q", zero.String())
	}
}

func TestConstructorsRejectInvalidInput(t *testing.T) {
	if _, err := New("1", Currency{}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid currency, got %v", err)
	}
	if _, err := New("abc", USD); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected invalid amount, got %v", err)
	}
	if _, err := NewFromFloat64(math.NaN(), USD); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected invalid NaN amount, got %v", err)
	}
	if _, err := NewFromFloat64(math.Inf(1), USD); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected invalid Inf amount, got %v", err)
	}
}

func TestParseAndValueMethods(t *testing.T) {
	parsed, err := Parse("USD 12.34")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Currency() != USD {
		t.Fatalf("currency = %v", parsed.Currency())
	}
	units, err := parsed.MinorUnits()
	if err != nil {
		t.Fatalf("MinorUnits failed: %v", err)
	}
	if units != 1234 {
		t.Fatalf("MinorUnits = %d", units)
	}
	f, err := parsed.Float64()
	if err != nil {
		t.Fatalf("Float64 failed: %v", err)
	}
	if f != 12.34 {
		t.Fatalf("Float64 = %v", f)
	}
}

func TestMinorUnitsRoundsToCurrencyScale(t *testing.T) {
	value, err := New("1.234", USD)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	units, err := value.MinorUnits()
	if err != nil {
		t.Fatalf("MinorUnits failed: %v", err)
	}
	if units != 123 {
		t.Fatalf("MinorUnits = %d", units)
	}
}

func TestMinorUnitsOverflow(t *testing.T) {
	value, err := New("9999999999999999999", JPY)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	_, err = value.MinorUnits()
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected ErrOverflow, got %v", err)
	}
}

func TestArithmetic(t *testing.T) {
	left, _ := New("10.00", USD)
	right, _ := New("2.50", USD)

	sum, err := left.Add(right)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if sum.String() != "USD 12.50" {
		t.Fatalf("Add = %q", sum.String())
	}

	diff, err := left.Sub(right)
	if err != nil {
		t.Fatalf("Sub failed: %v", err)
	}
	if diff.String() != "USD 7.50" {
		t.Fatalf("Sub = %q", diff.String())
	}

	product, err := right.Mul("3")
	if err != nil {
		t.Fatalf("Mul failed: %v", err)
	}
	if product.String() != "USD 7.50" {
		t.Fatalf("Mul = %q", product.String())
	}

	quotient, err := left.Quo("4")
	if err != nil {
		t.Fatalf("Quo failed: %v", err)
	}
	if quotient.String() != "USD 2.50" {
		t.Fatalf("Quo = %q", quotient.String())
	}
}

func TestArithmeticErrors(t *testing.T) {
	usd, _ := New("1.00", USD)
	krw, _ := New("1", KRW)
	if _, err := usd.Add(krw); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
	if _, err := usd.Mul("bad"); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected invalid scalar, got %v", err)
	}
	if _, err := usd.Mul("999999999999999999999"); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected scalar overflow, got %v", err)
	}
	if _, err := usd.Quo("0"); !errors.Is(err, ErrDivideByZero) {
		t.Fatalf("expected divide by zero, got %v", err)
	}
	if _, err := (Money{}).Add(usd); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected invalid money, got %v", err)
	}
	large, _ := New("99999999999999999.99", USD)
	if _, err := large.Mul("10"); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected multiplication overflow, got %v", err)
	}
	if _, err := large.Quo("0.000000000000000001"); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected quotient overflow, got %v", err)
	}
}

func TestRoundAndUnaryOperations(t *testing.T) {
	value, _ := New("1.245", USD)
	rounded, err := value.Round()
	if err != nil {
		t.Fatalf("Round failed: %v", err)
	}
	if rounded.String() != "USD 1.24" {
		t.Fatalf("Round = %q", rounded.String())
	}

	roundTo, err := value.RoundTo(1)
	if err != nil {
		t.Fatalf("RoundTo failed: %v", err)
	}
	if roundTo.String() != "USD 1.20" {
		t.Fatalf("RoundTo = %q", roundTo.String())
	}

	negative, _ := New("-2.50", USD)
	negated, err := negative.Neg()
	if err != nil {
		t.Fatalf("Neg failed: %v", err)
	}
	if negated.String() != "USD 2.50" {
		t.Fatalf("Neg = %q", negated.String())
	}
	absolute, err := negative.Abs()
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}
	if !absolute.Equal(negated) {
		t.Fatalf("Abs should equal Neg")
	}
}

func TestCompareAndSum(t *testing.T) {
	first, _ := New("1.00", USD)
	second, _ := New("2.00", USD)
	cmp, err := first.Cmp(second)
	if err != nil {
		t.Fatalf("Cmp failed: %v", err)
	}
	if cmp >= 0 {
		t.Fatalf("Cmp = %d", cmp)
	}
	if first.Equal(second) {
		t.Fatalf("different values should not be equal")
	}

	total, err := Sum(USD, first, second)
	if err != nil {
		t.Fatalf("Sum failed: %v", err)
	}
	if total.String() != "USD 3.00" {
		t.Fatalf("Sum = %q", total.String())
	}
	empty, err := Sum(USD)
	if err != nil {
		t.Fatalf("empty Sum failed: %v", err)
	}
	if empty.String() != "USD 0.00" {
		t.Fatalf("empty Sum = %q", empty.String())
	}
	krw, _ := New("1", KRW)
	if _, err := Sum(USD, first, krw); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
	if _, err := Sum(USD, Money{}); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected invalid money, got %v", err)
	}
	if _, err := Sum(Currency{}, first); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid currency, got %v", err)
	}
}
