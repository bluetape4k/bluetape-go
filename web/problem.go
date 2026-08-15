package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode"
	"unicode/utf8"
)

const (
	defaultProblemType = "about:blank"
	internalErrorTitle = "Internal Server Error"
	problemMediaType   = "application/problem+json"
)

var (
	// ErrInvalidProblem 은 problem 입력이 HTTP 응답으로 표현될 수 없을 때 반환된다.
	ErrInvalidProblem = errors.New("web: invalid problem")
)

// Problem 은 RFC 9457 Problem Details 응답을 표현한다.
type Problem struct {
	Type       string         `json:"type,omitempty"`
	Title      string         `json:"title,omitempty"`
	Status     int            `json:"status"`
	Detail     string         `json:"detail,omitempty"`
	Instance   string         `json:"instance,omitempty"`
	Extensions map[string]any `json:"-"`
}

// ProblemError 는 호출자가 공개해도 되는 Problem 세부 정보를 제공하는 오류다.
type ProblemError interface {
	error
	ProblemDetails() Problem
}

// NewProblem 은 유효한 HTTP status와 problem 세부 정보로 Problem을 만든다.
func NewProblem(status int, title, detail string) (Problem, error) {
	problem := Problem{Status: status, Title: title, Detail: detail}
	return normalizeProblem(problem)
}

// ProblemFromError 는 오류를 공개 가능한 Problem으로 매핑한다.
func ProblemFromError(err error) Problem {
	if errors.Is(err, context.DeadlineExceeded) {
		return knownProblem(http.StatusGatewayTimeout, "Request deadline exceeded")
	}
	if errors.Is(err, context.Canceled) {
		return knownProblem(http.StatusRequestTimeout, "Request canceled")
	}

	var problemErr ProblemError
	if errors.As(err, &problemErr) {
		return problemErr.ProblemDetails()
	}

	return knownProblem(http.StatusInternalServerError, internalErrorTitle)
}

// WriteProblem 은 Problem을 JSON response로 직렬화한 뒤 HTTP 응답에 쓴다.
func WriteProblem(w http.ResponseWriter, req *http.Request, err error) error {
	if w == nil || err == nil {
		return ErrInvalidProblem
	}

	problem := ProblemFromError(err)
	if problem.Instance == "" && req != nil && req.URL != nil {
		problem.Instance = req.URL.RequestURI()
	}

	body, marshalErr := marshalProblem(problem)
	if marshalErr != nil {
		return marshalErr
	}

	w.Header().Set("Content-Type", problemMediaType)
	w.WriteHeader(problem.Status)
	n, writeErr := w.Write(body)
	if writeErr != nil {
		return writeErr
	}
	if n != len(body) {
		return io.ErrShortWrite
	}
	return nil
}

// MarshalJSON 은 Problem을 표준 멤버와 검증된 extension member로 직렬화한다.
func (p Problem) MarshalJSON() ([]byte, error) {
	return marshalProblem(p)
}

func knownProblem(status int, detail string) Problem {
	return Problem{
		Type:   defaultProblemType,
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	}
}

func normalizeProblem(problem Problem) (Problem, error) {
	if err := validateStatus(problem.Status); err != nil {
		return Problem{}, err
	}
	if problem.Type == "" {
		problem.Type = defaultProblemType
	}
	if problem.Title == "" {
		problem.Title = http.StatusText(problem.Status)
	}
	if err := validateExtensions(problem.Extensions); err != nil {
		return Problem{}, err
	}
	return problem, nil
}

func marshalProblem(problem Problem) ([]byte, error) {
	normalized, err := normalizeProblem(problem)
	if err != nil {
		return nil, err
	}

	values := make(map[string]any, len(normalized.Extensions)+5)
	values["type"] = normalized.Type
	values["title"] = normalized.Title
	values["status"] = normalized.Status
	if normalized.Detail != "" {
		values["detail"] = normalized.Detail
	}
	if normalized.Instance != "" {
		values["instance"] = normalized.Instance
	}
	for key, value := range normalized.Extensions {
		values[key] = value
	}

	body, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidProblem, err)
	}
	return body, nil
}

func validateStatus(status int) error {
	if status < 100 || status > 599 {
		return fmt.Errorf("%w: status %d must be between 100 and 599", ErrInvalidProblem, status)
	}
	return nil
}

func validateExtensions(extensions map[string]any) error {
	for key := range extensions {
		if key == "" || isReservedProblemMember(key) || !utf8.ValidString(key) {
			return fmt.Errorf("%w: invalid extension key %q", ErrInvalidProblem, key)
		}
		for _, r := range key {
			if unicode.IsControl(r) {
				return fmt.Errorf("%w: invalid extension key %q", ErrInvalidProblem, key)
			}
		}
	}
	return nil
}

func isReservedProblemMember(key string) bool {
	switch key {
	case "type", "title", "status", "detail", "instance":
		return true
	default:
		return false
	}
}
