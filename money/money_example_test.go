package money_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/money"
)

type exampleRateProvider struct {
	quote money.ExchangeRateQuote
}

func (p exampleRateProvider) Rate(ctx context.Context, _ money.Currency, _ money.Currency) (money.ExchangeRateQuote, error) {
	if err := ctx.Err(); err != nil {
		return money.ExchangeRateQuote{}, err
	}
	return p.quote, nil
}

func ExampleNew() {
	price, _ := money.New("12.34", money.USD)
	tax, _ := money.New("0.66", money.USD)
	total, _ := price.Add(tax)

	fmt.Println(total)

	// Output:
	// USD 13.00
}

func ExampleMoney_MarshalJSON() {
	value, _ := money.New("12.34", money.USD)
	data, _ := json.Marshal(value)

	var decoded money.Money
	_ = json.Unmarshal(data, &decoded)

	fmt.Println(string(data))
	fmt.Println(decoded)

	// Output:
	// {"amount":"12.34","currency":"USD"}
	// USD 12.34
}

func ExampleMoney_MarshalText() {
	value, _ := money.New("12.34", money.USD)
	text, _ := value.MarshalText()

	var decoded money.Money
	_ = decoded.UnmarshalText(text)

	fmt.Println(string(text))
	fmt.Println(decoded)

	// Output:
	// USD 12.34
	// USD 12.34
}

func ExampleSum() {
	first, _ := money.New("10.00", money.USD)
	second, _ := money.New("2.50", money.USD)
	total, _ := money.Sum(money.USD, first, second)

	fmt.Println(total)

	// Output:
	// USD 12.50
}

func ExampleMoney_Add_mismatch() {
	usd, _ := money.New("1.00", money.USD)
	krw, _ := money.New("1", money.KRW)
	_, err := usd.Add(krw)

	fmt.Println(errors.Is(err, money.ErrCurrencyMismatch))

	// Output:
	// true
}

func ExampleConvert() {
	rate, _ := money.NewExchangeRate(money.USD, money.KRW, "1300")
	usd, _ := money.New("2.00", money.USD)
	krw, _ := money.Convert(usd, rate)

	fmt.Println(krw)

	// Output:
	// KRW 2600
}

func ExampleConvertWithProvider() {
	rate, _ := money.NewExchangeRate(money.USD, money.KRW, "1300")
	observedAt := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	provider := exampleRateProvider{
		quote: money.ExchangeRateQuote{
			Rate:       rate,
			Source:     money.ECBSource,
			ObservedAt: observedAt,
			FetchedAt:  observedAt.Add(16 * time.Hour),
			ExpiresAt:  observedAt.Add(40 * time.Hour),
		},
	}

	usd, _ := money.New("2.00", money.USD)
	krw, quote, _ := money.ConvertWithProvider(context.Background(), usd, money.KRW, provider)

	fmt.Println(krw)
	fmt.Println(quote.Source)

	// Output:
	// KRW 2600
	// ECB
}
