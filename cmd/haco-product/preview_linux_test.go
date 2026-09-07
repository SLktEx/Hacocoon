//go:build linux

package main

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type previewFixture struct {
	connections              []core.ClientConnection
	created, started, closed int
}

func (f *previewFixture) ListEnvironments(context.Context) ([]core.Environment, error) {
	return []core.Environment{{Name: "dev"}}, nil
}
func (f *previewFixture) StartEnvironment(context.Context, string) error { f.started++; return nil }
func (f *previewFixture) EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error) {
	return f.connections, nil
}
func (f *previewFixture) ForwardEnvironment(_ context.Context, _ string, r core.LocalPortRequest) (core.ClientConnection, error) {
	if r.HostPort != 0 || r.TargetPort != 3000 {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	f.created++
	c := core.ClientConnection{ID: "tcp-45000-3000", Kind: "tcp", Host: "127.0.0.1", Port: 45000, TargetPort: 3000}
	f.connections = append(f.connections, c)
	return c, nil
}
func (f *previewFixture) UnforwardEnvironment(context.Context, string, string) error {
	f.closed++
	f.connections = nil
	return nil
}
func TestPreviewReusesAndClosesConnection(t *testing.T) {
	f := &previewFixture{}
	for i := 0; i < 2; i++ {
		url, err := preview(context.Background(), f, "", 3000, false)
		if err != nil || url != "http://127.0.0.1:45000/" {
			t.Fatalf("%q %v", url, err)
		}
	}
	if f.created != 1 {
		t.Fatal("duplicate connection")
	}
	if _, err := preview(context.Background(), f, "dev", 3000, true); err != nil {
		t.Fatal(err)
	}
	if f.closed != 1 || f.started != 2 {
		t.Fatal("close resumed Environment or missed removal")
	}
}
func TestPreviewRefusesUntrustedEndpoint(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "example.com", "127.0.0.1'; bad"} {
		f := &previewFixture{connections: []core.ClientConnection{{Kind: "tcp", Host: host, Port: 45000, TargetPort: 3000, ID: "tcp-45000-3000"}}}
		if _, err := preview(context.Background(), f, "dev", 3000, false); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatal(err)
		}
		if f.created != 0 {
			t.Fatal("replaced untrusted connection")
		}
	}
}
