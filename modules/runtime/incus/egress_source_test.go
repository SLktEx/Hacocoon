package incus

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestResolveSourceIPUsesExactIncusIPv4Filter(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		want := []string{"list", "ipv4=10.42.0.10", "--project", defaultProject, "-c", "n", "--format", "csv,noheader"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("args=%#v want=%#v", args, want)
		}
		return host.Result{Stdout: "haco-env-a\n"}, nil
	}}
	got, err := New(runner).ResolveSourceIP(context.Background(), net.ParseIP("10.42.0.10"))
	if err != nil || got != "haco-env-a" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestResolveSourceIPRejectsUnknownAndAmbiguousOwnership(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
		want error
	}{
		{name: "unknown", out: "", want: core.ErrNotFound},
		{name: "ambiguous", out: "haco-a\nhaco-b\n", want: core.ErrIncompatibleState},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
				return host.Result{Stdout: tc.out}, nil
			}}
			_, err := New(runner).ResolveSourceIP(context.Background(), net.ParseIP("10.42.0.10"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
		})
	}
}

func TestResolveSourceIPRejectsIPv6(t *testing.T) {
	_, err := New(&fakeRunner{}).ResolveSourceIP(context.Background(), net.ParseIP("2001:db8::1"))
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v want ErrInvalidArgument", err)
	}
}
