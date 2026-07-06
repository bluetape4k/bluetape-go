package serialization_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/compression"
	"github.com/bluetape4k/bluetape-go/serialization"
)

type benchmarkAccount struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	Active       bool              `json:"active"`
	BalanceMinor int64             `json:"balance_minor"`
	Currency     string            `json:"currency"`
	CreatedAt    string            `json:"created_at"`
	Attributes   map[string]string `json:"attributes"`
}

type benchmarkCustomer struct {
	ID        string `json:"id"`
	Tier      string `json:"tier"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type benchmarkAddress struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type benchmarkLineItem struct {
	SKU             string            `json:"sku"`
	Quantity        int               `json:"quantity"`
	UnitMinor       int64             `json:"unit_minor"`
	Currency        string            `json:"currency"`
	Description     string            `json:"description"`
	FulfillmentTags []string          `json:"fulfillment_tags"`
	Attributes      map[string]string `json:"attributes"`
}

type benchmarkPayment struct {
	Method        string `json:"method"`
	Authorized    bool   `json:"authorized"`
	SubtotalText  string `json:"subtotal"`
	SubtotalMinor int64  `json:"subtotal_minor"`
	TaxText       string `json:"tax"`
	TaxMinor      int64  `json:"tax_minor"`
	TotalText     string `json:"total"`
	TotalMinor    int64  `json:"total_minor"`
}

type benchmarkAuditEntry struct {
	Sequence  int    `json:"sequence"`
	TenantID  string `json:"tenant_id"`
	Action    string `json:"action"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

type benchmarkCoupon struct {
	Code       string `json:"code"`
	ValueText  string `json:"value"`
	ValueMinor int64  `json:"value_minor"`
}

type benchmarkRefund struct {
	Reason string `json:"reason"`
	Amount string `json:"amount"`
}

type benchmarkOrder struct {
	ID              string                `json:"id"`
	Status          string                `json:"status"`
	Customer        benchmarkCustomer     `json:"customer"`
	ShippingAddress benchmarkAddress      `json:"shipping_address"`
	LineItems       []benchmarkLineItem   `json:"line_items"`
	Payment         benchmarkPayment      `json:"payment"`
	Audit           []benchmarkAuditEntry `json:"audit"`
	Coupon          *benchmarkCoupon      `json:"coupon,omitempty"`
	Refund          *benchmarkRefund      `json:"refund"`
	CreatedAt       string                `json:"created_at"`
	UpdatedAt       string                `json:"updated_at"`
}

type benchmarkEvent struct {
	Sequence  int               `json:"sequence"`
	TenantID  string            `json:"tenant_id"`
	Kind      string            `json:"kind"`
	CreatedAt string            `json:"created_at"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata"`
}

type serializationBenchmarkCase[T any] struct {
	name         string
	serializer   serialization.Serializer[T]
	value        T
	equal        func(T) bool
	fixtureBytes int
}

func BenchmarkSerializationEncode(b *testing.B) {
	runSerializationEncodeBenchmark(b, smallAccountCase())
	runSerializationEncodeBenchmark(b, mediumOrderCase())
	runSerializationEncodeBenchmark(b, repeatedEventsCase())
	runSerializationEncodeBenchmark(b, versionedSmallAccountCase(b))
	runSerializationEncodeBenchmark(b, binaryPayloadCase())
	runSerializationEncodeBenchmark(b, textPayloadCase())
}

func BenchmarkSerializationDecode(b *testing.B) {
	runSerializationDecodeBenchmark(b, smallAccountCase())
	runSerializationDecodeBenchmark(b, mediumOrderCase())
	runSerializationDecodeBenchmark(b, repeatedEventsCase())
	runSerializationDecodeBenchmark(b, versionedSmallAccountCase(b))
	runSerializationDecodeBenchmark(b, binaryPayloadCase())
	runSerializationDecodeBenchmark(b, textPayloadCase())
}

func BenchmarkSerializationRoundTrip(b *testing.B) {
	runSerializationRoundTripBenchmark(b, smallAccountCase())
	runSerializationRoundTripBenchmark(b, mediumOrderCase())
	runSerializationRoundTripBenchmark(b, repeatedEventsCase())
	runSerializationRoundTripBenchmark(b, versionedSmallAccountCase(b))
	runSerializationRoundTripBenchmark(b, binaryPayloadCase())
	runSerializationRoundTripBenchmark(b, textPayloadCase())
}

func BenchmarkSerializationSerializeThenCompress(b *testing.B) {
	for _, serialized := range serializationCompressionPayloads(b) {
		serialized := serialized
		b.Run(serialized.name, func(b *testing.B) {
			for _, compressor := range compression.All() {
				compressor := compressor
				b.Run(compressor.Name(), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(serialized.fixtureBytes))
					b.ResetTimer()

					var compressed []byte
					for range b.N {
						var err error
						compressed, err = compressor.Compress(serialized.data)
						if err != nil {
							b.Fatalf("Compress failed: %v", err)
						}
					}

					b.StopTimer()
					b.ReportMetric(float64(len(serialized.data)), "serialized_bytes")
					b.ReportMetric(float64(len(compressed)), "compressed_bytes")
					b.ReportMetric(float64(len(compressed))/float64(len(serialized.data)), "compressed/serialized")
				})
			}
		})
	}
}

