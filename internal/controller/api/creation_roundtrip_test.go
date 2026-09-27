package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
)

type creationBoundary struct {
	calls chan string
	err   error
}

func (f *creationBoundary) call(value string) error { f.calls <- value; return f.err }
func (f *creationBoundary) Create(_ context.Context, r creation.Request) (core.Environment, error) {
	return core.Environment{Name: r.Name}, f.call(fmt.Sprintf("create:%s:%s:%s", r.Name, r.Image, r.Volume))
}
func (f *creationBoundary) OpenTarget(_ context.Context, n string, r *creation.Request) (core.Environment, error) {
	if r != nil {
		n = r.Name
		return core.Environment{Name: n}, f.call("open-snapshot:" + r.Snapshot)
	}
	return core.Environment{Name: n}, f.call("open:" + n)
}
func (f *creationBoundary) DefaultImage(_ context.Context, n core.BaseName) (core.BaseName, error) {
	return "tools", f.call("default:" + string(n))
}
func (f *creationBoundary) Volumes(context.Context) ([]creation.Volume, error) {
	return []creation.Volume{{Name: "data"}}, f.call("list")
}
func (f *creationBoundary) CreateVolume(_ context.Context, n, s string) (creation.Volume, error) {
	return creation.Volume{Name: n}, f.call("volume:" + n + ":" + s)
}
func (f *creationBoundary) InspectVolume(_ context.Context, n string) (creation.Volume, error) {
	return creation.Volume{Name: n}, f.call("inspect:" + n)
}
func (f *creationBoundary) DeleteVolume(_ context.Context, n string) error {
	return f.call("delete-volume:" + n)
}
func (f *creationBoundary) TagImage(_ context.Context, s, t core.BaseName) (core.BaseInfo, error) {
	return core.BaseInfo{Name: t}, f.call("tag:" + string(s) + ":" + string(t))
}
func (f *creationBoundary) RemoveImageTag(_ context.Context, n core.BaseName) error {
	return f.call("untag:" + string(n))
}
func (f *creationBoundary) Commit(_ context.Context, n string, image core.BaseName) (core.BaseInfo, error) {
	return core.BaseInfo{Name: image}, f.call("commit:" + n + ":" + string(image))
}
func (f *creationBoundary) DeleteUser(_ context.Context, n string, force bool) error {
	return f.call(fmt.Sprintf("remove:%s:%t", n, force))
}

func TestCreationManagementRoundTripAndServiceFailures(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			f := &creationBoundary{calls: make(chan string, 20)}
			if failed {
				f.err = core.ErrStorageBusy
			}
			path := doctorTestSocket(t, func(s *control.Server) {
				for _, err := range []error{RegisterCreation(s, f), RegisterVolumes(s, f), RegisterImageCommands(s, f, f), RegisterRemove(s, f)} {
					if err != nil {
						t.Fatal(err)
					}
				}
			})
			c, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			check := func(err error, want string) {
				t.Helper()
				if failed && !creationStatus(err, "busy") || !failed && err != nil {
					t.Fatal(err)
				}
				if got := <-f.calls; got != want {
					t.Fatalf("%q != %q", got, want)
				}
			}
			env, err := c.Create(ctx, creation.Request{Name: "work", Image: "tools", Volume: "data"})
			check(err, "create:work:tools:data")
			if !failed && env.Name != "work" {
				t.Fatal(env)
			}
			_, err = c.OpenTarget(ctx, OpenRequest{Environment: "work"})
			check(err, "open:work")
			_, err = c.OpenTarget(ctx, OpenRequest{New: &creation.Request{Name: "new", Snapshot: "saved"}})
			check(err, "open-snapshot:saved")
			for _, name := range []core.BaseName{"", "tools"} {
				got, err := c.DefaultImage(ctx, name)
				check(err, "default:"+string(name))
				if !failed && got != "tools" {
					t.Fatal(got)
				}
			}
			for _, tc := range []struct {
				r    VolumeRequest
				call string
			}{
				{VolumeRequest{Operation: "ls"}, "list"},
				{VolumeRequest{Operation: "create", Name: "data", Container: "work"}, "volume:data:work"},
				{VolumeRequest{Operation: "inspect", Name: "data"}, "inspect:data"},
				{VolumeRequest{Operation: "rm", Name: "data"}, "delete-volume:data"},
			} {
				got, err := c.Volume(ctx, tc.r)
				check(err, tc.call)
				if !failed && tc.r.Operation != "rm" && (len(got) != 1 || got[0].Name != "data") {
					t.Fatal(got)
				}
			}
			for _, tc := range []struct {
				r    ImageCommandRequest
				call string
			}{
				{ImageCommandRequest{Operation: "tag", Source: "source", Target: "target"}, "tag:source:target"},
				{ImageCommandRequest{Operation: "commit", Source: "work", Target: "target"}, "commit:work:target"},
				{ImageCommandRequest{Operation: "untag", Source: "target"}, "untag:target"},
			} {
				got, err := c.ImageCommand(ctx, tc.r)
				check(err, tc.call)
				if !failed && tc.r.Operation != "untag" && got.Name != "target" {
					t.Fatal(got)
				}
			}
			for _, force := range []bool{false, true} {
				check(c.RemoveEnvironment(ctx, "work", force), fmt.Sprintf("remove:work:%t", force))
			}
		})
	}
}

