package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
)

type addRepositoryFunc func(context.Context, string, string) (gitrepo.Object, error)

func (f addRepositoryFunc) Add(ctx context.Context, id, remote string) (gitrepo.Object, error) {
	return f(ctx, id, remote)
}

func repositoryTestClient(t *testing.T, server *control.Server) *Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repo.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRepositoryRegistrationStreamsBeforeCompletion(t *testing.T) {
	server := control.NewServer()
	release := make(chan struct{})
	if err := registerRepositoryAdd(server, addRepositoryFunc(func(ctx context.Context, id, remote string) (gitrepo.Object, error) {
		io.WriteString(gitadapter.ProgressWriter(ctx), "Receiving objects: 50% (1/2)\r")
		<-release
		return gitrepo.Object{Kind: "repo", ID: id, Remote: remote, State: "ready"}, nil
	})); err != nil {
		t.Fatal(err)
	}
	client := repositoryTestClient(t, server)
	seen := make(chan struct{}, 1)
	finished := make(chan error, 1)
	go func() {
		_, err := client.AddRepository(context.Background(), RepositoryAddRequest{ID: "sample", Remote: "https://github.com/example/repo.git"}, repositoryProgressFunc(func(s string) error {
			if s != "Receiving objects: 50% (1/2)\n" {
				t.Error(s)
			}
			seen <- struct{}{}
			return nil
		}))
		finished <- err
	}()
	select {
	case <-seen:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("progress waited for completion")
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryDisconnectCancelsSilentOperation(t *testing.T) {
	server := control.NewServer()
	started, stopped := make(chan struct{}), make(chan struct{})
	registerRepositoryAdd(server, addRepositoryFunc(func(ctx context.Context, _, _ string) (gitrepo.Object, error) {
		if _, ok := ctx.Deadline(); ok {
			t.Error("blanket registration timeout")
		}
		close(started)
		<-ctx.Done()
		close(stopped)
		return gitrepo.Object{}, ctx.Err()
	}))
	client := repositoryTestClient(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := client.AddRepository(ctx, RepositoryAddRequest{ID: "sample", Remote: "https://github.com/example/repo.git"}, io.Discard)
		done <- err
	}()
	<-started
	cancel()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("silent operation survived disconnect")
	}
	if err := <-done; err == nil {
		t.Fatal("cancellation became success")
	}
}

func TestRepositoryRegistrationRejectsLegacyBranch(t *testing.T) {
	server := control.NewServer()
	registerRepositoryAdd(server, addRepositoryFunc(func(context.Context, string, string) (gitrepo.Object, error) {
		t.Error("invalid registration reached service")
		return gitrepo.Object{}, nil
	}))
	client := repositoryTestClient(t, server)
	_, err := client.wire.OpenStream(context.Background(), MethodRepositoryAdd, map[string]string{"id": "sample", "remote": "https://github.com/example/repo.git", "branch": "main"})
	if err == nil {
		t.Fatal("branch accepted in source identity")
	}
}

func TestRepositoryRegistrationPreservesRecoveryFailure(t *testing.T) {
	server := control.NewServer()
	registerRepositoryAdd(server, addRepositoryFunc(func(context.Context, string, string) (gitrepo.Object, error) {
		return gitrepo.Object{}, errors.Join(core.ErrRecoveryRequired, core.ErrAlreadyExists)
	}))
	client := repositoryTestClient(t, server)
	_, err := client.AddRepository(context.Background(), RepositoryAddRequest{ID: "sample", Remote: "https://github.com/example/repo.git"}, io.Discard)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "recovery_required" {
		t.Fatal("native collision hid retained ownership", err)
	}
}

func TestRepositoryStreamRejectsMissingFinalAndHostileFrames(t *testing.T) {
	for _, payload := range []string{"", `{"progress":"token=secret"}` + "\n", strings.Repeat("x", 33<<10), `{"done":true}` + "\n", `{"done":true,"result":{"kind":"repo","id":"other","state":"ready"}}` + "\n"} {
		server := control.NewServer()
		server.RegisterStream(MethodRepositoryAdd, func(context.Context, json.RawMessage) (control.Stream, error) {
			return func(_ context.Context, c net.Conn) error { _, err := io.WriteString(c, payload); return err }, nil
		})
		client := repositoryTestClient(t, server)
		var progress bytes.Buffer
		_, err := client.AddRepository(context.Background(), RepositoryAddRequest{ID: "sample", Remote: "https://github.com/example/repo.git"}, &progress)
		if err == nil || progress.Len() != 0 {
			t.Fatal("invalid stream accepted", err, &progress)
		}
	}
}