func runSerializationEncodeBenchmark[T any](b *testing.B, tc serializationBenchmarkCase[T]) {
	b.Run(tc.name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(tc.fixtureBytes))
		b.ResetTimer()

		var encoded []byte
		for range b.N {
			var err error
			encoded, err = tc.serializer.Marshal(tc.value)
			if err != nil {
				b.Fatalf("Marshal failed: %v", err)
			}
		}

		b.StopTimer()
		if len(encoded) == 0 {
			b.Fatal("Marshal returned empty payload")
		}
		b.ReportMetric(float64(len(encoded)), "encoded_bytes")
	})
}

func runSerializationDecodeBenchmark[T any](b *testing.B, tc serializationBenchmarkCase[T]) {
	encoded, err := tc.serializer.Marshal(tc.value)
	if err != nil {
		b.Fatalf("setup Marshal failed for %s: %v", tc.name, err)
	}
	decoded, err := tc.serializer.Unmarshal(encoded)
	if err != nil {
		b.Fatalf("setup Unmarshal failed for %s: %v", tc.name, err)
	}
	if !tc.equal(decoded) {
		b.Fatalf("setup decode mismatch for %s", tc.name)
	}

	b.Run(tc.name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(tc.fixtureBytes))
		b.ResetTimer()

		var decoded T
		for range b.N {
			var err error
			decoded, err = tc.serializer.Unmarshal(encoded)
			if err != nil {
				b.Fatalf("Unmarshal failed: %v", err)
			}
		}

		b.StopTimer()
		if !tc.equal(decoded) {
			b.Fatal("Unmarshal returned mismatched value")
		}
		b.ReportMetric(float64(len(encoded)), "encoded_bytes")
	})
}

func runSerializationRoundTripBenchmark[T any](b *testing.B, tc serializationBenchmarkCase[T]) {
	b.Run(tc.name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(tc.fixtureBytes))
		b.ResetTimer()

		var encoded []byte
		var decoded T
		for range b.N {
			var err error
			encoded, err = tc.serializer.Marshal(tc.value)
			if err != nil {
				b.Fatalf("Marshal failed: %v", err)
			}
			decoded, err = tc.serializer.Unmarshal(encoded)
			if err != nil {
				b.Fatalf("Unmarshal failed: %v", err)
			}
		}

		b.StopTimer()
		if !tc.equal(decoded) {
			b.Fatal("round trip returned mismatched value")
		}
		b.ReportMetric(float64(len(encoded)), "encoded_bytes")
	})
}

func smallAccountCase() serializationBenchmarkCase[benchmarkAccount] {
	value := benchmarkSmallAccount()
	serializer := serialization.NewJSONSerializer[benchmarkAccount]()
	return serializationBenchmarkCase[benchmarkAccount]{
		name:         "JSON/serde-small-object-v1",
		serializer:   serializer,
		value:        value,
		equal:        func(actual benchmarkAccount) bool { return reflect.DeepEqual(actual, value) },
		fixtureBytes: serializedFixtureBytes(serializer, value),
	}
}

func mediumOrderCase() serializationBenchmarkCase[benchmarkOrder] {
	value := benchmarkMediumOrder(72)
	serializer := serialization.NewJSONSerializer[benchmarkOrder]()
	return serializationBenchmarkCase[benchmarkOrder]{
		name:         "JSON/serde-medium-nested-v1",
		serializer:   serializer,
		value:        value,
		equal:        func(actual benchmarkOrder) bool { return reflect.DeepEqual(actual, value) },
		fixtureBytes: serializedFixtureBytes(serializer, value),
	}
}

