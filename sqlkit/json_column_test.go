package sqlkit_test

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

type jsonProfile struct {
	Name string `json:"name"`
}

type retainingJSON []byte

func (r *retainingJSON) UnmarshalJSON(data []byte) error {
	*r = data
	return nil
}

type panickingJSON struct{}

func (*panickingJSON) UnmarshalJSON([]byte) error {
	panic("unmarshal-secret")
}

func (panickingJSON) MarshalJSON() ([]byte, error) {
	panic("marshal-secret")
}

var errJSONCallback = errors.New("json-callback-secret")

type failingJSON struct{}

func (*failingJSON) UnmarshalJSON([]byte) error {
	return errJSONCallback
}

func (failingJSON) MarshalJSON() ([]byte, error) {
	return nil, errJSONCallback
}

var _ sql.Scanner = (*sqlkit.JSONColumn[jsonProfile])(nil)
var _ driver.Valuer = sqlkit.JSONColumn[jsonProfile]{}

func TestJSONColumnRoundTrip(t *testing.T) {
	input := sqlkit.JSONColumn[jsonProfile]{Data: jsonProfile{Name: "Ada"}, Valid: true}

	value, err := input.Value()
	if err != nil {
		t.Fatalf("Value failed: %v", err)
	}
	raw, ok := value.([]byte)
	if !ok {
		t.Fatalf("Value type = %T, want []byte", value)
	}

	var output sqlkit.JSONColumn[jsonProfile]
	if err := output.Scan(raw); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if !output.Valid || output.Data != (jsonProfile{Name: "Ada"}) {
		t.Fatalf("output = %#v, want valid Ada profile", output)
	}
}

func TestJSONColumnDistinguishesSQLNullAndJSONNull(t *testing.T) {
	column := sqlkit.JSONColumn[*jsonProfile]{Data: &jsonProfile{Name: "stale"}, Valid: true}
	if err := column.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) failed: %v", err)
	}
	if column.Valid || column.Data != nil {
		t.Fatalf("SQL NULL = %#v, want invalid nil data", column)
	}

	if err := column.Scan([]byte("null")); err != nil {
		t.Fatalf("Scan(JSON null) failed: %v", err)
	}
	if !column.Valid || column.Data != nil {
		t.Fatalf("JSON null = %#v, want valid nil data", column)
	}
	value, err := column.Value()
	if err != nil {
		t.Fatalf("JSON null Value failed: %v", err)
	}
	if raw, ok := value.([]byte); !ok || string(raw) != "null" {
		t.Fatalf("JSON null Value = %T(%q), want []byte(null)", value, value)
	}
}

func TestJSONColumnClearsStateOnFailure(t *testing.T) {
	tests := []struct {
		name string
		src  any
	}{
		{name: "malformed", src: []byte(`{"name":`)},
		{name: "trailing token", src: []byte(`{"name":"Ada"} {}`)},
		{name: "unsupported source", src: int64(7)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := sqlkit.JSONColumn[jsonProfile]{Data: jsonProfile{Name: "stale"}, Valid: true}
			err := column.Scan(tt.src)
			if !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
				t.Fatalf("Scan error = %v, want ErrInvalidColumnValue", err)
			}
			if column.Valid || column.Data != (jsonProfile{}) {
				t.Fatalf("failed Scan retained state: %#v", column)
			}
		})
	}
}

func TestJSONColumnEnforcesLimits(t *testing.T) {
	if sqlkit.DefaultJSONColumnMaxBytes != 1<<20 {
		t.Fatalf("DefaultJSONColumnMaxBytes = %d, want %d", sqlkit.DefaultJSONColumnMaxBytes, 1<<20)
	}

	tests := []struct {
		name string
		max  int
		want error
	}{
		{name: "oversized source", max: 4, want: sqlkit.ErrColumnValueTooLarge},
		{name: "negative limit", max: -1, want: sqlkit.ErrInvalidColumnValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := sqlkit.JSONColumn[jsonProfile]{Data: jsonProfile{Name: "stale"}, Valid: true, MaxBytes: tt.max}
			err := column.Scan([]byte(`{"name":"Ada"}`))
			if !errors.Is(err, tt.want) {
				t.Fatalf("Scan error = %v, want %v", err, tt.want)
			}
			if column.Valid || column.Data != (jsonProfile{}) {
				t.Fatalf("failed Scan retained state: %#v", column)
			}
		})
	}

	oversized := sqlkit.JSONColumn[jsonProfile]{Data: jsonProfile{Name: "Ada"}, Valid: true, MaxBytes: 4}
	if _, err := oversized.Value(); !errors.Is(err, sqlkit.ErrColumnValueTooLarge) {
		t.Fatalf("oversized Value error = %v, want ErrColumnValueTooLarge", err)
	}
	negative := sqlkit.JSONColumn[jsonProfile]{Data: jsonProfile{Name: "Ada"}, Valid: true, MaxBytes: -1}
	if _, err := negative.Value(); !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
		t.Fatalf("negative-limit Value error = %v, want ErrInvalidColumnValue", err)
	}
}

func TestJSONColumnCopiesDriverSource(t *testing.T) {
	source := []byte(`{"secret":"value"}`)
	var column sqlkit.JSONColumn[retainingJSON]
	if err := column.Scan(source); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	want := string(column.Data)
	for i := range source {
		source[i] = 'X'
	}
	if got := string(column.Data); got != want {
		t.Fatalf("retained JSON changed from %q to %q", want, got)
	}
}

func TestJSONColumnPreservesCallbackErrorsWithoutExposingThem(t *testing.T) {
	var scanned sqlkit.JSONColumn[failingJSON]
	err := scanned.Scan([]byte(`{}`))
	if !errors.Is(err, sqlkit.ErrInvalidColumnValue) || !errors.Is(err, errJSONCallback) {
		t.Fatalf("Scan error = %v, want both sentinels", err)
	}
	if strings.Contains(err.Error(), errJSONCallback.Error()) {
		t.Fatalf("Scan error exposes callback cause: %v", err)
	}

	valued := sqlkit.JSONColumn[failingJSON]{Data: failingJSON{}, Valid: true}
	_, err = valued.Value()
	if !errors.Is(err, sqlkit.ErrInvalidColumnValue) || !errors.Is(err, errJSONCallback) {
		t.Fatalf("Value error = %v, want both sentinels", err)
	}
	if strings.Contains(err.Error(), errJSONCallback.Error()) {
		t.Fatalf("Value error exposes callback cause: %v", err)
	}
}

func TestJSONColumnContainsCallbackPanics(t *testing.T) {
	var scanned sqlkit.JSONColumn[panickingJSON]
	err := scanned.Scan([]byte(`{}`))
	if !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
		t.Fatalf("Scan panic error = %v, want ErrInvalidColumnValue", err)
	}
	if strings.Contains(err.Error(), "unmarshal-secret") {
		t.Fatalf("Scan panic error exposes panic value: %v", err)
	}

	valued := sqlkit.JSONColumn[panickingJSON]{Data: panickingJSON{}, Valid: true}
	_, err = valued.Value()
	if !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
		t.Fatalf("Value panic error = %v, want ErrInvalidColumnValue", err)
	}
	if strings.Contains(err.Error(), "marshal-secret") {
		t.Fatalf("Value panic error exposes panic value: %v", err)
	}
}

func TestJSONColumnNilScanner(t *testing.T) {
	var column *sqlkit.JSONColumn[jsonProfile]
	if err := column.Scan([]byte(`{}`)); !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
		t.Fatalf("nil Scan error = %v, want ErrInvalidColumnValue", err)
	}
}
