//go:build linux

package incus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnvironmentNetworkRejectsProviderHostPID(t *testing.T) {
	const instance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch args[0] {
		case "config":
			return host.Result{Stdout: instance}, nil
		case "query":
			// A valid-looking ownership response cannot authorize Host namespace
			// entry. This test needs neither root nor a listener on Host port 22.
			return host.Result{Stdout: fmt.Sprintf(`{"pid":%d,"status":"Running"}`, os.Getpid())}, nil
		default:
			t.Fatalf("unexpected provider operation: %s", args[0])
			return host.Result{}, core.ErrUnsupported
		}
	}}
	conn, err := New(runner).DialEnvironmentNetwork(context.Background(), "haco-dev", instance, "tcp", 22)
	if conn != nil {
		conn.Close()
		t.Fatal("provider Host PID opened a connection")
	}
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("Host namespace was not rejected before dialing: %v", err)
	}
}
