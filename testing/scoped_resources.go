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

// CapturedOutput struct 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CapturedOutput struct {
	Stdout string
	Stderr string
}

// CheckTempOutputPath CheckTempOutputPath 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - root: CheckTempOutputPath가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - parts: CheckTempOutputPath 동작에 필요한 parts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// TempOutputDir TempOutputDir 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: TempOutputDir 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - parts: TempOutputDir 동작에 필요한 parts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func TempOutputDir(tb testing.TB, parts ...string) string {
	tb.Helper()

	path := tempOutputPath(tb, parts...)
	if err := os.MkdirAll(path, 0o700); err != nil {
		tb.Fatalf("create temp output dir %q: %v", path, err)
	}

	return path
}

// TempOutputPath TempOutputPath 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: TempOutputPath 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - parts: TempOutputPath 동작에 필요한 parts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func TempOutputPath(tb testing.TB, parts ...string) string {
	tb.Helper()

	path := tempOutputPath(tb, parts...)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		tb.Fatalf("create temp output parent for %q: %v", path, err)
	}

	return path
}

// SetEnv SetEnv 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: SetEnv 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - key: SetEnv가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - value: SetEnv가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
func SetEnv(tb testing.TB, key, value string) {
	tb.Helper()

	restore := snapshotEnv(key)
	if err := os.Setenv(key, value); err != nil {
		tb.Fatalf("set env %q: %v", key, err)
	}
	tb.Cleanup(restore)
}

// UnsetEnv UnsetEnv 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: UnsetEnv 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - key: UnsetEnv가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
func UnsetEnv(tb testing.TB, key string) {
	tb.Helper()

	restore := snapshotEnv(key)
	if err := os.Unsetenv(key); err != nil {
		tb.Fatalf("unset env %q: %v", key, err)
	}
	tb.Cleanup(restore)
}

// CaptureOutput CaptureOutput 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: CaptureOutput 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - run: CaptureOutput 동작에 필요한 run 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func CaptureOutput(tb testing.TB, run func()) CapturedOutput {
	tb.Helper()

	output, err := CheckCaptureOutput(run)
	if err != nil {
		tb.Fatalf("capture output: %v", err)
	}

	return output
}

// CheckCaptureOutput CheckCaptureOutput 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - run: CheckCaptureOutput 동작에 필요한 run 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
