# s3vectors

[한국어](README.ko.md)

`s3vectors` is a narrow, caller-owned bridge for Amazon S3 Vectors using the
AWS SDK for Go v2. It preserves the SDK's request and response types for vector
bucket/index discovery and vector put/get/list/query operations.

## Usage

```go
import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	awss3vectors "github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/bluetape4k/bluetape-go/s3vectors"
)

cfg, err := config.LoadDefaultConfig(context.Background())
if err != nil {
	return err
}

provider, err := s3vectors.New(s3vectors.Options{
	Client: awss3vectors.NewFromConfig(cfg),
})
if err != nil {
	return err
}

buckets, err := provider.ListVectorBuckets(context.Background(), nil)
if err != nil {
	return err
}
_ = buckets
```

`Client` contains only the eight SDK operations used by this package:

- `ListVectorBuckets` and `GetVectorBucket`;
- `ListIndexes` and `GetIndex`;
- `PutVectors`, `GetVectors`, `ListVectors`, and `QueryVectors`.

`Provider` accepts the SDK's input/output types and forwards optional SDK
`optFns`. AWS SDK paginators can therefore use the provider directly:

```go
import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3vectors "github.com/aws/aws-sdk-go-v2/service/s3vectors"
)

ctx := context.Background()
input := &awss3vectors.ListIndexesInput{
	VectorBucketName: aws.String("catalog-vectors"),
}
paginator := awss3vectors.NewListIndexesPaginator(provider, input)
for paginator.HasMorePages() {
	page, err := paginator.NextPage(ctx)
	if err != nil {
		return err
	}
	_ = page
}
```

## Validation and ownership

The adapter rejects malformed requests before making an SDK call:

- bucket references use exactly one of ARN or name;
- index operations use either an index ARN or both index and bucket names;
- vector writes and queries require non-empty finite `float32` data;
- vector keys must be non-empty; `GetVectors` requires at least one key;
- `TopK` and positive pagination limits must be positive;
- `ListVectors` requires a valid `SegmentIndex`/`SegmentCount` pair when using
  parallel listing.

Responses with nil output or missing SDK-required top-level fields are reported
as `ErrMalformedOutput` rather than being treated as successful empty results.

The adapter does not infer vector dimensions or distance metrics, generate
embeddings, own Bedrock/OpenSearch integration, interpret metadata schemas or
filters, collect all pages, retry requests, or install a logger. The caller
owns those policies, credentials, client lifecycle, timeout, retry, logging,
metrics, and cancellation deadline. S3 Vectors metadata and query filters stay
as the SDK's opaque `document.Interface` values.

Cancellation is checked before dispatch and immediately after the SDK returns;
caller cancellation wins over an SDK output or error. The adapter clones
mutable SDK request pointers, key slices, and known `float32` vector slices
before dispatch. Provider errors never include vector values, metadata/filter
documents, resource names, or raw provider messages; the original cause remains
available through `errors.Is`/`errors.As`.

## Testing boundary

The default test suite uses a fake client and does not require AWS credentials,
network access, or a local emulator. Floci S3 support does not establish S3
Vectors compatibility, so this package makes no emulator claim. A future live
test must be explicitly opt-in with a build tag or environment gate and must
not be treated as default CI proof.

```bash
go test -count=1 ./s3vectors
go test -race -count=1 ./s3vectors
go vet ./s3vectors
```

The fake tests cover request construction, opaque metadata/filter forwarding,
input immutability, nil/typed-nil clients, malformed outputs, SDK error
wrapping, redaction, and cancellation precedence.
