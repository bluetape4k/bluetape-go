package core_test

import (
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestXXH64Bytes(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
		want  uint64
	}{
		{name: "empty", value: nil, want: 0xef46db3751d8e999},
		{name: "ascii", value: []byte("hello"), want: 0x26c7827d889f6da3},
		{name: "unicode", value: []byte("안녕하세요"), want: 0x3bea2c8c7745eb13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.XXH64Bytes(tt.value); got != tt.want {
				t.Fatalf("XXH64Bytes(%q) = %#x, want %#x", string(tt.value), got, tt.want)
			}
		})
	}
}

func TestXXH64String(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  uint64
	}{
		{name: "empty", value: "", want: 0xef46db3751d8e999},
		{name: "ascii", value: "hello", want: 0x26c7827d889f6da3},
		{name: "unicode", value: "안녕하세요", want: 0x3bea2c8c7745eb13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.XXH64String(tt.value); got != tt.want {
				t.Fatalf("XXH64String(%q) = %#x, want %#x", tt.value, got, tt.want)
			}
		})
	}
}

func TestXXH64DeterministicAndConsistentAcrossInputTypes(t *testing.T) {
	value := "stable-cache-key-안녕"

	first := core.XXH64String(value)
	second := core.XXH64String(value)
	if first != second {
		t.Fatalf("XXH64String is not deterministic: %#x != %#x", first, second)
	}

	fromBytes := core.XXH64Bytes([]byte(value))
	if first != fromBytes {
		t.Fatalf("XXH64String and XXH64Bytes differ for UTF-8 bytes: %#x != %#x", first, fromBytes)
	}
}
