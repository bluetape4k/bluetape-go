package toxiproxytestcontainer_test

import (
	"context"
	"testing"
	"time"

	toxiproxyclient "github.com/Shopify/toxiproxy/v2/client"
	toxiproxytestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/toxiproxy"
	"github.com/redis/go-redis/v9"
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
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cleanupCancel()
		if err := nw.Remove(cleanupCtx); err != nil {
			t.Fatalf("remove network: %v", err)
		}
	})

	redisContainer, err := tcredis.Run(ctx, "redis:7.4-alpine", network.WithNetwork([]string{"redis"}, nw))
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cleanupCancel()
		if err := redisContainer.Terminate(cleanupCtx); err != nil {
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

func TestStartServerConnectionDetails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	srv := toxiproxytestcontainer.StartServer(ctx, t)
	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("toxiproxy details: %v", err)
	}
	controlURI, err := details.Require(toxiproxytestcontainer.ControlURIKey)
	if err != nil {
		t.Fatalf("toxiproxy control uri detail: %v", err)
	}
	if controlURI == "" {
		t.Fatalf("toxiproxy control uri must not be empty")
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if toxiproxytestcontainer.ControlURIKey != "toxiproxy.control_uri" {
		t.Fatalf("ControlURIKey = %q", toxiproxytestcontainer.ControlURIKey)
	}
}
