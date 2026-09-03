package s3vectors

import (
	"context"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"

	awss3vectors "github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/types"
)

const (
	// MaxListVectorBucketsResults 상수는 vector bucket page-size 상한이다.
	MaxListVectorBucketsResults int32 = 500
	// MaxListIndexesResults 상수는 index page-size 상한이다.
	MaxListIndexesResults int32 = 500
	// MaxPutVectors 상수는 PutVectors 한 호출의 vector 수 상한이다.
	MaxPutVectors = 500
	// MaxGetVectors 상수는 GetVectors 한 호출의 key 수 상한이다.
	MaxGetVectors = 100
	// MaxListVectorsResults 상수는 vector page-size 상한이다.
	MaxListVectorsResults int32 = 1000
	// MaxQueryTopK 상수는 QueryVectors top-K 상한이다.
	MaxQueryTopK int32 = 10_000
	// MaxVectorDimension 상수는 vector dimension 상한이다.
	MaxVectorDimension = 4096
	// MaxVectorKeyBytes 상수는 vector key byte 상한이다.
	MaxVectorKeyBytes = 63
	// MaxSegmentCount 상수는 병렬 ListVectors segment 수 상한이다.
	MaxSegmentCount int32 = 16
	// MaxVectorMetadataBytes 상수는 vector 하나의 metadata byte 상한이다.
	MaxVectorMetadataBytes = 40 << 10
	// MaxRequestPayloadBytes 상수는 JSON request payload byte 상한이다.
	MaxRequestPayloadBytes = 20 << 20
	// MaxIdentifierBytes 상수는 index/bucket 식별자 byte 상한이다.
	MaxIdentifierBytes = 1024
	// MaxPaginationTokenBytes 상수는 pagination token byte 상한이다.
	MaxPaginationTokenBytes = 2048
	// MaxPrefixBytes 상수는 list prefix byte 상한이다.
	MaxPrefixBytes = 1024
	// maxVectorComponentBytes는 JSON 직렬화 시 float32 하나에 보수적으로
	// 예약하는 byte 수다. 실제 SDK 인코더가 사용하는 표기 차이까지 포함해
	// request payload preflight가 documented 20 MiB를 넘기지 않도록 한다.
	maxVectorComponentBytes int64 = 32
)

// Client 는 Provider가 사용하는 최소 S3 Vectors SDK surface다.
//
// SDK client 생성, 설정, 관측, retry와 close는 caller가 소유한다. 이 interface는
// 이 package가 vector database abstraction으로 확장되지 않도록 AWS request와
// response type을 그대로 유지한다.
type Client interface {
	ListVectorBuckets(context.Context, *awss3vectors.ListVectorBucketsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorBucketsOutput, error)
	GetVectorBucket(context.Context, *awss3vectors.GetVectorBucketInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorBucketOutput, error)
	ListIndexes(context.Context, *awss3vectors.ListIndexesInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListIndexesOutput, error)
	GetIndex(context.Context, *awss3vectors.GetIndexInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetIndexOutput, error)
	PutVectors(context.Context, *awss3vectors.PutVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.PutVectorsOutput, error)
	GetVectors(context.Context, *awss3vectors.GetVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorsOutput, error)
	ListVectors(context.Context, *awss3vectors.ListVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorsOutput, error)
	QueryVectors(context.Context, *awss3vectors.QueryVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.QueryVectorsOutput, error)
}

// Options 는 caller-owned S3 Vectors client로 Provider를 구성한다.
type Options struct {
	// Client는 호출자가 생성하고 수명을 관리하는 AWS SDK client다.
	Client Client
}

// Provider 는 검증한 S3 Vectors operation을 caller-owned client로 전달한다.
//
// Provider는 credential/client를 생성하거나 request를 retry하지 않으며,
// paginator를 소유하거나 embedding, metadata/filter document를 해석하지
// 않는다. Provider는 New로 생성해야 하며 zero value는 SDK 호출 없이
// ErrInvalidProvider를 반환한다.
type Provider struct {
	client Client
}

var _ Client = (*awss3vectors.Client)(nil)
var _ Client = (*Provider)(nil)

// New 는 caller-owned client를 검증하고 immutable Provider를 반환한다.
func New(options Options) (*Provider, error) {
	if isNilClient(options.Client) {
		return nil, ErrNilClient
	}
	return &Provider{client: options.Client}, nil
}

