# Toxiproxy Testcontainers Wrapper Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first #219 messaging/HTTP/fault-injection slice as a narrow `testcontainers/toxiproxy` wrapper.

**Architecture:** Follow the existing Testcontainers wrapper pattern: a small package returns either a typed detail (`Start`) or the shared #217 `tcserver.Started` (`StartServer`). Tests that need configured proxy endpoints use `StartContainer` to access the upstream Toxiproxy container directly, while proxy setup remains upstream Testcontainers/Toxiproxy API surface through `testcontainers.ContainerCustomizer` options. One Redis-through-proxy integration test proves usable failure injection without latency sleeps.

**Tech Stack:** Go, Testcontainers-Go v0.42.0, `github.com/testcontainers/testcontainers-go/modules/toxiproxy`, `github.com/Shopify/toxiproxy/v2/client`, `github.com/redis/go-redis/v9`, `testcontainers/server`.

---

## File Structure

- Create `testcontainers/toxiproxy/doc.go`: package doc.
- Create `testcontainers/toxiproxy/toxiproxy.go`: constants, `Start`, `StartServer`, `ProxiedEndpoint`, and detail helper.
- Create `testcontainers/toxiproxy/toxiproxy_test.go`: Redis proxy integration and key contract tests.
- Create `testcontainers/toxiproxy/README.md`: English usage and operational notes.
- Create `testcontainers/toxiproxy/README.ko.md`: Korean usage and operational notes.
- Modify `go.mod` and `go.sum`: add the Toxiproxy module and direct Toxiproxy client dependency if `go mod tidy` requires it.

## Task 1: Add Failing Toxiproxy Tests

**Files:**
- Create: `testcontainers/toxiproxy/toxiproxy_test.go`

- [ ] **Step 1: Write the key and integration tests**

```go
package toxiproxytestcontainer_test

import (
    "context"
    "testing"
    "time"

    toxiproxyclient "github.com/Shopify/toxiproxy/v2/client"
    toxiproxytestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/toxiproxy"
    "github.com/redis/go-redis/v9"
    "github.com/testcontainers/testcontainers-go"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
    tctoxiproxy "github.com/testcontainers/testcontainers-go/modules/toxiproxy"
    "github.com/testcontainers/testcontainers-go/network"
)

func TestStartToxiproxyWithRedisProxy(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    t.Cleanup(cancel)

    nw, err := network.New(ctx)
    if err != nil {
        t.Fatalf("create network: %v", err)
    }
    t.Cleanup(func() {
        if err := nw.Remove(ctx); err != nil {
            t.Fatalf("remove network: %v", err)
        }
    })

    redisContainer, err := tcredis.Run(ctx, "redis:7.4-alpine", network.WithNetwork([]string{"redis"}, nw))
    if err != nil {
        t.Fatalf("start redis: %v", err)
    }
    t.Cleanup(func() {
        if err := testcontainers.TerminateContainer(redisContainer); err != nil {
            t.Fatalf("terminate redis: %v", err)
        }
    })

    toxiproxyContainer := toxiproxytestcontainer.StartContainer(
        ctx,
        t,
        tctoxiproxy.WithProxy("redis", "redis:6379"),
        network.WithNetwork([]string{"toxiproxy"}, nw),
    )
    controlURI, err := toxiproxyContainer.URI(ctx)
    if err != nil {
        t.Fatalf("toxiproxy control uri: %v", err)
    }

    endpoint := toxiproxytestcontainer.ProxiedEndpoint(ctx, t, toxiproxyContainer, 8666)
    client := redis.NewClient(&redis.Options{
        Addr:        endpoint,
        DialTimeout: 500 * time.Millisecond,
        ReadTimeout: 500 * time.Millisecond,
    })
    t.Cleanup(func() {
        if err := client.Close(); err != nil {
            t.Fatalf("close redis client: %v", err)
        }
    })

    const key = "bluetape:testcontainers:toxiproxy"
    if err := client.Set(ctx, key, "ok", 0).Err(); err != nil {
        t.Fatalf("set through proxy: %v", err)
    }

    proxies, err := toxiproxyclient.NewClient(controlURI).Proxies()
    if err != nil {
        t.Fatalf("list proxies: %v", err)
    }
    proxy := proxies["redis"]
    if proxy == nil {
        t.Fatalf("redis proxy not found")
    }
    if err := proxy.Disable(); err != nil {
        t.Fatalf("disable redis proxy: %v", err)
    }
    if err := client.Get(ctx, key).Err(); err == nil {
        t.Fatalf("get through disabled proxy: expected error")
    }
    if err := proxy.Enable(); err != nil {
        t.Fatalf("enable redis proxy: %v", err)
    }
    if got, err := client.Get(ctx, key).Result(); err != nil || got != "ok" {
        t.Fatalf("get after proxy enable = %q, %v; want ok, nil", got, err)
    }
}

func TestConnectionDetailKey(t *testing.T) {
    if toxiproxytestcontainer.ControlURIKey != "toxiproxy.control_uri" {
        t.Fatalf("ControlURIKey = %q", toxiproxytestcontainer.ControlURIKey)
    }
}
```

- [ ] **Step 2: Run the failing package test**

Run: `go test -p 1 -count=1 ./testcontainers/toxiproxy`

Expected: FAIL because package `testcontainers/toxiproxy` does not exist.

## Task 2: Implement the Toxiproxy Wrapper

**Files:**
- Create: `testcontainers/toxiproxy/doc.go`
- Create: `testcontainers/toxiproxy/toxiproxy.go`
- Modify: `go.mod`, `go.sum`
- Test: `testcontainers/toxiproxy/toxiproxy_test.go`

