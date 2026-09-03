package cloudwatchexample_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const (
	cloudWatchPayloadLimit = 1 << 20
	metricLimit            = 1000
	dimensionLimit         = 30
	logEventLimit          = 10_000
	logEventOverhead       = 26
	logSpanLimit           = 24 * time.Hour
)

var (
	errInvalidMetric = errors.New("cloudwatch example: invalid metric request")
	errMetricPayload = errors.New("cloudwatch example: metric payload exceeds limit")
	errMetricPublish = errors.New("cloudwatch example: metric publish failed")
	errInvalidLog    = errors.New("cloudwatch example: invalid log request")
	errLogPayload    = errors.New("cloudwatch example: log payload exceeds limit")
	errLogPublish    = errors.New("cloudwatch example: log publish failed")
)

// operationError deliberately keeps provider text out of example diagnostics.
// Applications can attach a caller-owned low-cardinality logger or hook around
// these helpers when they need operational telemetry.
type operationError struct {
	operation string
	kind      error
}

func (e *operationError) Error() string { return e.operation + ": " + e.kind.Error() }
func (e *operationError) Unwrap() error { return e.kind }

type metricClient interface {
	PutMetricData(context.Context, *cloudwatch.PutMetricDataInput, ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}

type logsClient interface {
	PutLogEvents(context.Context, *cloudwatchlogs.PutLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}

var (
	_ metricClient = cloudwatch.NewFromConfig(aws.Config{})
	_ logsClient   = cloudwatchlogs.NewFromConfig(aws.Config{})
)

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func buildMetricInput(namespace string, data []cloudwatchtypes.MetricDatum) (*cloudwatch.PutMetricDataInput, error) {
	if namespace == "" || len(namespace) > 255 || !utf8.ValidString(namespace) {
		return nil, errInvalidMetric
	}
	if len(data) == 0 || len(data) > metricLimit {
		return nil, errInvalidMetric
	}

	copyData := make([]cloudwatchtypes.MetricDatum, len(data))
	for i, datum := range data {
		copyData[i] = cloneMetricDatum(datum)
	}
	for i := range copyData {
		datum := &copyData[i]
		if datum.MetricName == nil || *datum.MetricName == "" || !utf8.ValidString(*datum.MetricName) {
			return nil, errInvalidMetric
		}
		if len(datum.Dimensions) > dimensionLimit {
			return nil, errInvalidMetric
		}
		seenDimensions := make(map[string]struct{}, len(datum.Dimensions))
		for _, dimension := range datum.Dimensions {
			if dimension.Name == nil || dimension.Value == nil || *dimension.Name == "" || *dimension.Value == "" ||
				!validDimensionToken(*dimension.Name) || !validDimensionToken(*dimension.Value) {
				return nil, errInvalidMetric
			}
			if _, exists := seenDimensions[*dimension.Name]; exists {
				return nil, errInvalidMetric
			}
			seenDimensions[*dimension.Name] = struct{}{}
		}
		if datum.Value != nil && !validMetricValue(*datum.Value) {
			return nil, errInvalidMetric
		}
		if len(datum.Values) != len(datum.Counts) && len(datum.Counts) != 0 {
			return nil, errInvalidMetric
		}
		if len(datum.Values) > 150 {
			return nil, errInvalidMetric
		}
		for _, value := range datum.Values {
			if !validMetricValue(value) {
				return nil, errInvalidMetric
			}
		}
		for _, value := range datum.Counts {
			if !validMetricValue(value) || value < 0 {
				return nil, errInvalidMetric
			}
		}
		if set := datum.StatisticValues; set != nil {
			for _, value := range []*float64{set.Maximum, set.Minimum, set.SampleCount, set.Sum} {
				if value == nil || !validMetricValue(*value) || *value < 0 && value != set.Minimum {
					return nil, errInvalidMetric
				}
			}
		}
		if datum.Value == nil && len(datum.Values) == 0 && datum.StatisticValues == nil {
			return nil, errInvalidMetric
		}
		if datum.Value != nil && (len(datum.Values) != 0 || datum.StatisticValues != nil) {
			return nil, errInvalidMetric
		}
		if len(datum.Values) != 0 && datum.StatisticValues != nil {
			return nil, errInvalidMetric
		}
	}

	input := &cloudwatch.PutMetricDataInput{Namespace: aws.String(namespace), MetricData: copyData}
	if payload, err := json.Marshal(input); err != nil || len(payload) > cloudWatchPayloadLimit {
		return nil, errMetricPayload
	}
	return input, nil
}

func validMetricValue(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validDimensionToken(value string) bool {
	if value == "" || value[0] == ':' || !utf8.ValidString(value) {
		return false
	}
	nonWhitespace := false
	for i := 0; i < len(value); i++ {
		if value[i] >= utf8.RuneSelf || value[i] < 0x20 || value[i] == 0x7f {
			return false
		}
		if value[i] != ' ' && value[i] != '\t' && value[i] != '\n' && value[i] != '\r' {
			nonWhitespace = true
		}
	}
	return nonWhitespace
}

func putMetricData(ctx context.Context, client metricClient, namespace string, data []cloudwatchtypes.MetricDatum) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if client == nil {
		return errInvalidMetric
	}
	input, err := buildMetricInput(namespace, data)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, callErr := client.PutMetricData(ctx, input); callErr != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		return &operationError{operation: "cloudwatch.PutMetricData", kind: errMetricPublish}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func buildLogInput(group, stream string, events []cloudwatchlogstypes.InputLogEvent) (*cloudwatchlogs.PutLogEventsInput, error) {
	if group == "" || stream == "" || !utf8.ValidString(group) || !utf8.ValidString(stream) {
		return nil, errInvalidLog
	}
	if len(events) == 0 || len(events) > logEventLimit {
		return nil, errInvalidLog
	}

	copyEvents := make([]cloudwatchlogstypes.InputLogEvent, len(events))
	for i, event := range events {
		copyEvents[i] = cloneLogEvent(event)
	}
	var firstTimestamp, previousTimestamp int64
	var totalBytes int
	for i := range copyEvents {
		event := &copyEvents[i]
		if event.Message == nil || event.Timestamp == nil || !utf8.ValidString(*event.Message) {
			return nil, errInvalidLog
		}
		messageBytes := len(*event.Message)
		if messageBytes > cloudWatchPayloadLimit {
			return nil, errLogPayload
		}
		if i > 0 && *event.Timestamp < previousTimestamp {
			return nil, errInvalidLog
		}
		if i == 0 {
			firstTimestamp = *event.Timestamp
		}
		previousTimestamp = *event.Timestamp
		if previousTimestamp-firstTimestamp > logSpanLimit.Milliseconds() {
			return nil, errInvalidLog
		}
		if messageBytes > cloudWatchPayloadLimit-totalBytes-logEventOverhead {
			return nil, errLogPayload
		}
		totalBytes += messageBytes + logEventOverhead
	}
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
		LogEvents:     copyEvents,
		// SequenceToken is intentionally omitted. CloudWatch Logs now accepts
		// parallel puts for a stream and ignores the legacy token contract.
	}
	if payload, err := json.Marshal(input); err != nil || len(payload) > cloudWatchPayloadLimit {
		return nil, errLogPayload
	}
	return input, nil
}

func putLogEvents(ctx context.Context, client logsClient, group, stream string, events []cloudwatchlogstypes.InputLogEvent) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if client == nil {
		return errInvalidLog
	}
	input, err := buildLogInput(group, stream, events)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, callErr := client.PutLogEvents(ctx, input); callErr != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		return &operationError{operation: "cloudwatchlogs.PutLogEvents", kind: errLogPublish}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

type metricFake struct {
	mu      sync.Mutex
	calls   int
	input   *cloudwatch.PutMetricDataInput
	err     error
	output  *cloudwatch.PutMetricDataOutput
	entered chan struct{}
	after   func()
}

func (f *metricFake) PutMetricData(_ context.Context, input *cloudwatch.PutMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error) {
	f.mu.Lock()
	f.calls++
	f.input = cloneMetricInput(input)
	entered := f.entered
	err := f.err
	output := f.output
	f.mu.Unlock()
	if entered != nil {
		close(entered)
	}
	if f.after != nil {
		f.after()
	}
	if err == nil && output == nil {
		output = &cloudwatch.PutMetricDataOutput{}
	}
	return output, err
}

func cloneMetricInput(input *cloudwatch.PutMetricDataInput) *cloudwatch.PutMetricDataInput {
	if input == nil {
		return nil
	}
	copyInput := *input
	copyInput.MetricData = make([]cloudwatchtypes.MetricDatum, len(input.MetricData))
	for i, datum := range input.MetricData {
		copyInput.MetricData[i] = cloneMetricDatum(datum)
	}
	return &copyInput
}

func cloneMetricDatum(datum cloudwatchtypes.MetricDatum) cloudwatchtypes.MetricDatum {
	copyDatum := datum
	if datum.MetricName != nil {
		value := *datum.MetricName
		copyDatum.MetricName = &value
	}
	copyDatum.Values = append([]float64(nil), datum.Values...)
	copyDatum.Counts = append([]float64(nil), datum.Counts...)
	copyDatum.Dimensions = make([]cloudwatchtypes.Dimension, len(datum.Dimensions))
	for i, dimension := range datum.Dimensions {
		copyDimension := dimension
		if dimension.Name != nil {
			value := *dimension.Name
			copyDimension.Name = &value
		}
		if dimension.Value != nil {
			value := *dimension.Value
			copyDimension.Value = &value
		}
		copyDatum.Dimensions[i] = copyDimension
	}
	if datum.Timestamp != nil {
		value := *datum.Timestamp
		copyDatum.Timestamp = &value
	}
	if datum.Value != nil {
		value := *datum.Value
		copyDatum.Value = &value
	}
	if datum.StorageResolution != nil {
		value := *datum.StorageResolution
		copyDatum.StorageResolution = &value
	}
	if datum.StatisticValues != nil {
		set := *datum.StatisticValues
		copyDatum.StatisticValues = cloneStatisticSet(set)
	}
	return copyDatum
}

func cloneStatisticSet(set cloudwatchtypes.StatisticSet) *cloudwatchtypes.StatisticSet {
	copySet := set
	if set.Maximum != nil {
		value := *set.Maximum
		copySet.Maximum = &value
	}
	if set.Minimum != nil {
		value := *set.Minimum
		copySet.Minimum = &value
	}
	if set.SampleCount != nil {
		value := *set.SampleCount
		copySet.SampleCount = &value
	}
	if set.Sum != nil {
		value := *set.Sum
		copySet.Sum = &value
	}
	return &copySet
}

type logsFake struct {
	mu      sync.Mutex
	calls   int
	input   *cloudwatchlogs.PutLogEventsInput
	err     error
	output  *cloudwatchlogs.PutLogEventsOutput
	entered chan struct{}
	after   func()
}

func (f *logsFake) PutLogEvents(_ context.Context, input *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	f.mu.Lock()
	f.calls++
	f.input = cloneLogInput(input)
	entered := f.entered
	err := f.err
	output := f.output
	f.mu.Unlock()
	if entered != nil {
		close(entered)
	}
	if f.after != nil {
		f.after()
	}
	if err == nil && output == nil {
		output = &cloudwatchlogs.PutLogEventsOutput{}
	}
	return output, err
}

func cloneLogInput(input *cloudwatchlogs.PutLogEventsInput) *cloudwatchlogs.PutLogEventsInput {
	if input == nil {
		return nil
	}
	copyInput := *input
	copyInput.LogEvents = make([]cloudwatchlogstypes.InputLogEvent, len(input.LogEvents))
	for i, event := range input.LogEvents {
		copyInput.LogEvents[i] = cloneLogEvent(event)
	}
	return &copyInput
}

func cloneLogEvent(event cloudwatchlogstypes.InputLogEvent) cloudwatchlogstypes.InputLogEvent {
	copyEvent := event
	if event.Message != nil {
		value := *event.Message
		copyEvent.Message = &value
	}
	if event.Timestamp != nil {
		value := *event.Timestamp
		copyEvent.Timestamp = &value
	}
	return copyEvent
}

func Example_putMetricData() {
	client := &metricFake{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data := []cloudwatchtypes.MetricDatum{{
		MetricName: aws.String("queue.depth"),
		Unit:       cloudwatchtypes.StandardUnitCount,
		Value:      aws.Float64(3),
		Dimensions: []cloudwatchtypes.Dimension{{Name: aws.String("queue"), Value: aws.String("orders")}},
	}}
	if err := putMetricData(ctx, client, "Bluetape/Example", data); err != nil {
		return
	}
}

func Example_putLogEvents() {
	client := &logsFake{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UnixMilli()
	events := []cloudwatchlogstypes.InputLogEvent{{
		Timestamp: aws.Int64(now),
		Message:   aws.String("request completed"),
	}}
	if err := putLogEvents(ctx, client, "/bluetape/example", "app", events); err != nil {
		return
	}
}

func TestMetricRequestAndLimits(t *testing.T) {
	valid := []cloudwatchtypes.MetricDatum{{MetricName: aws.String("requests"), Value: aws.Float64(1)}}
	tests := []struct {
		name string
		data []cloudwatchtypes.MetricDatum
		want error
	}{
		{name: "empty", data: nil, want: errInvalidMetric},
		{name: "too many metrics", data: makeMetricData(metricLimit + 1), want: errInvalidMetric},
		{name: "nan", data: []cloudwatchtypes.MetricDatum{{MetricName: aws.String("requests"), Value: aws.Float64(math.NaN())}}, want: errInvalidMetric},
		{name: "duplicate dimension", data: []cloudwatchtypes.MetricDatum{{MetricName: aws.String("requests"), Dimensions: []cloudwatchtypes.Dimension{{Name: aws.String("queue"), Value: aws.String("a")}, {Name: aws.String("queue"), Value: aws.String("b")}}}}, want: errInvalidMetric},
		{name: "valid", data: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := buildMetricInput("Bluetape/Example", tt.data)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if tt.want == nil && (err != nil || input == nil || len(input.MetricData) != 1) {
				t.Fatalf("input = %#v, err = %v", input, err)
			}
		})
	}

	tooManyDimensions := valid[0]
	tooManyDimensions.Dimensions = make([]cloudwatchtypes.Dimension, dimensionLimit+1)
	for i := range tooManyDimensions.Dimensions {
		tooManyDimensions.Dimensions[i] = cloudwatchtypes.Dimension{Name: aws.String(fmt.Sprintf("d%d", i)), Value: aws.String("v")}
	}
	if _, err := buildMetricInput("Bluetape/Example", []cloudwatchtypes.MetricDatum{tooManyDimensions}); !errors.Is(err, errInvalidMetric) {
		t.Fatalf("dimensions error = %v", err)
	}

	large := []cloudwatchtypes.MetricDatum{{MetricName: aws.String(strings.Repeat("m", 255)), Value: aws.Float64(1), Dimensions: []cloudwatchtypes.Dimension{{Name: aws.String("dimension"), Value: aws.String(strings.Repeat("v", cloudWatchPayloadLimit))}}}}
	if _, err := buildMetricInput("Bluetape/Example", large); !errors.Is(err, errMetricPayload) {
		t.Fatalf("large payload error = %v", err)
	}
}

func makeMetricData(count int) []cloudwatchtypes.MetricDatum {
	data := make([]cloudwatchtypes.MetricDatum, count)
	for i := range data {
		data[i] = cloudwatchtypes.MetricDatum{MetricName: aws.String("requests"), Value: aws.Float64(float64(i))}
	}
	return data
}

func TestLogRequestAndLimits(t *testing.T) {
	now := time.Now().UnixMilli()
	valid := []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String("ok")}}
	tests := []struct {
		name   string
		events []cloudwatchlogstypes.InputLogEvent
		want   error
	}{
		{name: "empty", want: errInvalidLog},
		{name: "too many events", events: makeLogEvents(logEventLimit+1, now), want: errInvalidLog},
		{name: "event too large", events: []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String(strings.Repeat("x", cloudWatchPayloadLimit+1))}}, want: errLogPayload},
		{name: "out of order", events: []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now + 1), Message: aws.String("a")}, {Timestamp: aws.Int64(now), Message: aws.String("b")}}, want: errInvalidLog},
		{name: "valid", events: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := buildLogInput("/bluetape/example", "app", tt.events)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if tt.want == nil && (err != nil || input == nil || input.SequenceToken != nil) {
				t.Fatalf("input = %#v, err = %v; legacy sequence token must be omitted", input, err)
			}
		})
	}

	largeBatch := makeLogEvents(2, now)
	largeBatch[0].Message = aws.String(strings.Repeat("x", cloudWatchPayloadLimit/2))
	largeBatch[1].Message = aws.String(strings.Repeat("y", cloudWatchPayloadLimit/2))
	if _, err := buildLogInput("/bluetape/example", "app", largeBatch); !errors.Is(err, errLogPayload) {
		t.Fatalf("large batch error = %v", err)
	}

	span := []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String("a")}, {Timestamp: aws.Int64(now + logSpanLimit.Milliseconds() + 1), Message: aws.String("b")}}
	if _, err := buildLogInput("/bluetape/example", "app", span); !errors.Is(err, errInvalidLog) {
		t.Fatalf("span error = %v", err)
	}
}