// ListVectorBuckets 는 AWS SDK input/output type으로 caller의 vector bucket을
// 조회한다. input이 nil이면 prefix와 pagination filter를 지정하지 않는다.
func (p *Provider) ListVectorBuckets(ctx context.Context, input *awss3vectors.ListVectorBucketsInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorBucketsOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateListVectorBucketsInput(input); err != nil {
		return nil, err
	}
	input = cloneListVectorBucketsInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.ListVectorBuckets(ctx, input, optFns...)
	if err := finish(ctx, "list vector buckets", output, callErr, validListVectorBucketsOutput); err != nil {
		return nil, err
	}
	return output, nil
}

// GetVectorBucket 은 ARN 또는 name으로 하나의 bucket을 조회한다. input에는 두
// 식별자 형식 중 정확히 하나가 있어야 한다.
func (p *Provider) GetVectorBucket(ctx context.Context, input *awss3vectors.GetVectorBucketInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorBucketOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateGetVectorBucketInput(input); err != nil {
		return nil, err
	}
	input = cloneGetVectorBucketInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.GetVectorBucket(ctx, input, optFns...)
	if err := finish(ctx, "get vector bucket", output, callErr, validGetVectorBucketOutput); err != nil {
		return nil, err
	}
	return output, nil
}

// ListIndexes 는 AWS SDK type으로 하나의 vector bucket에 속한 index를 조회한다.
func (p *Provider) ListIndexes(ctx context.Context, input *awss3vectors.ListIndexesInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.ListIndexesOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateListIndexesInput(input); err != nil {
		return nil, err
	}
	input = cloneListIndexesInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.ListIndexes(ctx, input, optFns...)
	if err := finish(ctx, "list indexes", output, callErr, validListIndexesOutput); err != nil {
		return nil, err
	}
	return output, nil
}

// GetIndex 는 ARN 또는 index name과 bucket name으로 하나의 index를 조회한다.
func (p *Provider) GetIndex(ctx context.Context, input *awss3vectors.GetIndexInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.GetIndexOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateGetIndexInput(input); err != nil {
		return nil, err
	}
	input = cloneGetIndexInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.GetIndex(ctx, input, optFns...)
	if err := finish(ctx, "get index", output, callErr, validGetIndexOutput); err != nil {
		return nil, err
	}
	return output, nil
}

// PutVectors 는 caller가 제공한 float32 vector와 opaque metadata를 index에
// 기록한다. Embedding 생성과 index dimension/schema 소유권은 caller에게 있다.
func (p *Provider) PutVectors(ctx context.Context, input *awss3vectors.PutVectorsInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.PutVectorsOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validatePutVectorsInput(input); err != nil {
		return nil, err
	}
	input = clonePutVectorsInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.PutVectors(ctx, input, optFns...)
	if err := finish(ctx, "put vectors", output, callErr, nil); err != nil {
		return nil, err
	}
	return output, nil
}

// GetVectors 는 key로 vector를 조회하고 response document decoding은 caller에게
// 남긴다.
func (p *Provider) GetVectors(ctx context.Context, input *awss3vectors.GetVectorsInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorsOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateGetVectorsInput(input); err != nil {
		return nil, err
	}
	input = cloneGetVectorsInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.GetVectors(ctx, input, optFns...)
	if err := finish(ctx, "get vectors", output, callErr, validGetVectorsOutput); err != nil {
		return nil, err
	}
	return output, nil
}

// ListVectors 는 index의 vector를 조회한다. Pagination과 optional parallel
// segment의 소유권은 caller에게 남긴다.
func (p *Provider) ListVectors(ctx context.Context, input *awss3vectors.ListVectorsInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorsOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateListVectorsInput(input); err != nil {
		return nil, err
	}
	input = cloneListVectorsInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.ListVectors(ctx, input, optFns...)
	if err := finish(ctx, "list vectors", output, callErr, validListVectorsOutput); err != nil {
		return nil, err
	}
	return output, nil
}

// QueryVectors 는 한 번의 nearest-neighbor query를 수행하고 filter document
// semantics와 결과 해석은 caller에게 남긴다.
func (p *Provider) QueryVectors(ctx context.Context, input *awss3vectors.QueryVectorsInput, optFns ...func(*awss3vectors.Options)) (*awss3vectors.QueryVectorsOutput, error) {
	ctx, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateQueryVectorsInput(input); err != nil {
		return nil, err
	}
	input = cloneQueryVectorsInput(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.client.QueryVectors(ctx, input, optFns...)
	if err := finish(ctx, "query vectors", output, callErr, validQueryVectorsOutput); err != nil {
		return nil, err
	}
	return output, nil
}

func (p *Provider) begin(ctx context.Context) (context.Context, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil || isNilClient(p.client) {
		return nil, newError(ErrInvalidProvider, "validate provider", nil)
	}
	return ctx, nil
}

func finish[T any](ctx context.Context, operation string, output *T, callErr error, valid func(*T) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return newError(ErrOperationFailed, operation, callErr)
	}
	if output == nil {
		return newError(ErrMalformedOutput, operation, nil)
	}
	if valid != nil && !valid(output) {
		return newError(ErrMalformedOutput, operation, nil)
	}
	return nil
}

