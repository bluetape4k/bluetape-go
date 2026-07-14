package sqlcheckpoint

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestNewRejectsInvalidInputsWithoutDatabaseIO(t *testing.T) {
	validCodec := testCodec()
	validWrite := testWrite

	tests := []struct {
		name    string
		nilDB   bool
		options Options
		codec   Codec[any]
		write   WriteTxFunc[any]
		wantErr string
	}{
		{
			name:    "nil DB",
			nilDB:   true,
			codec:   validCodec,
			write:   validWrite,
			wantErr: "sqlcheckpoint: db must not be nil",
		},
		{
			name:    "nil write",
			codec:   validCodec,
			wantErr: "sqlcheckpoint: write callback must not be nil",
		},
		{
			name: "nil encode",
			codec: Codec[any]{
				Decode: validCodec.Decode,
			},
			write:   validWrite,
			wantErr: "sqlcheckpoint: codec encode must not be nil",
		},
		{
			name: "nil decode",
			codec: Codec[any]{
				Encode: validCodec.Encode,
			},
			write:   validWrite,
			wantErr: "sqlcheckpoint: codec decode must not be nil",
		},
		{
			name:    "namespace over byte limit",
			options: Options{Namespace: strings.Repeat("n", 129)},
			codec:   validCodec,
			write:   validWrite,
			wantErr: "sqlcheckpoint: namespace exceeds 128 bytes",
		},
		{
			name:    "negative key byte limit",
			options: Options{MaxKeyBytes: -1},
			codec:   validCodec,
			write:   validWrite,
			wantErr: "sqlcheckpoint: max key bytes must be between 1 and 1024",
		},
		{
			name:    "negative payload byte limit",
			options: Options{MaxPayloadBytes: -1},
			codec:   validCodec,
			write:   validWrite,
			wantErr: "sqlcheckpoint: max payload bytes must be between 1 and 16777216",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := new(countingConnector)
			ownedDB := sql.OpenDB(connector)
			t.Cleanup(func() { _ = ownedDB.Close() })
			db := ownedDB
			if tt.nilDB {
				db = nil
			}

			_, err := New[any, any](db, tt.options, tt.codec, tt.write)
			if err == nil {
				t.Fatalf("New() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("New() error = %q, want %q", err, tt.wantErr)
			}
			if got := connector.connects.Load(); got != 0 {
				t.Fatalf("invalid constructor acquired %d database connections, want 0", got)
			}
		})
	}
}

func TestOptionsNamespaceNormalization(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      []byte
	}{
		{name: "empty defaults exactly", want: []byte("default")},
		{name: "plain bytes preserved", namespace: " Tenant-A ", want: []byte(" Tenant-A ")},
		{name: "NUL and invalid UTF-8 preserved", namespace: string([]byte{'a', 0, 0xff, 0xfe}), want: []byte{'a', 0, 0xff, 0xfe}},
		{name: "128 bytes accepted", namespace: strings.Repeat("n", 128), want: []byte(strings.Repeat("n", 128))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := mustNewWriter(t, Options{Namespace: tt.namespace})
			if string(writer.options.namespace) != string(tt.want) {
				t.Fatalf("namespace bytes = %v, want %v", writer.options.namespace, tt.want)
			}
		})
	}

	raw := []byte("owned")
	writer := mustNewWriter(t, Options{Namespace: string(raw)})
	raw[0] = 'X'
	if got := string(writer.options.namespace); got != "owned" {
		t.Fatalf("stored namespace aliases caller bytes: got %q", got)
	}

	_, err := New[any, any](new(sql.DB), Options{Namespace: strings.Repeat("n", 129)}, testCodec(), testWrite)
	if err == nil || err.Error() != "sqlcheckpoint: namespace exceeds 128 bytes" {
		t.Fatalf("129-byte namespace error = %v", err)
	}
}

