package sqlcheckpoint

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestLoadNormalizesNilContextAndUsesOneBoundQuery(t *testing.T) {
	recorder := &loadQueryRecorder{row: loadRow{revision: 7, payloadLength: 5, payload: []byte("value")}}
	writer := newLoadWriter(t, Options{Namespace: " tenant ", MaxKeyBytes: 8, MaxPayloadBytes: 5}, func(payload []byte) (string, error) {
		return string(payload), nil
	}, recorder.query)

	checkpoint, exists, err := writer.Load(nil, "key\x00") //nolint:staticcheck // nil context normalization is the contract under test.
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !exists || checkpoint.Version != 7 || checkpoint.Value != "value" {
		t.Fatalf("Load() = %#v, %v, want value/version 7", checkpoint, exists)
	}
	if recorder.calls != 1 {
		t.Fatalf("query calls = %d, want 1", recorder.calls)
	}
	if recorder.ctx == nil {
		t.Fatal("nil context was not normalized")
	}
	if recorder.queryText != loadSQL {
		t.Fatalf("query = %q, want loadSQL", recorder.queryText)
	}
	if len(recorder.args) != 3 {
		t.Fatalf("query args = %#v, want 3", recorder.args)
	}
	assertBytesArgument(t, recorder.args[0], []byte(" tenant "))
	assertBytesArgument(t, recorder.args[1], []byte("key\x00"))
	if got, ok := recorder.args[2].(int); !ok || got != 5 {
		t.Fatalf("payload limit arg = %#v, want int(5)", recorder.args[2])
	}
}

func TestLoadSQLUsesConditionalPayloadProjectionAndFixedByteaIdentity(t *testing.T) {
	for _, fragment := range []string{
		"case when pg_catalog.octet_length(payload) <= $3 then payload end",
		"from public.bluetape_batch_checkpoints",
		"where namespace = $1::bytea and checkpoint_key = $2::bytea",
	} {
		if !strings.Contains(loadSQL, fragment) {
			t.Fatalf("loadSQL does not contain %q:\n%s", fragment, loadSQL)
		}
	}
}

func TestLoadReturnsPreCanceledContextWithoutDatabaseDispatch(t *testing.T) {
	recorder := new(loadQueryRecorder)
	writer := newLoadWriter(t, Options{}, decodeString, recorder.query)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, exists, err := writer.Load(ctx, "key")
	if !errors.Is(err, context.Canceled) || exists {
		t.Fatalf("Load() = exists %v, error %v, want context.Canceled", exists, err)
	}
	if recorder.calls != 0 {
		t.Fatalf("pre-canceled Load dispatched %d queries", recorder.calls)
	}
	assertNotTypedLoadError(t, err)
}

func TestLoadValidatesRawKeyBytesBeforeDispatch(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantError bool
	}{
		{name: "empty", key: "", wantError: true},
		{name: "oversized", key: "12345", wantError: true},
		{name: "exact limit", key: string([]byte{'a', 0, 0xff, 'z'})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &loadQueryRecorder{row: loadRow{revision: 1, payloadLength: 1, payload: []byte("v")}}
			writer := newLoadWriter(t, Options{MaxKeyBytes: 4}, decodeString, recorder.query)

			_, _, err := writer.Load(context.Background(), tt.key)
			if tt.wantError {
				if err == nil {
					t.Fatal("Load() error = nil")
				}
				if recorder.calls != 0 {
					t.Fatalf("invalid key dispatched %d queries", recorder.calls)
				}
				assertNotTypedLoadError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if recorder.calls != 1 {
				t.Fatalf("exact-limit key query calls = %d, want 1", recorder.calls)
			}
			assertBytesArgument(t, recorder.args[1], []byte(tt.key))
		})
	}
}

func TestLoadMapsMissingRowToNotExists(t *testing.T) {
	recorder := &loadQueryRecorder{row: loadRow{err: sql.ErrNoRows}}
	writer := newLoadWriter(t, Options{}, decodeString, recorder.query)

	checkpoint, exists, err := writer.Load(context.Background(), "key")
	if err != nil || exists || checkpoint.Version != 0 || checkpoint.Value != nil {
		t.Fatalf("Load() = %#v, %v, %v; want zero, false, nil", checkpoint, exists, err)
	}
	if recorder.calls != 1 {
		t.Fatalf("query calls = %d, want 1", recorder.calls)
	}
}

