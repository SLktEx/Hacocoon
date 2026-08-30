package ebs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestFileJournalLockIsExclusiveAcrossProcessesAndReleasedOnCrash(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Hacocoon EBS transaction locking is supported on Linux")
	}
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFileJournalLockHelper$")
	cmd.Env = append(os.Environ(), "HACO_EBS_LOCK_HELPER=1", "HACO_EBS_LOCK_ROOT="+root)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("wait for helper lock: %v stderr=%s", err, stderr.String())
	}
	if strings.TrimSpace(line) != "LOCKED" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("helper did not acquire lock: %q stderr=%s", line, stderr.String())
	}

	journal := NewFileJournal(root)
	if release, err := journal.Lock(context.Background(), "source:vol-source"); !errors.Is(err, core.ErrStorageBusy) {
		if release != nil {
			_ = release()
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("second process acquired active lock: err=%v", err)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected killed helper process to report an error")
	}

	release, err := journal.Lock(context.Background(), "source:vol-source")
	if err != nil {
		t.Fatalf("lock remained stuck after holder crash: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}

func TestFileJournalLockHelper(t *testing.T) {
	if os.Getenv("HACO_EBS_LOCK_HELPER") != "1" {
		return
	}
	root := os.Getenv("HACO_EBS_LOCK_ROOT")
	journal := NewFileJournal(root)
	release, err := journal.Lock(context.Background(), "source:vol-source")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	fmt.Println("LOCKED")
	_, _ = io.Copy(io.Discard, os.Stdin)
}
