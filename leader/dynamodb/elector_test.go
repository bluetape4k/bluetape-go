package dynamodbleader

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bluetape4k/bluetape-go/leader"
)

type fakeClient struct {
	mu          sync.Mutex
	items       map[string]map[string]types.AttributeValue
	calls       []string
	lastPut     *dynamodb.PutItemInput
	lastUpdate  *dynamodb.UpdateItemInput
	lastDelete  *dynamodb.DeleteItemInput
	lastGet     *dynamodb.GetItemInput
	putErr      error
	updateErr   error
	deleteErr   error
	getErr      error
	afterPut    func()
	afterUpdate func()
	afterDelete func()
}

func newFakeClient() *fakeClient {
	return &fakeClient{items: make(map[string]map[string]types.AttributeValue)}
}

func (f *fakeClient) PutItem(ctx context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "put")
	f.lastPut = clonePutInput(input)
	if f.putErr != nil {
		err := f.putErr
		f.putErr = nil
		if f.afterPut != nil {
			f.afterPut()
		}
		return nil, err
	}
	key := itemKey(input.Item, input.ExpressionAttributeNames["#key"])
	if _, exists := f.items[key]; exists {
		if f.afterPut != nil {
			f.afterPut()
		}
		return nil, &types.ConditionalCheckFailedException{}
	}
	f.items[key] = cloneItem(input.Item)
	if f.afterPut != nil {
		f.afterPut()
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeClient) UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "update")
	f.lastUpdate = cloneUpdateInput(input)
	if f.updateErr != nil {
		err := f.updateErr
		f.updateErr = nil
		if f.afterUpdate != nil {
			f.afterUpdate()
		}
		return nil, err
	}
	key := itemKey(input.Key, input.ExpressionAttributeNames["#key"])
	item, exists := f.items[key]
	if !updateCondition(input, item, exists) {
		if f.afterUpdate != nil {
			f.afterUpdate()
		}
		return nil, &types.ConditionalCheckFailedException{}
	}
	if !exists {
		item = cloneItem(input.Key)
	}
	applyUpdate(item, input)
	f.items[key] = item
	if f.afterUpdate != nil {
		f.afterUpdate()
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (f *fakeClient) DeleteItem(ctx context.Context, input *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "delete")
	f.lastDelete = cloneDeleteInput(input)
	if f.deleteErr != nil {
		err := f.deleteErr
		f.deleteErr = nil
		if f.afterDelete != nil {
			f.afterDelete()
		}
		return nil, err
	}
	key := itemKey(input.Key, input.ExpressionAttributeNames["#key"])
	item, exists := f.items[key]
	if !deleteCondition(input, item, exists) {
		if f.afterDelete != nil {
			f.afterDelete()
		}
		return nil, &types.ConditionalCheckFailedException{}
	}
	delete(f.items, key)
	if f.afterDelete != nil {
		f.afterDelete()
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func (f *fakeClient) GetItem(ctx context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "get")
	f.lastGet = cloneGetInput(input)
	if f.getErr != nil {
		err := f.getErr
		f.getErr = nil
		return nil, err
	}
	keyName := "group"
	for candidate := range input.Key {
		keyName = candidate
		break
	}
	key := itemKey(input.Key, keyName)
	if item, ok := f.items[key]; ok {
		return &dynamodb.GetItemOutput{Item: cloneItem(item)}, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func clonePutInput(input *dynamodb.PutItemInput) *dynamodb.PutItemInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Item = cloneItem(input.Item)
	clone.ExpressionAttributeNames = cloneStrings(input.ExpressionAttributeNames)
	return &clone
}

func cloneUpdateInput(input *dynamodb.UpdateItemInput) *dynamodb.UpdateItemInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Key = cloneItem(input.Key)
	clone.ExpressionAttributeNames = cloneStrings(input.ExpressionAttributeNames)
	clone.ExpressionAttributeValues = cloneItem(input.ExpressionAttributeValues)
	return &clone
}

func cloneDeleteInput(input *dynamodb.DeleteItemInput) *dynamodb.DeleteItemInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Key = cloneItem(input.Key)
	clone.ExpressionAttributeNames = cloneStrings(input.ExpressionAttributeNames)
	clone.ExpressionAttributeValues = cloneItem(input.ExpressionAttributeValues)
	return &clone
}