func TestLoadRejectsInvalidStoredRevision(t *testing.T) {
	for _, tt := range []struct {
		name     string
		revision int64
	}{
		{name: "zero", revision: 0},
		{name: "negative", revision: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &loadQueryRecorder{row: loadRow{revision: tt.revision, payloadLength: 1, payload: []byte("v")}}
			writer := newLoadWriter(t, Options{}, decodeString, recorder.query)

			_, exists, err := writer.Load(context.Background(), "key")
			if err == nil || exists {
				t.Fatalf("Load() revision %d = exists %v, error %v", tt.revision, exists, err)
			}
			assertNotTypedLoadError(t, err)
		})
	}
}

func TestLoadAcceptsPayloadAtConfiguredLimit(t *testing.T) {
	recorder := &loadQueryRecorder{row: loadRow{revision: 2, payloadLength: 4, payload: []byte("okay")}}
	writer := newLoadWriter(t, Options{MaxPayloadBytes: 4}, decodeString, recorder.query)

	checkpoint, exists, err := writer.Load(context.Background(), "key")
	if err != nil || !exists || checkpoint.Value != "okay" || checkpoint.Version != 2 {
		t.Fatalf("Load() = %#v, %v, %v", checkpoint, exists, err)
	}
}

func TestLoadRejectsInvalidStoredPayloadSizesWithoutDecode(t *testing.T) {
	tests := []struct {
		name       string
		options    Options
		storedSize int64
	}{
		{name: "negative", options: Options{MaxPayloadBytes: 4}, storedSize: -1},
		{name: "over configured", options: Options{MaxPayloadBytes: 4}, storedSize: 5},
		{name: "over hard", options: Options{MaxPayloadBytes: MaxPayloadBytes}, storedSize: MaxPayloadBytes + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := false
			recorder := &loadQueryRecorder{row: loadRow{revision: 1, payloadLength: tt.storedSize, payload: nil}}
			writer := newLoadWriter(t, tt.options, func([]byte) (string, error) {
				decoded = true
				return "", nil
			}, recorder.query)

			_, exists, err := writer.Load(context.Background(), "key")
			if err == nil || exists {
				t.Fatalf("Load() size %d = exists %v, error %v", tt.storedSize, exists, err)
			}
			if decoded {
				t.Fatal("Decode ran for invalid/oversized stored payload")
			}
			assertNotTypedLoadError(t, err)
		})
	}
}

func TestLoadWrapsScanFailuresInRedactedOpError(t *testing.T) {
	cause := errors.New("postgres://user:secret@host/db hostile-scan-cause")
	recorder := &loadQueryRecorder{row: loadRow{err: cause}}
	writer := newLoadWriter(t, Options{Namespace: "hostile-namespace"}, decodeString, recorder.query)

	_, exists, err := writer.Load(context.Background(), "hostile-key")
	if err == nil || exists || !errors.Is(err, cause) {
		t.Fatalf("Load() = exists %v, error %v", exists, err)
	}
	var opErr *OpError
	if !errors.As(err, &opErr) || opErr.Operation() != "load" {
		t.Fatalf("Load() error type = %T, value %v", err, err)
	}
	for _, marker := range []string{"postgres://", "secret", "hostile-scan-cause", "hostile-namespace", "hostile-key"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("OpError leaked %q: %v", marker, err)
		}
	}
}

func TestLoadWrapsDecodeFailureWithoutDatabaseOpError(t *testing.T) {
	cause := errors.New("hostile-payload hostile-codec-cause")
	recorder := &loadQueryRecorder{row: loadRow{revision: 3, payloadLength: 15, payload: []byte("hostile-payload")}}
	writer := newLoadWriter(t, Options{MaxPayloadBytes: 15}, func([]byte) (string, error) {
		return "", cause
	}, recorder.query)

	_, exists, err := writer.Load(context.Background(), "key")
	if err == nil || exists || !errors.Is(err, cause) {
		t.Fatalf("Load() = exists %v, error %v", exists, err)
	}
	var codecErr *CodecError
	var opErr *OpError
	if !errors.As(err, &codecErr) || codecErr.Operation() != "decode" || errors.As(err, &opErr) {
		t.Fatalf("Load() error = %T %v", err, err)
	}
	for _, marker := range []string{"hostile-payload", "hostile-codec-cause"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("CodecError leaked %q: %v", marker, err)
		}
	}
}

