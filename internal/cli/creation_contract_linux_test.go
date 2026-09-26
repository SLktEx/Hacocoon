//go:build linux

package cli

import (
	"bytes"
	"context"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"reflect"
	"testing"
)

type creationCLI struct {
	calls   []string
	request creation.Request
	open    controlapi.OpenRequest
}

func (f *creationCLI) Create(_ context.Context, r creation.Request) (core.Environment, error) {
	f.calls = append(f.calls, "create")
	f.request = r
	return core.Environment{Name: "new"}, nil
}
func (f *creationCLI) OpenTarget(_ context.Context, r controlapi.OpenRequest) (core.Environment, error) {
	f.calls = append(f.calls, "create-start")
	f.open = r
	return core.Environment{Name: "new"}, nil
}
func TestCreateCLIRequiresImageAndNeverOpens(t *testing.T) {
	f := &creationCLI{}
	var out, err bytes.Buffer
	for _, args := range [][]string{nil, {""}} {
		if code := createCommand(context.Background(), f, args, &out, &err); code != 2 || len(f.calls) != 0 {
			t.Fatal(code, f.calls)
		}
	}
	if code := createCommand(context.Background(), f, []string{"tools", "--name", "new", "--volume", "data"}, &out, &err); code != 0 || !reflect.DeepEqual(f.calls, []string{"create"}) || f.request.Image != "tools" || f.request.Volume != "data" {
		t.Fatal(code, f)
	}
}
func TestOpenCLISelectionAndLaunchOrdering(t *testing.T) {
	for _, tc := range []struct {
		args             []string
		image            core.BaseName
		snapshot, volume string
	}{
		{[]string{"--new"}, "", "", ""},
		{[]string{"--new", "tools", "--volume", "data"}, "tools", "", "data"},
		{[]string{"--new", "--snapshot", "saved"}, "", "saved", ""},
	} {
		f := &creationCLI{}
		var out, err bytes.Buffer
		code := openCreationCommand(context.Background(), f, tc.args, &out, &err, func(e core.Environment, _ string) int {
			if e.Name != "new" {
				t.Fatal(e)
			}
			f.calls = append(f.calls, "open")
			return 0
		})
		if code != 0 || !reflect.DeepEqual(f.calls, []string{"create-start", "open"}) || f.open.New == nil || f.open.New.Image != tc.image || f.open.New.Snapshot != tc.snapshot || f.open.New.Volume != tc.volume {
			t.Fatal(code, f, err.String())
		}
	}
}
func TestRetiredCommandsAreUnavailable(t *testing.T) {
	for _, args := range [][]string{{"run", "--help"}, {"volume", "attach", "v", "e"}, {"volume", "detach", "v"}, {"snapshot", "restore", "saved"}} {
		code, _, _ := captureRun(t, args...)
		if code != 2 {
			t.Fatal(args, code)
		}
	}
}
