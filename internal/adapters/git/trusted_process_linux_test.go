//go:build linux

package gitadapter

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestTrustedGitCancellationStopsDescendantHelpers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 60 & echo ready; wait")
	cmd.Stdout = writer
	if err := configureGitProcess(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }()
	_ = writer.Close()
	output := bufio.NewReader(reader)
	if line, err := output.ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatal(line, err)
	}
	cancel()
	if err := cmd.Wait(); err == nil {
		t.Fatal("canceled process succeeded")
	}
	closed := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, output); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("descendant retained its output pipe after cancellation")
	}
}
