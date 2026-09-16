package environment

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// This provider makes the routing contract observable: the returned connection,
// process bytes, error and native identity must all come from the selected owner.
type contractProvider struct {
	calls        int
	ref          string
	payload      any
	ctx          context.Context
	failure      error
	inputSupport bool
	conn         net.Conn
}

func (p *contractProvider) observe(ctx context.Context, ref string, payload any) error {
	p.calls++
	p.ref, p.payload, p.ctx = ref, payload, ctx
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return p.failure
}
func (p *contractProvider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{Ref: "owned-native", Resources: spec.Resources}, p.observe(ctx, "", spec)
}
func (p *contractProvider) ExecEnvironment(ctx context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{ExitCode: 17, Stdout: string(req.Stdin)}, p.observe(ctx, ref, req)
}
func (p *contractProvider) ShellEnvironment(ctx context.Context, ref string) error {
	return p.observe(ctx, ref, nil)
}
func (p *contractProvider) DeleteEnvironment(ctx context.Context, ref string) error {
	return p.observe(ctx, ref, nil)
}
func (p *contractProvider) StopEnvironment(ctx context.Context, ref string) error {
	return p.observe(ctx, ref, nil)
}
func (p *contractProvider) StartEnvironment(ctx context.Context, ref string) error {
	return p.observe(ctx, ref, nil)
}
func (p *contractProvider) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	return core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}, p.observe(ctx, ref, nil)
}
func (p *contractProvider) ForwardLocalPort(ctx context.Context, ref string, req core.LocalPortRequest) (core.ClientConnection, error) {
	return core.ClientConnection{ID: "owned-connection", Port: req.HostPort, TargetPort: req.TargetPort}, p.observe(ctx, ref, req)
}
func (p *contractProvider) RemoveClientConnection(ctx context.Context, ref, id string) error {
	return p.observe(ctx, ref, id)
}
func (p *contractProvider) PrepareSSHAccess(ctx context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	return core.ClientConnection{ID: "owned-grant", HostPublicKey: "pinned-host-key"}, p.observe(ctx, ref, req)
}
func (p *contractProvider) RevokeSSHAccess(ctx context.Context, ref, id string) error {
	return p.observe(ctx, ref, id)
}
func (p *contractProvider) ListClientConnections(ctx context.Context, ref string) ([]core.ClientConnection, error) {
	return []core.ClientConnection{{ID: "owned-connection"}}, p.observe(ctx, ref, nil)
}
func (p *contractProvider) ResolveEnvironmentName(ctx context.Context, ref, instance, name string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("192.0.2.10")}, p.observe(ctx, ref, []string{instance, name})
}
func (p *contractProvider) DialEnvironmentTCP(ctx context.Context, ref, instance, address string, port int) (net.Conn, error) {
	return p.conn, p.observe(ctx, ref, []any{instance, address, port})
}
func (p *contractProvider) DialEnvironmentNetwork(ctx context.Context, ref, instance, protocol string, port int) (net.Conn, error) {
	return p.conn, p.observe(ctx, ref, []any{instance, protocol, port})
}
func (p *contractProvider) ExecEnvironmentStream(ctx context.Context, ref string, req core.ProcessRequest, stdin io.Reader, stdout, stderr io.Writer) (core.ExecutionResult, error) {
	if err := p.observe(ctx, ref, req); err != nil {
		return core.ExecutionResult{}, err
	}
	if _, err := io.Copy(stdout, stdin); err != nil {
		return core.ExecutionResult{}, err
	}
	if _, err := io.WriteString(stderr, "process diagnostic"); err != nil {
		return core.ExecutionResult{}, err
	}
	return core.ExecutionResult{ExitCode: 17}, nil
}
func (p *contractProvider) SupportsWorkingDirectory() bool { return p.inputSupport }
func (p *contractProvider) SupportsStdin() bool            { return p.inputSupport }

type requiredOnlyProvider struct{ Provider }

type routeOperation struct {
	name     string
	call     func(context.Context, *Router, string) (any, error)
	want     any
	payload  any
	optional bool
}