func cloneGetInput(input *dynamodb.GetItemInput) *dynamodb.GetItemInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Key = cloneItem(input.Key)
	return &clone
}

func cloneStrings(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func itemKey(item map[string]types.AttributeValue, keyName string) string {
	if keyName == "" {
		keyName = "group"
	}
	value, ok := item[keyName].(*types.AttributeValueMemberS)
	if !ok {
		return "<invalid>"
	}
	return value.Value
}

func cloneItem(item map[string]types.AttributeValue) map[string]types.AttributeValue {
	clone := make(map[string]types.AttributeValue, len(item))
	for key, value := range item {
		switch typed := value.(type) {
		case *types.AttributeValueMemberS:
			clone[key] = &types.AttributeValueMemberS{Value: typed.Value}
		case *types.AttributeValueMemberN:
			clone[key] = &types.AttributeValueMemberN{Value: typed.Value}
		default:
			clone[key] = value
		}
	}
	return clone
}

func updateCondition(input *dynamodb.UpdateItemInput, item map[string]types.AttributeValue, exists bool) bool {
	condition := aws.ToString(input.ConditionExpression)
	if strings.Contains(condition, "attribute_not_exists(#lease)") {
		if !exists {
			return true
		}
		lease, ok := fakeNumberValue(item[input.ExpressionAttributeNames["#lease"]])
		now, _ := fakeNumberValue(input.ExpressionAttributeValues[":now"])
		return !ok || lease <= now
	}
	if strings.Contains(condition, "#owner = :owner AND #lease > :now") {
		owner, ownerOK := fakeStringValue(item[input.ExpressionAttributeNames["#owner"]])
		expected, expectedOK := fakeStringValue(input.ExpressionAttributeValues[":owner"])
		lease, leaseOK := fakeNumberValue(item[input.ExpressionAttributeNames["#lease"]])
		now, nowOK := fakeNumberValue(input.ExpressionAttributeValues[":now"])
		return ownerOK && expectedOK && leaseOK && nowOK && owner == expected && lease > now
	}
	return false
}

func applyUpdate(item map[string]types.AttributeValue, input *dynamodb.UpdateItemInput) {
	for alias, name := range input.ExpressionAttributeNames {
		if alias == "#owner" || alias == "#lease" || alias == "#ttl" {
			valueAlias := ":owner"
			switch alias {
			case "#lease":
				valueAlias = ":lease"
			case "#ttl":
				valueAlias = ":ttl"
			}
			if value, ok := input.ExpressionAttributeValues[valueAlias]; ok {
				item[name] = cloneItem(map[string]types.AttributeValue{"value": value})["value"]
			}
		}
	}
}

func deleteCondition(input *dynamodb.DeleteItemInput, item map[string]types.AttributeValue, exists bool) bool {
	if !exists {
		return false
	}
	owner, ownerOK := fakeStringValue(item[input.ExpressionAttributeNames["#owner"]])
	expected, expectedOK := fakeStringValue(input.ExpressionAttributeValues[":owner"])
	return ownerOK && expectedOK && owner == expected
}

func fakeStringValue(value types.AttributeValue) (string, bool) {
	typed, ok := value.(*types.AttributeValueMemberS)
	if !ok {
		return "", false
	}
	return typed.Value, true
}

func fakeNumberValue(value types.AttributeValue) (int64, bool) {
	typed, ok := value.(*types.AttributeValueMemberN)
	if !ok {
		return 0, false
	}
	var number int64
	if _, err := fmt.Sscan(typed.Value, &number); err != nil {
		return 0, false
	}
	return number, true
}

func testOptions(_ func() time.Time) leader.Options {
	return leader.Options{Group: "group", MemberID: "member", Lease: 200 * time.Millisecond, RenewInterval: 50 * time.Millisecond, KeyPrefix: "leadertest"}
}

func TestNewRejectsNilAndTypedNilClient(t *testing.T) {
	var typedNil *fakeClient
	for name, client := range map[string]Client{"nil": nil, "typed-nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(client, "leaders", testOptions(nil)); !errors.Is(err, ErrInvalidClient) {
				t.Fatalf("New() error = %v, want ErrInvalidClient", err)
			}
		})
	}
}

