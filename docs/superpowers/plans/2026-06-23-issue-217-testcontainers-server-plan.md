# Testcontainers Server Abstraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the #217 Go-native Testcontainers server abstraction, adapt existing wrappers with `StartServer`, and keep current `Start` callers source-compatible.

**Architecture:** Create a small public `testcontainers/server` package that wraps a narrow Testcontainers-compatible container interface, owns connection-detail helpers, and offers explicit reversible env export. Existing service packages keep their current typed `Start` return values and add opt-in `StartServer` functions backed by the shared server adapter.

**Tech Stack:** Go 1.26, Testcontainers-Go v0.42.0, `internal/testcleanup`, package-local Docker smoke tests, Markdown README pairs.

---

## File Structure

- Create `testcontainers/server/doc.go`: package documentation and usage constraints.
- Create `testcontainers/server/details.go`: `ConnectionDetails`, clone/merge/get/require helpers, and sentinel detail errors.
- Create `testcontainers/server/server.go`: narrow `Container` interface, `Server` interface, `Started` adapter, options, validation, host/port/endpoint/delegation, cleanup, and termination.
- Create `testcontainers/server/env.go`: `ExportEnv(testing.TB, ConnectionDetails, map[string]string) error`.
- Create `testcontainers/server/*_test.go`: Docker-free contract tests using a fake container.
- Modify `testcontainers/{postgres,mysql,redis,kafka,nats}/*.go`: add `StartServer(ctx, tb) *server.Started`; keep `Start(ctx, tb)` compatible.
- Modify `testcontainers/{postgres,mysql,redis,kafka,nats}/*_test.go`: assert `StartServer` connection details in existing Docker smoke coverage.
- Modify `testcontainers/{postgres,mysql,redis,kafka,nats}/README.md` and `README.ko.md`: document dynamic mapped ports, `StartServer`, env export, cleanup, and fixed-port collision risk.

## Task 1: Add `testcontainers/server` Contract

**Files:**
- Create: `testcontainers/server/doc.go`
- Create: `testcontainers/server/details.go`
- Create: `testcontainers/server/server.go`
- Create: `testcontainers/server/env.go`
- Create: `testcontainers/server/details_test.go`
- Create: `testcontainers/server/server_test.go`
- Create: `testcontainers/server/env_test.go`

- [ ] **Step 1: Write failing connection detail tests**

Create `testcontainers/server/details_test.go` with tests for clone isolation, merge immutability, lookup, and missing required keys:

```go
package server_test

import (
	"errors"
	"testing"

	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
)

func TestConnectionDetailsCloneAndMergeAreImmutable(t *testing.T) {
	base := tcserver.ConnectionDetails{"redis.address": "localhost:6379"}
	clone := base.Clone()
	clone["redis.address"] = "changed"
	if got := base["redis.address"]; got != "localhost:6379" {
		t.Fatalf("base detail mutated through clone: %q", got)
	}

	merged := base.Merge(tcserver.ConnectionDetails{"nats.url": "nats://localhost:4222"})
	merged["redis.address"] = "changed-again"
	if got := base["redis.address"]; got != "localhost:6379" {
		t.Fatalf("base detail mutated through merge: %q", got)
	}
	if got := merged["nats.url"]; got != "nats://localhost:4222" {
		t.Fatalf("merged detail = %q", got)
	}
}

func TestConnectionDetailsRequire(t *testing.T) {
	details := tcserver.ConnectionDetails{"mysql.dsn": "user:pass@tcp(localhost:3306)/db"}
	got, err := details.Require("mysql.dsn")
	if err != nil {
		t.Fatalf("Require existing key: %v", err)
	}
	if got != "user:pass@tcp(localhost:3306)/db" {
		t.Fatalf("Require = %q", got)
	}

	if _, err := details.Require("missing"); !errors.Is(err, tcserver.ErrMissingDetail) {
		t.Fatalf("Require missing error = %v, want ErrMissingDetail", err)
	}
}
```

- [ ] **Step 2: Run the failing detail tests**

Run:

```bash
go test -count=1 ./testcontainers/server
```

Expected: FAIL because `testcontainers/server` does not exist yet.

- [ ] **Step 3: Write failing server adapter tests**

Create `testcontainers/server/server_test.go` with a fake container that implements the narrow `Container` interface:

```go
package server_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"

	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
)

type fakeContainer struct {
	host       string
	mappedPort network.Port
	endpoint   string
	terminated bool
}

func (f *fakeContainer) Host(context.Context) (string, error) {
	return f.host, nil
}

func (f *fakeContainer) MappedPort(context.Context, string) (network.Port, error) {
	return f.mappedPort, nil
}

func (f *fakeContainer) PortEndpoint(context.Context, string, string) (string, error) {
	return f.endpoint, nil
}

func (f *fakeContainer) Terminate(context.Context, ...testcontainers.TerminateOption) error {
	f.terminated = true
	return nil
}

func TestStartedDelegatesContainerOperations(t *testing.T) {
	ctx := context.Background()
	container := &fakeContainer{
		host:       "127.0.0.1",
		mappedPort: "16379/tcp",
		endpoint:   "redis://127.0.0.1:16379",
	}

	srv, err := tcserver.New(
		"redis",
		container,
		tcserver.WithConnectionDetails(func(context.Context, *tcserver.Started) (tcserver.ConnectionDetails, error) {
			return tcserver.ConnectionDetails{"redis.address": "127.0.0.1:16379"}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := srv.Name(); got != "redis" {
		t.Fatalf("Name = %q", got)
	}
	if got, err := srv.Host(ctx); err != nil || got != "127.0.0.1" {
		t.Fatalf("Host = %q, %v", got, err)
	}
	if got, err := srv.MappedPort(ctx, "6379/tcp"); err != nil || got != "16379" {
		t.Fatalf("MappedPort = %q, %v", got, err)
	}
	if got, err := srv.Endpoint(ctx, "6379/tcp", "redis"); err != nil || got != "redis://127.0.0.1:16379" {
		t.Fatalf("Endpoint = %q, %v", got, err)
	}
	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("ConnectionDetails: %v", err)
	}
	details["redis.address"] = "changed"
	again, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("ConnectionDetails again: %v", err)
	}
	if got := again["redis.address"]; got != "127.0.0.1:16379" {
		t.Fatalf("details leaked mutable state: %q", got)
	}
	if err := srv.Terminate(ctx); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if !container.terminated {
		t.Fatal("container was not terminated")
	}
}

func TestNewValidatesInputs(t *testing.T) {
	if _, err := tcserver.New("", &fakeContainer{}); !errors.Is(err, tcserver.ErrInvalidServer) {
		t.Fatalf("blank name error = %v, want ErrInvalidServer", err)
	}
	if _, err := tcserver.New("redis", nil); !errors.Is(err, tcserver.ErrInvalidServer) {
		t.Fatalf("nil container error = %v, want ErrInvalidServer", err)
	}
}
```

- [ ] **Step 4: Write failing env export tests**

Create `testcontainers/server/env_test.go`:

```go
package server_test

import (
	"errors"
	"os"
	"testing"

	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
)

func TestExportEnvSetsMappedValues(t *testing.T) {
	details := tcserver.ConnectionDetails{"redis.address": "127.0.0.1:16379"}
	if err := tcserver.ExportEnv(t, details, map[string]string{"redis.address": "BLUETAPE_REDIS_ADDR"}); err != nil {
		t.Fatalf("ExportEnv: %v", err)
	}
	if got := os.Getenv("BLUETAPE_REDIS_ADDR"); got != "127.0.0.1:16379" {
		t.Fatalf("BLUETAPE_REDIS_ADDR = %q", got)
	}
}

func TestExportEnvValidatesBeforeMutating(t *testing.T) {
	t.Setenv("BLUETAPE_REDIS_ADDR", "before")
	err := tcserver.ExportEnv(
		t,
		tcserver.ConnectionDetails{"redis.address": "127.0.0.1:16379"},
		map[string]string{
			"redis.address": "BLUETAPE_REDIS_ADDR",
			"missing":       "BLUETAPE_MISSING",
		},
	)
	if !errors.Is(err, tcserver.ErrMissingDetail) {
		t.Fatalf("ExportEnv error = %v, want ErrMissingDetail", err)
	}
	if got := os.Getenv("BLUETAPE_REDIS_ADDR"); got != "before" {
		t.Fatalf("ExportEnv mutated before validation finished: %q", got)
	}
}

func TestExportEnvRejectsBlankEnvName(t *testing.T) {
	err := tcserver.ExportEnv(t, tcserver.ConnectionDetails{"redis.address": "x"}, map[string]string{"redis.address": ""})
	if !errors.Is(err, tcserver.ErrInvalidEnvName) {
		t.Fatalf("ExportEnv error = %v, want ErrInvalidEnvName", err)
	}
}
```

