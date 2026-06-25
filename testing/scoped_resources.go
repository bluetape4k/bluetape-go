package bttesting

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var captureOutputMu sync.Mutex

// CapturedOutput contains process stdout and stderr text captured during a test helper call.
type CapturedOutput struct {
	Stdout string
	Stderr string
}

// CheckTempOutputPath joins path parts under root and rejects absolute or parent-traversing parts.
func CheckTempOutputPath(root string, parts ...string) (string, error) {
	if root == "" {
		return "", errors.New("root must not be empty")
	}

	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve temp output root: %w", err)
	}

	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return "", errors.New("path part must not be empty")
		}
		if filepath.IsAbs(part) {
			return "", fmt.Errorf("path part must be relative: %q", part)
		}

		cleanPart := filepath.Clean(part)
		if cleanPart == "." {
			return "", fmt.Errorf("path part must not be empty: %q", part)
		}
		cleanParts = append(cleanParts, cleanPart)
	}

	path := filepath.Join(append([]string{cleanRoot}, cleanParts...)...)
	if !isUnderRoot(cleanRoot, path) {
		return "", fmt.Errorf("path must stay under root: %q", path)
	}

	return path, nil
}

// TempOutputDir creates a scoped output directory under tb.TempDir.
func TempOutputDir(tb testing.TB, parts ...string) string {
	tb.Helper()

	path := tempOutputPath(tb, parts...)
	if err := os.MkdirAll(path, 0o700); err != nil {
		tb.Fatalf("create temp output dir %q: %v", path, err)
	}

	return path
}

// TempOutputPath returns a scoped output path under tb.TempDir and creates its parent directory.
func TempOutputPath(tb testing.TB, parts ...string) string {
	tb.Helper()

	path := tempOutputPath(tb, parts...)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		tb.Fatalf("create temp output parent for %q: %v", path, err)
	}

	return path
}

// SetEnv sets an environment variable and restores its previous state during tb.Cleanup.
//
// Environment variables are process-global. Do not use this helper from parallel tests.
func SetEnv(tb testing.TB, key, value string) {
	tb.Helper()

	restore := snapshotEnv(key)
	if err := os.Setenv(key, value); err != nil {
		tb.Fatalf("set env %q: %v", key, err)
	}
	tb.Cleanup(restore)
}

// UnsetEnv unsets an environment variable and restores its previous state during tb.Cleanup.
//
// Environment variables are process-global. Do not use this helper from parallel tests.
func UnsetEnv(tb testing.TB, key string) {
	tb.Helper()

	restore := snapshotEnv(key)
	if err := os.Unsetenv(key); err != nil {
		tb.Fatalf("unset env %q: %v", key, err)
	}
	tb.Cleanup(restore)
}

// CaptureOutput captures process stdout and stderr while run executes and fails tb on setup errors.
//
// Stdout and stderr are process-global. Do not use this helper from parallel tests or while other
// goroutines may write to os.Stdout/os.Stderr.
func CaptureOutput(tb testing.TB, run func()) CapturedOutput {
	tb.Helper()

	output, err := CheckCaptureOutput(run)
	if err != nil {
		tb.Fatalf("capture output: %v", err)
	}

	return output
}

// CheckCaptureOutput captures process stdout and stderr while run executes.
//
// The helper serializes capture calls and restores os.Stdout/os.Stderr before returning or
// re-panicking. Stdout and stderr are process-global, so callers must not use it from parallel tests
// or while unrelated goroutines may write to the process streams.
func CheckCaptureOutput(run func()) (CapturedOutput, error) {
	if run == nil {
		return CapturedOutput{}, errors.New("run must not be nil")
	}

	captureOutputMu.Lock()
	defer captureOutputMu.Unlock()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return CapturedOutput{}, fmt.Errorf("open stdout pipe: %w", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return CapturedOutput{}, fmt.Errorf("open stderr pipe: %w", err)
	}

	stdoutDone := readCapturedFile(stdoutR)
	stderrDone := readCapturedFile(stderrR)

	os.Stdout = stdoutW
	os.Stderr = stderrW

	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
		}()
		run()
	}()

	closeErr := restoreCapturedFiles(originalStdout, originalStderr, stdoutW, stderrW)
	stdoutResult := <-stdoutDone
	stderrResult := <-stderrDone

	if panicValue != nil {
		panic(panicValue)
	}

	if err := errors.Join(closeErr, stdoutResult.err, stderrResult.err); err != nil {
		return CapturedOutput{}, fmt.Errorf("capture output: %w", err)
	}

	return CapturedOutput{Stdout: stdoutResult.text, Stderr: stderrResult.text}, nil
}

func tempOutputPath(tb testing.TB, parts ...string) string {
	tb.Helper()

	path, err := CheckTempOutputPath(tb.TempDir(), parts...)
	if err != nil {
		tb.Fatalf("temp output path: %v", err)
	}
	return path
}

func isUnderRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func snapshotEnv(key string) func() {
	previous, existed := os.LookupEnv(key)
	return func() {
		if existed {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	}
}

type captureReadResult struct {
	text string
	err  error
}

func readCapturedFile(file *os.File) <-chan captureReadResult {
	done := make(chan captureReadResult, 1)
	go func() {
		defer func() {
			_ = file.Close()
		}()

		bytes, err := io.ReadAll(file)
		done <- captureReadResult{text: string(bytes), err: err}
	}()
	return done
}

func restoreCapturedFiles(stdout, stderr, stdoutW, stderrW *os.File) error {
	os.Stdout = stdout
	os.Stderr = stderr
	return errors.Join(stdoutW.Close(), stderrW.Close())
}
