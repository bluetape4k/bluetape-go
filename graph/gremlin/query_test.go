package gremlin

import (
	"errors"
	"testing"
)

func TestValidateTraversal(t *testing.T) {
	for _, value := range []string{"", "   ", "g.V()\x00", string(make([]byte, maxQueryBytes+1))} {
		if err := validateTraversal(value); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("validateTraversal(%q) = %v", value, err)
		}
	}
	if err := validateTraversal("g.V()"); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeBindings(t *testing.T) {
	if _, err := normalizeBindings([]map[string]any{{"a": 1}, {"b": 2}}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("multiple bindings error=%v", err)
	}
	if _, err := normalizeBindings([]map[string]any{{" ": 1}}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("blank binding error=%v", err)
	}
	bindings, err := normalizeBindings([]map[string]any{{"a": 1}})
	if err != nil || bindings["a"] != 1 {
		t.Fatalf("bindings=%#v err=%v", bindings, err)
	}
}
