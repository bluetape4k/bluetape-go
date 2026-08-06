// Package batchwrite 좁은 DynamoDB BatchWriteItem helper를 제공한다.
//
// 이 package는 호출자가 AWS SDK for Go v2 request type을 계속 사용하게 한다. package가 소유하는 범위는
// DynamoDB 전용 operational loop뿐이다. write를 25-item request로 나누고, 반환된
// UnprocessedItems를 다시 제출하며, 호출자 context cancellation에서 중단한다.
package batchwrite
