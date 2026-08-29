package incus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestExecRunnerCrossesProcessBoundaryForIncusWorkflow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed fake incus is for Linux/WSL CI")
	}

	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "state")
	logPath := filepath.Join(root, "incus.log")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(bin, "incus"), fakeIncusScript)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_FAKE_INCUS_STATE", state)
	t.Setenv("HACO_FAKE_INCUS_LOG", logPath)

	r := New(host.ExecRunner{})
	ctx := context.Background()

	caps, err := r.Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Available || len(caps.Details) != 1 || caps.Details[0] != "6.0-fake" {
		t.Fatalf("caps = %#v", caps)
	}

	attachment := map[string]string{"incus_pool": "haco-pool", "driver": "btrfs", "source": "/dev/loop42"}
	if err := r.Prepare(ctx, core.RuntimePrepareSpec{StorageAttachment: attachment}); err != nil {
		t.Fatal(err)
	}

	created, err := r.Create(ctx, core.RuntimeSessionSpec{ID: "abc", Name: "demo", StorageAttachment: attachment})
	if err != nil {
		t.Fatal(err)
	}
	if created.Ref != "haco-abc" {
		t.Fatalf("ref = %q", created.Ref)
	}

	state1, err := r.Inspect(ctx, created.Ref)
	if err != nil || state1.Observed != core.ObservedRunning {
		t.Fatalf("running inspect = %#v, err=%v", state1, err)
	}

	execResult, err := r.Exec(ctx, created.Ref, core.ExecRequest{Argv: []string{"printf", "%s", "hello world"}})
	if err != nil {
		t.Fatal(err)
	}
	if execResult.Stdout != "fake-exec:printf %s hello world\n" || execResult.ExitCode != 0 {
		t.Fatalf("exec result = %#v", execResult)
	}

	if err := r.Stop(ctx, created.Ref); err != nil {
		t.Fatal(err)
	}
	state2, err := r.Inspect(ctx, created.Ref)
	if err != nil || state2.Observed != core.ObservedStopped {
		t.Fatalf("stopped inspect = %#v, err=%v", state2, err)
	}
	if err := r.Start(ctx, created.Ref); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, created.Ref); err != nil {
		t.Fatal(err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	for _, want := range []string{
		"version",
		"project show hacocoon",
		"project create hacocoon",
		"storage show haco-pool --project hacocoon",
		"storage create haco-pool btrfs source=/dev/loop42 --project hacocoon",
		"image show hacocoon-v0.1 --project hacocoon",
		"launch images:ubuntu/26.04 haco-image-builder-v0-1 --project hacocoon -c security.nesting=true --storage haco-pool",
		"stop haco-image-builder-v0-1 --project hacocoon",
		"publish haco-image-builder-v0-1 --project hacocoon --alias hacocoon-v0.1 --reuse",
		"launch hacocoon-v0.1 haco-abc --project hacocoon -c security.nesting=true --storage haco-pool",
		"exec haco-abc --project hacocoon -- printf %s hello world",
		"stop haco-abc --project hacocoon",
		"start haco-abc --project hacocoon",
		"delete haco-abc --project hacocoon --force",
	} {
		if !strings.Contains(log, want+"\n") {
			t.Fatalf("incus process log missing %q\nlog:\n%s", want, log)
		}
	}
}

func TestBaseImageProvisionScriptExecutesWithExpectedToolFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed provision test is for Linux/WSL CI")
	}

	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	trace := filepath.Join(root, "trace.log")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}

	tools := map[string]string{
		"apt-get":   traceToolScript,
		"systemctl": traceToolScript,
		"tar":       traceToolScript,
		"mkdir":     traceToolScript,
		"ln":        traceToolScript,
		"rm":        traceToolScript,
		"nerdctl":   traceToolScript,
		"dpkg": `#!/bin/sh
printf 'dpkg %s\n' "$*" >> "$HACO_TOOL_TRACE"
if [ "$1" = "--print-architecture" ]; then echo amd64; fi
`,
		"curl": `#!/bin/sh
printf 'curl %s\n' "$*" >> "$HACO_TOOL_TRACE"
: > "$2"
`,
		"grep": `#!/bin/sh
printf 'grep %s\n' "$*" >> "$HACO_TOOL_TRACE"
echo '0000000000000000000000000000000000000000000000000000000000000000  nerdctl-2.3.5-linux-amd64.tar.gz'
`,
		"sha256sum": `#!/bin/sh
printf 'sha256sum %s\n' "$*" >> "$HACO_TOOL_TRACE"
cat >/dev/null
exit 0
`,
	}
	for name, body := range tools {
		writeExecutable(t, filepath.Join(bin, name), body)
	}

	// Keep the host untouched: even mkdir/ln/rm/tar are shims. curl only creates
	// the two expected harmless /tmp files, which are cleaned below.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_TOOL_TRACE", trace)
	asset := "/tmp/nerdctl-" + nerdctlVersion + "-linux-amd64.tar.gz"
	checksums := "/tmp/nerdctl-SHA256SUMS"
	defer os.Remove(asset)
	defer os.Remove(checksums)

	cmd := hostShellCommand(baseImageProvisionScript())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("provision script failed: %v\n%s", err, out)
	}

	traceBytes, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	traceText := string(traceBytes)
	assertTraceOrder(t, traceText, []string{
		"apt-get update",
		"apt-get install -y --no-install-recommends ca-certificates curl tar containerd containernetworking-plugins",
		"systemctl enable --now containerd",
		"dpkg --print-architecture",
		"curl -fsSLo /tmp/nerdctl-" + nerdctlVersion + "-linux-amd64.tar.gz",
		"curl -fsSLo /tmp/nerdctl-SHA256SUMS",
		"tar -C /usr/local/bin -xzf /tmp/nerdctl-" + nerdctlVersion + "-linux-amd64.tar.gz nerdctl",
		"rm -f /tmp/nerdctl-" + nerdctlVersion + "-linux-amd64.tar.gz /tmp/nerdctl-SHA256SUMS",
		"mkdir -p /opt/cni",
		"ln -sfn /usr/lib/cni /opt/cni/bin",
		"systemctl is-active --quiet containerd",
		"nerdctl --version",
		"nerdctl info",
		"nerdctl run --rm " + nestedSmokeImage + " true",
	})
	assertBothBefore(t, traceText, "grep -E", "sha256sum -c -", "tar -C /usr/local/bin")
}

