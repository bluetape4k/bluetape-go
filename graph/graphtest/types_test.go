package graphtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	want := Config{
		StartupTimeout: 90 * time.Second, CaseTimeout: 10 * time.Second,
		CleanupTimeout: 30 * time.Second, CloseTimeout: 10 * time.Second,
		MaxVertices: 16, MaxEdges: 16, MaxTraversalResults: 32,
	}
	if got := DefaultConfig(); got != want {
		t.Fatalf("DefaultConfig() = %#v, want %#v", got, want)
	}
}

func TestValidateAdapterAndDiagnosticHelpers(t *testing.T) {
	t.Parallel()
	adapter := Adapter{
		VerifyConnectivity: func(context.Context) error { return nil },
		CreateFixture:      func(context.Context, Fixture) error { return nil },
		ReadVertices:       func(context.Context, Fixture) ([]graph.Vertex, error) { return nil, nil },
		ReadEdges:          func(context.Context, Fixture) ([]graph.Edge, error) { return nil, nil },
		InvalidOperation:   func(context.Context, Fixture) error { return nil },
		BlockUntilCanceled: func(context.Context, Fixture, Started) error { return nil },
		CleanupFixture:     func(context.Context, Fixture) error { return nil },
		Close:              func(context.Context) error { return nil },
		IsProviderError:    func(error) bool { return false },
	}
	capabilities := Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "not-implemented"}}
	if err := validateAdapter(adapter, capabilities); err != nil {
		t.Fatalf("validateAdapter() error = %v", err)
	}

	cause := errors.New("secret cause")
	err := category("read", cause)
	if !errors.Is(err, cause) || err.Error() != "graphtest: read failed" {
		t.Fatalf("category() = %v", err)
	}
	if got := imageDigest("fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); got != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("imageDigest() = %q", got)
	}
}

func TestRunWithConfigRejectsZeroConfigBeforeFactory(t *testing.T) {
	called := false
	h := Harness{
		Provider: ProviderMetadata{Name: "fake", Version: "1.0.0", ImageReference: "fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		New: func(context.Context, testing.TB, Config) (Adapter, error) {
			called = true
			return Adapter{}, nil
		},
		Capabilities: Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "not-implemented"}},
	}
	if err := validateHarness(h, Config{}); err == nil {
		t.Fatal("validateHarness() error = nil, want invalid config")
	}
	if called {
		t.Fatal("factory called for invalid config")
	}
}

func TestValidateRejectsUnsafeMetadataAndConfigBoundaries(t *testing.T) {
	t.Parallel()
	valid := DefaultConfig()
	base := Harness{
		Provider: ProviderMetadata{Name: "fake", Version: "1.0.0", ImageReference: "fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		New:      func(context.Context, testing.TB, Config) (Adapter, error) { return Adapter{}, nil },
		Capabilities: Capabilities{
			CapabilityTraversal: {Enabled: false, ReasonCode: "unsupported-by-fake"},
		},
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Harness, *Config)
	}{
		{"zero-startup", func(_ *Harness, c *Config) { c.StartupTimeout = 0 }},
		{"startup-over-max", func(_ *Harness, c *Config) { c.StartupTimeout = MaxStartupTimeout + time.Nanosecond }},
		{"result-over-max", func(_ *Harness, c *Config) { c.MaxVertices = MaxResultLimit + 1 }},
		{"credential-uri", func(h *Harness, _ *Config) { h.Provider.ImageReference = "bolt://user:secret@example" }},
		{"newline-name", func(h *Harness, _ *Config) { h.Provider.Name = "fake\nquery" }},
		{"credential-like-name", func(h *Harness, _ *Config) { h.Provider.Name = "user:secret" }},
		{"credential-like-version", func(h *Harness, _ *Config) { h.Provider.Version = "https://token@example" }},
		{"credential-image", func(h *Harness, _ *Config) {
			h.Provider.ImageReference = "registry/path:user:secret@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"shell-image", func(h *Harness, _ *Config) {
			h.Provider.ImageReference = "repo/image:1;echo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"bidi-image", func(h *Harness, _ *Config) {
			h.Provider.ImageReference = "repo/image:\u202e1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"line-separator-image", func(h *Harness, _ *Config) {
			h.Provider.ImageReference = "repo/image:1\u2028@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"missing-capability", func(h *Harness, _ *Config) { h.Capabilities = nil }},
		{"unknown-capability", func(h *Harness, _ *Config) {
			h.Capabilities = Capabilities{Capability("unknown"): {Enabled: false, ReasonCode: "not-implemented"}}
		}},
		{"enabled-with-reason", func(h *Harness, _ *Config) {
			h.Capabilities[CapabilityTraversal] = Support{Enabled: true, ReasonCode: "not-implemented"}
		}},
		{"unsafe-reason", func(h *Harness, _ *Config) {
			h.Capabilities[CapabilityTraversal] = Support{ReasonCode: "not supported\nMATCH (n)"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cfg := base, valid
			h.Capabilities = Capabilities{CapabilityTraversal: base.Capabilities[CapabilityTraversal]}
			tc.mutate(&h, &cfg)
			if err := validateHarness(h, cfg); err == nil {
				t.Fatal("validateHarness() error = nil")
			}
		})
	}
}

func TestValidateAdapterRequiresCapabilityCallbackParity(t *testing.T) {
	t.Parallel()
	adapter := completeAdapter()
	if err := validateAdapter(adapter, Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "not-implemented"}}); err == nil {
		t.Fatal("disabled traversal accepted a callback")
	}
	adapter.Traverse = nil
	if err := validateAdapter(adapter, Capabilities{CapabilityTraversal: {Enabled: true}}); err == nil {
		t.Fatal("enabled traversal accepted a nil callback")
	}
}
