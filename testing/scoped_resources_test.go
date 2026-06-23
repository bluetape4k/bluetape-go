package bttesting_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func TestCheckTempOutputPath(t *testing.T) {
	root := t.TempDir()

	path, err := bttesting.CheckTempOutputPath(root, "reports", "daily.json")
	if err != nil {
		t.Fatalf("CheckTempOutputPath error = %v", err)
	}
	if want := filepath.Join(root, "reports", "daily.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestCheckTempOutputPathDiagnostics(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name  string
		root  string
		parts []string
		want  string
	}{
		{name: "empty root", root: "", parts: []string{"report.txt"}, want: "root must not be empty"},
		{name: "empty part", root: root, parts: []string{""}, want: "path part must not be empty"},
		{name: "absolute part", root: root, parts: []string{"/tmp/out.txt"}, want: "path part must be relative"},
		{name: "parent traversal", root: root, parts: []string{"..", "out.txt"}, want: "path must stay under root"},
		{name: "cleaned parent traversal", root: root, parts: []string{"reports/../../out.txt"}, want: "path must stay under root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bttesting.CheckTempOutputPath(tt.root, tt.parts...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestTempOutputDirAndPath(t *testing.T) {
	dir := bttesting.TempOutputDir(t, "reports", "daily")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("TempOutputDir did not create dir: info=%v err=%v", info, err)
	}

	path := bttesting.TempOutputPath(t, "golden", "result.txt")
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("TempOutputPath did not create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write temp output path: %v", err)
	}
}

func TestSetEnvRestoresPreviousValue(t *testing.T) {
	key := "BT_TEST_SCOPED_ENV_SET"
	t.Setenv(key, "before")

	t.Run("scope", func(t *testing.T) {
		bttesting.SetEnv(t, key, "during")
		if got := os.Getenv(key); got != "during" {
			t.Fatalf("env = %q, want during", got)
		}
	})

	if got := os.Getenv(key); got != "before" {
		t.Fatalf("env after cleanup = %q, want before", got)
	}
}

func TestUnsetEnvRestoresPreviousState(t *testing.T) {
	t.Run("restore previous value", func(t *testing.T) {
		key := "BT_TEST_SCOPED_ENV_UNSET_VALUE"
		t.Setenv(key, "before")

		t.Run("scope", func(t *testing.T) {
			bttesting.UnsetEnv(t, key)
			if _, ok := os.LookupEnv(key); ok {
				t.Fatalf("env %s should be unset", key)
			}
		})

		if got := os.Getenv(key); got != "before" {
			t.Fatalf("env after cleanup = %q, want before", got)
		}
	})

	t.Run("restore missing state", func(t *testing.T) {
		key := "BT_TEST_SCOPED_ENV_UNSET_MISSING"
		_ = os.Unsetenv(key)

		t.Run("scope", func(t *testing.T) {
			bttesting.SetEnv(t, key, "during")
			if got := os.Getenv(key); got != "during" {
				t.Fatalf("env = %q, want during", got)
			}
		})

		if _, ok := os.LookupEnv(key); ok {
			t.Fatalf("env %s should be unset after cleanup", key)
		}
	})
}

func TestCheckCaptureOutput(t *testing.T) {
	captured, err := bttesting.CheckCaptureOutput(func() {
		_, _ = fmt.Fprint(os.Stdout, "hello stdout")
		_, _ = fmt.Fprint(os.Stderr, "hello stderr")
	})
	if err != nil {
		t.Fatalf("CheckCaptureOutput error = %v", err)
	}
	if captured.Stdout != "hello stdout" || captured.Stderr != "hello stderr" {
		t.Fatalf("captured = %+v", captured)
	}
}

func TestCheckCaptureOutputRejectsNilRun(t *testing.T) {
	_, err := bttesting.CheckCaptureOutput(nil)
	if err == nil || !strings.Contains(err.Error(), "run must not be nil") {
		t.Fatalf("error = %v, want nil run diagnostic", err)
	}
}

func TestCheckCaptureOutputRestoresAfterPanic(t *testing.T) {
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	expected := errors.New("boom")

	func() {
		defer func() {
			recovered := recover()
			if !errors.Is(recovered.(error), expected) {
				t.Fatalf("recovered = %v, want %v", recovered, expected)
			}
		}()

		_, _ = bttesting.CheckCaptureOutput(func() {
			panic(expected)
		})
	}()

	if os.Stdout != originalStdout || os.Stderr != originalStderr {
		t.Fatal("stdout/stderr were not restored after panic")
	}
}
