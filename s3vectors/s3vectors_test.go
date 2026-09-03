package s3vectors

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3vectors "github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/document"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/types"
)

type fakeClient struct {
	mu sync.Mutex

	calls    map[string]int
	contexts map[string]context.Context

	listVectorBucketsInput *awss3vectors.ListVectorBucketsInput
	getVectorBucketInput   *awss3vectors.GetVectorBucketInput
	listIndexesInput       *awss3vectors.ListIndexesInput
	getIndexInput          *awss3vectors.GetIndexInput
	putVectorsInput        *awss3vectors.PutVectorsInput
	getVectorsInput        *awss3vectors.GetVectorsInput
	listVectorsInput       *awss3vectors.ListVectorsInput
	queryVectorsInput      *awss3vectors.QueryVectorsInput

	listVectorBucketsOutput *awss3vectors.ListVectorBucketsOutput
	getVectorBucketOutput   *awss3vectors.GetVectorBucketOutput
	listIndexesOutput       *awss3vectors.ListIndexesOutput
	getIndexOutput          *awss3vectors.GetIndexOutput
	putVectorsOutput        *awss3vectors.PutVectorsOutput
	getVectorsOutput        *awss3vectors.GetVectorsOutput
	listVectorsOutput       *awss3vectors.ListVectorsOutput
	queryVectorsOutput      *awss3vectors.QueryVectorsOutput

	err           error
	entered       chan struct{}
	release       chan struct{}
	afterResponse func()
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		calls:    make(map[string]int),
		contexts: make(map[string]context.Context),
	}
}

func (f *fakeClient) before(ctx context.Context, op string) {
	f.mu.Lock()
	f.calls[op]++
	f.contexts[op] = ctx
	if f.entered != nil {
		select {
		case f.entered <- struct{}{}:
		default:
		}
	}
	release := f.release
	f.mu.Unlock()

	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
}

func (f *fakeClient) after() {
	f.mu.Lock()
	afterResponse := f.afterResponse
	f.mu.Unlock()
	if afterResponse != nil {
		afterResponse()
	}
}

func (f *fakeClient) result(_ string) error {
	f.mu.Lock()
	err := f.err
	f.mu.Unlock()
	return err
}

func (f *fakeClient) ListVectorBuckets(ctx context.Context, input *awss3vectors.ListVectorBucketsInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorBucketsOutput, error) {
	f.before(ctx, "list vector buckets")
	f.mu.Lock()
	f.listVectorBucketsInput = cloneListVectorBucketsInput(input)
	out := f.listVectorBucketsOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("list vector buckets")
}

func (f *fakeClient) GetVectorBucket(ctx context.Context, input *awss3vectors.GetVectorBucketInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorBucketOutput, error) {
	f.before(ctx, "get vector bucket")
	f.mu.Lock()
	f.getVectorBucketInput = cloneGetVectorBucketInput(input)
	out := f.getVectorBucketOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("get vector bucket")
}

func (f *fakeClient) ListIndexes(ctx context.Context, input *awss3vectors.ListIndexesInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.ListIndexesOutput, error) {
	f.before(ctx, "list indexes")
	f.mu.Lock()
	f.listIndexesInput = cloneListIndexesInput(input)
	out := f.listIndexesOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("list indexes")
}

func (f *fakeClient) GetIndex(ctx context.Context, input *awss3vectors.GetIndexInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.GetIndexOutput, error) {
	f.before(ctx, "get index")
	f.mu.Lock()
	f.getIndexInput = cloneGetIndexInput(input)
	out := f.getIndexOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("get index")
}

func (f *fakeClient) PutVectors(ctx context.Context, input *awss3vectors.PutVectorsInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.PutVectorsOutput, error) {
	f.before(ctx, "put vectors")
	f.mu.Lock()
	f.putVectorsInput = clonePutVectorsInput(input)
	out := f.putVectorsOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("put vectors")
}

func (f *fakeClient) GetVectors(ctx context.Context, input *awss3vectors.GetVectorsInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorsOutput, error) {
	f.before(ctx, "get vectors")
	f.mu.Lock()
	f.getVectorsInput = cloneGetVectorsInput(input)
	out := f.getVectorsOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("get vectors")
}