func makeLogEvents(count int, timestamp int64) []cloudwatchlogstypes.InputLogEvent {
	events := make([]cloudwatchlogstypes.InputLogEvent, count)
	for i := range events {
		events[i] = cloudwatchlogstypes.InputLogEvent{Timestamp: aws.Int64(timestamp + int64(i)), Message: aws.String("event")}
	}
	return events
}

func TestMetricCancellationAndRedaction(t *testing.T) {
	fake := &metricFake{err: errors.New("provider included secret metric value"), entered: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	if err := putMetricData(ctx, fake, "Bluetape/Example", []cloudwatchtypes.MetricDatum{{MetricName: aws.String("requests"), Value: aws.Float64(1)}}); err == nil || !errors.Is(err, errMetricPublish) {
		t.Fatalf("publish error = %v", err)
	} else if strings.Contains(fmt.Sprintf("%+v", err), "secret metric value") {
		t.Fatalf("provider text leaked: %v", err)
	}
	cancel()
	called := &metricFake{}
	if err := putMetricData(ctx, called, "Bluetape/Example", []cloudwatchtypes.MetricDatum{{MetricName: aws.String("requests"), Value: aws.Float64(1)}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	called.mu.Lock()
	calls := called.calls
	called.mu.Unlock()
	if calls != 0 {
		t.Fatalf("canceled call count = %d, want 0", calls)
	}
}

func TestRequestsAreCopiedAndMapped(t *testing.T) {
	metricName := "requests"
	metricData := []cloudwatchtypes.MetricDatum{{MetricName: &metricName, Value: aws.Float64(1)}}
	metricFakeClient := &metricFake{}
	if err := putMetricData(context.Background(), metricFakeClient, "Bluetape/Example", metricData); err != nil {
		t.Fatalf("put metric data: %v", err)
	}
	metricData[0].MetricName = aws.String("mutated")
	metricFakeClient.mu.Lock()
	metricInput := metricFakeClient.input
	metricCalls := metricFakeClient.calls
	metricFakeClient.mu.Unlock()
	if metricCalls != 1 || metricInput == nil || aws.ToString(metricInput.Namespace) != "Bluetape/Example" || aws.ToString(metricInput.MetricData[0].MetricName) != "requests" {
		t.Fatalf("captured metric request = %#v, calls = %d", metricInput, metricCalls)
	}

	message := "event"
	events := []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(time.Now().UnixMilli()), Message: &message}}
	logsFakeClient := &logsFake{}
	if err := putLogEvents(context.Background(), logsFakeClient, "/bluetape/example", "app", events); err != nil {
		t.Fatalf("put log events: %v", err)
	}
	events[0].Message = aws.String("mutated")
	logsFakeClient.mu.Lock()
	logsInput := logsFakeClient.input
	logsCalls := logsFakeClient.calls
	logsFakeClient.mu.Unlock()
	if logsCalls != 1 || logsInput == nil || logsInput.SequenceToken != nil || aws.ToString(logsInput.LogEvents[0].Message) != "event" {
		t.Fatalf("captured log request = %#v, calls = %d", logsInput, logsCalls)
	}
}

func TestLogPutsCanRunInParallel(t *testing.T) {
	fake := &logsFake{}
	now := time.Now().UnixMilli()
	event := []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String("event")}}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := putLogEvents(context.Background(), fake, "/bluetape/example", "app", event); err != nil {
				t.Errorf("parallel put: %v", err)
			}
		}()
	}
	wg.Wait()
	fake.mu.Lock()
	calls := fake.calls
	fake.mu.Unlock()
	if calls != 2 {
		t.Fatalf("parallel call count = %d, want 2", calls)
	}
}

