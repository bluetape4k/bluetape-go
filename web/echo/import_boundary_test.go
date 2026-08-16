package echoadapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

var (
	_ echo.HandlerFunc = func(echo.Context) error { return nil }
	_ echo.HandlerFunc = echo.WrapHandler(http.NotFoundHandler())
	_ web.ProblemError = conformanceProblemError{}
)

func TestEchoImportBoundary(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	command := exec.CommandContext(context.Background(), "go", "list", "-json", "./...")
	command.Dir = moduleRoot
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -json ./...: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var packageInfo struct {
			ImportPath string
			Imports    []string
		}
		err := decoder.Decode(&packageInfo)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode package metadata: %v", err)
		}
		for _, imported := range packageInfo.Imports {
			if imported == "github.com/labstack/echo/v4" && packageInfo.ImportPath != "github.com/bluetape4k/bluetape-go/web/echo" {
				t.Fatalf("package %s imports Echo outside web/echo", packageInfo.ImportPath)
			}
		}
	}
}