func (f *fakeClient) ListVectors(ctx context.Context, input *awss3vectors.ListVectorsInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorsOutput, error) {
	f.before(ctx, "list vectors")
	f.mu.Lock()
	f.listVectorsInput = cloneListVectorsInput(input)
	out := f.listVectorsOutput
	f.mu.Unlock()
	f.after()
	return out, f.result("list vectors")
}

func (f *fakeClient) QueryVectors(ctx context.Context, input *awss3vectors.QueryVectorsInput, _ ...func(*awss3vectors.Options)) (*awss3vectors.QueryVectorsOutput, error) {
	f.before(ctx, "query vectors")
	f.mu.Lock()
	f.queryVectorsInput = cloneQueryVectorsInput(input)
	out := f.queryVectorsOutput
	f.mu.Unlock()
	err := f.result("query vectors")
	f.after()
	return out, err
}

func (f *fakeClient) callCount(operation string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[operation]
}

func (f *fakeClient) context(operation string) context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contexts[operation]
}

func TestNewRejectsNilAndTypedNilClient(t *testing.T) {
	var typedNil *fakeClient
	for name, client := range map[string]Client{
		"nil":       nil,
		"typed nil": typedNil,
		"nil map":   nilClientMap(nil),
		"nil func":  nilClientFunc(nil),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(Options{Client: client}); !errors.Is(err, ErrNilClient) {
				t.Fatalf("New error = %v, want ErrNilClient", err)
			}
		})
	}
}

// 이 구현들은 typed-nil 검사가 pointer에만 의존하지 않는지 확인한다.
type nilClientMap map[string]string

func (nilClientMap) ListVectorBuckets(context.Context, *awss3vectors.ListVectorBucketsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorBucketsOutput, error) {
	return nil, nil
}
func (nilClientMap) GetVectorBucket(context.Context, *awss3vectors.GetVectorBucketInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorBucketOutput, error) {
	return nil, nil
}
func (nilClientMap) ListIndexes(context.Context, *awss3vectors.ListIndexesInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListIndexesOutput, error) {
	return nil, nil
}
func (nilClientMap) GetIndex(context.Context, *awss3vectors.GetIndexInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetIndexOutput, error) {
	return nil, nil
}
func (nilClientMap) PutVectors(context.Context, *awss3vectors.PutVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.PutVectorsOutput, error) {
	return nil, nil
}
func (nilClientMap) GetVectors(context.Context, *awss3vectors.GetVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorsOutput, error) {
	return nil, nil
}
func (nilClientMap) ListVectors(context.Context, *awss3vectors.ListVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorsOutput, error) {
	return nil, nil
}
func (nilClientMap) QueryVectors(context.Context, *awss3vectors.QueryVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.QueryVectorsOutput, error) {
	return nil, nil
}

type nilClientFunc func()

func (nilClientFunc) ListVectorBuckets(context.Context, *awss3vectors.ListVectorBucketsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorBucketsOutput, error) {
	return nil, nil
}
func (nilClientFunc) GetVectorBucket(context.Context, *awss3vectors.GetVectorBucketInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorBucketOutput, error) {
	return nil, nil
}
func (nilClientFunc) ListIndexes(context.Context, *awss3vectors.ListIndexesInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListIndexesOutput, error) {
	return nil, nil
}
func (nilClientFunc) GetIndex(context.Context, *awss3vectors.GetIndexInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetIndexOutput, error) {
	return nil, nil
}
func (nilClientFunc) PutVectors(context.Context, *awss3vectors.PutVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.PutVectorsOutput, error) {
	return nil, nil
}
func (nilClientFunc) GetVectors(context.Context, *awss3vectors.GetVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorsOutput, error) {
	return nil, nil
}
func (nilClientFunc) ListVectors(context.Context, *awss3vectors.ListVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorsOutput, error) {
	return nil, nil
}
func (nilClientFunc) QueryVectors(context.Context, *awss3vectors.QueryVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.QueryVectorsOutput, error) {
	return nil, nil
}