func TestLogCancellationAndRedaction(t *testing.T) {
	now := time.Now().UnixMilli()
	fake := &logsFake{err: errors.New("provider included secret log body")}
	if err := putLogEvents(context.Background(), fake, "/bluetape/example", "app", []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String("secret log body")}}); err == nil || !errors.Is(err, errLogPublish) {
		t.Fatalf("publish error = %v", err)
	} else if strings.Contains(fmt.Sprintf("%+v", err), "secret log body") {
		t.Fatalf("provider text leaked: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake = &logsFake{}
	if err := putLogEvents(ctx, fake, "/bluetape/example", "app", []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String("event")}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	fake.mu.Lock()
	calls := fake.calls
	fake.mu.Unlock()
	if calls != 0 {
		t.Fatalf("canceled call count = %d, want 0", calls)
	}
}

func TestCancellationAfterProviderResponseWins(t *testing.T) {
	metricCtx, metricCancel := context.WithCancel(context.Background())
	metricFakeClient := &metricFake{after: metricCancel}
	if err := putMetricData(metricCtx, metricFakeClient, "Bluetape/Example", []cloudwatchtypes.MetricDatum{{MetricName: aws.String("requests"), Value: aws.Float64(1)}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("metric post-response error = %v", err)
	}

	logCtx, logCancel := context.WithCancel(context.Background())
	logFakeClient := &logsFake{after: logCancel}
	now := time.Now().UnixMilli()
	if err := putLogEvents(logCtx, logFakeClient, "/bluetape/example", "app", []cloudwatchlogstypes.InputLogEvent{{Timestamp: aws.Int64(now), Message: aws.String("event")}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("log post-response error = %v", err)
	}
}