func TestCampaignLeaderRenewAndResign(t *testing.T) {
	fake := newFakeClient()
	elector, err := New(fake, "leaders", testOptions(nil), WithRetryDelay(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("Campaign() error = %v", err)
	}
	if !elector.IsLeader() {
		t.Fatal("Campaign() did not publish local leadership")
	}
	owner, err := elector.Leader(ctx)
	if err != nil || owner == "" {
		t.Fatalf("Leader() = %q, %v", owner, err)
	}
	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("Resign() error = %v", err)
	}
	if elector.IsLeader() {
		t.Fatal("Resign() left local leadership active")
	}
	owner, err = elector.Leader(ctx)
	if err != nil || owner != "" {
		t.Fatalf("Leader() after resign = %q, %v", owner, err)
	}
}

func TestCampaignTakesOverExpiredOwnerWithInjectedClock(t *testing.T) {
	fake := newFakeClient()
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	first, err := New(fake, "leaders", testOptions(clock), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := first.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	second, err := New(fake, "leaders", testOptions(clock), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := second.Leader(ctx)
	if err != nil || owner == "" {
		t.Fatalf("Leader() after stale takeover = %q, %v", owner, err)
	}
	if err := first.Resign(ctx); err != nil && !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("stale owner resign = %v", err)
	}
	if err := second.Resign(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignWaitsForActiveOwnerUntilContextEnds(t *testing.T) {
	fake := newFakeClient()
	clock := time.Now
	first, err := New(fake, "leaders", testOptions(clock), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Resign(context.Background()) }()

	second, err := New(fake, "leaders", testOptions(clock), WithClock(clock), WithRetryDelay(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := second.Campaign(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contention Campaign() = %v, want deadline exceeded", err)
	}
}

func TestCampaignReturnsCommitUnknownAfterProviderFailure(t *testing.T) {
	fake := newFakeClient()
	fake.putErr = errors.New("raw table secret group token")
	fake.getErr = errors.New("raw probe failure")
	elector, err := New(fake, "leaders", testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	err = elector.Campaign(context.Background())
	if err == nil || !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("Campaign() error = %v, want commit unknown", err)
	}
	if strings.Contains(err.Error(), "raw table secret group token") {
		t.Fatalf("Campaign() leaked provider text: %v", err)
	}
	if err := elector.Resign(context.Background()); err != nil && !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("cleanup Resign() error = %v", err)
	}
}

func TestNilContextDoesNotDispatch(t *testing.T) {
	fake := newFakeClient()
	elector, err := New(fake, "leaders", testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		t.Fatalf("Campaign(nil) = %v", err)
	}
	if _, err := elector.Leader(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		t.Fatalf("Leader(nil) = %v", err)
	}
	if err := elector.Resign(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		t.Fatalf("Resign(nil) = %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.calls) != 0 {
		t.Fatalf("nil context dispatched calls = %v", fake.calls)
	}
}

func TestPostDispatchCancellationWinsAndLeavesCleanupPending(t *testing.T) {
	fake := newFakeClient()
	ctx, cancel := context.WithCancel(context.Background())
	fake.afterPut = cancel
	elector, err := New(fake, "leaders", testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	err = elector.Campaign(ctx)
	if err == nil || !errors.Is(err, context.Canceled) || !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("Campaign() error = %v, want canceled+commit unknown", err)
	}
	if err := elector.Campaign(context.Background()); !errors.Is(err, leader.ErrCleanupPending) {
		t.Fatalf("Campaign() after cancellation = %v, want cleanup pending", err)
	}
}

func TestRequestSchemaUsesEpochMillisecondsAndCeiledTTL(t *testing.T) {
	fake := newFakeClient()
	now := time.Unix(1_700_000_000, int64(123*time.Millisecond))
	clock := func() time.Time { return now }
	elector, err := New(fake, "leaders", testOptions(clock), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	input := clonePutInput(fake.lastPut)
	fake.mu.Unlock()
	if input == nil || aws.ToString(input.ConditionExpression) != "attribute_not_exists(#key)" {
		t.Fatalf("PutItem condition = %#v", input)
	}
	lease, ok := input.Item["lease_until_ms"].(*types.AttributeValueMemberN)
	if !ok || lease.Value != "1700000000323" {
		t.Fatalf("lease_until_ms = %#v, want 1700000000323", input.Item["lease_until_ms"])
	}
	ttl, ok := input.Item["expires_at"].(*types.AttributeValueMemberN)
	if !ok || ttl.Value != "1700000001" {
		t.Fatalf("expires_at = %#v, want 1700000001", input.Item["expires_at"])
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLeaderUsesStronglyConsistentReadAndClock(t *testing.T) {
	fake := newFakeClient()
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	elector, err := New(fake, "leaders", testOptions(clock), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	owner, err := elector.Leader(context.Background())
	if err != nil || owner == "" {
		t.Fatalf("Leader() = %q, %v", owner, err)
	}
	fake.mu.Lock()
	consistent := fake.lastGet != nil && aws.ToBool(fake.lastGet.ConsistentRead)
	fake.mu.Unlock()
	if !consistent {
		t.Fatal("Leader() did not request ConsistentRead")
	}
	now = now.Add(time.Second)
	owner, err = elector.Leader(context.Background())
	if err != nil || owner != "" {
		t.Fatalf("expired Leader() = %q, %v", owner, err)
	}
	_ = elector.Resign(context.Background())
}

func TestLeaderRejectsMalformedActiveItem(t *testing.T) {
	fake := newFakeClient()
	now := time.Unix(1_700_000_000, 0)
	fake.items["group"] = map[string]types.AttributeValue{
		"group":          &types.AttributeValueMemberS{Value: "group"},
		"owner_token":    &types.AttributeValueMemberS{Value: ""},
		"lease_until_ms": &types.AttributeValueMemberN{Value: "1700000001000"},
	}
	clock := func() time.Time { return now }
	elector, err := New(fake, "leaders", testOptions(clock), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := elector.Leader(context.Background()); !errors.Is(err, ErrMalformedItem) {
		t.Fatalf("Leader() error = %v, want ErrMalformedItem", err)
	}
}

func TestCustomAttributeNamesAreUsedForEveryRequest(t *testing.T) {
	fake := newFakeClient()
	clock := time.Unix(1_700_000_000, 0)
	elector, err := New(
		fake,
		"leaders",
		testOptions(func() time.Time { return clock }),
		WithClock(func() time.Time { return clock }),
		WithAttributeNames("pk", "holder", "deadline", "ttl"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := elector.Leader(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	put, get := clonePutInput(fake.lastPut), cloneGetInput(fake.lastGet)
	fake.mu.Unlock()
	if put == nil || put.Item["pk"] == nil || put.Item["holder"] == nil || put.Item["deadline"] == nil || put.Item["ttl"] == nil {
		t.Fatalf("custom PutItem schema = %#v", put)
	}
	if get == nil || get.Key["pk"] == nil {
		t.Fatalf("custom GetItem key = %#v", get)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRenewChecksOwnerAndUnexpiredLease(t *testing.T) {
	fake := newFakeClient()
	elector, err := New(fake, "leaders", testOptions(nil), WithRetryDelay(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	ok, err := elector.renew(context.Background())
	if err != nil || !ok {
		t.Fatalf("renew() = %v, %v", ok, err)
	}
	fake.mu.Lock()
	input := cloneUpdateInput(fake.lastUpdate)
	fake.mu.Unlock()
	if input == nil || aws.ToString(input.ConditionExpression) != "#owner = :owner AND #lease > :now" {
		t.Fatalf("renew condition = %#v", input)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestResignRetriesAfterUnknownDelete(t *testing.T) {
	fake := newFakeClient()
	elector, err := New(fake, "leaders", testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.deleteErr = errors.New("raw delete failure")
	err = elector.Resign(context.Background())
	if err == nil || !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("first Resign() = %v, want commit unknown", err)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatalf("retry Resign() = %v", err)
	}
}
