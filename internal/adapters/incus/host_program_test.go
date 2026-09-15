package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/host"
	"reflect"
	"strings"
	"testing"
)

type programRunner struct {
	owned    bool
	executed bool
	fail     bool
}

func (p *programRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "incus" || !reflect.DeepEqual(args, []string{"config", "get", "haco-host", "user.hacocoon.role", "--project", "hacocoon"}) {
		return host.Result{}, errors.New("unexpected ownership check")
	}
	if p.owned {
		return host.Result{Stdout: "trusted-host"}, nil
	}
	return host.Result{Stdout: "unowned"}, nil
}
func (p *programRunner) RunWithInput(ctx context.Context, input []byte, name string, args ...string) (host.Result, error) {
	p.executed = true
	if string(input) != "opaque-request" || strings.Contains(strings.Join(args, " "), "opaque-request") || name != "incus" {
		return host.Result{}, errors.New("unsafe input")
	}
	if _, ok := ctx.Deadline(); !ok {
		return host.Result{}, errors.New("no deadline")
	}
	if !strings.Contains(strings.Join(args, " "), "--property=KillMode=control-group") || !strings.Contains(strings.Join(args, " "), "--property=RuntimeMaxSec=110s") {
		return host.Result{}, errors.New("no lifetime boundary")
	}
	if p.fail {
		return host.Result{Stderr: "secret-child-output"}, errors.New("secret-child-output")
	}
	return host.Result{Stdout: "{}"}, nil
}
func TestTrustedHostProgramChecksOwnershipBoundsLifetimeAndHidesChildErrors(t *testing.T) {
	for _, owned := range []bool{false, true} {
		p := &programRunner{owned: owned}
		r := New(p)
		out, err := r.RunTrustedHostPython(context.Background(), "print('{}')", []byte("opaque-request"))
		if !owned {
			if err == nil || p.executed {
				t.Fatal("unowned Host executed")
			}
			continue
		}
		if err != nil || string(out) != "{}" {
			t.Fatal(err)
		}
		p.fail = true
		_, err = r.RunTrustedHostPython(context.Background(), "print('{}')", []byte("opaque-request"))
		if err == nil || strings.Contains(err.Error(), "secret-child-output") {
			t.Fatal("raw error exposed")
		}
	}
}
