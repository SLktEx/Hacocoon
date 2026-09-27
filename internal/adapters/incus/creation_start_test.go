package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestCreatedEnvironmentStartRefusesIncompleteMarker(t *testing.T) {
	for _, result := range []host.Result{{ExitCode: 1}, {StdoutTruncated: true}, {Stdout: "unexpected"}} {
		calls := 0
		runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) { calls++; return result, nil }}
		provider, err := NewSandboxProvider(New(runner))
		if err != nil {
			t.Fatal(err)
		}
		if err = provider.initializeCreatedEnvironment(context.Background(), "haco-demo"); err == nil {
			t.Fatal("accepted invalid provider marker")
		}
		if calls != 1 {
			t.Fatal("continued guest initialization after invalid marker")
		}
	}
}

func TestCreatedEnvironmentInitializationIsBoundToItsGeneration(t *testing.T) {
	identity := core.NewEnvironmentInstanceID()
	var script string
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) > 3 && args[0] == "config" {
			if args[3] == "user.hacocoon.creation" {
				return host.Result{Stdout: "stopped"}, nil
			}
			if args[3] == environmentInstanceKey {
				return host.Result{Stdout: identity}, nil
			}
		}
		if args[0] == "query" {
			return host.Result{Stdout: `{"name":"haco-demo","devices":{"persistent-resource":{"type":"disk","path":"/var/lib/hacocoon-oci"}}}`}, nil
		}
		if args[0] == "exec" {
			script = args[len(args)-3]
			if args[len(args)-1] != identity {
				t.Fatal(args)
			}
			return host.Result{}, nil
		}
		t.Fatal(args)
		return host.Result{}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	if err = provider.initializeCreatedEnvironment(context.Background(), "haco-demo"); err != nil {
		t.Fatal(err)
	}
	guard := strings.Index(script, "if test ! -f /etc/hacocoon-instance")
	config := strings.Index(script, persistentOCIConfiguration)
	receipt := strings.Index(script, " > /etc/hacocoon-instance")
	if guard < 0 || config < guard || receipt < config {
		t.Fatal("initial configuration escaped generation guard")
	}
}