func TestPersistedEnvironmentRoutePinsOperationsAfterDefaultChanges(t *testing.T) {
	const instance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	client, peer := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = peer.Close() }()
	execution := core.ExecutionRequest{Argv: []string{"cat"}, Stdin: []byte("input\x00bytes"), WorkingDirectory: "/workspace"}
	port := core.LocalPortRequest{Protocol: "udp", HostPort: 18000, TargetPort: 9000}
	ssh := core.SSHAccessRequest{PublicKey: "caller-owned-key"}
	ops := []routeOperation{
		{"exec", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.ExecEnvironment(ctx, ref, execution)
		}, core.ExecutionResult{ExitCode: 17, Stdout: string(execution.Stdin)}, execution, false},
		{"shell", func(ctx context.Context, r *Router, ref string) (any, error) {
			return nil, r.ShellEnvironment(ctx, ref)
		}, nil, nil, false},
		{"delete", func(ctx context.Context, r *Router, ref string) (any, error) {
			return nil, r.DeleteEnvironment(ctx, ref)
		}, nil, nil, false},
		{"stop", func(ctx context.Context, r *Router, ref string) (any, error) { return nil, r.StopEnvironment(ctx, ref) }, nil, nil, true},
		{"start", func(ctx context.Context, r *Router, ref string) (any, error) {
			return nil, r.StartEnvironment(ctx, ref)
		}, nil, nil, true},
		{"inspect", func(ctx context.Context, r *Router, ref string) (any, error) { return r.InspectEnvironment(ctx, ref) }, core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}, nil, true},
		{"forward", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.ForwardLocalPort(ctx, ref, port)
		}, core.ClientConnection{ID: "owned-connection", Port: port.HostPort, TargetPort: port.TargetPort}, port, true},
		{"disconnect", func(ctx context.Context, r *Router, ref string) (any, error) {
			return nil, r.RemoveClientConnection(ctx, ref, "owned-connection")
		}, nil, "owned-connection", true},
		{"ssh", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.PrepareSSHAccess(ctx, ref, ssh)
		}, core.ClientConnection{ID: "owned-grant", HostPublicKey: "pinned-host-key"}, ssh, true},
		{"revoke", func(ctx context.Context, r *Router, ref string) (any, error) {
			return nil, r.RevokeSSHAccess(ctx, ref, "owned-grant")
		}, nil, "owned-grant", true},
		{"connections", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.ListClientConnections(ctx, ref)
		}, []core.ClientConnection{{ID: "owned-connection"}}, nil, true},
		{"dns", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.ResolveEnvironmentName(ctx, ref, instance, "example.test")
		}, []netip.Addr{netip.MustParseAddr("192.0.2.10")}, []string{instance, "example.test"}, true},
		{"tcp", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.DialEnvironmentTCP(ctx, ref, instance, "127.0.0.1", 9000)
		}, client, []any{instance, "127.0.0.1", 9000}, true},
		{"network", func(ctx context.Context, r *Router, ref string) (any, error) {
			return r.DialEnvironmentNetwork(ctx, ref, instance, "udp", 9000)
		}, client, []any{instance, "udp", 9000}, true},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			owner := &contractProvider{inputSupport: true, conn: client}
			original, err := NewRouter(testProvider, Register(testProvider, owner))
			if err != nil {
				t.Fatal(err)
			}
			created, err := original.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			other := &contractProvider{inputSupport: true}
			router, err := NewRouter("other", Register("other", other), Register(testProvider, owner))
			if err != nil {
				t.Fatal(err)
			}
			for _, failure := range []error{nil, core.ErrCapabilityStale, core.ErrRuntimeUnavailable} {
				owner.calls, owner.failure = 0, failure
				ctx, cancel := context.WithCancel(context.Background())
				got, err := op.call(ctx, router, created.Ref)
				cancel()
				if !errors.Is(err, failure) || owner.calls != 1 || other.calls != 0 || owner.ref != "owned-native" || owner.ctx != ctx || !reflect.DeepEqual(owner.payload, op.payload) {
					t.Fatalf("wrong authority/request/error: got=%#v error=%v owner=%+v other calls=%d", got, err, owner, other.calls)
				}
				if failure == nil && !reflect.DeepEqual(got, op.want) {
					t.Fatalf("result=%#v, want=%#v", got, op.want)
				}
			}
			for _, invalid := range []string{"", "haco-runtime-v1:", "haco-runtime-v1:missing:bmF0aXZl", "haco-runtime-v1:runtime.test:%%%"} {
				owner.calls = 0
				if _, err := op.call(context.Background(), router, invalid); err == nil || owner.calls != 0 || other.calls != 0 {
					t.Fatal("unresolved route reached provider", invalid, err)
				}
			}
			if _, err := op.call(context.Background(), nil, created.Ref); !errors.Is(err, core.ErrRuntimeUnavailable) {
				t.Fatal("nil router accepted", err)
			}
			if op.optional {
				limited, err := NewRouter(testProvider, Register(testProvider, requiredOnlyProvider{owner}))
				if err != nil {
					t.Fatal(err)
				}
				owner.calls = 0
				if _, err := op.call(context.Background(), limited, created.Ref); !errors.Is(err, core.ErrUnsupported) || owner.calls != 0 {
					t.Fatal("missing capability was silently substituted", err)
				}
			}
		})
	}
}

