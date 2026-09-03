# s3vectors

[English](README.md)

`s3vectors`는 AWS SDK for Go v2를 사용해 Amazon S3 Vectors에 접근하는 얇은
caller-owned bridge입니다. AWS SDK의 request/response type을 그대로 유지하고
vector bucket/index discovery와 vector put/get/list/query operation을 전달합니다.

## 사용법

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

`Client`에는 이 package가 사용하는 다음 eight SDK operation만 포함합니다.

- `ListVectorBuckets`, `GetVectorBucket`;
- `ListIndexes`, `GetIndex`;
- `PutVectors`, `GetVectors`, `ListVectors`, `QueryVectors`.

`Provider`는 SDK input/output type과 optional SDK `optFns`를 그대로 받습니다.
따라서 AWS SDK paginator에도 provider를 직접 전달할 수 있습니다.

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

## 검증과 소유권

adapter는 SDK를 호출하기 전에 다음 malformed request를 거부합니다.

- bucket reference에는 ARN 또는 name 중 정확히 하나를 사용합니다.
- index operation에는 index ARN 또는 index name과 bucket name을 함께 지정합니다.
- vector write/query에는 비어 있지 않고 finite한 `float32` data가 필요합니다.
- vector key는 valid UTF-8이며 최대 63 byte이고, `GetVectors`에는 최대 100개
  key를 지정합니다.
- `PutVectors`는 최대 500개 vector와 dimension 4,096까지 허용합니다.
- `TopK`는 1 이상 10,000 이하이고 pagination limit는 양수이면서 AWS page-size
  상한(버킷/index 500, vector 1,000) 이하여야 합니다.
- parallel listing을 사용할 때 `ListVectors`의 `SegmentIndex`와 `SegmentCount`
  조합이 유효해야 하며 segment는 최대 16개입니다.
- vector 하나의 metadata document는 40 KiB 이하이고 request preflight는
  추정 JSON payload를 20 MiB 이하로 제한합니다.

output이 nil이거나 SDK가 요구하는 최상위 field가 없으면 성공한 빈 결과로
처리하지 않고 `ErrMalformedOutput`으로 반환합니다.

adapter는 index vector dimension이나 distance metric을 추론하지 않습니다. embedding
생성, Bedrock/OpenSearch 연동, metadata schema/filter 해석, 전체 page 수집,
request retry와 logger 설치도 제공하지 않습니다. 이 정책과 credentials, client
lifecycle, timeout, retry, logging, metrics와 cancellation deadline은 caller가
소유합니다. S3 Vectors metadata와 query filter는 SDK의 opaque
`document.Interface` 값으로 유지합니다. 호출자는 SDK dispatch가 끝날 때까지
이 opaque 값을 변경하지 않아야 하며, adapter는 provider-specific document 구현을
deep-copy하지 않습니다.

cancellation은 dispatch 전과 SDK 응답 직후 확인하며 SDK output/error보다 caller의
cancellation을 우선합니다. adapter는 먼저 request를 검증한 뒤 SDK request의
mutable pointer, key slice와 알려진 `float32` vector slice를 dispatch 전에 복사합니다. Provider 오류에는
vector 값, metadata/filter document, resource name 또는 provider raw message를
포함하지 않으며 원인은 `errors.Is`/`errors.As`로 확인할 수 있습니다.

## 테스트 경계

기본 test suite는 fake client만 사용하므로 AWS credentials, network 또는 local
emulator가 필요하지 않습니다. Floci의 일반 S3 지원만으로 S3 Vectors 호환성을
입증할 수 없으므로 이 package는 emulator 지원을 주장하지 않습니다. 향후 live
test는 build tag 또는 environment gate 뒤에 명시적으로 두어야 하며 기본 CI의
성공 근거로 사용하지 않습니다.

```bash
go test -count=1 ./s3vectors
go test -race -count=1 ./s3vectors
go vet ./s3vectors
```

Fake test는 request construction, opaque metadata/filter 전달, input 불변성,
nil/typed-nil client, malformed output, SDK 오류 래핑, redaction과 cancellation
우선순위를 검증합니다.
