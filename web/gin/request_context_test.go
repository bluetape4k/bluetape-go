package ginadapter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/web"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

func TestRequestContextAddsFrameworkNeutralValueAndRestoresRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var original *http.Request
	var restored bool
	router.Use(func(c *gin.Context) {
		original = c.Request
		defer func() { restored = c.Request == original }()
		c.Next()
	})
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		GenerateID: func() (string, error) { return "request-1", nil },
	}))
	router.GET("/orders", func(c *gin.Context) {
		value, ok := web.RequestContextFromContext(c.Request.Context())
		if !ok || value.RequestID != "request-1" || value.CorrelationID != "request-1" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), struct{}{}, "temporary"))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if !restored {
		t.Fatal("middleware did not restore original request pointer")
	}
}

func TestRequestContextFailsClosedForUntrustedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	downstreamCalls := 0
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		GenerateID: func() (string, error) { return "request-2", nil },
	}))
	router.GET("/orders", func(c *gin.Context) {
		downstreamCalls++
		value, _ := web.RequestContextFromContext(c.Request.Context())
		if value.AuthSubject != "" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set(web.AuthSubjectHeader, "attacker")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent || downstreamCalls != 1 {
		t.Fatalf("response = (%d, downstream=%d), want (204, 1)", recorder.Code, downstreamCalls)
	}
}

func TestRequestContextAcceptsTrustedServerPredicateOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		TrustedProxy: func(*http.Request) bool { return true },
		GenerateID:   func() (string, error) { return "request-3", nil },
	}))
	router.GET("/orders", func(c *gin.Context) {
		value, _ := web.RequestContextFromContext(c.Request.Context())
		if value.AuthSubject != "trusted-subject" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set(web.AuthSubjectHeader, "trusted-subject")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

func TestRequestContextPropagatesCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		GenerateID: func() (string, error) { return "request-4", nil },
	}))
	router.GET("/orders", func(c *gin.Context) {
		if c.Request.Context().Err() != context.Canceled {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/orders", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

func TestRequestContextRejectsInvalidConfigurationWithoutCallingDownstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	downstreamCalls := 0
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		RequestIDHeader: "invalid header",
	}))
	router.GET("/orders", func(c *gin.Context) {
		downstreamCalls++
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if downstreamCalls != 0 {
		t.Fatalf("downstream calls = %d, want 0", downstreamCalls)
	}
	if strings.Contains(recorder.Body.String(), "invalid header") {
		t.Fatalf("response exposed internal validation detail: %q", recorder.Body.String())
	}
}

func TestRequestContextRestoresRequestAfterPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var restored bool
	router.Use(func(c *gin.Context) {
		original := c.Request
		defer func() {
			restored = c.Request == original
			_ = recover()
		}()
		c.Next()
	})
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		GenerateID: func() (string, error) { return "request-5", nil },
	}))
	router.GET("/orders", func(c *gin.Context) { panic("route panic") })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if !restored {
		t.Fatal("middleware did not restore request after panic")
	}
}
