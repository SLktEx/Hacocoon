//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
)

func TestCreationCommandsPreserveSourceArgumentsOverRPC(t *testing.T) {
	for _, tc := range []struct {
		name            string
		args            []string
		method, request string
		result          any
		failure         bool
		code            int
		output          string
	}{
		{"create", []string{"create", "tools", "--name", "work", "--volume", "data"}, controlapi.MethodCreate, `{"name":"work","image":"tools","volume":"data"}`, core.Environment{Name: "work"}, false, 0, "work"},
		{"create-json", []string{"create", "--json", "tools"}, controlapi.MethodCreate, `{"image":"tools"}`, core.Environment{Name: "work"}, false, 0, `"name":"work"`},
		{"create-failure", []string{"create", "tools"}, controlapi.MethodCreate, `{"image":"tools"}`, nil, true, 1, ""},
		{"open-new", []string{"open", "--new", "tools", "--volume", "data", "--client", "none"}, controlapi.MethodOpen, `{"new":{"image":"tools","volume":"data"}}`, core.Environment{Name: "work"}, false, 0, "work"},
		{"open-snapshot", []string{"open", "--new", "--snapshot", "saved", "--client", "none", "--json"}, controlapi.MethodOpen, `{"new":{"snapshot":"saved"}}`, core.Environment{Name: "work"}, false, 0, `"name":"work"`},
		{"continue", []string{"open", "work", "--client", "none"}, controlapi.MethodOpen, `{"environment":"work"}`, core.Environment{Name: "work"}, false, 0, "work"},
		{"open-failure", []string{"open", "work", "--client", "none"}, controlapi.MethodOpen, `{"environment":"work"}`, nil, true, 1, ""},
		{"default-get", []string{"image", "default"}, controlapi.MethodImageDefault, `{}`, core.BaseName("tools"), false, 0, "tools"},
		{"default-set", []string{"image", "default", "tools"}, controlapi.MethodImageDefault, `{"image":"tools"}`, core.BaseName("tools"), false, 0, "tools"},
		{"default-empty", []string{"image", "default"}, controlapi.MethodImageDefault, `{}`, core.BaseName(""), false, 1, ""},
		{"default-failure", []string{"image", "default", "missing"}, controlapi.MethodImageDefault, `{"image":"missing"}`, nil, true, 1, ""},
		{"tag", []string{"image", "tag", "source", "target"}, controlapi.MethodImageCommand, `{"operation":"tag","source":"source","target":"target"}`, core.BaseInfo{Name: "target"}, false, 0, "target"},
		{"commit", []string{"commit", "work", "target"}, controlapi.MethodImageCommand, `{"operation":"commit","source":"work","target":"target"}`, core.BaseInfo{Name: "target"}, false, 0, "target"},
		{"image-remove", []string{"image", "rm", "tools"}, controlapi.MethodImageCommand, `{"operation":"untag","source":"tools"}`, core.BaseInfo{}, false, 0, ""},
		{"image-remove-failure", []string{"image", "rm", "tools"}, controlapi.MethodImageCommand, `{"operation":"untag","source":"tools"}`, nil, true, 1, ""},
		{"tag-failure", []string{"image", "tag", "source", "target"}, controlapi.MethodImageCommand, `{"operation":"tag","source":"source","target":"target"}`, nil, true, 1, ""},
		{"volume-list", []string{"volume", "list"}, controlapi.MethodVolume, `{"operation":"ls"}`, []creation.Volume{{Name: "data"}}, false, 0, "data"},
		{"volume-copy", []string{"volume", "create", "data", "--container", "work"}, controlapi.MethodVolume, `{"operation":"create","name":"data","container":"work"}`, []creation.Volume{{Name: "data"}}, false, 0, "data"},
		{"volume-inspect", []string{"volume", "inspect", "--json", "data"}, controlapi.MethodVolume, `{"operation":"inspect","name":"data"}`, []creation.Volume{{Name: "data"}}, false, 0, `"name":"data"`},
		{"volume-remove", []string{"volume", "rm", "data"}, controlapi.MethodVolume, `{"operation":"rm","name":"data"}`, []creation.Volume{}, false, 0, ""},
		{"volume-busy", []string{"volume", "rm", "data"}, controlapi.MethodVolume, `{"operation":"rm","name":"data"}`, nil, true, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := control.NewServer()
			requests := make(chan json.RawMessage, 1)
			if err := s.Register(tc.method, func(_ context.Context, p json.RawMessage) (any, error) {
				requests <- append(json.RawMessage(nil), p...)
				if tc.failure {
					return nil, control.NewStatusError("busy", "owned resource busy")
				}
				return tc.result, nil
			}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "control.sock")
			listener, err := control.ListenUnix(path, 0600)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- s.Serve(ctx, listener) }()
			t.Cleanup(func() { cancel(); <-done })
			t.Setenv("HACO_CONTROL_SOCKET", path)
			code, out, diagnostic := captureRun(t, tc.args...)
			if code != tc.code || !strings.Contains(out, tc.output) {
				t.Fatalf("code=%d out=%s diagnostic=%s", code, out, diagnostic)
			}
			select {
			case raw := <-requests:
				var got, want map[string]any
				if json.Unmarshal(raw, &got) != nil || json.Unmarshal([]byte(tc.request), &want) != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("request %s, want %s", raw, tc.request)
				}
			default:
				t.Fatal("no controller request")
			}
		})
	}
}

func TestCreationCommandUsageRejectsContradictoryConfiguration(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "absent.sock"))
	for _, args := range [][]string{
		{"create", "--unknown"}, {"create", "--name"},
		{"image"}, {"image", "default", "a", "b"}, {"image", "tag", "source"}, {"commit", "work"},
		{"volume"}, {"volume", "ls", "data"}, {"volume", "create"}, {"volume", "rm", "--container", "work", "data"},
		{"open", "--new", "tools", "--snapshot", "saved"}, {"open", "work", "--volume", "data"},
		{"open", "--new", "--snapshot", "saved", "--volume", "data"}, {"open", "--json"}, {"open", "--client", "invalid"},
	} {
		if code, _, _ := captureRun(t, args...); code != 2 {
			t.Fatal(args, code)
		}
	}
}