func repeatedEventsCase() serializationBenchmarkCase[[]benchmarkEvent] {
	value := benchmarkRepeatedEvents(1200)
	serializer := serialization.NewJSONSerializer[[]benchmarkEvent]()
	return serializationBenchmarkCase[[]benchmarkEvent]{
		name:         "JSON/serde-repeated-collection-v1",
		serializer:   serializer,
		value:        value,
		equal:        func(actual []benchmarkEvent) bool { return reflect.DeepEqual(actual, value) },
		fixtureBytes: serializedFixtureBytes(serializer, value),
	}
}

func versionedSmallAccountCase(b *testing.B) serializationBenchmarkCase[benchmarkAccount] {
	b.Helper()
	value := benchmarkSmallAccount()
	jsonSerializer := serialization.NewJSONSerializer[benchmarkAccount]()
	serializer, err := serialization.NewVersionedSerializer[benchmarkAccount](jsonSerializer, 1)
	if err != nil {
		b.Fatalf("NewVersionedSerializer failed: %v", err)
	}
	return serializationBenchmarkCase[benchmarkAccount]{
		name:         "Versioned/serde-versioned-envelope-v1",
		serializer:   serializer,
		value:        value,
		equal:        func(actual benchmarkAccount) bool { return reflect.DeepEqual(actual, value) },
		fixtureBytes: serializedFixtureBytes(serializer, value),
	}
}

func binaryPayloadCase() serializationBenchmarkCase[[]byte] {
	value := benchmarkBinaryPayload(96 * 1024)
	serializer := serialization.BytesSerializer{}
	return serializationBenchmarkCase[[]byte]{
		name:         "Bytes/serde-binary-payload-v1",
		serializer:   serializer,
		value:        value,
		equal:        func(actual []byte) bool { return bytes.Equal(actual, value) },
		fixtureBytes: len(value),
	}
}

func textPayloadCase() serializationBenchmarkCase[string] {
	value := benchmarkTextPayload(64 * 1024)
	serializer := serialization.StringSerializer{}
	return serializationBenchmarkCase[string]{
		name:         "String/serde-text-payload-v1",
		serializer:   serializer,
		value:        value,
		equal:        func(actual string) bool { return actual == value },
		fixtureBytes: len(value),
	}
}

type serializedCompressionPayload struct {
	name         string
	data         []byte
	fixtureBytes int
}

func serializationCompressionPayloads(b *testing.B) []serializedCompressionPayload {
	b.Helper()

	cases := []serializedCompressionPayload{
		mustSerializeForCompression(b, smallAccountCase()),
		mustSerializeForCompression(b, mediumOrderCase()),
		mustSerializeForCompression(b, repeatedEventsCase()),
		mustSerializeForCompression(b, versionedSmallAccountCase(b)),
	}
	return cases
}

func mustSerializeForCompression[T any](b *testing.B, tc serializationBenchmarkCase[T]) serializedCompressionPayload {
	b.Helper()

	data, err := tc.serializer.Marshal(tc.value)
	if err != nil {
		b.Fatalf("setup Marshal failed for %s: %v", tc.name, err)
	}
	decoded, err := tc.serializer.Unmarshal(data)
	if err != nil {
		b.Fatalf("setup Unmarshal failed for %s: %v", tc.name, err)
	}
	if !tc.equal(decoded) {
		b.Fatalf("setup round trip mismatch for %s", tc.name)
	}
	return serializedCompressionPayload{
		name:         tc.name,
		data:         data,
		fixtureBytes: tc.fixtureBytes,
	}
}

func serializedFixtureBytes[T any](serializer serialization.Serializer[T], value T) int {
	data, err := serializer.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("serialize benchmark fixture: %v", err))
	}
	return len(data)
}

func benchmarkSmallAccount() benchmarkAccount {
	return benchmarkAccount{
		ID:           "acct-20260707-0001",
		Status:       "active",
		Active:       true,
		BalanceMinor: 120045,
		Currency:     "KRW",
		CreatedAt:    "2026-07-07T00:00:00Z",
		Attributes: map[string]string{
			"tenant":  "blue",
			"region":  "ap-northeast-2",
			"segment": "benchmark",
			"risk":    "low",
			"notes":   strings.Repeat("small-object-low-cardinality-note-", 18),
		},
	}
}

