package ginadapter_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/web"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

func TestAuthenticationErrorIsRedactedProblem(t *testing.T) {
	err := ginadapter.AuthenticationError{Kind: ginadapter.JWTErrorInvalid}
	if !strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "parser") {
		t.Fatalf("Error() = %q, want stable redacted kind", err.Error())
	}
	problem := err.ProblemDetails()
	if problem.Status != http.StatusUnauthorized {
		t.Fatalf("ProblemDetails().Status = %d, want 401", problem.Status)
	}
	if strings.Contains(problem.Detail, "parser") {
		t.Fatalf("ProblemDetails().Detail = %q, contains raw parser detail", problem.Detail)
	}
}

func TestAbortWithProblemWritesRFC9457AndAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	raw := errors.New("database secret must not be exposed")
	if err := ginadapter.AbortWithProblem(c, raw); err != nil {
		t.Fatalf("AbortWithProblem() error = %v", err)
	}
	if !c.IsAborted() {
		t.Fatal("context is not aborted")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}
	if strings.Contains(recorder.Body.String(), raw.Error()) {
		t.Fatalf("body = %q, contains raw error", recorder.Body.String())
	}
}

func TestAbortWithProblemDoesNotOverwriteCommittedWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	c.String(http.StatusAccepted, "already written")
	if err := ginadapter.AbortWithProblem(c, errors.New("ignored")); err != nil {
		t.Fatalf("AbortWithProblem() error = %v", err)
	}
	if recorder.Code != http.StatusAccepted || recorder.Body.String() != "already written" {
		t.Fatalf("committed response = (%d, %q), want (202, already written)", recorder.Code, recorder.Body.String())
	}
}

func TestAbortWithProblemFallsBackAfterPreWriteMarshalFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	writeErr := ginadapter.AbortWithProblem(c, invalidProblemError{})
	if !errors.Is(writeErr, web.ErrInvalidProblem) {
		t.Fatalf("AbortWithProblem() error = %v, want invalid problem", writeErr)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want fallback 500", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "internal server error") {
		t.Fatalf("fallback body = %q, want generic detail", recorder.Body.String())
	}
}

func TestAbortWithProblemRejectsNilInputs(t *testing.T) {
	if err := ginadapter.AbortWithProblem(nil, errors.New("boom")); !errors.Is(err, web.ErrInvalidProblem) {
		t.Fatalf("nil context error = %v, want invalid problem", err)
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if err := ginadapter.AbortWithProblem(c, nil); !errors.Is(err, web.ErrInvalidProblem) {
		t.Fatalf("nil error = %v, want invalid problem", err)
	}
}

func TestJWTReaderReadsStoredReader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var reader *jwt.Reader
	c.Set(ginadapter.DefaultJWTContextKey, reader)
	got, ok := ginadapter.JWTReader(c, "")
	if !ok || got != nil {
		t.Fatalf("JWTReader() = (%v, %t), want (nil, true)", got, ok)
	}
	if got, ok := ginadapter.JWTReader(nil, ""); ok || got != nil {
		t.Fatalf("JWTReader(nil) = (%v, %t), want (nil, false)", got, ok)
	}
}

type invalidProblemError struct{}

func (invalidProblemError) Error() string { return "raw invalid problem" }

func (invalidProblemError) ProblemDetails() web.Problem {
	return web.Problem{Status: 0, Extensions: map[string]any{"raw": "must not escape"}}
}
