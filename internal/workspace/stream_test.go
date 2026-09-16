package workspace

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	environment "github.com/SLktEx/Hacocoon/internal/env"
)

type shellStreamProvider struct {
	*fakeEnvironmentRuntime
	calls    int
	terminal core.TerminalMetadata
}

func (p *shellStreamProvider) ShellEnvironmentStream(ctx context.Context, ref string, stdin io.Reader, stdout, stderr io.Writer) error {
	p.calls++
	p.shellRef = ref
	p.terminal = core.TerminalMetadataFromContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.shellErr != nil {
		return p.shellErr
	}
	if _, err := io.Copy(stdout, stdin); err != nil {
		return err
	}
	_, err := io.WriteString(stderr, "shell diagnostic")
	return err
}

// Exercise the same Service -> Router -> provider composition as the controller;
// a direct fake runtime would hide a missing optional method on the Router.
func TestRoutedShellStreamPreservesOwnerTerminalAndIO(t *testing.T) {
	owner := &shellStreamProvider{fakeEnvironmentRuntime: &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-owned"}}}
	other := &shellStreamProvider{fakeEnvironmentRuntime: &fakeEnvironmentRuntime{}}
	router, err := environment.NewRouter("owner", environment.Register("owner", owner))
	if err != nil {
		t.Fatal(err)
	}
	created, err := router.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{})
	if err != nil {
		t.Fatal(err)
	}
	router, err = environment.NewRouter("other", environment.Register("owner", owner), environment.Register("other", other))
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: created.Ref}
	service := New(router, store)
	terminal := core.TerminalMetadata{Term: "xterm-256color", ColorTerm: "truecolor"}
	prepareCtx := core.WithTerminalMetadata(context.Background(), terminal)
	prepared, err := service.PrepareShellStream(prepareCtx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if owner.calls != 0 || other.calls != 0 {
		t.Fatal("preparation opened a shell")
	}
	var stdout, stderr strings.Builder
	if err := prepared(context.Background(), strings.NewReader("shell input\x00\n"), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if owner.shellRef != "haco-owned" || owner.terminal != terminal || other.calls != 0 || stdout.String() != "shell input\x00\n" || stderr.String() != "shell diagnostic" {
		t.Fatal("shell route lost owner, terminal or streams", owner, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	owner.shellErr = core.ErrRuntimeUnavailable
	if err := service.ShellStream(prepareCtx, "demo", strings.NewReader(""), &stdout, &stderr); !errors.Is(err, owner.shellErr) {
		t.Fatal("provider failure lost", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := prepared(ctx, strings.NewReader(""), &stdout, &stderr); !errors.Is(err, context.Canceled) || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("cancellation or failed-shell stream isolation lost", err)
	}
	if err := prepared(context.Background(), nil, &stdout, &stderr); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := service.PrepareShellStream(context.Background(), "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := New(&fakeEnvironmentRuntime{}, store).PrepareShellStream(context.Background(), "demo"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}
