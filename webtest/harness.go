package webtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

const defaultScenarioTimeout = 2 * time.Second

// Adapter wraps the next HTTP handler and connects it to a conformance scenario.
type Adapter func(http.Handler) http.Handler

// Scenario defines 하나의 독립된 HTTP middleware conformance 실행.
type Scenario struct {
	Name       string
	Adapter    Adapter
	NewRequest func(context.Context) *http.Request
	Next       http.Handler
	Timeout    time.Duration
	Assert     func(*testing.T, Observation)
}

// Observation contains scenario 실행 뒤 복사한 응답과 next 호출 관찰값.
type Observation struct {
	StatusCode  int
	Header      http.Header
	Body        []byte
	NextCalls   int
	NextRequest *http.Request
}

// Run executes 각 scenario를 독립된 recorder, request, observer로 실행한다.
//
// Timeout이 0 이하이면 2초를 사용한다. 실행이 timeout을 넘기면 request
// context를 취소하고 같은 상한 동안 cleanup을 기다린 뒤 테스트를 실패시킨다.
// 늦게 반환한 handler를 성공으로 바꾸거나 강제로 종료하지 않는다.
func Run(t *testing.T, scenarios ...Scenario) {
	t.Helper()
	if len(scenarios) == 0 {
		t.Fatal("webtest: at least one scenario is required")
	}

	for _, scenario := range scenarios {
		scenario := scenario
		if scenario.Name == "" {
			t.Fatal("webtest: scenario name must not be empty")
		}
		t.Run(scenario.Name, func(t *testing.T) {
			runScenario(t, scenario)
		})
	}
}

func runScenario(t *testing.T, scenario Scenario) {
	t.Helper()
	if scenario.Adapter == nil {
		t.Fatal("webtest: scenario adapter must not be nil")
	}
	if scenario.NewRequest == nil {
		t.Fatal("webtest: scenario request factory must not be nil")
	}
	if scenario.Next == nil {
		t.Fatal("webtest: scenario next handler must not be nil")
	}
	if scenario.Assert == nil {
		t.Fatal("webtest: scenario assertion must not be nil")
	}

	timeout := scenario.Timeout
	if timeout <= 0 {
		timeout = defaultScenarioTimeout
	}
	requestContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := scenario.NewRequest(requestContext)
	if req == nil {
		t.Fatal("webtest: scenario request factory returned nil")
	}

	var nextMu sync.Mutex
	nextCalls := 0
	var nextRequest *http.Request
	next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		nextMu.Lock()
		nextCalls++
		nextRequest = req
		nextMu.Unlock()
		scenario.Next.ServeHTTP(w, req)
	})

	handler := scenario.Adapter(next)
	if handler == nil {
		t.Fatal("webtest: scenario adapter returned nil handler")
	}

	recorder := httptest.NewRecorder()
	execution := make(chan executionResult, 1)
	go func() {
		result := executionResult{}
		defer func() {
			result.panicValue = recover()
			execution <- result
		}()
		handler.ServeHTTP(recorder, req)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-execution:
		if result.panicValue != nil {
			t.Fatalf("webtest: handler panicked: %v", result.panicValue)
		}
		scenario.Assert(t, snapshotObservation(recorder, &nextMu, &nextCalls, &nextRequest))
	case <-timer.C:
		cancel()
		cleanupTimer := time.NewTimer(timeout)
		defer cleanupTimer.Stop()
		select {
		case <-execution:
			t.Fatalf("webtest: scenario %q exceeded timeout %s", scenario.Name, timeout)
		case <-cleanupTimer.C:
			t.Fatalf("webtest: scenario %q did not return after cancellation within %s", scenario.Name, timeout)
		}
	}
}

type executionResult struct {
	panicValue any
}

func snapshotObservation(recorder *httptest.ResponseRecorder, nextMu *sync.Mutex, nextCalls *int, nextRequest **http.Request) Observation {
	nextMu.Lock()
	calls := *nextCalls
	req := *nextRequest
	nextMu.Unlock()

	return Observation{
		StatusCode:  recorder.Code,
		Header:      cloneHeader(recorder.Header()),
		Body:        append([]byte(nil), recorder.Body.Bytes()...),
		NextCalls:   calls,
		NextRequest: req,
	}
}

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return nil
	}
	clone := make(http.Header, len(header))
	for key, values := range header {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}

// CloseTracker records 소유자가 명확한 io.ReadCloser의 close 호출.
type CloseTracker struct {
	reader     io.Reader
	mu         sync.Mutex
	closed     bool
	closeCount int
}

// NewCloseTracker creates reader를 감싼 close 추적기.
func NewCloseTracker(reader io.Reader) *CloseTracker {
	if reader == nil {
		reader = bytes.NewReader(nil)
	}
	return &CloseTracker{reader: reader}
}

// Read는 감싼 reader에서 데이터를 읽는다.
func (t *CloseTracker) Read(p []byte) (int, error) {
	if t == nil {
		return 0, errors.New("webtest: nil close tracker")
	}
	return t.reader.Read(p)
}

// Close increments close 호출 횟수를 증가시키고 nil 오류를 반환한다.
func (t *CloseTracker) Close() error {
	if t == nil {
		return errors.New("webtest: nil close tracker")
	}
	t.mu.Lock()
	t.closed = true
	t.closeCount++
	t.mu.Unlock()
	return nil
}

// Closed reports 한 번 이상 Close가 호출됐는지 여부.
func (t *CloseTracker) Closed() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// CloseCount returns Close 호출 횟수.
func (t *CloseTracker) CloseCount() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closeCount
}