func TestProviderZeroValueFailsBeforeSDKCall(t *testing.T) {
	var provider Provider
	if _, err := provider.ListVectorBuckets(context.Background(), nil); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("ListVectorBuckets error = %v, want ErrInvalidProvider", err)
	}
}

func TestListVectorBucketsForwardsRequestAndContext(t *testing.T) {
	fake := newFakeClient()
	fake.listVectorBucketsOutput = &awss3vectors.ListVectorBucketsOutput{VectorBuckets: []types.VectorBucketSummary{}}
	provider := mustProvider(t, fake)
	maxResults := int32(20)
	ctx := context.WithValue(context.Background(), contextKey{name: "request-id"}, "req-525")

	_, err := provider.ListVectorBuckets(ctx, &awss3vectors.ListVectorBucketsInput{
		MaxResults: &maxResults,
		Prefix:     aws.String("team/")})
	if err != nil {
		t.Fatalf("ListVectorBuckets: %v", err)
	}
	if fake.callCount("list vector buckets") != 1 {
		t.Fatalf("calls = %d, want 1", fake.callCount("list vector buckets"))
	}
	if fake.context("list vector buckets") != ctx {
		t.Fatalf("context was not propagated")
	}
	fake.mu.Lock()
	got := fake.listVectorBucketsInput
	fake.mu.Unlock()
	if got == nil || got.Prefix == nil || *got.Prefix != "team/" || got.MaxResults == nil || *got.MaxResults != maxResults {
		t.Fatalf("request = %#v, want prefix and max results", got)
	}
}

func TestProviderRejectsInvalidIndexReferencesWithoutSDKCall(t *testing.T) {
	fake := newFakeClient()
	provider := mustProvider(t, fake)
	tests := []struct {
		name string
		call func(context.Context) error
	}{
		{"get index missing reference", func(ctx context.Context) error {
			_, err := provider.GetIndex(ctx, &awss3vectors.GetIndexInput{})
			return err
		}},
		{"get index ambiguous reference", func(ctx context.Context) error {
			_, err := provider.GetIndex(ctx, &awss3vectors.GetIndexInput{
				IndexArn:         aws.String("arn:aws:s3vectors:ap-northeast-2:123:index/x"),
				IndexName:        aws.String("index"),
				VectorBucketName: aws.String("bucket"),
			})
			return err
		}},
		{"get bucket blank arn with name", func(ctx context.Context) error {
			_, err := provider.GetVectorBucket(ctx, &awss3vectors.GetVectorBucketInput{
				VectorBucketArn: aws.String("  "), VectorBucketName: aws.String("bucket"),
			})
			return err
		}},
		{"list indexes missing bucket", func(ctx context.Context) error {
			_, err := provider.ListIndexes(ctx, &awss3vectors.ListIndexesInput{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(context.Background()); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("error = %v, want ErrInvalidRequest", err)
			}
		})
	}
	if totalCalls(fake) != 0 {
		t.Fatalf("invalid references made %d SDK calls, want 0", totalCalls(fake))
	}
}

func TestPutVectorsValidatesFiniteValuesAndPreservesMetadata(t *testing.T) {
	fake := newFakeClient()
	fake.putVectorsOutput = &awss3vectors.PutVectorsOutput{}
	provider := mustProvider(t, fake)
	metadata := document.NewLazyDocument(map[string]any{"tenant": "acme", "rank": 3})
	input := &awss3vectors.PutVectorsInput{
		IndexName:        aws.String("catalog"),
		VectorBucketName: aws.String("vectors"),
		Vectors: []types.PutInputVector{{
			Key:      aws.String("item-1"),
			Data:     &types.VectorDataMemberFloat32{Value: []float32{0.25, -0.5}},
			Metadata: metadata,
		}},
	}
	if _, err := provider.PutVectors(context.Background(), input); err != nil {
		t.Fatalf("PutVectors: %v", err)
	}
	fake.mu.Lock()
	got := fake.putVectorsInput
	fake.mu.Unlock()
	if got == nil || len(got.Vectors) != 1 || got.Vectors[0].Metadata != metadata {
		t.Fatalf("request did not preserve metadata: %#v", got)
	}
	data, ok := got.Vectors[0].Data.(*types.VectorDataMemberFloat32)
	if !ok || !reflect.DeepEqual(data.Value, []float32{0.25, -0.5}) {
		t.Fatalf("vector data = %#v, want finite float32 values", got.Vectors[0].Data)
	}

	for name, value := range map[string]float32{"NaN": float32(math.NaN()), "+Inf": float32(math.Inf(1)), "-Inf": float32(math.Inf(-1))} {
		t.Run(name, func(t *testing.T) {
			before := fake.callCount("put vectors")
			bad := &awss3vectors.PutVectorsInput{
				IndexArn: aws.String("arn:aws:s3vectors:ap-northeast-2:123:index/catalog"),
				Vectors:  []types.PutInputVector{{Key: aws.String("secret-item"), Data: &types.VectorDataMemberFloat32{Value: []float32{value}}}},
			}
			if _, err := provider.PutVectors(context.Background(), bad); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("error = %v, want ErrInvalidRequest", err)
			}
			if got := fake.callCount("put vectors"); got != before {
				t.Fatalf("invalid vector calls = %d, want %d", got, before)
			}
		})
	}
}

