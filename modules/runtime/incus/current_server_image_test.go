package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestCurrentServerIncusArgsNormalizesOnlyImageSourceSlots(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "init",
			in:   []string{"init", "local:" + testFingerprintA, "builder", "--project", defaultProject},
			want: []string{"init", testFingerprintA, "builder", "--project", defaultProject},
		},
		{
			name: "image info",
			in:   []string{"image", "info", "local:seed-alias", "--project", defaultProject, "--format", "json"},
			want: []string{"image", "info", "seed-alias", "--project", defaultProject, "--format", "json"},
		},
		{
			name: "image list remote token",
			in:   []string{"image", "list", "local:", testFingerprintA, "--format", "csv"},
			want: []string{"image", "list", testFingerprintA, "--format", "csv"},
		},
		{
			name: "guest argv untouched",
			in:   []string{"exec", "haco-demo", "--project", defaultProject, "--", "printf", "local:keep-me"},
			want: []string{"exec", "haco-demo", "--project", defaultProject, "--", "printf", "local:keep-me"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := currentServerIncusArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args=%#v want=%#v", got, tt.want)
			}
			if len(tt.in) > 0 && tt.in[len(tt.in)-1] != "" && tt.name == "guest argv untouched" && tt.in[len(tt.in)-1] != "local:keep-me" {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestImageFingerprintFallbackSupportsCurrentServerReference(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		switch call {
		case 0:
			assertStringSlice(t, args, []string{"image", "info", compatImageFingerprint, "--project", defaultProject, "--format", "json"})
			return host.Result{ExitCode: 1, Stderr: "Error: unknown flag: --format\n"}, errors.New("exit status 1")
		case 1:
			assertStringSlice(t, args, []string{"image", "list", compatImageFingerprint, "--format", "csv", "-c", "L,F,t", "--project", defaultProject})
			return host.Result{Stdout: "," + compatImageFingerprint + ",CONTAINER\n"}, nil
		default:
			t.Fatalf("unexpected call %d: %#v", call, args)
			return host.Result{}, nil
		}
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	got, err := provider.imageFingerprint(context.Background(), compatImageFingerprint, defaultProject)
	if err != nil {
		t.Fatal(err)
	}
	if got != compatImageFingerprint {
		t.Fatalf("fingerprint=%q want=%q", got, compatImageFingerprint)
	}
}
