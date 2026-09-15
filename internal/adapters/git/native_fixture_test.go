package gitadapter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture setup uses native Git independently of the adapter under test.
func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("/usr/bin/git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=PoC", "GIT_AUTHOR_EMAIL=poc@example.invalid", "GIT_COMMITTER_NAME=PoC", "GIT_COMMITTER_EMAIL=poc@example.invalid")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func testCommit(t *testing.T, dir, file, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", "--", file)
	testGit(t, dir, "commit", "-m", "update "+file)
	return testGit(t, dir, "rev-parse", "HEAD")
}
