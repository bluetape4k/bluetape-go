package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/bluetape4k/bluetape-go/web"
)

type exampleProblemError struct{}

func (exampleProblemError) Error() string {
	return "order total is invalid"
}

func (exampleProblemError) ProblemDetails() web.Problem {
	problem, err := web.NewProblem(http.StatusUnprocessableEntity, "Invalid order", "order total is invalid")
	if err != nil {
		panic(err)
	}
	return problem
}

func ExampleWriteProblem() {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders/42", nil)

	if err := web.WriteProblem(recorder, request, exampleProblemError{}); err != nil {
		panic(err)
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		panic("unexpected status")
	}
	if recorder.Header().Get("Content-Type") != "application/problem+json" {
		panic("unexpected content type")
	}
}

func ExampleWithRequestContextOnRequest() {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
	request.Header.Set(web.RequestIDHeader, "request-1")

	requestWithContext, value, err := web.WithRequestContextOnRequest(request, web.RequestContextOptions{
		TrustedProxy: func(*http.Request) bool { return true },
	})
	if err != nil {
		panic(err)
	}
	if requestWithContext == request || value.RequestID != "request-1" {
		panic("request context was not attached")
	}
}
