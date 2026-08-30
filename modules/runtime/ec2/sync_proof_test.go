package ec2

import (
	"context"
	"strings"
	"testing"
)

func TestDeleteKeepsSyncProofWhenRecursiveStagingCleanupFails(t *testing.T) {
	runner := &fakeRunner{instanceState: "terminated", syncRestored: true, failContains: "--recursive"}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(runtimeRef{
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: t.TempDir(),
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
	})

	if err := runtime.DeleteEnvironment(context.Background(), raw); err == nil {
		t.Fatal("expected staging cleanup failure")
	}
	joined := strings.Join(runner.calls, "\n")
	if !strings.Contains(joined, "--exclude .hacocoon/sync-restored-i-0123456789abcdef0") {
		t.Fatalf("sync proof was not excluded from recursive cleanup:\n%s", joined)
	}
	if strings.Contains(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo/.hacocoon/sync-restored-i-0123456789abcdef0") {
		t.Fatalf("sync proof was deleted after failed staging cleanup:\n%s", joined)
	}

	runner.failContains = ""
	runner.calls = nil
	if err := runtime.DeleteEnvironment(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(runner.calls, "\n")
	stagingAt := strings.Index(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive")
	proofAt := strings.Index(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo/.hacocoon/sync-restored-i-0123456789abcdef0")
	if stagingAt < 0 || proofAt < 0 || stagingAt > proofAt {
		t.Fatalf("sync proof must be removed only after staging cleanup succeeds:\n%s", joined)
	}
}

func TestSyncProofIsBoundToInstanceIdentity(t *testing.T) {
	first := runtimeRef{InstanceID: "i-0123456789abcdef0", Prefix: "tests/demo"}
	second := runtimeRef{InstanceID: "i-fedcba9876543210", Prefix: "tests/demo"}
	if syncRestoredMarkerKey(first) == syncRestoredMarkerKey(second) {
		t.Fatalf("different instances share sync proof key: %q", syncRestoredMarkerKey(first))
	}
	if !strings.Contains(syncRestoredMarkerKey(first), first.InstanceID) {
		t.Fatalf("sync proof key is not bound to instance identity: %q", syncRestoredMarkerKey(first))
	}
}