func TestCreationRPCRejectsExtraAuthorityBeforeDispatch(t *testing.T) {
	f := &creationBoundary{calls: make(chan string, 20)}
	path := doctorTestSocket(t, func(s *control.Server) {
		for _, err := range []error{RegisterCreation(s, f), RegisterVolumes(s, f), RegisterImageCommands(s, f, f), RegisterRemove(s, f)} {
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, payload string }{
		{MethodCreate, `{"name":"work","workspace_path":"/host"}`},
		{MethodOpen, `{"environment":"work","repair":true}`},
		{MethodImageDefault, `{"image":"tools","force":true}`},
		{MethodVolume, `{"operation":"attach","name":"data"}`},
		{MethodVolume, `{"operation":"detach","name":"data"}`},
		{MethodVolume, `{"operation":"inspect","name":"data","container":"work"}`},
		{MethodVolume, `{"operation":"ls","name":"data"}`},
		{MethodVolume, `{"operation":"create","name":"data","path":"/host"}`},
		{MethodImageCommand, `{"operation":"tag","source":"source","target":"target","path":"/host"}`},
		{MethodImageCommand, `{"operation":"tag","source":""}`},
		{MethodImageCommand, `{"operation":"untag","source":"source","target":"target"}`},
		{MethodImageCommand, `{"operation":"pull","source":"source"}`},
		{MethodEnvironmentRemove, `{"environment":"../work"}`},
		{MethodEnvironmentRemove, `{"environment":"work","workspace":"data"}`},
	} {
		if err := c.wire.Call(context.Background(), tc.method, json.RawMessage(tc.payload), nil); !creationStatus(err, "invalid_argument") {
			t.Fatalf("%s %s: %v", tc.method, tc.payload, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatal("invalid request reached service")
	}
	for _, raw := range []string{`{`, `{} {}`, `{} null`, `{"unexpected":true}`} {
		var request creation.Request
		if strictDecode(json.RawMessage(raw), &request) == nil {
			t.Fatal("invalid JSON accepted", raw)
		}
	}
}

func TestCreationRegistrationRefusesMissingAndDuplicateHandlers(t *testing.T) {
	f := &creationBoundary{}
	if !errors.Is(RegisterCreation(nil, f), core.ErrInvalidArgument) || !errors.Is(RegisterCreation(control.NewServer(), nil), core.ErrInvalidArgument) {
		t.Fatal("invalid registration accepted")
	}
	for _, method := range []string{MethodCreate, MethodOpen, MethodImageDefault} {
		s := control.NewServer()
		if err := s.Register(method, func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err != nil {
			t.Fatal(err)
		}
		if RegisterCreation(s, f) == nil {
			t.Fatal("duplicate handler accepted", method)
		}
	}
}

func creationStatus(err error, code string) bool {
	var status *control.StatusError
	return errors.As(err, &status) && status.Code == code
}
