// Package batchwrite provides a narrow DynamoDB BatchWriteItem helper.
//
// The package keeps callers on AWS SDK for Go v2 request types. It only owns
// the DynamoDB-specific operational loop: split writes into 25-item requests,
// resubmit returned UnprocessedItems, and stop on caller context cancellation.
package batchwrite