- [ ] **Step 1: Add module dependencies**

Run:

```bash
go get github.com/testcontainers/testcontainers-go/modules/toxiproxy@v0.42.0
go get github.com/Shopify/toxiproxy/v2@v2.12.0
```

Expected: `go.mod` adds the Toxiproxy module and client dependency without changing existing Testcontainers module versions.

- [ ] **Step 2: Add package documentation**

```go
// Package toxiproxytestcontainer starts Toxiproxy containers for fault-injection integration tests.
package toxiproxytestcontainer
```

- [ ] **Step 3: Add wrapper implementation**

```go
package toxiproxytestcontainer

import (
    "context"
    "fmt"
    "testing"

    "github.com/bluetape4k/bluetape-go/internal/testcleanup"
    tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
    "github.com/testcontainers/testcontainers-go"
    tctoxiproxy "github.com/testcontainers/testcontainers-go/modules/toxiproxy"
)

const (
    defaultImage = "ghcr.io/shopify/toxiproxy:2.12.0"

    // ControlURIKey is the documented key for the Toxiproxy control API URI.
    ControlURIKey = "toxiproxy.control_uri"
)

// Start launches a Toxiproxy test container and returns its control API URI.
func Start(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) string {
    tb.Helper()
    return mustDetail(ctx, tb, StartServer(ctx, tb, opts...), ControlURIKey)
}

// StartServer launches a Toxiproxy test container and returns the shared server view.
func StartServer(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) *tcserver.Started {
    tb.Helper()
    container, err := tctoxiproxy.Run(ctx, defaultImage, opts...)
    if err != nil {
        tb.Fatal(testcleanup.FormatStartError("toxiproxy", defaultImage, err))
    }

    srv, err := tcserver.New("toxiproxy", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
        uri, err := container.URI(ctx)
        if err != nil {
            return nil, err
        }
        return tcserver.ConnectionDetails{ControlURIKey: uri}, nil
    }))
    if err != nil {
        if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
            tb.Fatalf("toxiproxy server: %v; terminate after construction failure: %v", err, terminateErr)
        }
        tb.Fatalf("toxiproxy server: %v", err)
    }

    srv.RegisterCleanup(ctx, tb)
    return srv
}

// StartContainer launches a Toxiproxy test container and returns the upstream container.
func StartContainer(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) *tctoxiproxy.Container {
    tb.Helper()

    container, err := tctoxiproxy.Run(ctx, defaultImage, opts...)
    if err != nil {
        tb.Fatal(testcleanup.FormatStartError("toxiproxy", defaultImage, err))
    }
    testcleanup.Register(ctx, tb, "toxiproxy", container)
    return container
}

// ProxiedEndpoint returns the host:port endpoint for a configured Toxiproxy proxy port.
func ProxiedEndpoint(ctx context.Context, tb testing.TB, container *tctoxiproxy.Container, port int) string {
    tb.Helper()
    if container == nil {
        tb.Fatal("toxiproxy container must not be nil")
    }
    host, mappedPort, err := container.ProxiedEndpoint(port)
    if err != nil {
        tb.Fatalf("toxiproxy proxied endpoint %d: %v", port, err)
    }
    return fmt.Sprintf("%s:%s", host, mappedPort)
}

func mustDetail(ctx context.Context, tb testing.TB, srv *tcserver.Started, key string) string {
    tb.Helper()
    details, err := srv.ConnectionDetails(ctx)
    if err != nil {
        tb.Fatalf("%s: %v", key, err)
    }
    value, err := details.Require(key)
    if err != nil {
        tb.Fatalf("%s: %v", key, err)
    }
    return value
}
```

- [ ] **Step 4: Verify the API does not widen `testcontainers/server`**

Expected: `tcserver.Started` remains unchanged. Toxiproxy-specific endpoint
access is handled by `StartContainer` and `ProxiedEndpoint`; `StartServer`
continues to serve the shared connection-detail path.

- [ ] **Step 5: Run targeted tests**

Run: `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/toxiproxy`

Expected: PASS.

## Task 3: Add README Pair

**Files:**
- Create: `testcontainers/toxiproxy/README.md`
- Create: `testcontainers/toxiproxy/README.ko.md`

- [ ] **Step 1: Write English README**

Include: import path, `Start` usage, `StartServer` + `tcserver.ExportEnv`, Redis proxy example, dynamic host ports, Docker requirement, bounded timeout caveat, serial Testcontainers command, and deferrals to #58/#224/#220.

- [ ] **Step 2: Write Korean README**

Mirror the English behavior and caveats. Keep command names, env keys, and issue links identical.

- [ ] **Step 3: Verify docs mention key caveats**

Run:

```bash
rg -n "toxiproxy.control_uri|BLUETAPE_TOXIPROXY_CONTROL_URI|dynamic|bounded|#58|#224|#220" testcontainers/toxiproxy/README.md testcontainers/toxiproxy/README.ko.md
```

Expected: both README files mention the control URI, dynamic ports, bounded timeouts, and deferrals.

## Task 4: Verification and Commit

**Files:**
- All files touched by Tasks 1-3.

- [ ] **Step 1: Run targeted Docker tests serially**

Run:

```bash
go test -p 1 -count=1 ./testcontainers/toxiproxy
go test -race -p 1 -count=1 ./testcontainers/toxiproxy
go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy
go test -race -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy
```

Expected: PASS.

- [ ] **Step 2: Run repository gates**

Run:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
git diff --check
```

Expected: PASS.

- [ ] **Step 3: Commit implementation**

Use a Lore commit message that records the Toxiproxy-first constraint, rejected broad service catalog, and exact validation evidence.