func validListVectorBucketsOutput(output *awss3vectors.ListVectorBucketsOutput) bool {
	return output.VectorBuckets != nil
}

func validGetVectorBucketOutput(output *awss3vectors.GetVectorBucketOutput) bool {
	return output.VectorBucket != nil
}

func validListIndexesOutput(output *awss3vectors.ListIndexesOutput) bool {
	return output.Indexes != nil
}

func validGetIndexOutput(output *awss3vectors.GetIndexOutput) bool {
	return output.Index != nil
}

func validGetVectorsOutput(output *awss3vectors.GetVectorsOutput) bool {
	return output.Vectors != nil
}

func validListVectorsOutput(output *awss3vectors.ListVectorsOutput) bool {
	return output.Vectors != nil
}

func validQueryVectorsOutput(output *awss3vectors.QueryVectorsOutput) bool {
	return output.DistanceMetric != "" && output.Vectors != nil
}

func validateListVectorBucketsInput(input *awss3vectors.ListVectorBucketsInput) error {
	if input == nil {
		return nil
	}
	if input.MaxResults != nil && (*input.MaxResults <= 0 || *input.MaxResults > MaxListVectorBucketsResults) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if !validOptionalString(input.Prefix, MaxPrefixBytes) || !validOptionalString(input.NextToken, MaxPaginationTokenBytes) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validateGetVectorBucketInput(input *awss3vectors.GetVectorBucketInput) error {
	if input == nil || !validIdentifierPair(input.VectorBucketArn, input.VectorBucketName) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validateListIndexesInput(input *awss3vectors.ListIndexesInput) error {
	if input == nil || !validIdentifierPair(input.VectorBucketArn, input.VectorBucketName) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if input.MaxResults != nil && (*input.MaxResults <= 0 || *input.MaxResults > MaxListIndexesResults) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if !validOptionalString(input.Prefix, MaxPrefixBytes) || !validOptionalString(input.NextToken, MaxPaginationTokenBytes) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validateGetIndexInput(input *awss3vectors.GetIndexInput) error {
	if input == nil || !validIndexReference(input.IndexArn, input.IndexName, input.VectorBucketName) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validatePutVectorsInput(input *awss3vectors.PutVectorsInput) error {
	if input == nil || !validIndexReference(input.IndexArn, input.IndexName, input.VectorBucketName) || len(input.Vectors) == 0 || len(input.Vectors) > MaxPutVectors {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	var requestBytes int64
	for i := range input.Vectors {
		vector := &input.Vectors[i]
		if !validVectorKey(vector.Key) || !validVectorData(vector.Data) {
			return newError(ErrInvalidRequest, "validate request", nil)
		}
		metadataBytes, ok := serializedDocumentSize(vector.Metadata)
		if !ok || metadataBytes > MaxVectorMetadataBytes {
			return newError(ErrInvalidRequest, "validate request", nil)
		}
		if !addRequestBytes(&requestBytes, int64(len(*vector.Key))+int64(vectorDimension(vector.Data))*maxVectorComponentBytes+int64(metadataBytes)+64) {
			return newError(ErrInvalidRequest, "validate request", nil)
		}
	}
	return nil
}

func validateGetVectorsInput(input *awss3vectors.GetVectorsInput) error {
	if input == nil || !validIndexReference(input.IndexArn, input.IndexName, input.VectorBucketName) || len(input.Keys) == 0 || len(input.Keys) > MaxGetVectors {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	var requestBytes int64
	for _, key := range input.Keys {
		if !validVectorKey(&key) || !addRequestBytes(&requestBytes, int64(len(key))+8) {
			return newError(ErrInvalidRequest, "validate request", nil)
		}
	}
	return nil
}

func validateListVectorsInput(input *awss3vectors.ListVectorsInput) error {
	if input == nil || !validIndexReference(input.IndexArn, input.IndexName, input.VectorBucketName) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if input.MaxResults != nil && (*input.MaxResults <= 0 || *input.MaxResults > MaxListVectorsResults) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if !validOptionalString(input.NextToken, MaxPaginationTokenBytes) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if input.SegmentCount == nil {
		if input.SegmentIndex != 0 {
			return newError(ErrInvalidRequest, "validate request", nil)
		}
		return nil
	}
	if *input.SegmentCount <= 0 || *input.SegmentCount > MaxSegmentCount || input.SegmentIndex < 0 || input.SegmentIndex >= *input.SegmentCount {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validateQueryVectorsInput(input *awss3vectors.QueryVectorsInput) error {
	if input == nil || !validIndexReference(input.IndexArn, input.IndexName, input.VectorBucketName) || input.TopK == nil || *input.TopK <= 0 || *input.TopK > MaxQueryTopK || !validVectorData(input.QueryVector) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	filterBytes, ok := serializedDocumentSize(input.Filter)
	if !ok || filterBytes > MaxRequestPayloadBytes {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	requestBytes := int64(vectorDimension(input.QueryVector))*maxVectorComponentBytes + int64(filterBytes) + 128
	if requestBytes > MaxRequestPayloadBytes {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	if !validOptionalString(input.NextToken, MaxPaginationTokenBytes) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validIdentifierPair(arn, name *string) bool {
	arnPresent := nonBlankPointer(arn)
	namePresent := nonBlankPointer(name)
	if arn != nil && !arnPresent || name != nil && !namePresent {
		return false
	}
	return arnPresent != namePresent
}

func validIndexReference(arn, indexName, bucketName *string) bool {
	arnPresent := nonBlankPointer(arn)
	indexPresent := nonBlankPointer(indexName)
	bucketPresent := nonBlankPointer(bucketName)
	if arn != nil && !arnPresent || indexName != nil && !indexPresent || bucketName != nil && !bucketPresent {
		return false
	}
	if arnPresent {
		return !indexPresent && !bucketPresent
	}
	return indexPresent && bucketPresent
}

func nonBlankPointer(value *string) bool {
	return value != nil && utf8.ValidString(*value) && len(*value) <= MaxIdentifierBytes && strings.TrimSpace(*value) != ""
}

func validOptionalString(value *string, maxBytes int) bool {
	return value == nil || (utf8.ValidString(*value) && len(*value) <= maxBytes)
}

func validVectorKey(value *string) bool {
	return value != nil && utf8.ValidString(*value) && strings.TrimSpace(*value) != "" && len(*value) <= MaxVectorKeyBytes
}

func validVectorData(data types.VectorData) bool {
	if data == nil {
		return false
	}
	switch value := data.(type) {
	case *types.VectorDataMemberFloat32:
		if value == nil || len(value.Value) == 0 || len(value.Value) > MaxVectorDimension {
			return false
		}
		for _, item := range value.Value {
			if math.IsNaN(float64(item)) || math.IsInf(float64(item), 0) {
				return false
			}
		}
		return true
	case *types.UnknownUnionMember:
		return false
	default:
		return false
	}
}

func vectorDimension(data types.VectorData) int {
	if value, ok := data.(*types.VectorDataMemberFloat32); ok && value != nil {
		return len(value.Value)
	}
	return 0
}

type documentMarshaler interface {
	MarshalSmithyDocument() ([]byte, error)
}

func serializedDocumentSize(value any) (size int, ok bool) {
	defer func() {
		if recover() != nil {
			// 호출자 소유 custom document의 panic은 요청 preflight 밖으로 전파하지 않는다.
			size, ok = 0, false
		}
	}()
	if value == nil || isNilValue(value) {
		return 0, true
	}
	marshaler, ok := value.(documentMarshaler)
	if !ok {
		return 0, false
	}
	encoded, err := marshaler.MarshalSmithyDocument()
	if err != nil {
		return 0, false
	}
	return len(encoded), true
}

func addRequestBytes(total *int64, amount int64) bool {
	if total == nil || amount < 0 || amount > MaxRequestPayloadBytes-*total {
		return false
	}
	*total += amount
	return true
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func isNilClient(client Client) bool {
	return isNilValue(client)
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	valueOf := reflect.ValueOf(value)
	switch valueOf.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return valueOf.IsNil()
	default:
		return false
	}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt32(value *int32) *int32 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneListVectorBucketsInput(input *awss3vectors.ListVectorBucketsInput) *awss3vectors.ListVectorBucketsInput {
	if input == nil {
		return &awss3vectors.ListVectorBucketsInput{}
	}
	return &awss3vectors.ListVectorBucketsInput{
		MaxResults: cloneInt32(input.MaxResults),
		NextToken:  cloneString(input.NextToken),
		Prefix:     cloneString(input.Prefix),
	}
}

func cloneGetVectorBucketInput(input *awss3vectors.GetVectorBucketInput) *awss3vectors.GetVectorBucketInput {
	if input == nil {
		return nil
	}
	return &awss3vectors.GetVectorBucketInput{
		VectorBucketArn:  cloneString(input.VectorBucketArn),
		VectorBucketName: cloneString(input.VectorBucketName),
	}
}

func cloneListIndexesInput(input *awss3vectors.ListIndexesInput) *awss3vectors.ListIndexesInput {
	if input == nil {
		return nil
	}
	return &awss3vectors.ListIndexesInput{
		MaxResults:       cloneInt32(input.MaxResults),
		NextToken:        cloneString(input.NextToken),
		Prefix:           cloneString(input.Prefix),
		VectorBucketArn:  cloneString(input.VectorBucketArn),
		VectorBucketName: cloneString(input.VectorBucketName),
	}
}

func cloneGetIndexInput(input *awss3vectors.GetIndexInput) *awss3vectors.GetIndexInput {
	if input == nil {
		return nil
	}
	return &awss3vectors.GetIndexInput{
		IndexArn:         cloneString(input.IndexArn),
		IndexName:        cloneString(input.IndexName),
		VectorBucketName: cloneString(input.VectorBucketName),
	}
}

func clonePutVectorsInput(input *awss3vectors.PutVectorsInput) *awss3vectors.PutVectorsInput {
	if input == nil {
		return nil
	}
	cloned := &awss3vectors.PutVectorsInput{
		IndexArn:         cloneString(input.IndexArn),
		IndexName:        cloneString(input.IndexName),
		VectorBucketName: cloneString(input.VectorBucketName),
		Vectors:          make([]types.PutInputVector, len(input.Vectors)),
	}
	for i := range input.Vectors {
		cloned.Vectors[i] = clonePutInputVector(input.Vectors[i])
	}
	return cloned
}

func cloneGetVectorsInput(input *awss3vectors.GetVectorsInput) *awss3vectors.GetVectorsInput {
	if input == nil {
		return nil
	}
	return &awss3vectors.GetVectorsInput{
		Keys:             append([]string(nil), input.Keys...),
		IndexArn:         cloneString(input.IndexArn),
		IndexName:        cloneString(input.IndexName),
		ReturnData:       input.ReturnData,
		ReturnMetadata:   input.ReturnMetadata,
		VectorBucketName: cloneString(input.VectorBucketName),
	}
}

func cloneListVectorsInput(input *awss3vectors.ListVectorsInput) *awss3vectors.ListVectorsInput {
	if input == nil {
		return nil
	}
	return &awss3vectors.ListVectorsInput{
		IndexArn:         cloneString(input.IndexArn),
		IndexName:        cloneString(input.IndexName),
		MaxResults:       cloneInt32(input.MaxResults),
		NextToken:        cloneString(input.NextToken),
		ReturnData:       input.ReturnData,
		ReturnMetadata:   input.ReturnMetadata,
		SegmentCount:     cloneInt32(input.SegmentCount),
		SegmentIndex:     input.SegmentIndex,
		VectorBucketName: cloneString(input.VectorBucketName),
	}
}

func cloneQueryVectorsInput(input *awss3vectors.QueryVectorsInput) *awss3vectors.QueryVectorsInput {
	if input == nil {
		return nil
	}
	return &awss3vectors.QueryVectorsInput{
		QueryVector:      cloneVectorData(input.QueryVector),
		TopK:             cloneInt32(input.TopK),
		Filter:           input.Filter,
		IndexArn:         cloneString(input.IndexArn),
		IndexName:        cloneString(input.IndexName),
		NextToken:        cloneString(input.NextToken),
		ReturnDistance:   input.ReturnDistance,
		ReturnMetadata:   input.ReturnMetadata,
		VectorBucketName: cloneString(input.VectorBucketName),
	}
}

func clonePutInputVector(input types.PutInputVector) types.PutInputVector {
	return types.PutInputVector{
		Data:     cloneVectorData(input.Data),
		Key:      cloneString(input.Key),
		Metadata: input.Metadata,
	}
}

func cloneVectorData(data types.VectorData) types.VectorData {
	switch value := data.(type) {
	case *types.VectorDataMemberFloat32:
		if value == nil {
			return (*types.VectorDataMemberFloat32)(nil)
		}
		return &types.VectorDataMemberFloat32{Value: append([]float32(nil), value.Value...)}
	case *types.UnknownUnionMember:
		if value == nil {
			return (*types.UnknownUnionMember)(nil)
		}
		return &types.UnknownUnionMember{Tag: value.Tag, Value: append([]byte(nil), value.Value...)}
	default:
		return data
	}
}
