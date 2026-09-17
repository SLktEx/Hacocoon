package gitadapter

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGitDiagnosticsFilterSecretsAndFragmentedProgress(t *testing.T) {
	var out bytes.Buffer
	d := NewGitDiagnostic(&out)
	input := "Cloning into '/secret/path'...\nremote: Enumerating objects: 5, done.\rReceiving objects: 100% (5/5), 1.23 KiB | 1.00 MiB/s, done.\n" +
		"remote: token=ghp_secret\nAuthorization: Bearer secret\n" +
		"fatal: Authentication failed for 'https://secret@github.com/x/y'\n" +
		strings.Repeat("x", 5000) + "Receiving objects: 50% (1/2)\n" +
		"remote: Receiving objects: 50% (1/2)\x1b]secret\n"
	for i := range len(input) {
		if _, err := d.Write([]byte{input[i]}); err != nil {
			t.Fatal(err)
		}
	}
	d.Flush()
	for _, secret := range []string{"secret", "/path", "https://", "50%", "\x1b"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("leaked %q: %s", secret, &out)
		}
	}
	for _, want := range []string{"Cloning into managed repository", "Enumerating objects: 5", "Receiving objects: 100%", "Authentication failed"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, &out)
		}
	}
	if !strings.Contains(d.Failure(), "Host gh authentication") {
		t.Fatal(d.Failure())
	}
}

func TestAgentUsesRequestLifetimeWithoutFiveMinuteDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	input, err := AgentRequestBody(AgentRequest{Operation: "clone"})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan error, 1)
	var out bytes.Buffer
	go func() {
		finished <- serveAgentWithRunner(ctx, input, &out, func(ctx context.Context, _ AgentRequest) (Response, error) {
			if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < 55*time.Minute {
				t.Error("agent imposed a short deadline")
			}
			close(started)
			<-ctx.Done()
			return Response{}, ctx.Err()
		})
	}()
	<-started
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled agent did not finish")
	}
	response, err := ReadResponse(&out, io.Discard)
	if err == nil {
		t.Fatalf("cancellation not reported: %+v %v", response, err)
	}
}