func TestProcessRoutePreservesStreamsExitAndCancellation(t *testing.T) {
	p := &contractProvider{}
	r, err := NewRouter(testProvider, Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	created, err := r.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{})
	if err != nil {
		t.Fatal(err)
	}
	req := core.ProcessRequest{Argv: []string{"cat"}}
	var stdout, stderr strings.Builder
	result, err := r.ExecEnvironmentStream(context.Background(), created.Ref, req, strings.NewReader("raw\x00input\n"), &stdout, &stderr)
	if err != nil || result.ExitCode != 17 || stdout.String() != "raw\x00input\n" || stderr.String() != "process diagnostic" || p.ref != "owned-native" || !reflect.DeepEqual(p.payload, req) {
		t.Fatal(result, err, stdout.String(), stderr.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stdout.Reset()
	stderr.Reset()
	if _, err := r.ExecEnvironmentStream(ctx, created.Ref, req, strings.NewReader("discard"), &stdout, &stderr); !errors.Is(err, context.Canceled) || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("cancellation or stream isolation lost", err)
	}
	limited, _ := NewRouter(testProvider, Register(testProvider, requiredOnlyProvider{p}))
	p.calls = 0
	for _, tc := range []struct {
		router *Router
		ref    string
		want   error
	}{
		{limited, created.Ref, core.ErrUnsupported}, {r, "", core.ErrIncompatibleState}, {nil, created.Ref, core.ErrRuntimeUnavailable},
	} {
		if _, err := tc.router.ExecEnvironmentStream(context.Background(), tc.ref, req, nil, io.Discard, io.Discard); !errors.Is(err, tc.want) || p.calls != 0 {
			t.Fatal("invalid process request reached backend", err)
		}
	}
}

func TestShellStreamRouteRejectsUnavailableProvider(t *testing.T) {
	p := &contractProvider{}
	r, err := NewRouter(testProvider, Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		router *Router
		ref    string
		want   error
	}{
		{r, encodeRouteRef(testProvider, "owned-native"), core.ErrUnsupported},
		{r, "", core.ErrIncompatibleState},
		{r, encodeRouteRef("removed-provider", "owned-native"), core.ErrUnsupported},
		{nil, encodeRouteRef(testProvider, "owned-native"), core.ErrRuntimeUnavailable},
	} {
		if err := tc.router.ShellEnvironmentStream(context.Background(), tc.ref, strings.NewReader("input"), io.Discard, io.Discard); !errors.Is(err, tc.want) || p.calls != 0 {
			t.Fatal("unavailable shell route reached provider", err)
		}
	}
}

func TestExecutionRejectsUnsupportedInputBeforeProviderSideEffects(t *testing.T) {
	for _, advertised := range []bool{false, true} {
		p := &contractProvider{inputSupport: advertised}
		r, _ := NewRouter(testProvider, Register(testProvider, p))
		for _, req := range []core.ExecutionRequest{
			{Argv: []string{"cat"}, Stdin: make([]byte, core.MaxExecutionInputBytes+1)},
			{Argv: []string{"cat"}, Stdin: []byte{}},
			{Argv: []string{"pwd"}, WorkingDirectory: "/workspace"},
		} {
			p.calls = 0
			_, err := r.ExecEnvironment(context.Background(), encodeRouteRef(testProvider, "owned-native"), req)
			if len(req.Stdin) > core.MaxExecutionInputBytes {
				if !errors.Is(err, core.ErrInvalidArgument) || p.calls != 0 {
					t.Fatal("oversized input reached provider", err)
				}
			} else if advertised {
				if err != nil || p.calls != 1 {
					t.Fatal("supported input refused", err)
				}
			} else if !errors.Is(err, core.ErrUnsupported) || p.calls != 0 {
				t.Fatal("unsupported input reached provider", err)
			}
		}
	}
}

func TestRegistrationRejectsAmbiguousRouteIDsBeforeCreation(t *testing.T) {
	p := &contractProvider{}
	for _, id := range []string{"", " spaced", "line\nbreak", "nul\x00id", "runtime:ambiguous"} {
		if _, err := NewRouter(testProvider, Register(testProvider, p), Register(id, p)); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("registration %q: %v", id, err)
		}
	}
	for _, tc := range []struct {
		name string
		regs []Registration
		want error
	}{
		{"", []Registration{Register(testProvider, p)}, core.ErrInvalidArgument},
		{testProvider, []Registration{Register(testProvider, nil)}, core.ErrInvalidArgument},
		{testProvider, []Registration{Register(testProvider, p), Register(testProvider, p)}, core.ErrAlreadyExists},
		{"missing", []Registration{Register(testProvider, p)}, core.ErrNotFound},
	} {
		if _, err := NewRouter(tc.name, tc.regs...); !errors.Is(err, tc.want) {
			t.Fatal(tc.name, err)
		}
	}
	if p.calls != 0 {
		t.Fatal("invalid registration created a resource")
	}
	if _, err := NewRouter(testProvider, Register(testProvider, p)); err != nil {
		t.Fatal("supported provider ID refused", err)
	}
	var unavailable *Router
	if _, err := unavailable.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{}); !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatal(err)
	}
}