func assertTraceOrder(t *testing.T, trace string, wants []string) {
	t.Helper()
	pos := 0
	for _, want := range wants {
		i := strings.Index(trace[pos:], want)
		if i < 0 {
			t.Fatalf("trace missing %q after byte %d\ntrace:\n%s", want, pos, trace)
		}
		pos += i + len(want)
	}
}

func assertBothBefore(t *testing.T, trace, first, second, after string) {
	t.Helper()
	iFirst := strings.Index(trace, first)
	iSecond := strings.Index(trace, second)
	iAfter := strings.Index(trace, after)
	if iFirst < 0 || iSecond < 0 || iAfter < 0 || iFirst > iAfter || iSecond > iAfter {
		t.Fatalf("expected %q and %q before %q\ntrace:\n%s", first, second, after, trace)
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func hostShellCommand(script string) *exec.Cmd {
	return exec.Command("sh", "-ceu", script)
}

const traceToolScript = `#!/bin/sh
printf '%s %s\n' "$(basename "$0")" "$*" >> "$HACO_TOOL_TRACE"
exit 0
`

const fakeIncusScript = `#!/bin/sh
set -eu
state="$HACO_FAKE_INCUS_STATE"
log="$HACO_FAKE_INCUS_LOG"
printf '%s\n' "$*" >> "$log"
cmd="${1:-}"
case "$cmd" in
  version)
    echo 6.0-fake
    ;;
  project)
    action="${2:-}"; name="${3:-}"
    case "$action" in
      show) [ -f "$state/project-$name" ] ;;
      create) : > "$state/project-$name" ;;
      *) exit 2 ;;
    esac
    ;;
  storage)
    action="${2:-}"; name="${3:-}"
    case "$action" in
      show) [ -f "$state/storage-$name" ] ;;
      create) : > "$state/storage-$name" ;;
      *) exit 2 ;;
    esac
    ;;
  image)
    action="${2:-}"; name="${3:-}"
    [ "$action" = show ] || exit 2
    [ -f "$state/image-$name" ]
    ;;
  launch)
    name="${3:-}"
    echo RUNNING > "$state/instance-$name"
    ;;
  list)
    name="${2:-}"
    cat "$state/instance-$name"
    ;;
  exec)
    shift
    instance="$1"; shift
    while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do shift; done
    [ "$#" -gt 0 ] && shift
    if [ "$instance" = "haco-image-builder-v0-1" ]; then
      exit 0
    fi
    echo "fake-exec:$*"
    ;;
  publish)
    shift
    instance="$1"; shift
    alias=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--alias" ]; then alias="$2"; shift 2; continue; fi
      shift
    done
    [ -n "$alias" ] || exit 2
    : > "$state/image-$alias"
    ;;
  stop)
    echo STOPPED > "$state/instance-${2:-}"
    ;;
  start)
    echo RUNNING > "$state/instance-${2:-}"
    ;;
  delete)
    rm -f "$state/instance-${2:-}"
    ;;
  *) exit 2 ;;
esac
`