func TestS3VectorsPreflightEnforcesServiceBounds(t *testing.T) {
	fake := newFakeClient()
	provider := mustProvider(t, fake)
	validVector := func(key string, dimension int) types.PutInputVector {
		return types.PutInputVector{
			Key:  aws.String(key),
			Data: &types.VectorDataMemberFloat32{Value: make([]float32, dimension)},
		}
	}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "put count", call: func() error {
			vectors := make([]types.PutInputVector, MaxPutVectors+1)
			for i := range vectors {
				vectors[i] = validVector(fmt.Sprintf("item-%d", i), 1)
			}
			_, err := provider.PutVectors(context.Background(), &awss3vectors.PutVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), Vectors: vectors})
			return err
		}},
		{name: "put dimension", call: func() error {
			_, err := provider.PutVectors(context.Background(), &awss3vectors.PutVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), Vectors: []types.PutInputVector{validVector("item", MaxVectorDimension+1)}})
			return err
		}},
		{name: "put key length", call: func() error {
			_, err := provider.PutVectors(context.Background(), &awss3vectors.PutVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), Vectors: []types.PutInputVector{validVector(strings.Repeat("k", MaxVectorKeyBytes+1), 1)}})
			return err
		}},
		{name: "put estimated request bytes", call: func() error {
			vectors := make([]types.PutInputVector, 200)
			for i := range vectors {
				vectors[i] = validVector(fmt.Sprintf("item-%d", i), MaxVectorDimension)
			}
			_, err := provider.PutVectors(context.Background(), &awss3vectors.PutVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), Vectors: vectors})
			return err
		}},
		{name: "metadata bytes", call: func() error {
			_, err := provider.PutVectors(context.Background(), &awss3vectors.PutVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), Vectors: []types.PutInputVector{{Key: aws.String("item"), Data: &types.VectorDataMemberFloat32{Value: []float32{1}}, Metadata: document.NewLazyDocument(strings.Repeat("m", MaxVectorMetadataBytes))}}})
			return err
		}},
		{name: "get count", call: func() error {
			keys := make([]string, MaxGetVectors+1)
			for i := range keys {
				keys[i] = fmt.Sprintf("item-%d", i)
			}
			_, err := provider.GetVectors(context.Background(), &awss3vectors.GetVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), Keys: keys})
			return err
		}},
		{name: "list results", call: func() error {
			max := MaxListVectorsResults + 1
			_, err := provider.ListVectors(context.Background(), &awss3vectors.ListVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), MaxResults: &max})
			return err
		}},
		{name: "segment count", call: func() error {
			_, err := provider.ListVectors(context.Background(), &awss3vectors.ListVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), SegmentCount: aws.Int32(MaxSegmentCount + 1), SegmentIndex: 0})
			return err
		}},
		{name: "query top k", call: func() error {
			_, err := provider.QueryVectors(context.Background(), &awss3vectors.QueryVectorsInput{IndexName: aws.String("index"), VectorBucketName: aws.String("bucket"), QueryVector: &types.VectorDataMemberFloat32{Value: []float32{1}}, TopK: aws.Int32(MaxQueryTopK + 1)})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("error = %v, want ErrInvalidRequest", err)
			}
		})
	}
	if totalCalls(fake) != 0 {
		t.Fatalf("invalid bounded requests made %d SDK calls, want 0", totalCalls(fake))
	}
}

