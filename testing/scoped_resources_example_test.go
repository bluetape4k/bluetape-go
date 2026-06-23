package bttesting_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func ExampleCheckTempOutputPath() {
	root := filepath.Join(os.TempDir(), "bt-example")
	path, _ := bttesting.CheckTempOutputPath(root, "reports", "result.txt")
	fmt.Println(strings.HasSuffix(path, filepath.Join("reports", "result.txt")))

	// Output:
	// true
}

func ExampleCheckCaptureOutput() {
	captured, _ := bttesting.CheckCaptureOutput(func() {
		_, _ = fmt.Fprint(os.Stdout, "ready")
	})

	fmt.Println(captured.Stdout)

	// Output:
	// ready
}
