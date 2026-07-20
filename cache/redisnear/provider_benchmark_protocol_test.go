package redisnear

import "testing"

// TestProviderBenchmarkProtocolAuthority pins the private wire contract mirrored
// by cache/provider_benchmark_test.go so benchmark-only direct publication fails
// closed when the production channel or envelope changes.
func TestProviderBenchmarkProtocolAuthority(t *testing.T) {
	const (
		namespace = "0123456789abcdef0123456789abcdef"
		originID  = "abcdef0123456789abcdef0123456789"
		key       = "11111111111111111111111111111111"
	)
	if got, want := defaultChannel(namespace), "bluetape:cache:near:"+namespace+":invalidate"; got != want {
		t.Fatalf("default channel = %q, want %q", got, want)
	}
	payload, err := encodeMessage(invalidationMessage{
		Namespace: namespace,
		OriginID:  originID,
		Operation: operationSet,
		Key:       key,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"version":1,"namespace":"0123456789abcdef0123456789abcdef","originID":"abcdef0123456789abcdef0123456789","operation":"set","key":"11111111111111111111111111111111"}`; string(payload) != want {
		t.Fatalf("production payload = %q, want %q", payload, want)
	}
}
