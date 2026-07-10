package rediscoordfory

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/apache/fory/go/fory"
)

func TestBTFYV1LayoutRemainsStable(t *testing.T) {
	codec, err := NewNativeFast[string](Options{Register: func(*fory.Fory) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := codec.Marshal("value")
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) < 10 {
		t.Fatalf("encoded length = %d", len(encoded))
	}
	if string(encoded[:4]) != "BTFY" || encoded[4] != 1 || encoded[5] != 1 {
		t.Fatalf("header = %x", encoded[:10])
	}
	if binary.BigEndian.Uint32(encoded[6:10]) != uint32(len(encoded)-10) {
		t.Fatalf("length = %d", binary.BigEndian.Uint32(encoded[6:10]))
	}
}

type testValue struct {
	Name  string
	Count int
}

type compatibleV1 struct{ Name string }
type compatibleV2 struct {
	Name  string
	Count int
}

func registerTestValue(f *fory.Fory) error {
	return f.RegisterStructByName(testValue{}, "rediscoordfory.testValue")
}

func TestNewNativeFastRejectsInvalidOptions(t *testing.T) {
	_, err := NewNativeFast[testValue](Options{Register: registerTestValue, MaxDepth: -1})
	var ce *CodecError
	if !errors.As(err, &ce) || ce.Reason() != ReasonConfiguration {
		t.Fatalf("error = %v", err)
	}
	_, err = NewNativeFast[testValue](Options{})
	if !errors.As(err, &ce) || ce.Reason() != ReasonRegistration {
		t.Fatalf("registration error = %v", err)
	}
	if strconv.IntSize > 32 {
		overWireLimit := uint64(^uint32(0)) + 1
		_, err = NewNativeFast[testValue](Options{Register: registerTestValue, MaxPayloadBytes: int(overWireLimit)})
		if !errors.As(err, &ce) || ce.Reason() != ReasonConfiguration {
			t.Fatalf("uint32 bound error = %v", err)
		}
	}
}

func TestNewNativeFastRejectsUnsupportedRootShapes(t *testing.T) {
	for _, test := range []struct {
		name      string
		construct func() error
	}{
		{name: "pointer", construct: func() error { _, err := NewNativeFast[*testValue](Options{Register: registerTestValue}); return err }},
		{name: "map", construct: func() error {
			_, err := NewNativeFast[map[string]string](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "non-byte-slice", construct: func() error {
			_, err := NewNativeFast[[]int](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "array", construct: func() error {
			_, err := NewNativeFast[[1]byte](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "complex", construct: func() error {
			_, err := NewNativeFast[complex64](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "interface", construct: func() error {
			_, err := NewNativeFast[any](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "function", construct: func() error {
			_, err := NewNativeFast[func()](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "channel", construct: func() error {
			_, err := NewNativeFast[chan int](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
		{name: "unsafe-pointer", construct: func() error {
			_, err := NewNativeFast[unsafe.Pointer](Options{Register: func(*fory.Fory) error { return nil }})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var ce *CodecError
			if err := test.construct(); !errors.As(err, &ce) || ce.Reason() != ReasonUnsupportedValue {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestZeroCodecReturnsUninitializedError(t *testing.T) {
	var codec Codec[testValue]
	_, err := codec.Marshal(testValue{})
	var ce *CodecError
	if !errors.As(err, &ce) || ce.Reason() != ReasonUninitialized {
		t.Fatalf("marshal error = %v", err)
	}
	_, err = codec.Unmarshal(nil)
	if !errors.As(err, &ce) || ce.Reason() != ReasonUninitialized {
		t.Fatalf("unmarshal error = %v", err)
	}
}

func TestNewNativeFastRecoversRegistrationPanic(t *testing.T) {
	marker := "registration-panic-secret"
	_, err := NewNativeFast[testValue](Options{Register: func(*fory.Fory) error { panic(marker) }})
	var ce *CodecError
	if !errors.As(err, &ce) || ce.Reason() != ReasonRegistration {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), marker) || strings.Contains(errors.Unwrap(err).Error(), marker) {
		t.Fatalf("registration panic leaked: %v", err)
	}
}

func TestCodecErrorRedactsPayloadCauseAndRegistrationText(t *testing.T) {
	secret := "secret-registration-payload-owner-token"
	rawCause := errors.New(secret)
	_, err := NewNativeFast[testValue](Options{Register: func(*fory.Fory) error { return rawCause }})
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked secret: %s", err)
	}
	var ce *CodecError
	if !errors.As(err, &ce) || ce.Reason() != ReasonRegistration {
		t.Fatalf("error = %v", err)
	}
	if errors.Is(err, rawCause) || strings.Contains(errors.Unwrap(err).Error(), secret) {
		t.Fatalf("unwrapped error leaked registration cause: %v", errors.Unwrap(err))
	}
}

func TestNativeProfilesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(Options) (*Codec[testValue], error)
	}{
		{"fast", NewNativeFast[testValue]}, {"compatible", NewNativeCompatible[testValue]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			codec, err := tc.make(Options{Register: registerTestValue})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := codec.Marshal(testValue{Name: "ok", Count: 7})
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded[:4]) != "BTFY" {
				t.Fatalf("magic = %q", encoded[:4])
			}
			decoded, err := codec.Unmarshal(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != (testValue{Name: "ok", Count: 7}) {
				t.Fatalf("decoded = %#v", decoded)
			}
		})
	}
}

func TestNativeCompatibleReadsAddedFieldSchema(t *testing.T) {
	writer, err := NewNativeCompatible[compatibleV1](Options{Register: func(f *fory.Fory) error {
		return f.RegisterStructByName(compatibleV1{}, "rediscoordfory.compatibleValue")
	}})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := NewNativeCompatible[compatibleV2](Options{Register: func(f *fory.Fory) error {
		return f.RegisterStructByName(compatibleV2{}, "rediscoordfory.compatibleValue")
	}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := writer.Marshal(compatibleV1{Name: "old"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := reader.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "old" || got.Count != 0 {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestEnvelopeRejectsMalformedInput(t *testing.T) {
	codec, err := NewNativeFast[testValue](Options{Register: registerTestValue, MaxPayloadBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = codec.Unmarshal([]byte("JSON"))
	var ce *CodecError
	if !errors.As(err, &ce) || ce.Reason() != ReasonInvalidMagic {
		t.Fatalf("error = %v", err)
	}
	_, err = codec.Marshal(testValue{Name: "too-large"})
	if !errors.As(err, &ce) || ce.Reason() != ReasonPayloadTooLarge {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestNativeValueShapes(t *testing.T) {
	codec, err := NewNativeFast[string](Options{Register: func(*fory.Fory) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	b, err := codec.Marshal("hello")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := codec.Unmarshal(b); err != nil || got != "hello" {
		t.Fatalf("string round trip = %q, %v", got, err)
	}
	bytesCodec, err := NewNativeFast[[]byte](Options{Register: func(*fory.Fory) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	b, err = bytesCodec.Marshal([]byte("bytes"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := bytesCodec.Unmarshal(b)
	if err != nil || string(got) != "bytes" {
		t.Fatalf("bytes round trip = %q, %v", got, err)
	}
}

func TestNativeProfilesPreserveZeroAndEmptyValues(t *testing.T) {
	profiles := []struct {
		name  string
		bytes func(Options) (*Codec[[]byte], error)
		text  func(Options) (*Codec[string], error)
		value func(Options) (*Codec[testValue], error)
	}{
		{name: "fast", bytes: NewNativeFast[[]byte], text: NewNativeFast[string], value: NewNativeFast[testValue]},
		{name: "compatible", bytes: NewNativeCompatible[[]byte], text: NewNativeCompatible[string], value: NewNativeCompatible[testValue]},
	}
	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			noRegistration := Options{Register: func(*fory.Fory) error { return nil }}
			bytesCodec, err := profile.bytes(noRegistration)
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name    string
				value   []byte
				wantNil bool
			}{
				{name: "nil", value: nil, wantNil: true},
				{name: "empty", value: []byte{}, wantNil: false},
			} {
				t.Run(tc.name+"-bytes", func(t *testing.T) {
					encoded, err := bytesCodec.Marshal(tc.value)
					if err != nil {
						t.Fatal(err)
					}
					got, err := bytesCodec.Unmarshal(encoded)
					if err != nil {
						t.Fatal(err)
					}
					if (got == nil) != tc.wantNil || len(got) != 0 {
						t.Fatalf("decoded = %#v", got)
					}
				})
			}
			textCodec, err := profile.text(noRegistration)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := textCodec.Marshal("")
			if err != nil {
				t.Fatal(err)
			}
			if got, err := textCodec.Unmarshal(encoded); err != nil || got != "" {
				t.Fatalf("empty string = %q, %v", got, err)
			}
			valueCodec, err := profile.value(Options{Register: registerTestValue})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err = valueCodec.Marshal(testValue{})
			if err != nil {
				t.Fatal(err)
			}
			if got, err := valueCodec.Unmarshal(encoded); err != nil || got != (testValue{}) {
				t.Fatalf("zero struct = %#v, %v", got, err)
			}
		})
	}
}

func TestEnvelopeRejectsWrongVersionProfileAndLength(t *testing.T) {
	codec, err := NewNativeFast[string](Options{Register: func(*fory.Fory) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := codec.Marshal("value")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func([]byte)
		reason Reason
	}{
		{name: "version", mutate: func(b []byte) { b[4]++ }, reason: ReasonUnsupportedVersion},
		{name: "profile", mutate: func(b []byte) { b[5] = 2 }, reason: ReasonProfileMismatch},
		{name: "length", mutate: func(b []byte) { b[9]++ }, reason: ReasonLengthMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]byte(nil), encoded...)
			tc.mutate(bad)
			_, err := codec.Unmarshal(bad)
			var ce *CodecError
			if !errors.As(err, &ce) || ce.Reason() != tc.reason {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestUnmarshalWrapsMalformedForyPayload(t *testing.T) {
	codec, err := NewNativeFast[string](Options{Register: func(*fory.Fory) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	marker := "malformed-provider-payload-marker"
	_, err = codec.Unmarshal(wrap(ProfileNativeFast, []byte(marker)))
	var ce *CodecError
	if !errors.As(err, &ce) || ce.Reason() != ReasonForyFailure {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), marker) {
		t.Fatalf("error leaked payload: %v", err)
	}
	if strings.Contains(errors.Unwrap(err).Error(), marker) {
		t.Fatalf("unwrapped error leaked payload: %v", errors.Unwrap(err))
	}
}

func TestCodecConcurrentRoundTrip(t *testing.T) {
	codec, err := NewNativeFast[testValue](Options{Register: registerTestValue})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				want := testValue{Name: fmt.Sprintf("worker-%d-%d", worker, i), Count: worker*iterations + i}
				encoded, err := codec.Marshal(want)
				if err != nil {
					errs <- err
					return
				}
				got, err := codec.Unmarshal(encoded)
				if err != nil {
					errs <- err
					return
				}
				if got != want {
					errs <- fmt.Errorf("got %#v, want %#v", got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestCopiedCodecSharesRuntimeLock(t *testing.T) {
	codec, err := NewNativeFast[testValue](Options{Register: registerTestValue})
	if err != nil {
		t.Fatal(err)
	}
	copied := *codec
	codecs := []*Codec[testValue]{codec, &copied}
	const workers = 8
	const iterations = 50
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selected := codecs[worker%len(codecs)]
			for i := range iterations {
				want := testValue{Name: fmt.Sprintf("copy-%d-%d", worker, i), Count: i}
				encoded, err := selected.Marshal(want)
				if err != nil {
					errs <- err
					return
				}
				got, err := selected.Unmarshal(encoded)
				if err != nil {
					errs <- err
					return
				}
				if got != want {
					errs <- fmt.Errorf("got %#v, want %#v", got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
