package ginadapter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bluetape4k/bluetape-go/jwt"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
)

func TestGinImportBoundary(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	command := exec.Command("go", "list", "-json", "./...")
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
			if imported == "github.com/gin-gonic/gin" && packageInfo.ImportPath != "github.com/bluetape4k/bluetape-go/web/gin" {
				t.Fatalf("package %s imports Gin outside web/gin", packageInfo.ImportPath)
			}
		}
	}
}

func TestContextParserAcceptsDistributedProvider(t *testing.T) {
	var _ ginadapter.ContextParser = (*jwt.DistributedProvider)(nil)
}
