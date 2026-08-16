package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/web"
)

func TestNewProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		valid  bool
	}{
		{name: "below range", status: http.StatusContinue - 1},
		{name: "above range", status: 600},
		{name: "minimum", status: 100, valid: true},
		{name: "maximum", status: 599, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem, err := web.NewProblem(tt.status, "", "invalid input")
			if !tt.valid {
				if !errors.Is(err, web.ErrInvalidProblem) {
					t.Fatalf("NewProblem(%d) error = %v, want ErrInvalidProblem", tt.status, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewProblem(%d) error = %v", tt.status, err)
			}
			if problem.Type != "about:blank" {
				t.Errorf("Type = %q, want about:blank", problem.Type)
			}
			if problem.Title != http.StatusText(tt.status) {
				t.Errorf("Title = %q, want %q", problem.Title, http.StatusText(tt.status))
			}
			if problem.Status != tt.status {
				t.Errorf("Status = %d, want %d", problem.Status, tt.status)
			}
			if problem.Detail != "invalid input" {
				t.Errorf("Detail = %q, want invalid input", problem.Detail)
			}
		})
	}
}

type problemErrorStub struct {
	problem web.Problem
}

func (e problemErrorStub) Error() string {
	return "problem error: " + e.problem.Detail
}

func (e problemErrorStub) ProblemDetails() web.Problem {
	return e.problem
}

func TestProblemFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantDetail string
		wantType   string
	}{
		{
			name:       "generic error is redacted",
			err:        fmt.Errorf("database password leaked: %w", errors.New("secret")),
			wantStatus: http.StatusInternalServerError,
			wantDetail: "Internal Server Error",
			wantType:   "about:blank",
		},
		{
			name:       "canceled",
			err:        fmt.Errorf("request ended: %w", context.Canceled),
			wantStatus: http.StatusRequestTimeout,
			wantDetail: "Request canceled",
			wantType:   "about:blank",
		},
		{
			name:       "deadline exceeded",
			err:        fmt.Errorf("upstream ended: %w", context.DeadlineExceeded),
			wantStatus: http.StatusGatewayTimeout,
			wantDetail: "Request deadline exceeded",
			wantType:   "about:blank",
		},
		{
			name: "problem error is preserved",
			err: problemErrorStub{problem: web.Problem{
				Type:   "https://example.com/problems/invalid-order",
				Title:  "Invalid order",
				Status: http.StatusUnprocessableEntity,
				Detail: "order total is invalid",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantDetail: "order total is invalid",
			wantType:   "https://example.com/problems/invalid-order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem := web.ProblemFromError(tt.err)
			if problem.Status != tt.wantStatus {
				t.Errorf("Status = %d, want %d", problem.Status, tt.wantStatus)
			}
			if problem.Detail != tt.wantDetail {
				t.Errorf("Detail = %q, want %q", problem.Detail, tt.wantDetail)
			}
			if problem.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", problem.Type, tt.wantType)
			}
		})
	}
}

func TestWriteProblem(t *testing.T) {
	t.Parallel()

	t.Run("writes problem response after serialization", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders?x=1", nil)
		request.URL.Fragment = "fragment"
		recorder := httptest.NewRecorder()
		problemErr := problemErrorStub{problem: web.Problem{
			Type:   "https://example.test/problems/invalid-order",
			Title:  "Invalid order",
			Status: http.StatusUnprocessableEntity,
			Detail: "order total is invalid",
			Extensions: map[string]any{
				"zeta":  "last",
				"alpha": true,
			},
		}}

		if err := web.WriteProblem(recorder, request, problemErr); err != nil {
			t.Fatalf("WriteProblem() error = %v", err)
		}
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
		}
		if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
			t.Errorf("Content-Type = %q, want application/problem+json", got)
		}

		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("response body is not JSON: %v", err)
		}
		if body["instance"] != "/orders" {
			t.Errorf("instance = %v, want path-only /orders", body["instance"])
		}
		if body["alpha"] != true || body["zeta"] != "last" {
			t.Errorf("extensions = %#v, want alpha and zeta", body)
		}
	})

	t.Run("does not echo credential-like query values", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
			"https://example.test/orders?token=raw-token&password=secret", nil)
		recorder := httptest.NewRecorder()
		if err := web.WriteProblem(recorder, request, errors.New("private parser detail")); err != nil {
			t.Fatalf("WriteProblem() error = %v", err)
		}
		body := recorder.Body.String()
		if strings.Contains(body, "raw-token") || strings.Contains(body, "secret") || strings.Contains(body, "?") {
			t.Fatalf("problem body = %q, contains query credentials", body)
		}
		var decoded map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		if decoded["instance"] != "/orders" {
			t.Fatalf("instance = %v, want /orders", decoded["instance"])
		}
	})

	t.Run("rejects invalid input before response starts", func(t *testing.T) {
		tests := []struct {
			name    string
			problem web.Problem
		}{
			{
				name: "zero status",
				problem: web.Problem{
					Detail: "invalid",
				},
			},
			{
				name: "reserved extension",
				problem: web.Problem{
					Status: http.StatusInternalServerError,
					Extensions: map[string]any{
						"type": "override",
					},
				},
			},
			{
				name: "cyclic extension",
				problem: func() web.Problem {
					cyclic := map[string]any{}
					cyclic["self"] = cyclic
					return web.Problem{
						Status: http.StatusInternalServerError,
						Extensions: map[string]any{
							"metadata": cyclic,
						},
					}
				}(),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				err := web.WriteProblem(recorder, nil, problemErrorStub{problem: tt.problem})
				if !errors.Is(err, web.ErrInvalidProblem) {
					t.Fatalf("WriteProblem() error = %v, want ErrInvalidProblem", err)
				}
				if recorder.Code != http.StatusOK {
					t.Errorf("status = %d, want untouched recorder status %d", recorder.Code, http.StatusOK)
				}
				if got := recorder.Header().Get("Content-Type"); got != "" {
					t.Errorf("Content-Type = %q, want empty", got)
				}
			})
		}
	})

	t.Run("rejects nil inputs", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		if err := web.WriteProblem(recorder, nil, nil); !errors.Is(err, web.ErrInvalidProblem) {
			t.Errorf("nil error result = %v, want ErrInvalidProblem", err)
		}
		var writer http.ResponseWriter
		if err := web.WriteProblem(writer, nil, errors.New("internal")); !errors.Is(err, web.ErrInvalidProblem) {
			t.Errorf("nil writer result = %v, want ErrInvalidProblem", err)
		}
	})
}
