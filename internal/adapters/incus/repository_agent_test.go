package incus

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/adapters/git"
)

func TestRepositoryAgentProcess(t *testing.T) {
	mode := os.Getenv("HACO_TEST_REPOSITORY_AGENT")
	if mode == "" {
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if _, err := gitadapter.ReadAgentRequest(os.Stdin); err != nil {
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "Receiving objects: 50% (1/2)")
	fmt.Fprintln(os.Stderr, "remote: Authorization: Bearer secret")
	if mode == "cancel" {
		<-ctx.Done()
	}
	_ = binary.Write(os.Stdout, binary.BigEndian, uint32(0))
	metadata, _ := json.Marshal(gitadapter.Response{Ref: "refs/heads/main"})
	_ = binary.Write(os.Stdout, binary.BigEndian, uint32(len(metadata)))
	_, _ = os.Stdout.Write(metadata)
	os.Exit(0)
}

type repoProgressWriter func([]byte) (int, error)

func (f repoProgressWriter) Write(p []byte) (int, error) { return f(p) }

func TestRepositoryAgentTransportProgressAndCancellation(t *testing.T) {
	for _, mode := range []string{"success", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var progress bytes.Buffer
			seen := make(chan struct{}, 1)
			ctx = gitadapter.WithProgress(ctx, repoProgressWriter(func(p []byte) (int, error) {
				select {
				case seen <- struct{}{}:
				default:
				}
				return progress.Write(p)
			}))
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRepositoryAgentProcess$")
			cmd.Env = append(os.Environ(), "HACO_TEST_REPOSITORY_AGENT="+mode)
			done := make(chan error, 1)
			go func() {
				input, _ := gitadapter.AgentRequestBody(gitadapter.AgentRequest{Operation: "clone"})
				result, err := runGitAgentCommand(ctx, cancel, cmd, input, io.Discard)
				if mode == "success" && result.Ref != "refs/heads/main" {
					t.Error(result)
				}
				done <- err
			}()
			select {
			case <-seen:
			case <-time.After(3 * time.Second):
				t.Fatal("no live agent progress")
			}
			if mode == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if mode == "success" && err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("agent request did not finish")
			}
			if strings.Contains(progress.String(), "secret") || !strings.Contains(progress.String(), "Receiving objects:") {
				t.Fatal(&progress)
			}
		})
	}
}
