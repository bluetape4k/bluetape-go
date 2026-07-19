package etcdleader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	mobyclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	tcetcd "github.com/testcontainers/testcontainers-go/modules/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const etcdVersion = "v3.6.13"

var etcdDigest = map[string]string{
	"linux/amd64": "sha256:946dfbae58b1dec56af786a23e7322484b58281547bef1e848321f6beeb388d5",
	"linux/arm64": "sha256:23c14fbdf70105a54146cf5ed3a81613b99a973c60d5907851a251ca15664e96",
}

type daemonInfoFunc func(context.Context) (string, string, error)

type etcdFixture struct {
	client    *clientv3.Client
	container *tcetcd.EtcdContainer
	endpoints []string
	platform  string
}

func newEtcdFixture(t *testing.T) *etcdFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	platform, err := containerPlatform(ctx)
	if err != nil {
		t.Fatalf("resolve etcd container platform: %v", err)
	}
	digest, ok := etcdDigest[platform]
	if !ok {
		t.Fatalf("no approved etcd digest for platform %q", platform)
	}
	image := "gcr.io/etcd-development/etcd@" + digest
	container, err := tcetcd.Run(ctx, image, testcontainers.WithImagePlatform(platform))
	if err != nil {
		if container != nil {
			if cleanupErr := testcleanup.Terminate(ctx, 0, container); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("terminate partially created etcd container: %w", cleanupErr))
			}
		}
		t.Fatal(testcleanup.FormatStartError("etcd", image, err))
	}
	testcleanup.Register(ctx, t, "etcd", container)

	endpoints, err := container.ClientEndpoints(ctx)
	if err != nil {
		t.Fatalf("resolve etcd client endpoints: %v", err)
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("create etcd client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close etcd client: %v", err)
		}
	})

	waitForEtcdReady(t, client, endpoints)
	t.Logf("etcd fixture tag=%s digest=%s platform=%s endpoints=%d", etcdVersion, digest, platform, len(endpoints))
	return &etcdFixture{
		client:    client,
		container: container,
		endpoints: endpoints,
		platform:  platform,
	}
}

func containerPlatform(ctx context.Context) (string, error) {
	return resolveContainerPlatform(ctx, os.Getenv("DOCKER_DEFAULT_PLATFORM"), dockerDaemonPlatform)
}

func resolveContainerPlatform(ctx context.Context, env string, daemon daemonInfoFunc) (string, error) {
	platform := strings.TrimSpace(env)
	if platform != "" {
		return normalizeContainerPlatform(platform)
	}
	if daemon == nil {
		return "", errors.New("docker daemon platform resolver is nil")
	}
	osType, architecture, err := daemon(ctx)
	if err != nil {
		return "", fmt.Errorf("read docker daemon platform: %w", err)
	}
	return normalizeContainerPlatform(osType + "/" + architecture)
}

func normalizeContainerPlatform(platform string) (string, error) {
	parts := strings.Split(strings.TrimSpace(platform), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("unsupported docker platform %q", platform)
	}
	osType := strings.TrimSpace(parts[0])
	architecture := strings.TrimSpace(parts[1])
	switch architecture {
	case "x86_64":
		architecture = "amd64"
	case "aarch64":
		architecture = "arm64"
	}
	normalized := osType + "/" + architecture
	if osType != "linux" {
		return "", fmt.Errorf("unsupported docker operating system %q", osType)
	}
	if _, ok := etcdDigest[normalized]; !ok {
		return "", fmt.Errorf("unsupported docker platform %q", normalized)
	}
	return normalized, nil
}

func dockerDaemonPlatform(ctx context.Context) (string, string, error) {
	client, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = client.Close() }()
	info, err := client.Info(ctx, mobyclient.InfoOptions{})
	if err != nil {
		return "", "", err
	}
	return info.Info.OSType, info.Info.Architecture, nil
}

func waitForEtcdReady(t *testing.T, client *clientv3.Client, endpoints []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var lastErr error
	for ctx.Err() == nil {
		ready := len(endpoints) > 0
		for _, endpoint := range endpoints {
			status, err := client.Status(ctx, endpoint)
			if err != nil || status == nil || status.Header == nil || status.Header.MemberId == 0 || status.Leader == 0 {
				ready = false
				lastErr = err
				break
			}
		}
		if ready {
			break
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("etcd did not become ready: %v", errors.Join(ctx.Err(), lastErr))
	}

	key := fmt.Sprintf("/bluetape4k/test/readiness/%d", time.Now().UnixNano())
	value := "ready"
	if _, err := client.Put(ctx, key, value); err != nil {
		t.Fatalf("etcd readiness put: %v", err)
	}
	response, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("etcd readiness get: %v", err)
	}
	if len(response.Kvs) != 1 || string(response.Kvs[0].Value) != value {
		t.Fatalf("etcd readiness get returned %d values", len(response.Kvs))
	}
	if _, err := client.Delete(ctx, key); err != nil {
		t.Fatalf("etcd readiness delete: %v", err)
	}
}

func TestResolveContainerPlatform(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		daemonOS   string
		daemonArch string
		want       string
		wantErr    bool
	}{
		{
			name: "environment override",
			env:  "linux/amd64",
			want: "linux/amd64",
		},
		{
			name:       "daemon fallback",
			daemonOS:   "linux",
			daemonArch: "arm64",
			want:       "linux/arm64",
		},
		{
			name:       "x86 alias",
			daemonOS:   "linux",
			daemonArch: "x86_64",
			want:       "linux/amd64",
		},
		{
			name:       "arm alias",
			daemonOS:   "linux",
			daemonArch: "aarch64",
			want:       "linux/arm64",
		},
		{
			name:       "unsupported os",
			daemonOS:   "windows",
			daemonArch: "amd64",
			wantErr:    true,
		},
		{
			name:       "unsupported architecture",
			daemonOS:   "linux",
			daemonArch: "s390x",
			wantErr:    true,
		},
		{
			name:    "malformed environment override",
			env:     "linux/amd64/v8",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform, err := resolveContainerPlatform(context.Background(), tt.env, func(context.Context) (string, string, error) {
				return tt.daemonOS, tt.daemonArch, nil
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveContainerPlatform() error = %v, wantErr %v", err, tt.wantErr)
			}
			if platform != tt.want {
				t.Fatalf("resolveContainerPlatform() = %q, want %q", platform, tt.want)
			}
		})
	}
}

func TestResolveContainerPlatformEnvironmentPrecedesDaemon(t *testing.T) {
	called := false
	platform, err := resolveContainerPlatform(context.Background(), "linux/arm64", func(context.Context) (string, string, error) {
		called = true
		return "", "", errors.New("daemon should not be queried")
	})
	if err != nil {
		t.Fatalf("resolveContainerPlatform() error = %v", err)
	}
	if platform != "linux/arm64" {
		t.Fatalf("resolveContainerPlatform() = %q, want linux/arm64", platform)
	}
	if called {
		t.Fatal("daemon queried despite environment override")
	}
}

func TestResolveContainerPlatformPropagatesDaemonFailure(t *testing.T) {
	want := errors.New("docker unavailable")
	_, err := resolveContainerPlatform(context.Background(), "", func(context.Context) (string, string, error) {
		return "", "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("resolveContainerPlatform() error = %v, want %v", err, want)
	}
}
