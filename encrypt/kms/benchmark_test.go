package kms

import (
	"bytes"
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/encrypt"
)

var benchmarkSizes = []struct {
	name string
	size int
}{
	{name: "1KiB", size: 1 << 10},
	{name: "1MiB", size: 1 << 20},
	{name: "MaxPlaintextSize", size: MaxPlaintextSize},
}

func BenchmarkDetached(b *testing.B) {
	key := bytes.Repeat([]byte{7}, 32)
	enc, err := encrypt.New(key)
	if err != nil {
		b.Fatal(err)
	}
	for _, fixture := range benchmarkSizes {
		b.Run(fixture.name, func(b *testing.B) {
			plaintext := bytes.Repeat([]byte{'p'}, fixture.size)
			associatedData := []byte("benchmark-ad")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := enc.EncryptDetached(plaintext, associatedData); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEnvelopeMarshalParse(b *testing.B) {
	for _, fixture := range benchmarkSizes {
		b.Run(fixture.name, func(b *testing.B) {
			envelope := Envelope{
				Version:           EnvelopeVersion,
				Algorithm:         AlgorithmAES256GCM,
				KeyID:             "alias/benchmark",
				EncryptedDataKey:  bytes.Repeat([]byte{1}, 32),
				EncryptionContext: map[string]string{"fixture": fixture.name},
				Nonce:             bytes.Repeat([]byte{2}, 12),
				Ciphertext:        bytes.Repeat([]byte{'c'}, fixture.size+16),
			}
			wire, err := envelope.MarshalBinary()
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parsed, err := ParseEnvelope(wire)
				if err != nil {
					b.Fatal(err)
				}
				if len(parsed.Ciphertext) != len(envelope.Ciphertext) {
					b.Fatalf("parsed ciphertext length = %d, want %d", len(parsed.Ciphertext), len(envelope.Ciphertext))
				}
			}
		})
	}
}

func BenchmarkProviderEncrypt(b *testing.B) {
	for _, fixture := range benchmarkSizes {
		b.Run(fixture.name, func(b *testing.B) {
			fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("benchmark-blob"))
			provider, err := New(fake, "alias/benchmark")
			if err != nil {
				b.Fatal(err)
			}
			plaintext := bytes.Repeat([]byte{'p'}, fixture.size)
			associatedData := []byte("benchmark-ad")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := provider.Encrypt(context.Background(), plaintext, associatedData); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			assertBenchmarkCounts(b, fake, b.N, 0)
		})
	}
}

func BenchmarkProviderDecrypt(b *testing.B) {
	for _, fixture := range benchmarkSizes {
		b.Run(fixture.name, func(b *testing.B) {
			fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("benchmark-blob"))
			provider, err := New(fake, "alias/benchmark")
			if err != nil {
				b.Fatal(err)
			}
			plaintext := bytes.Repeat([]byte{'p'}, fixture.size)
			wire, err := provider.Encrypt(context.Background(), plaintext, []byte("benchmark-ad"))
			if err != nil {
				b.Fatal(err)
			}
			fake.resetCounts()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := provider.Decrypt(context.Background(), wire, []byte("benchmark-ad")); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			assertBenchmarkCounts(b, fake, 0, b.N)
		})
	}
}

func BenchmarkProviderRoundTrip(b *testing.B) {
	for _, fixture := range benchmarkSizes {
		b.Run(fixture.name, func(b *testing.B) {
			fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("benchmark-blob"))
			provider, err := New(fake, "alias/benchmark")
			if err != nil {
				b.Fatal(err)
			}
			plaintext := bytes.Repeat([]byte{'p'}, fixture.size)
			associatedData := []byte("benchmark-ad")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				wire, err := provider.Encrypt(context.Background(), plaintext, associatedData)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := provider.Decrypt(context.Background(), wire, associatedData); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			assertBenchmarkCounts(b, fake, b.N, b.N)
		})
	}
}

func BenchmarkProviderRoundTripParallel(b *testing.B) {
	for _, fixture := range benchmarkSizes[:2] {
		b.Run(fixture.name, func(b *testing.B) {
			fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("benchmark-blob"))
			provider, err := New(fake, "alias/benchmark")
			if err != nil {
				b.Fatal(err)
			}
			plaintext := bytes.Repeat([]byte{'p'}, fixture.size)
			associatedData := []byte("benchmark-ad")
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					wire, err := provider.Encrypt(context.Background(), plaintext, associatedData)
					if err != nil {
						b.Error(err)
						return
					}
					if _, err := provider.Decrypt(context.Background(), wire, associatedData); err != nil {
						b.Error(err)
						return
					}
				}
			})
			b.StopTimer()
			assertBenchmarkCounts(b, fake, b.N, b.N)
		})
	}
}

func assertBenchmarkCounts(b *testing.B, fake *fakeClient, wantGenerate, wantDecrypt int) {
	b.Helper()
	gotGenerate, gotDecrypt := fake.counts()
	if gotGenerate != wantGenerate || gotDecrypt != wantDecrypt {
		b.Fatalf("logical KMS calls = %d/%d, want %d/%d", gotGenerate, gotDecrypt, wantGenerate, wantDecrypt)
	}
}
