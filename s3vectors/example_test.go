package s3vectors_test

import (
	"context"
	"fmt"

	awss3vectors "github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/types"
	"github.com/bluetape4k/bluetape-go/s3vectors"
)

type exampleClient struct{}

func (exampleClient) ListVectorBuckets(context.Context, *awss3vectors.ListVectorBucketsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorBucketsOutput, error) {
	return &awss3vectors.ListVectorBucketsOutput{VectorBuckets: []types.VectorBucketSummary{}}, nil
}

func (exampleClient) GetVectorBucket(context.Context, *awss3vectors.GetVectorBucketInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorBucketOutput, error) {
	return &awss3vectors.GetVectorBucketOutput{VectorBucket: &types.VectorBucket{}}, nil
}

func (exampleClient) ListIndexes(context.Context, *awss3vectors.ListIndexesInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListIndexesOutput, error) {
	return &awss3vectors.ListIndexesOutput{Indexes: []types.IndexSummary{}}, nil
}

func (exampleClient) GetIndex(context.Context, *awss3vectors.GetIndexInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetIndexOutput, error) {
	return &awss3vectors.GetIndexOutput{Index: &types.Index{}}, nil
}

func (exampleClient) PutVectors(context.Context, *awss3vectors.PutVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.PutVectorsOutput, error) {
	return &awss3vectors.PutVectorsOutput{}, nil
}

func (exampleClient) GetVectors(context.Context, *awss3vectors.GetVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.GetVectorsOutput, error) {
	return &awss3vectors.GetVectorsOutput{Vectors: []types.GetOutputVector{}}, nil
}

func (exampleClient) ListVectors(context.Context, *awss3vectors.ListVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.ListVectorsOutput, error) {
	return &awss3vectors.ListVectorsOutput{Vectors: []types.ListOutputVector{}}, nil
}

func (exampleClient) QueryVectors(context.Context, *awss3vectors.QueryVectorsInput, ...func(*awss3vectors.Options)) (*awss3vectors.QueryVectorsOutput, error) {
	return &awss3vectors.QueryVectorsOutput{DistanceMetric: types.DistanceMetricCosine, Vectors: []types.QueryOutputVector{}}, nil
}

func ExampleNew() {
	provider, err := s3vectors.New(s3vectors.Options{Client: exampleClient{}})
	if err != nil {
		fmt.Println(err)
		return
	}
	buckets, err := provider.ListVectorBuckets(context.Background(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(buckets.VectorBuckets))
	// Output:
	// 0
}