func benchmarkMediumOrder(itemCount int) benchmarkOrder {
	items := make([]benchmarkLineItem, itemCount)
	for index := range items {
		items[index] = benchmarkLineItem{
			SKU:         fmt.Sprintf("sku-%04d", index),
			Quantity:    1 + index%5,
			UnitMinor:   int64(1299 + index*17),
			Currency:    "KRW",
			Description: strings.Repeat(fmt.Sprintf("line item %02d carries deterministic benchmark text ", index%31), 8),
			FulfillmentTags: []string{
				fmt.Sprintf("warehouse-%02d", index%7),
				fmt.Sprintf("route-%02d", index%11),
				"standard",
			},
			Attributes: map[string]string{
				"color":  fmt.Sprintf("color-%02d", index%9),
				"season": fmt.Sprintf("season-%02d", index%4),
				"batch":  fmt.Sprintf("batch-%03d", index%19),
			},
		}
	}

	audit := make([]benchmarkAuditEntry, 48)
	for index := range audit {
		audit[index] = benchmarkAuditEntry{
			Sequence:  index,
			TenantID:  fmt.Sprintf("tenant-%02d", index%13),
			Action:    []string{"created", "priced", "reserved", "packed", "shipped", "notified"}[index%6],
			CreatedAt: fmt.Sprintf("2026-07-07T00:%02d:%02dZ", (index/60)%60, index%60),
			Message:   strings.Repeat(fmt.Sprintf("audit message %02d ", index%17), 12),
		}
	}

	return benchmarkOrder{
		ID:     "ord-20260707-0001",
		Status: "ready_to_ship",
		Customer: benchmarkCustomer{
			ID:        "cust-20260707-0001",
			Tier:      "gold",
			Email:     "benchmark@example.com",
			CreatedAt: "2024-01-02T03:04:05Z",
		},
		ShippingAddress: benchmarkAddress{
			Line1:      "100 Benchmark Road",
			Line2:      "Suite 1400",
			City:       "Seoul",
			Region:     "Seoul",
			PostalCode: "04524",
			Country:    "KR",
		},
		LineItems: items,
		Payment: benchmarkPayment{
			Method:        "card",
			Authorized:    true,
			SubtotalText:  "9421.33",
			SubtotalMinor: 942133,
			TaxText:       "942.13",
			TaxMinor:      94213,
			TotalText:     "10363.46",
			TotalMinor:    1036346,
		},
		Audit:     audit,
		Refund:    nil,
		CreatedAt: "2026-07-07T00:00:00Z",
		UpdatedAt: "2026-07-07T00:05:00Z",
	}
}

func benchmarkRepeatedEvents(count int) []benchmarkEvent {
	events := make([]benchmarkEvent, count)
	for index := range events {
		events[index] = benchmarkEvent{
			Sequence:  index,
			TenantID:  fmt.Sprintf("tenant-%02d", index%23),
			Kind:      []string{"account.updated", "order.created", "payment.authorized", "shipment.ready"}[index%4],
			CreatedAt: fmt.Sprintf("2026-07-07T01:%02d:%02dZ", (index/60)%60, index%60),
			Message:   strings.Repeat(fmt.Sprintf("event payload %03d ", index%101), 5),
			Metadata: map[string]string{
				"region": fmt.Sprintf("region-%02d", index%17),
				"route":  fmt.Sprintf("route-%02d", index%11),
				"level":  []string{"info", "warn", "debug"}[index%3],
			},
		}
	}
	return events
}

func benchmarkBinaryPayload(size int) []byte {
	payload := make([]byte, size)
	copy(payload, []byte("BTGB"))
	payload[4] = 1
	for index := 5; index < len(payload)-4; index++ {
		switch {
		case index%128 < 16:
			payload[index] = 0
		case index%128 < 32:
			payload[index] = 0xff
		default:
			payload[index] = byte((index*37 + index/11) % 251)
		}
	}
	binary.BigEndian.PutUint32(payload[len(payload)-4:], crc32.ChecksumIEEE(payload[:len(payload)-4]))
	return payload
}

func benchmarkTextPayload(size int) string {
	lines := []string{
		"2026-07-07T00:00:00Z INFO serde benchmark fixture emitted service=bluetape-go status=ok",
		"2026-07-07T00:00:01Z INFO serialized payload accepted route=/benchmark/serde tenant=blue",
		"Serialization benchmark text remains deterministic across locale and timezone defaults.",
	}
	var builder strings.Builder
	builder.Grow(size + len(lines[0]))
	for index := 0; builder.Len() < size; index++ {
		builder.WriteString(lines[index%len(lines)])
		builder.WriteString(" sequence=")
		fmt.Fprintf(&builder, "%06d", index)
		builder.WriteByte('\n')
	}
	return builder.String()[:size]
}