- [ ] **Step 5: Implement the server package**

Implement the minimum code to pass the tests with these exact contracts:

```go
var ErrMissingDetail = errors.New("missing connection detail")
var ErrInvalidEnvName = errors.New("invalid environment variable name")
var ErrInvalidServer = errors.New("invalid testcontainer server")

type ConnectionDetails map[string]string

type Container interface {
	Host(context.Context) (string, error)
	MappedPort(context.Context, string) (network.Port, error)
	PortEndpoint(context.Context, string, string) (string, error)
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

type Server interface {
	Name() string
	Host(context.Context) (string, error)
	MappedPort(context.Context, string) (string, error)
	Endpoint(context.Context, string, string) (string, error)
	ConnectionDetails(context.Context) (ConnectionDetails, error)
	RegisterCleanup(context.Context, testing.TB)
	Terminate(context.Context) error
}
```

Implementation rules:

- `ConnectionDetails.Clone` returns a new map and treats nil as an empty map.
- `ConnectionDetails.Merge` returns a new map where keys from `other` override
  keys from the receiver.
- `ConnectionDetails.Get` returns `("", false)` for missing keys.
- `ConnectionDetails.Require` wraps `ErrMissingDetail` when the key is missing.
- `New` trims and validates `name`, rejects nil containers with
  `ErrInvalidServer`, applies options in order, and stores no global state.
- `WithConnectionDetails` rejects nil functions with `ErrInvalidServer`.
- `WithPort` stores metadata for future docs/validation and rejects blank name
  or blank container port with `ErrInvalidServer`.
- `ConnectionDetails(ctx)` calls the configured detail function and clones the
  returned map before returning it.
- `MappedPort` delegates to the container and returns `port.Port()`.
- `Endpoint` delegates to `PortEndpoint`.
- `RegisterCleanup` and `Terminate` use `internal/testcleanup`.
- `ExportEnv` calls `tb.Helper()`, validates every requested key and env name
  before calling `tb.Setenv`, wraps `ErrMissingDetail` or `ErrInvalidEnvName`,
  and returns nil only after all env values are registered.

- [ ] **Step 6: Run server package tests**

Run:

```bash
go test -count=1 ./testcontainers/server
```

Expected: PASS.

- [ ] **Step 7: Commit Task 1**

```bash
git add testcontainers/server
git commit -m "Add Testcontainers server abstraction"
```

Use the Lore commit trailer block with `Tested: go test -count=1 ./testcontainers/server`.

## Task 2: Adapt Existing Wrappers

**Files:**
- Modify: `testcontainers/postgres/postgres.go`
- Modify: `testcontainers/mysql/mysql.go`
- Modify: `testcontainers/redis/redis.go`
- Modify: `testcontainers/kafka/kafka.go`
- Modify: `testcontainers/nats/nats.go`
- Modify: `testcontainers/*/*_test.go`

- [ ] **Step 1: Add wrapper tests for `StartServer` details**

For each package test, extend the existing Docker smoke test to create one
server through `StartServer`, read `ConnectionDetails`, and use that value for
the client connection. Do not add a second Docker-starting test only to recheck
`Start`; instead implement `Start` through the same private detail extraction
helper used by `StartServer`.

Example Redis shape:

```go
srv := redistestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
	t.Fatalf("redis server details: %v", err)
}
addr, err := details.Require(redistestcontainer.AddressKey)
if err != nil {
	t.Fatalf("redis address detail: %v", err)
}
client := redis.NewClient(&redis.Options{Addr: addr})
```

Kafka must keep `Start(ctx, tb) []string` and use `strings.Split(details[kafkatestcontainer.BrokersKey], ",")` only inside package implementation after validating the detail.

- [ ] **Step 2: Run wrapper tests to confirm missing APIs**

Run:

```bash
go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats
```

Expected: FAIL because `StartServer` does not exist yet.

- [ ] **Step 3: Implement `StartServer` for each wrapper**

For each wrapper:

1. Move module `Run` call into `StartServer(ctx, tb)`.
2. Use `server.New(serviceName, container, server.WithConnectionDetails(serviceDetails))`.
3. If `server.New` fails after the container has started, immediately terminate
   the container through `testcleanup.Terminate` before failing the test.
4. Call `srv.RegisterCleanup(ctx, tb)` immediately after successful construction.
5. Keep `Start(ctx, tb)` by calling `StartServer` and requiring the package key.

Example Redis implementation shape:

```go
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()
	srv := StartServer(ctx, tb)
	return mustDetail(ctx, tb, srv, AddressKey)
}

func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()
	container, err := tcredis.Run(ctx, defaultImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("redis", defaultImage, err))
	}
	srv, err := tcserver.New("redis", container, tcserver.WithConnectionDetails(redisDetails))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("redis server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("redis server: %v", err)
	}
	srv.RegisterCleanup(ctx, tb)
	return srv
}
```

Use small unexported helpers per package for service-specific detail extraction:
`postgresDetails`, `mysqlDetails`, `redisDetails`, `kafkaDetails`, `natsDetails`,
and `mustDetail`.

- [ ] **Step 4: Run wrapper tests serially**

Run:

```bash
go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats
```

Expected: PASS, assuming Docker is available.

- [ ] **Step 5: Commit Task 2**

```bash
git add testcontainers/postgres testcontainers/mysql testcontainers/redis testcontainers/kafka testcontainers/nats
git commit -m "Adapt Testcontainers wrappers to shared server contract"
```

Use Lore trailers and include the serial Docker test command in `Tested:`.

## Task 3: Update Documentation

**Files:**
- Modify: `testcontainers/postgres/README.md`
- Modify: `testcontainers/postgres/README.ko.md`
- Modify: `testcontainers/mysql/README.md`
- Modify: `testcontainers/mysql/README.ko.md`
- Modify: `testcontainers/redis/README.md`
- Modify: `testcontainers/redis/README.ko.md`
- Modify: `testcontainers/kafka/README.md`
- Modify: `testcontainers/kafka/README.ko.md`
- Modify: `testcontainers/nats/README.md`
- Modify: `testcontainers/nats/README.ko.md`

- [ ] **Step 1: Add English README sections**

Each English README must mention:

- `Start(ctx, tb)` remains the easiest typed helper.
- `StartServer(ctx, tb)` returns the shared server abstraction for host, mapped
  port, endpoint, connection details, cleanup, and env export.
- Dynamic host port mapping is the default; read mapped ports after the
  container starts.
- Env export uses `tcserver.ExportEnv` and `testing.TB.Setenv`, so do not use it
  in tests with `t.Parallel` or parallel ancestors.
- Fixed host ports are intentionally not exposed in #217 because they collide in
  parallel/CI runs.

- [ ] **Step 2: Add Korean README sections**

Mirror the English content in each `README.ko.md` with the same API names and
key constants.

- [ ] **Step 3: Run markdown diff check**

Run:

```bash
git diff --check
```

Expected: PASS.

- [ ] **Step 4: Commit Task 3**

```bash
git add testcontainers/*/README.md testcontainers/*/README.ko.md
git commit -m "Document shared Testcontainers server usage"
```

Use Lore trailers with `Tested: git diff --check`.

## Task 4: Final Validation and PR Preparation

**Files:**
- Modify if needed: implementation or docs found by validation.
- Create if needed: Step 6/7 review artifacts under `docs/superpowers/reviews/`.

- [ ] **Step 1: Run focused tests**

```bash
go test -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats
```

Expected: PASS.

- [ ] **Step 2: Run race tests for Testcontainers scope**

```bash
go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats
```

Expected: PASS.

- [ ] **Step 3: Run repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
```

Expected: all PASS. Run `make test` or `make ci` only if time and Docker
availability make it practical; otherwise document the gap in PR DoD.

- [ ] **Step 4: Run Step 6/7 review gates**

Use the bluetape4k full-feature review references. Record six-lane review
findings and the main integration verdict. P0 and P1 must be zero before PR.

- [ ] **Step 5: Create PR**

Push the branch and create a PR linked to #217. Set:

- assignee: `debop`
- milestone: `0.6.5`
- labels copied from issue #217
- final PR heading: `## DoD Status`

PR body must include validation evidence and any skipped commands.