func TestLoadCopiesPayloadBeforeDecode(t *testing.T) {
	stored := []byte("owned")
	recorder := &loadQueryRecorder{row: loadRow{revision: 1, payloadLength: int64(len(stored)), payload: stored}}
	writer := newLoadWriter(t, Options{}, func(payload []byte) (string, error) {
		payload[0] = 'X'
		return string(payload), nil
	}, recorder.query)

	checkpoint, exists, err := writer.Load(context.Background(), "key")
	if err != nil || !exists || checkpoint.Value != "Xwned" {
		t.Fatalf("Load() = %#v, %v, %v", checkpoint, exists, err)
	}
	if got := string(stored); got != "owned" {
		t.Fatalf("Decode mutated scanner-owned payload: %q", got)
	}
}

func TestLoadPreservesNonNilEmptyPayloadForDecode(t *testing.T) {
	stored := []byte{}
	recorder := &loadQueryRecorder{row: loadRow{revision: 1, payloadLength: 0, payload: stored}}
	receivedNil := true
	writer := newLoadWriter(t, Options{}, func(payload []byte) (string, error) {
		receivedNil = payload == nil
		return "empty", nil
	}, recorder.query)

	checkpoint, exists, err := writer.Load(context.Background(), "key")
	if err != nil || !exists || checkpoint.Value != "empty" {
		t.Fatalf("Load() = %#v, %v, %v", checkpoint, exists, err)
	}
	if receivedNil {
		t.Fatal("Decode received nil for a non-nil empty stored payload")
	}
}

func TestLoadNilAndZeroWriterReturnInitializationError(t *testing.T) {
	writers := []*Writer[any, string]{nil, {}}
	for _, writer := range writers {
		_, exists, err := writer.Load(context.Background(), "key")
		if !errors.Is(err, errWriterUninitialized) || exists {
			t.Fatalf("Load() = exists %v, error %v, want initialization error", exists, err)
		}
		assertNotTypedLoadError(t, err)
	}
}

type loadRow struct {
	revision      int64
	payloadLength int64
	payload       []byte
	err           error
}

func (row loadRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != 3 {
		return errors.New("loadRow: unexpected destination count")
	}
	*dest[0].(*int64) = row.revision
	*dest[1].(*int64) = row.payloadLength
	*dest[2].(*[]byte) = row.payload
	return nil
}

type loadQueryRecorder struct {
	calls     int
	ctx       context.Context
	queryText string
	args      []any
	row       rowScanner
}

func (recorder *loadQueryRecorder) query(ctx context.Context, query string, args ...any) rowScanner {
	recorder.calls++
	recorder.ctx = ctx
	recorder.queryText = query
	recorder.args = append([]any(nil), args...)
	return recorder.row
}

func newLoadWriter(t *testing.T, options Options, decode func([]byte) (string, error), query func(context.Context, string, ...any) rowScanner) *Writer[any, string] {
	t.Helper()
	writer, err := New[any, string](new(sql.DB), options, Codec[string]{
		Encode: func(value string) ([]byte, error) { return []byte(value), nil },
		Decode: decode,
	}, func(context.Context, sqlkit.Session, []any) error { return nil })
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	writer.queryRow = query
	return writer
}

func decodeString(payload []byte) (string, error) { return string(payload), nil }

func assertBytesArgument(t *testing.T, got any, want []byte) {
	t.Helper()
	value, ok := got.([]byte)
	if !ok || string(value) != string(want) {
		t.Fatalf("query argument = %#v, want bytes %v", got, want)
	}
}

func assertNotTypedLoadError(t *testing.T, err error) {
	t.Helper()
	var opErr *OpError
	var codecErr *CodecError
	if errors.As(err, &opErr) || errors.As(err, &codecErr) {
		t.Fatalf("unexpected typed load error %T: %v", err, err)
	}
}