func TestGetVectorsRejectsEmptyKeys(t *testing.T) {
	fake := newFakeClient()
	provider := mustProvider(t, fake)
	_, err := provider.GetVectors(context.Background(), &awss3vectors.GetVectorsInput{
		IndexName:        aws.String("catalog"),
		VectorBucketName: aws.String("vectors"),
		Keys:             []string{"item-1", "  "},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if totalCalls(fake) != 0 {
		t.Fatalf("invalid keys made %d SDK calls, want 0", totalCalls(fake))
	}
}

func TestListVectorsValidatesParallelSegmentPair(t *testing.T) {
	fake := newFakeClient()
	provider := mustProvider(t, fake)
	segmentCount := int32(2)
	tests := []struct {
		name  string
		input *awss3vectors.ListVectorsInput
	}{
		{name: "index without count", input: &awss3vectors.ListVectorsInput{
			IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"), SegmentIndex: 1,
		}},
		{name: "index out of range", input: &awss3vectors.ListVectorsInput{
			IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"), SegmentCount: &segmentCount, SegmentIndex: 2,
		}},
		{name: "non-positive count", input: &awss3vectors.ListVectorsInput{
			IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"), SegmentCount: aws.Int32(0),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := provider.ListVectors(context.Background(), tt.input); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("error = %v, want ErrInvalidRequest", err)
			}
		})
	}
	if totalCalls(fake) != 0 {
		t.Fatalf("invalid segments made %d SDK calls, want 0", totalCalls(fake))
	}
}

func TestQueryVectorsForwardsFilterAndTopK(t *testing.T) {
	fake := newFakeClient()
	fake.queryVectorsOutput = &awss3vectors.QueryVectorsOutput{DistanceMetric: types.DistanceMetricCosine, Vectors: []types.QueryOutputVector{}}
	provider := mustProvider(t, fake)
	filter := document.NewLazyDocument(map[string]any{"tenant": "acme", "active": true})
	_, err := provider.QueryVectors(context.Background(), &awss3vectors.QueryVectorsInput{
		IndexName:        aws.String("catalog"),
		VectorBucketName: aws.String("vectors"),
		QueryVector:      &types.VectorDataMemberFloat32{Value: []float32{0.1, 0.2}},
		TopK:             aws.Int32(5),
		Filter:           filter,
		ReturnDistance:   true,
		ReturnMetadata:   true,
	})
	if err != nil {
		t.Fatalf("QueryVectors: %v", err)
	}
	fake.mu.Lock()
	got := fake.queryVectorsInput
	fake.mu.Unlock()
	if got == nil || got.TopK == nil || *got.TopK != 5 || got.Filter != filter || !got.ReturnDistance || !got.ReturnMetadata {
		t.Fatalf("query request = %#v, want filter/top-k/return flags", got)
	}
}

func TestQueryVectorsRejectsInvalidTopK(t *testing.T) {
	fake := newFakeClient()
	provider := mustProvider(t, fake)
	_, err := provider.QueryVectors(context.Background(), &awss3vectors.QueryVectorsInput{
		IndexArn:    aws.String("arn:aws:s3vectors:ap-northeast-2:123:index/catalog"),
		QueryVector: &types.VectorDataMemberFloat32{Value: []float32{0.1}},
		TopK:        aws.Int32(0),
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if totalCalls(fake) != 0 {
		t.Fatalf("invalid top-k made %d SDK calls, want 0", totalCalls(fake))
	}
}

func TestProviderCancellationBeforeAndAfterCall(t *testing.T) {
	fake := newFakeClient()
	fake.listVectorBucketsOutput = &awss3vectors.ListVectorBucketsOutput{VectorBuckets: []types.VectorBucketSummary{}}
	provider := mustProvider(t, fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.ListVectorBuckets(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel error = %v, want context.Canceled", err)
	}
	if totalCalls(fake) != 0 {
		t.Fatalf("pre-cancel calls = %d, want 0", totalCalls(fake))
	}

	postCtx, postCancel := context.WithCancel(context.Background())
	fake.afterResponse = postCancel
	if _, err := provider.ListVectorBuckets(postCtx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("post-response error = %v, want context.Canceled", err)
	}
	if fake.callCount("list vector buckets") != 1 {
		t.Fatalf("post-response calls = %d, want 1", fake.callCount("list vector buckets"))
	}
}

func TestProviderWrapsSDKErrorWithoutLeakingDetails(t *testing.T) {
	fake := newFakeClient()
	cause := errors.New("provider secret metadata item-1")
	fake.err = cause
	provider := mustProvider(t, fake)
	_, err := provider.GetIndex(context.Background(), &awss3vectors.GetIndexInput{
		IndexName:        aws.String("catalog"),
		VectorBucketName: aws.String("private-bucket"),
	})
	if !errors.Is(err, ErrOperationFailed) || !errors.Is(err, cause) {
		t.Fatalf("error = %v, want operation and cause matching", err)
	}
	message := fmt.Sprintf("%v %+v %#v", err, err, err)
	for _, secret := range []string{"provider secret metadata item-1", "catalog", "private-bucket"} {
		if strings.Contains(message, secret) {
			t.Fatalf("error %q leaks %q", message, secret)
		}
	}
}

func TestProviderRejectsMalformedOutput(t *testing.T) {
	fake := newFakeClient()
	fake.getIndexOutput = nil
	provider := mustProvider(t, fake)
	_, err := provider.GetIndex(context.Background(), &awss3vectors.GetIndexInput{
		IndexName:        aws.String("catalog"),
		VectorBucketName: aws.String("vectors"),
	})
	if !errors.Is(err, ErrMalformedOutput) {
		t.Fatalf("error = %v, want ErrMalformedOutput", err)
	}
}

func TestProviderRejectsMissingRequiredOutputFields(t *testing.T) {
	providerTests := []struct {
		name string
		call func(*fakeClient, *Provider) error
	}{
		{name: "bucket list", call: func(fake *fakeClient, provider *Provider) error {
			fake.listVectorBucketsOutput = &awss3vectors.ListVectorBucketsOutput{}
			_, err := provider.ListVectorBuckets(context.Background(), nil)
			return err
		}},
		{name: "bucket get", call: func(fake *fakeClient, provider *Provider) error {
			fake.getVectorBucketOutput = &awss3vectors.GetVectorBucketOutput{}
			_, err := provider.GetVectorBucket(context.Background(), &awss3vectors.GetVectorBucketInput{VectorBucketName: aws.String("vectors")})
			return err
		}},
		{name: "index list", call: func(fake *fakeClient, provider *Provider) error {
			fake.listIndexesOutput = &awss3vectors.ListIndexesOutput{}
			_, err := provider.ListIndexes(context.Background(), &awss3vectors.ListIndexesInput{VectorBucketName: aws.String("vectors")})
			return err
		}},
		{name: "index get", call: func(fake *fakeClient, provider *Provider) error {
			fake.getIndexOutput = &awss3vectors.GetIndexOutput{}
			_, err := provider.GetIndex(context.Background(), &awss3vectors.GetIndexInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors")})
			return err
		}},
		{name: "vector get", call: func(fake *fakeClient, provider *Provider) error {
			fake.getVectorsOutput = &awss3vectors.GetVectorsOutput{}
			_, err := provider.GetVectors(context.Background(), &awss3vectors.GetVectorsInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"), Keys: []string{"item"}})
			return err
		}},
		{name: "vector list", call: func(fake *fakeClient, provider *Provider) error {
			fake.listVectorsOutput = &awss3vectors.ListVectorsOutput{}
			_, err := provider.ListVectors(context.Background(), &awss3vectors.ListVectorsInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors")})
			return err
		}},
		{name: "vector query", call: func(fake *fakeClient, provider *Provider) error {
			fake.queryVectorsOutput = &awss3vectors.QueryVectorsOutput{Vectors: []types.QueryOutputVector{}}
			_, err := provider.QueryVectors(context.Background(), &awss3vectors.QueryVectorsInput{
				IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"),
				QueryVector: &types.VectorDataMemberFloat32{Value: []float32{0.1}}, TopK: aws.Int32(1),
			})
			return err
		}},
	}
	for _, tt := range providerTests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeClient()
			provider := mustProvider(t, fake)
			if err := tt.call(fake, provider); !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("error = %v, want ErrMalformedOutput", err)
			}
		})
	}
}

func TestProviderForwardsAllOperations(t *testing.T) {
	fake := newFakeClient()
	fake.listVectorBucketsOutput = &awss3vectors.ListVectorBucketsOutput{VectorBuckets: []types.VectorBucketSummary{}}
	fake.getVectorBucketOutput = &awss3vectors.GetVectorBucketOutput{VectorBucket: &types.VectorBucket{}}
	fake.listIndexesOutput = &awss3vectors.ListIndexesOutput{Indexes: []types.IndexSummary{}}
	fake.getIndexOutput = &awss3vectors.GetIndexOutput{Index: &types.Index{}}
	fake.putVectorsOutput = &awss3vectors.PutVectorsOutput{}
	fake.getVectorsOutput = &awss3vectors.GetVectorsOutput{Vectors: []types.GetOutputVector{}}
	fake.listVectorsOutput = &awss3vectors.ListVectorsOutput{Vectors: []types.ListOutputVector{}}
	fake.queryVectorsOutput = &awss3vectors.QueryVectorsOutput{DistanceMetric: types.DistanceMetricCosine, Vectors: []types.QueryOutputVector{}}
	provider := mustProvider(t, fake)
	ctx := context.Background()
	if _, err := provider.ListVectorBuckets(ctx, nil); err != nil {
		t.Fatalf("ListVectorBuckets: %v", err)
	}
	if _, err := provider.GetVectorBucket(ctx, &awss3vectors.GetVectorBucketInput{VectorBucketName: aws.String("vectors")}); err != nil {
		t.Fatalf("GetVectorBucket: %v", err)
	}
	if _, err := provider.ListIndexes(ctx, &awss3vectors.ListIndexesInput{VectorBucketName: aws.String("vectors")}); err != nil {
		t.Fatalf("ListIndexes: %v", err)
	}
	if _, err := provider.GetIndex(ctx, &awss3vectors.GetIndexInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors")}); err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	if _, err := provider.PutVectors(ctx, &awss3vectors.PutVectorsInput{
		IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"),
		Vectors: []types.PutInputVector{{Key: aws.String("item"), Data: &types.VectorDataMemberFloat32{Value: []float32{1}}}},
	}); err != nil {
		t.Fatalf("PutVectors: %v", err)
	}
	if _, err := provider.GetVectors(ctx, &awss3vectors.GetVectorsInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"), Keys: []string{"item"}}); err != nil {
		t.Fatalf("GetVectors: %v", err)
	}
	if _, err := provider.ListVectors(ctx, &awss3vectors.ListVectorsInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors")}); err != nil {
		t.Fatalf("ListVectors: %v", err)
	}
	if _, err := provider.QueryVectors(ctx, &awss3vectors.QueryVectorsInput{IndexName: aws.String("catalog"), VectorBucketName: aws.String("vectors"), QueryVector: &types.VectorDataMemberFloat32{Value: []float32{1}}, TopK: aws.Int32(1)}); err != nil {
		t.Fatalf("QueryVectors: %v", err)
	}
	for _, operation := range []string{"list vector buckets", "get vector bucket", "list indexes", "get index", "put vectors", "get vectors", "list vectors", "query vectors"} {
		if got := fake.callCount(operation); got != 1 {
			t.Errorf("%s calls = %d, want 1", operation, got)
		}
	}
}

func mustProvider(t *testing.T, fake *fakeClient) *Provider {
	t.Helper()
	provider, err := New(Options{Client: fake})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return provider
}

type contextKey struct{ name string }

func totalCalls(fake *fakeClient) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var total int
	for _, calls := range fake.calls {
		total += calls
	}
	return total
}
