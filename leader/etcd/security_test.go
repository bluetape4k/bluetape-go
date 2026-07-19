package etcdleader

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdAuthorizationBoundaries(t *testing.T) {
	// This isolated plaintext container is test-only. Production requires TLS.
	fixture := newEtcdFixture(t)
	t.Log("etcd authorization fixture transport=plaintext scope=test-only")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pathsA := electionPaths(mustNormalizeSecurityOptions(t, "tenant-a"))
	pathsB := electionPaths(mustNormalizeSecurityOptions(t, "tenant-b"))
	bootstrapEtcdAuth(ctx, t, fixture.client, pathsA, pathsB)

	root := newAuthenticatedEtcdClient(t, fixture.endpoints, "root", "root-password")
	clientA := newAuthenticatedEtcdClient(t, fixture.endpoints, "principal-a", "password-a")
	clientB := newAuthenticatedEtcdClient(t, fixture.endpoints, "principal-b", "password-b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		if _, err := root.AuthDisable(cleanupCtx); err != nil {
			t.Errorf("disable etcd auth: %v", err)
		}
	})

	t.Run("principal A owns only its encoded range", func(t *testing.T) {
		assertOwnRangeAccess(t, clientA, pathsA.root+"candidate-a")
		assertSiblingRangeDenied(t, clientA, pathsB.root+"candidate-b")
	})
	t.Run("principal B owns only its encoded range", func(t *testing.T) {
		assertOwnRangeAccess(t, clientB, pathsB.root+"candidate-b")
		assertSiblingRangeDenied(t, clientB, pathsA.root+"candidate-a")
	})
	t.Run("attached leases inherit key authorization", func(t *testing.T) {
		unattached, err := clientB.Grant(ctx, 5)
		if err != nil {
			t.Fatalf("grant unattached B lease: %v", err)
		}
		if _, err := clientB.Revoke(ctx, unattached.ID); err != nil {
			t.Fatalf("B revoke unattached lease: %v", err)
		}

		leaseA, err := clientA.Grant(ctx, 5)
		if err != nil {
			t.Fatalf("grant A lease: %v", err)
		}
		if _, err := clientA.Put(ctx, pathsA.root+"lease-a", "owner-a", clientv3.WithLease(leaseA.ID)); err != nil {
			t.Fatalf("attach A key to A lease: %v", err)
		}
		if _, err := clientB.Revoke(ctx, leaseA.ID); !errors.Is(err, rpctypes.ErrPermissionDenied) {
			t.Fatalf("B revoke A attached lease error = %v, want permission denied", err)
		}

		leaseB, err := clientB.Grant(ctx, 5)
		if err != nil {
			t.Fatalf("grant B lease: %v", err)
		}
		if _, err := clientA.Put(ctx, pathsA.root+"lease-b", "owner-a", clientv3.WithLease(leaseB.ID)); err != nil {
			t.Fatalf("attach A key to B-granted lease: %v", err)
		}
		if _, err := clientB.Revoke(ctx, leaseB.ID); !errors.Is(err, rpctypes.ErrPermissionDenied) {
			t.Fatalf("B revoke own lease after A attachment error = %v, want permission denied", err)
		}

		if _, err := root.Revoke(ctx, leaseA.ID); err != nil {
			t.Fatalf("root revoke A lease: %v", err)
		}
		if _, err := root.Revoke(ctx, leaseB.ID); err != nil {
			t.Fatalf("root revoke B lease: %v", err)
		}
	})
}

func bootstrapEtcdAuth(ctx context.Context, t *testing.T, client *clientv3.Client, pathsA, pathsB electionPath) {
	t.Helper()
	for _, role := range []string{"root", "role-a", "role-b"} {
		if _, err := client.RoleAdd(ctx, role); err != nil {
			t.Fatalf("add role %s: %v", role, err)
		}
	}
	users := []struct {
		name     string
		password string
		role     string
	}{
		{name: "root", password: "root-password", role: "root"},
		{name: "principal-a", password: "password-a", role: "role-a"},
		{name: "principal-b", password: "password-b", role: "role-b"},
	}
	for _, user := range users {
		if _, err := client.UserAdd(ctx, user.name, user.password); err != nil {
			t.Fatalf("add user %s: %v", user.name, err)
		}
		if _, err := client.UserGrantRole(ctx, user.name, user.role); err != nil {
			t.Fatalf("grant role %s to %s: %v", user.role, user.name, err)
		}
	}
	if _, err := client.RoleGrantPermission(ctx, "role-a", pathsA.root, pathsA.end, clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
		t.Fatalf("grant principal A range: %v", err)
	}
	if _, err := client.RoleGrantPermission(ctx, "role-b", pathsB.root, pathsB.end, clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
		t.Fatalf("grant principal B range: %v", err)
	}
	if _, err := client.AuthEnable(ctx); err != nil {
		t.Fatalf("enable etcd auth: %v", err)
	}
}

func newAuthenticatedEtcdClient(t *testing.T, endpoints []string, username, password string) *clientv3.Client {
	t.Helper()
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 3 * time.Second,
		Username:    username,
		Password:    password,
	})
	if err != nil {
		t.Fatalf("create authenticated etcd client for %s: %v", username, err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close authenticated etcd client for %s: %v", username, err)
		}
	})
	return client
}

func assertOwnRangeAccess(t *testing.T, client *clientv3.Client, key string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	watch := client.Watch(ctx, key, clientv3.WithCreatedNotify())
	response := <-watch
	if err := response.Err(); err != nil || !response.Created {
		t.Fatalf("create own-range watch: created=%v err=%v", response.Created, err)
	}
	if _, err := client.Put(ctx, key, "owner"); err != nil {
		t.Fatalf("put own-range key: %v", err)
	}
	if _, err := client.Get(ctx, key); err != nil {
		t.Fatalf("get own-range key: %v", err)
	}
	if _, err := client.Delete(ctx, key); err != nil {
		t.Fatalf("delete own-range key: %v", err)
	}
}

func assertSiblingRangeDenied(t *testing.T, client *clientv3.Client, key string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.Put(ctx, key, "forbidden"); !errors.Is(err, rpctypes.ErrPermissionDenied) {
		t.Fatalf("sibling Put() error = %v, want permission denied", err)
	}
	if _, err := client.Get(ctx, key); !errors.Is(err, rpctypes.ErrPermissionDenied) {
		t.Fatalf("sibling Get() error = %v, want permission denied", err)
	}
	if _, err := client.Delete(ctx, key); !errors.Is(err, rpctypes.ErrPermissionDenied) {
		t.Fatalf("sibling Delete() error = %v, want permission denied", err)
	}
	response := <-client.Watch(ctx, key, clientv3.WithCreatedNotify())
	if err := response.Err(); !isPermissionDenied(err) {
		t.Fatalf("sibling Watch() error = %v, want permission denied", err)
	}
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	want := rpctypes.ErrorDesc(rpctypes.ErrPermissionDenied)
	return errors.Is(err, rpctypes.ErrPermissionDenied) || strings.HasSuffix(rpctypes.ErrorDesc(err), want)
}

func mustNormalizeSecurityOptions(t *testing.T, group string) leader.Options {
	t.Helper()
	opts, err := (leader.Options{
		Group:         group,
		MemberID:      "security-test",
		Lease:         3 * time.Second,
		RenewInterval: time.Second,
		KeyPrefix:     "security",
	}).Normalize()
	if err != nil {
		t.Fatalf("normalize security options: %v", err)
	}
	return opts
}
