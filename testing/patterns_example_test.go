package bttesting_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

type orderFixture struct {
	ID        string      `json:"id"`
	Customer  string      `json:"customer"`
	CreatedAt time.Time   `json:"created_at"`
	Lines     []orderLine `json:"lines"`
}

type orderLine struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type orderBuilder struct {
	order orderFixture
}

func newOrderBuilder() *orderBuilder {
	return &orderBuilder{
		order: orderFixture{
			ID:        "order-001",
			Customer:  "customer-001",
			CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
			Lines: []orderLine{
				{SKU: "sku-001", Quantity: 1},
			},
		},
	}
}

func (b *orderBuilder) withID(id string) *orderBuilder {
	b.order.ID = id
	return b
}

func (b *orderBuilder) withCustomer(customer string) *orderBuilder {
	b.order.Customer = customer
	return b
}

func (b *orderBuilder) withLine(sku string, quantity int) *orderBuilder {
	b.order.Lines = append([]orderLine(nil), b.order.Lines...)
	b.order.Lines = append(b.order.Lines, orderLine{SKU: sku, Quantity: quantity})
	return b
}

func (b *orderBuilder) build() orderFixture {
	order := b.order
	order.Lines = append([]orderLine(nil), b.order.Lines...)
	return order
}

func TestTableDrivenBuilderWithCmp(t *testing.T) {
	tests := []struct {
		name string
		give *orderBuilder
		want orderFixture
	}{
		{
			name: "default order",
			give: newOrderBuilder(),
			want: orderFixture{
				ID:        "order-001",
				Customer:  "customer-001",
				CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
				Lines:     []orderLine{{SKU: "sku-001", Quantity: 1}},
			},
		},
		{
			name: "custom customer with extra line",
			give: newOrderBuilder().withCustomer("customer-002").withLine("sku-002", 3),
			want: orderFixture{
				ID:        "order-001",
				Customer:  "customer-002",
				CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
				Lines: []orderLine{
					{SKU: "sku-001", Quantity: 1},
					{SKU: "sku-002", Quantity: 3},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.give.build()); diff != "" {
				t.Fatalf("order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGoldenFixtureWithTempOutputPath(t *testing.T) {
	order := newOrderBuilder().withID("order-golden").withLine("sku-002", 2).build()

	got, err := renderOrder(order)
	if err != nil {
		t.Fatalf("render order: %v", err)
	}

	golden, err := os.ReadFile(filepath.Join("testdata", "order.golden.json"))
	if err != nil {
		t.Fatalf("read golden fixture: %v", err)
	}
	if diff := cmp.Diff(string(golden), got); diff != "" {
		t.Fatalf("golden mismatch (-want +got):\n%s", diff)
	}

	outputPath := bttesting.TempOutputPath(t, "golden", "order.json")
	if err := os.WriteFile(outputPath, []byte(got), 0o600); err != nil {
		t.Fatalf("write temp output: %v", err)
	}
}

func TestDeterministicRandomData(t *testing.T) {
	rng := rand.New(rand.NewPCG(222, 606))

	got := deterministicLines(rng, 3)
	want := []orderLine{
		{SKU: "sku-300", Quantity: 1},
		{SKU: "sku-841", Quantity: 1},
		{SKU: "sku-581", Quantity: 3},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("deterministic random lines mismatch (-want +got):\n%s", diff)
	}
}

func TestCancellationAssertionExample(t *testing.T) {
	bttesting.RequireContextCanceled(t, func(ctx context.Context) error {
		return saveOrder(ctx, newOrderBuilder().build())
	})

	bttesting.RequireCleanupOnCancel(t, 50*time.Millisecond, func(ctx context.Context, ready func(), cleaned func()) error {
		ready()
		<-ctx.Done()
		cleaned()
		return ctx.Err()
	})
}

func Example_focusedFixtureBuilder() {
	order := newOrderBuilder().
		withID("order-doc").
		withCustomer("customer-doc").
		withLine("sku-doc", 2).
		build()

	fmt.Println(order.ID, order.Customer, len(order.Lines))

	// Output:
	// order-doc customer-doc 2
}

func renderOrder(order orderFixture) (string, error) {
	payload, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}

func deterministicLines(rng *rand.Rand, count int) []orderLine {
	lines := make([]orderLine, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, orderLine{
			SKU:      fmt.Sprintf("sku-%03d", rng.IntN(1000)),
			Quantity: rng.IntN(3) + 1,
		})
	}
	return lines
}

func saveOrder(ctx context.Context, order orderFixture) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if order.ID == "" {
		return errors.New("order ID must not be empty")
	}
	return nil
}
