package forynative

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/apache/fory/go/fory"
)

type testValue struct {
	Name  string
	Count int
}

func registerTestValue(runtime *fory.Fory) error {
	return runtime.RegisterStructByName(testValue{}, "forynative.testValue")
}

func TestNewUsesBoundedDefaultsAndRejectsInvalidLimits(t *testing.T) {
	runtime, err := New[testValue](ProfileNativeFast, Limits{}, registerTestValue)
	if err != nil {
		t.Fatal(err)
	}
	want := Limits{
		MaxPayloadBytes:                 1 << 20,
		MaxDepth:                        20,
		MaxTypeFields:                   512,
		MaxTypeMetaBytes:                4096,
		MaxSchemaVersionsPerType:        10,
		MaxAverageSchemaVersionsPerType: 3,
	}
	if runtime.limits != want {
		t.Fatalf("limits = %#v, want %#v", runtime.limits, want)
	}

	invalid := []Limits{
		{MaxPayloadBytes: -1},
		{MaxDepth: -1},
		{MaxTypeFields: -1},
		{MaxTypeMetaBytes: -1},
		{MaxSchemaVersionsPerType: -1},
		{MaxAverageSchemaVersionsPerType: -1},
	}
	for _, limits := range invalid {
		_, err := New[testValue](ProfileNativeFast, limits, registerTestValue)
		assertReason(t, err, ReasonConfiguration)
	}
	if strconv.IntSize > 32 {
		overWireLimit := uint64(^uint32(0)) + 1
		_, err := New[testValue](
			ProfileNativeFast,
			Limits{MaxPayloadBytes: int(overWireLimit)},
			registerTestValue,
		)
		assertReason(t, err, ReasonConfiguration)
	}
}

func TestNewRejectsUnknownProfileAndMissingRegistration(t *testing.T) {
	_, err := New[testValue](Profile(255), Limits{}, registerTestValue)
	assertReason(t, err, ReasonConfiguration)

	_, err = New[testValue](ProfileNativeFast, Limits{}, nil)
	assertReason(t, err, ReasonRegistration)
}

func TestNewRejectsUnsupportedRootShapes(t *testing.T) {
	tests := []struct {
		name      string
		construct func() error
	}{
		{name: "pointer", construct: func() error {
			_, err := New[*testValue](ProfileNativeFast, Limits{}, registerTestValue)
			return err
		}},
		{name: "map", construct: func() error {
			_, err := New[map[string]string](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "non-byte-slice", construct: func() error {
			_, err := New[[]int](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "array", construct: func() error {
			_, err := New[[1]byte](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "complex", construct: func() error {
			_, err := New[complex64](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "interface", construct: func() error {
			_, err := New[any](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "function", construct: func() error {
			_, err := New[func()](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "channel", construct: func() error {
			_, err := New[chan int](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
		{name: "unsafe-pointer", construct: func() error {
			_, err := New[unsafe.Pointer](ProfileNativeFast, Limits{}, noRegistration)
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertReason(t, tc.construct(), ReasonUnsupportedValue)
		})
	}
}

func TestNewSanitizesRegistrationErrorAndPanic(t *testing.T) {
	const marker = "registration-secret-marker"
	tests := []struct {
		name     string
		register Registration
	}{
		{name: "error", register: func(*fory.Fory) error { return errors.New(marker) }},
		{name: "panic", register: func(*fory.Fory) error { panic(marker) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New[testValue](ProfileNativeFast, Limits{}, tc.register)
			assertReason(t, err, ReasonRegistration)
			assertRedacted(t, err, marker)
		})
	}
}

func TestRuntimeRoundTripsSupportedValues(t *testing.T) {
	structRuntime, err := New[testValue](ProfileNativeFast, Limits{}, registerTestValue)
	if err != nil {
		t.Fatal(err)
	}
	want := testValue{Name: "struct", Count: 7}
	raw, err := structRuntime.Serialize(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := structRuntime.Deserialize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("decoded = %#v, want %#v", got, want)
	}

	bytesRuntime, err := New[[]byte](ProfileNativeCompatible, Limits{}, noRegistration)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = bytesRuntime.Serialize([]byte("bytes"))
	if err != nil {
		t.Fatal(err)
	}
	gotBytes, err := bytesRuntime.Deserialize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBytes) != "bytes" {
		t.Fatalf("decoded bytes = %q", gotBytes)
	}
}

func TestRuntimeEnforcesPayloadLimitAndOwnsReturnedBytes(t *testing.T) {
	limited, err := New[string](ProfileNativeFast, Limits{MaxPayloadBytes: 1}, noRegistration)
	if err != nil {
		t.Fatal(err)
	}
	_, err = limited.Serialize("too-large")
	assertReason(t, err, ReasonPayloadTooLarge)

	runtime, err := New[string](ProfileNativeFast, Limits{}, noRegistration)
	if err != nil {
		t.Fatal(err)
	}
	first, err := runtime.Serialize("first-value")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := append([]byte(nil), first...)
	if _, err := runtime.Serialize(strings.Repeat("second", 16)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, snapshot) {
		t.Fatal("previous serialized bytes changed after a later call")
	}
}

func TestRuntimeSanitizesProviderFailure(t *testing.T) {
	runtime, err := New[string](ProfileNativeFast, Limits{}, noRegistration)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "malformed-provider-secret"
	_, err = runtime.Deserialize([]byte(marker))
	assertReason(t, err, ReasonForyFailure)
	assertRedacted(t, err, marker)
}

func TestZeroRuntimeReturnsUninitialized(t *testing.T) {
	var runtime Runtime[testValue]
	_, err := runtime.Serialize(testValue{})
	assertReason(t, err, ReasonUninitialized)
	_, err = runtime.Deserialize(nil)
	assertReason(t, err, ReasonUninitialized)
}

func TestRuntimeCopiesShareSynchronizationState(t *testing.T) {
	runtime, err := New[testValue](ProfileNativeFast, Limits{}, registerTestValue)
	if err != nil {
		t.Fatal(err)
	}
	copied := *runtime
	runtimes := []*Runtime[testValue]{runtime, &copied}

	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selected := runtimes[worker%len(runtimes)]
			for i := range iterations {
				want := testValue{
					Name:  fmt.Sprintf("worker-%d-%d", worker, i),
					Count: worker*iterations + i,
				}
				raw, err := selected.Serialize(want)
				if err != nil {
					errs <- err
					return
				}
				got, err := selected.Deserialize(raw)
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

func noRegistration(*fory.Fory) error {
	return nil
}

func assertReason(t *testing.T, err error, want Reason) {
	t.Helper()
	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("error = %v, want *Error", err)
	}
	if runtimeErr.Reason() != want {
		t.Fatalf("reason = %q, want %q", runtimeErr.Reason(), want)
	}
}

func assertRedacted(t *testing.T, err error, marker string) {
	t.Helper()
	if strings.Contains(err.Error(), marker) {
		t.Fatalf("error leaked marker: %v", err)
	}
	if cause := errors.Unwrap(err); cause != nil && strings.Contains(cause.Error(), marker) {
		t.Fatalf("unwrapped error leaked marker: %v", cause)
	}
}