func TestRuntimeEvidenceRequiresProviderAndNativeIdentity(t *testing.T) {
	raw := encodeRouteRef(testProvider, "native:opaque/reference")
	for _, tc := range []struct {
		raw, provider, native string
		want                  bool
	}{
		{raw, testProvider, "native:opaque/reference", true},
		{raw, "other", "native:opaque/reference", false},
		{raw, testProvider, "replacement", false},
		{raw, "", "native:opaque/reference", false},
		{raw, testProvider, "", false},
		{"haco-runtime-v1:broken", testProvider, "native", false},
		{"haco-retained", ProviderIncus, "haco-retained", true},
	} {
		if MatchesRuntimeRef(tc.raw, tc.provider, tc.native) != tc.want {
			t.Fatalf("evidence binding mismatch: %+v", tc)
		}
	}
}

func TestTCPRouteRejectsInvalidTargetsBeforeDialing(t *testing.T) {
	p := &contractProvider{}
	r, _ := NewRouter(testProvider, Register(testProvider, p))
	const validInstance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		address, instance string
		port              int
	}{
		{"localhost", validInstance, 80}, {"127.0.0.1", "", 80}, {"127.0.0.1", validInstance, 0}, {"127.0.0.1", validInstance, 65536},
	} {
		conn, err := r.DialEnvironmentTCP(context.Background(), encodeRouteRef(testProvider, "native"), tc.instance, tc.address, tc.port)
		if !errors.Is(err, core.ErrInvalidArgument) || conn != nil || p.calls != 0 {
			t.Fatal("invalid target reached provider", tc, err)
		}
	}
}

func TestMissingEnvironmentResourceProviderIsNotAdvertised(t *testing.T) {
	p := &fakeProvider{}
	limited, _ := NewRouter(testProvider, Register(testProvider, p))
	for _, r := range []*Router{nil, {defaultProvider: "missing"}, limited} {
		if r.SupportsEnvironmentResources() {
			t.Fatal("missing capability advertised")
		}
		if err := r.StartEnvironmentWithResources(context.Background(), "", core.EnvironmentResourceBinding{}); err == nil || p.ref != "" {
			t.Fatal("unresolved resource binding reached provider", err)
		}
	}
}