func TestOptionsMaxKeyBytes(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		want    int
		wantErr bool
	}{
		{name: "zero defaults", value: 0, want: DefaultMaxKeyBytes},
		{name: "one accepted", value: 1, want: 1},
		{name: "hard maximum accepted", value: MaxKeyBytes, want: MaxKeyBytes},
		{name: "negative rejected", value: -1, wantErr: true},
		{name: "over hard maximum rejected", value: MaxKeyBytes + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := New[any, any](new(sql.DB), Options{MaxKeyBytes: tt.value}, testCodec(), testWrite)
			if tt.wantErr {
				if err == nil || err.Error() != "sqlcheckpoint: max key bytes must be between 1 and 1024" {
					t.Fatalf("New() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if writer.options.maxKeyBytes != tt.want {
				t.Fatalf("maxKeyBytes = %d, want %d", writer.options.maxKeyBytes, tt.want)
			}
		})
	}
}

func TestOptionsMaxPayloadBytes(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		want    int
		wantErr bool
	}{
		{name: "zero defaults", value: 0, want: DefaultMaxPayloadBytes},
		{name: "one accepted", value: 1, want: 1},
		{name: "hard maximum accepted", value: MaxPayloadBytes, want: MaxPayloadBytes},
		{name: "negative rejected", value: -1, wantErr: true},
		{name: "over hard maximum rejected", value: MaxPayloadBytes + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := New[any, any](new(sql.DB), Options{MaxPayloadBytes: tt.value}, testCodec(), testWrite)
			if tt.wantErr {
				if err == nil || err.Error() != "sqlcheckpoint: max payload bytes must be between 1 and 16777216" {
					t.Fatalf("New() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if writer.options.maxPayloadBytes != tt.want {
				t.Fatalf("maxPayloadBytes = %d, want %d", writer.options.maxPayloadBytes, tt.want)
			}
		})
	}
}

func TestNewPerformsNoDatabaseIOOrPoolMutation(t *testing.T) {
	connector := new(countingConnector)
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(7)
	t.Cleanup(func() { _ = db.Close() })
	before := db.Stats()

	writer, err := New[any, any](db, Options{}, testCodec(), testWrite)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if writer.db != db {
		t.Fatal("New() did not retain the caller-owned DB")
	}
	if got := connector.connects.Load(); got != 0 {
		t.Fatalf("constructor acquired %d database connections, want 0", got)
	}
	after := db.Stats()
	if after.MaxOpenConnections != before.MaxOpenConnections {
		t.Fatalf("MaxOpenConnections changed from %d to %d", before.MaxOpenConnections, after.MaxOpenConnections)
	}

	err = db.PingContext(context.Background())
	if !errors.Is(err, errProbeConnect) {
		t.Fatalf("caller-owned DB is unusable or was closed: PingContext error = %v", err)
	}
	if got := connector.connects.Load(); got != 1 {
		t.Fatalf("PingContext connection attempts = %d, want 1", got)
	}
}

func TestNewWriterStagingMethodsAreNilSafe(t *testing.T) {
	writers := []struct {
		name   string
		writer *Writer[any, any]
	}{
		{name: "nil"},
		{name: "zero", writer: new(Writer[any, any])},
	}

	for _, tt := range writers {
		t.Run(tt.name, func(t *testing.T) {
			_, commitErr := tt.writer.Commit(context.Background(), "key", 0, nil, nil)
			if !errors.Is(commitErr, errWriterUninitialized) {
				t.Fatalf("Commit() error = %v, want %v", commitErr, errWriterUninitialized)
			}
		})
	}
}

func mustNewWriter(t *testing.T, options Options) *Writer[any, any] {
	t.Helper()
	writer, err := New[any, any](new(sql.DB), options, testCodec(), testWrite)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return writer
}

func testCodec() Codec[any] {
	return Codec[any]{
		Encode: func(any) ([]byte, error) { return nil, nil },
		Decode: func([]byte) (any, error) { return nil, nil },
	}
}

func testWrite(context.Context, sqlkit.Session, []any) error { return nil }

var errProbeConnect = errors.New("probe connector invoked")

type countingConnector struct {
	connects atomic.Int64
}

func (c *countingConnector) Connect(context.Context) (driver.Conn, error) {
	c.connects.Add(1)
	return nil, errProbeConnect
}

func (*countingConnector) Driver() driver.Driver { return countingDriver{} }

type countingDriver struct{}

func (countingDriver) Open(string) (driver.Conn, error) { return nil, errProbeConnect }
